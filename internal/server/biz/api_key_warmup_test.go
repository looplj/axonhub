package biz

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/intercept"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyServiceExpiredWarmupFallsBackToLazyAuthentication(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()
	setup := authz.WithTestBypass(context.Background())
	owner, err := client.User.Create().SetEmail("warmup@example.com").SetPassword("test-password").SetFirstName("Warmup").SetLastName("Test").Save(setup)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("warmup-project").Save(setup)
	require.NoError(t, err)
	key, err := client.APIKey.Create().SetName("warmup-key").SetKey("ah-warmup-test").SetUser(owner).SetProject(project).Save(setup)
	require.NoError(t, err)
	config := xcache.Config{Mode: xcache.ModeMemory}
	config.Memory.Expiration = time.Second
	projects := NewProjectService(ProjectServiceParams{Ent: client, CacheConfig: config})
	synctest.Test(t, func(t *testing.T) {
		var loads atomic.Int32
		client.Intercept(intercept.Func(func(_ context.Context, query intercept.Query) error {
			if query.Type() == ent.TypeAPIKey && loads.Add(1) == 1 {
				time.Sleep(2 * time.Second)
			}
			return nil
		}))
		service := NewAPIKeyService(APIKeyServiceParams{Ent: client, CacheConfig: config, ProjectService: projects})
		defer service.Stop()
		require.Empty(t, service.APIKeyCache.GetAll())
		auth := NewAuthService(AuthServiceParams{Ent: client, APIKeyService: service})
		authenticated, err := auth.AuthenticateAPIKey(context.Background(), key.Key)
		require.NoError(t, err)
		require.Equal(t, key.ID, authenticated.ID)
		require.EqualValues(t, 2, loads.Load())
	})
}

func TestAPIKeyServiceWarmupPreservesSourceFailures(t *testing.T) {
	for _, sourceErr := range []error{errors.New("database unavailable"), context.DeadlineExceeded, context.Canceled} {
		t.Run(sourceErr.Error(), func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
			defer client.Close()
			client.Intercept(intercept.Func(func(context.Context, intercept.Query) error { return sourceErr }))
			require.PanicsWithError(t, "api key cache initial load failed: "+sourceErr.Error(), func() {
				NewAPIKeyService(APIKeyServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
			})
		})
	}
}
