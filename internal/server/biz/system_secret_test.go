package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestSystemService_SecretKeyRequiresSystemPrincipal(t *testing.T) {
	for _, cacheState := range []string{"cold", "warm"} {
		t.Run(cacheState, func(t *testing.T) {
			service, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
			t.Cleanup(func() { db.Close() })
			base := ent.NewContext(t.Context(), db)
			systemCtx := authz.NewSystemContext(base)
			const secret = "test-internal-signing-key"
			// Seed the database directly so the getter's cache starts empty.
			db.System.Create().SetKey(SystemKeySecretKey).SetValue(secret).SaveX(authz.WithTestBypass(base))
			if cacheState == "warm" {
				value, err := service.SecretKey(systemCtx)
				require.NoError(t, err)
				require.Equal(t, secret, value)
				_, err = service.Cache.Get(base, "system:"+SystemKeySecretKey)
				require.NoError(t, err)
			}

			user := &ent.User{ID: 1}
			userCtx := contexts.WithUser(authz.NewUserContext(base, user.ID), user)
			reader := &ent.User{ID: 2, Scopes: []string{"read_settings"}}
			readerCtx := contexts.WithUser(authz.NewUserContext(base, reader.ID), reader)
			owner := &ent.User{ID: 3, IsOwner: true}
			ownerCtx := contexts.WithUser(authz.NewUserContext(base, owner.ID), owner)
			pluginCtx, err := authz.DelegatePlugin(ownerCtx, 1, []string{"read_settings"})
			require.NoError(t, err)
			for name, ctx := range map[string]context.Context{
				"anonymous":    base,
				"user":         userCtx,
				"settings":     readerCtx,
				"owner":        ownerCtx,
				"api-key":      authz.NewAPIKeyContext(base, 1, 1),
				"plugin":       pluginCtx,
				"user-allow":   authz.WithScopeDecision(readerCtx, scopes.ScopeReadSettings),
				"plugin-allow": authz.WithScopeDecision(pluginCtx, scopes.ScopeReadSettings),
				"test-bypass":  authz.WithTestBypass(base),
			} {
				t.Run(name, func(t *testing.T) {
					value, err := service.SecretKey(ctx)
					require.Error(t, err)
					require.Empty(t, value)
				})
			}
			if cacheState == "cold" {
				_, err := service.Cache.Get(base, "system:"+SystemKeySecretKey)
				require.Error(t, err, "rejected callers must not load the secret into the cache")
			}
			value, err := service.SecretKey(systemCtx)
			require.NoError(t, err)
			require.Equal(t, secret, value)
		})
	}
}
