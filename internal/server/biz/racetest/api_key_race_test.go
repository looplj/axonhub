//go:build dbrace

package racetest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/db"
)

const raceConcurrency = 50

// raceDialects lists the dialects exercised by the name-uniqueness race test.
// Each subtest runs only when its DSN env var is set.
//
// PostgreSQL/MySQL/TiDB are multi-writer servers whose check-then-write race is
// closed by the per-project row lock (SELECT ... FOR UPDATE). SQLite is included
// even though the row lock is a NO-OP there (SQLite rejects SELECT ... FOR UPDATE):
// it proves SQLite's own single-writer + WAL BUSY/BUSY_SNAPSHOT semantics make the
// burst fail closed (loser gets DuplicateNameError or SQLITE_BUSY, never a
// duplicate live row). SQLite needs no server — point AXONHUB_TEST_SQLITE_DSN at a
// temp file, e.g. file:/tmp/race.db.
var raceDialects = []struct {
	name    string
	dialect string
	envVar  string
}{
	{name: "postgres", dialect: "postgres", envVar: "AXONHUB_TEST_PG_DSN"},
	{name: "mysql", dialect: "mysql", envVar: "AXONHUB_TEST_MYSQL_DSN"},
	{name: "tidb", dialect: "tidb", envVar: "AXONHUB_TEST_TIDB_DSN"},
	{name: "sqlite", dialect: "sqlite3", envVar: "AXONHUB_TEST_SQLITE_DSN"},
}

// TestAPIKeyNameRace proves API key name uniqueness is race-safe WITHOUT any DB
// unique constraint (Path A') on every supported dialect: the per-project row lock
// closes the race on PostgreSQL/MySQL/TiDB, and SQLite's single-writer semantics
// close it on SQLite (where the lock is a no-op).
//
// Many clients concurrently create an LLM API key with the SAME name in the SAME
// project. There is no unique index on (project_id, name); the only thing closing
// the check-then-write race is the SELECT ... FOR UPDATE on the parent project row
// taken in CreateLLMAPIKey's transaction. The lock must serialize the burst so
// exactly one create succeeds, every other gets DuplicateNameError, exactly one
// live row remains, and the name-based lookup (the feature this PR ships) resolves
// to it.
//
// Without the lock, this same burst would leave duplicate live rows and break
// GetForRead's .Only() with a not-singular error.
//
// NOTE for TiDB: FOR UPDATE only locks eagerly in pessimistic transaction mode
// (the cluster default since 3.0.8); an optimistic-mode cluster would defer the
// lock to commit and could still race.
func TestAPIKeyNameRace(t *testing.T) {
	ran := 0

	for _, d := range raceDialects {
		dsn := os.Getenv(d.envVar)
		if dsn == "" {
			t.Logf("skipping %s: set %s to run", d.name, d.envVar)
			continue
		}

		ran++

		t.Run(d.name, func(t *testing.T) {
			runAPIKeyNameRace(t, d.dialect, dsn)
		})
	}

	if ran == 0 {
		t.Skip("set AXONHUB_TEST_PG_DSN / AXONHUB_TEST_MYSQL_DSN / AXONHUB_TEST_TIDB_DSN to run the concurrency test")
	}
}

func runAPIKeyNameRace(t *testing.T, dialect, dsn string) {
	t.Helper()

	client := db.NewEntClient(db.Config{Dialect: dialect, DSN: dsn})
	defer client.Close()

	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{
		CacheConfig:    xcache.Config{Mode: xcache.ModeMemory},
		Ent:            client,
		ProjectService: &biz.ProjectService{ProjectCache: xcache.NewFromConfig[xcache.Entry[ent.Project]](xcache.Config{Mode: xcache.ModeMemory})},
		KeyPrefix:      "ah",
	})
	defer apiKeyService.Stop()

	setupCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	owner := seedOwnerAPIKey(t, client, setupCtx)
	ctx := contexts.WithAPIKey(ent.NewContext(context.Background(), client), owner)

	const name = "race-name-row-lock"

	ok, dup, other, errs := fireConcurrentCreates(apiKeyService, ctx, owner, name)
	t.Logf("[%s row lock] success=%d duplicate=%d other=%d", dialect, ok, dup, other)

	require.Empty(t, errs, "no unexpected errors expected")
	require.Equal(t, 1, ok, "exactly one concurrent create should succeed")
	require.Equal(t, raceConcurrency-1, dup, "all other creates should get DuplicateNameError")
	require.Equal(t, 1, countLiveByName(t, client, setupCtx, owner.ProjectID, name),
		"the row lock must leave exactly one live row for the name")

	// Name lookup (the feature this PR adds) must resolve to the single key.
	got, err := apiKeyService.GetForRead(ctx, nil, nil, ptr(name))
	require.NoError(t, err, "name lookup must succeed (single match)")
	require.Equal(t, name, got.Name)
}

// fireConcurrentCreates launches raceConcurrency goroutines that all create an
// LLM API key with the same name, and tallies the outcomes.
func fireConcurrentCreates(svc *biz.APIKeyService, ctx context.Context, owner *ent.APIKey, name string) (success, duplicate, other int, otherErrs []error) {
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs []error
	)

	start := make(chan struct{})

	for i := 0; i < raceConcurrency; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			<-start // release all goroutines at once to maximize contention

			_, err := svc.CreateLLMAPIKey(ctx, owner, name)

			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	for _, err := range errs {
		switch {
		case err == nil:
			success++
		case strings.Contains(err.Error(), "already exists"):
			duplicate++
		default:
			other++

			otherErrs = append(otherErrs, err)
		}
	}

	return success, duplicate, other, otherErrs
}

func seedOwnerAPIKey(t *testing.T, client *ent.Client, setupCtx context.Context) *ent.APIKey {
	t.Helper()

	hashed, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	u, err := client.User.Create().
		SetEmail(fmt.Sprintf("race-%d@example.com", time.Now().UnixNano())).
		SetPassword(hashed).
		SetFirstName("Race").
		SetLastName("Owner").
		SetStatus(user.StatusActivated).
		Save(setupCtx)
	require.NoError(t, err)

	p, err := client.Project.Create().
		SetName(uuid.NewString()).
		SetDescription("race test").
		SetStatus(project.StatusActive).
		Save(setupCtx)
	require.NoError(t, err)

	key, err := biz.GenerateAPIKey("ah")
	require.NoError(t, err)

	owner, err := client.APIKey.Create().
		SetName("Service Account").
		SetKey(key).
		SetUserID(u.ID).
		SetProjectID(p.ID).
		SetType(apikey.TypeServiceAccount).
		SetScopes([]string{string(scopes.ScopeWriteAPIKeys), string(scopes.ScopeReadAPIKeys)}).
		Save(setupCtx)
	require.NoError(t, err)

	return owner
}

func countLiveByName(t *testing.T, client *ent.Client, setupCtx context.Context, projectID int, name string) int {
	t.Helper()

	n, err := client.APIKey.Query().
		Where(apikey.NameEQ(name), apikey.ProjectIDEQ(projectID)).
		Count(setupCtx)
	require.NoError(t, err)

	return n
}

func ptr[T any](v T) *T { return &v }
