//go:build pgrace

package racetest

import (
	"context"
	"database/sql"
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

// TestAPIKeyNameRace_Postgres simulates the exact problem scenario: many clients
// concurrently create an LLM API key with the SAME name in the SAME project.
//
// Phase 1 (with the fix): the (project_id, name, deleted_at) unique index must
// make exactly one create succeed and all others fail with DuplicateNameError,
// leaving exactly one live row — so the name stays a reliable identifier and the
// name-based lookup (.Only) keeps working.
//
// Phase 2 (drop the index = simulate the old behavior): with no DB constraint
// and the old non-atomic check gone, the same concurrent burst now produces
// duplicate rows and the name lookup breaks with NotSingularError — proving the
// race window is real and the index is what closes it.
func TestAPIKeyNameRace_Postgres(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set AXONHUB_TEST_PG_DSN to run the Postgres concurrency test")
	}

	// Real production bootstrap path: opens PG, runs the duplicate guard, then
	// auto-migrates (creating api_keys_by_project_name).
	client := db.NewEntClient(db.Config{Dialect: "postgres", DSN: dsn})
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

	// ---- Phase 1: with the unique index ------------------------------------
	const name1 = "race-name-with-index"
	ok1, dup1, other1, errs1 := fireConcurrentCreates(apiKeyService, ctx, owner, name1)
	t.Logf("[with index]   success=%d duplicate=%d other=%d", ok1, dup1, other1)

	require.Empty(t, errs1, "no unexpected errors expected with the index in place")
	require.Equal(t, 1, ok1, "exactly one concurrent create should succeed")
	require.Equal(t, raceConcurrency-1, dup1, "all other creates should get DuplicateNameError")

	require.Equal(t, 1, countLiveByName(t, client, setupCtx, owner.ProjectID, name1),
		"exactly one live row should exist for the name")

	// Name lookup (the feature this PR adds) must resolve to the single key.
	got, err := apiKeyService.GetForRead(ctx, nil, nil, ptr(name1))
	require.NoError(t, err, "name lookup must succeed (single match)")
	require.Equal(t, name1, got.Name)

	// ---- Phase 2: drop the index to demonstrate the old (broken) behavior ---
	dropUniqueIndex(t, dsn)

	const name2 = "race-name-no-index"
	ok2, dup2, other2, _ := fireConcurrentCreates(apiKeyService, ctx, owner, name2)
	live2 := countLiveByName(t, client, setupCtx, owner.ProjectID, name2)
	t.Logf("[no index]     success=%d duplicate=%d other=%d  -> live rows for name=%d", ok2, dup2, other2, live2)

	require.Greater(t, live2, 1, "without the index the race must leave duplicate rows")

	// And the name lookup now breaks exactly as described in the issue.
	_, err = apiKeyService.GetForRead(ctx, nil, nil, ptr(name2))
	require.Error(t, err, "duplicate names must break the .Only() name lookup")
	require.True(t, ent.IsNotSingular(err), "expected ent NotSingularError, got: %v", err)
	t.Logf("[no index]     name lookup correctly failed: %v", err)

	// ---- Phase 3: the startup guard catches the now-dirty DB ----------------
	// Phase 2 left duplicate live names AND a missing index — exactly the state
	// an existing deployment would be in on upgrade. A fresh client bootstrap
	// must now fail fast with an actionable message (never mutating the keys),
	// instead of an opaque "CREATE UNIQUE INDEX failed" panic.
	func() {
		defer func() {
			r := recover()
			require.NotNil(t, r, "NewEntClient must panic when duplicate names exist")

			msg := fmt.Sprint(r)
			require.Contains(t, msg, "duplicate api key names found")
			require.Contains(t, msg, "Rename or disable")
			t.Logf("[guard]        startup correctly aborted:\n%s", msg)
		}()

		c := db.NewEntClient(db.Config{Dialect: "postgres", DSN: dsn})
		c.Close()
	}()

	// The guard must not have touched the data: all 50 duplicates still live.
	require.Equal(t, raceConcurrency, countLiveByName(t, client, setupCtx, owner.ProjectID, name2),
		"the fail-fast guard must not mutate/delete any keys")
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

func dropUniqueIndex(t *testing.T, dsn string) {
	t.Helper()

	raw, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer raw.Close()

	_, err = raw.ExecContext(context.Background(), "DROP INDEX IF EXISTS api_keys_by_project_name")
	require.NoError(t, err)
}

func ptr[T any](v T) *T { return &v }
