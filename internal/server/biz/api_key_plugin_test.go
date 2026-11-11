package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestPluginAPIKeyWritesPreserveAuthorizationAndUniqueness(t *testing.T) {
	svc, db := setupTestAPIKeyService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer svc.Stop()
	defer db.Close()
	setup := authz.WithTestBypass(context.Background())
	owner := db.User.Create().SetEmail("plugin-key-owner@example.com").SetPassword("password").SetIsOwner(true).SaveX(setup)
	project := db.Project.Create().SetName("plugin-keys").SaveX(setup)
	parent := contexts.WithUser(authz.NewUserContext(ent.NewContext(context.Background(), db), owner.ID), owner)
	parent = contexts.WithProjectID(parent, project.ID)
	ctx, err := authz.DelegatePlugin(parent, 1, scopes.AllScopesAsStrings())
	require.NoError(t, err)

	first, err := svc.CreateAPIKey(ctx, ent.CreateAPIKeyInput{Name: "first", ProjectID: project.ID})
	require.NoError(t, err)
	second, err := svc.CreateAPIKey(ctx, ent.CreateAPIKeyInput{Name: "second", ProjectID: project.ID})
	require.NoError(t, err)
	_, err = svc.CreateAPIKey(ctx, ent.CreateAPIKeyInput{Name: "first", ProjectID: project.ID})
	require.Error(t, err)
	require.Equal(t, 2, db.APIKey.Query().CountX(setup), "duplicate insert must roll back")
	_, err = svc.UpdateAPIKey(ctx, second.ID, ent.UpdateAPIKeyInput{Name: lo.ToPtr("first")})
	require.Error(t, err)
	require.Equal(t, "second", db.APIKey.GetX(setup, second.ID).Name)
	_, err = svc.UpdateAPIKey(ctx, first.ID, ent.UpdateAPIKeyInput{Name: lo.ToPtr("renamed")})
	require.NoError(t, err)

	limited, err := authz.DelegatePlugin(parent, 2, []string{"write_api_keys", "read_api_keys", "read_channels"})
	require.NoError(t, err)
	_, err = svc.CreateAPIKey(limited, ent.CreateAPIKeyInput{Name: "no-admin", ProjectID: project.ID})
	require.Error(t, err, "reloading the owner must not restore project admin authority")
	serviceInput := ent.CreateAPIKeyInput{Name: "service", ProjectID: project.ID, Type: lo.ToPtr(apikey.TypeServiceAccount), Scopes: []string{"read_channels"}}
	key, err := svc.CreateAPIKey(limited, serviceInput)
	require.NoError(t, err)
	serviceInput.Name = "escalated"
	serviceInput.Scopes = []string{"write_settings"}
	_, err = svc.CreateAPIKey(limited, serviceInput)
	require.Error(t, err)
	_, err = svc.UpdateAPIKey(limited, key.ID, ent.UpdateAPIKeyInput{AppendScopes: []string{"write_settings"}})
	require.Error(t, err)
	require.Equal(t, []string{"read_channels"}, db.APIKey.GetX(setup, key.ID).Scopes)

	revoked, err := authz.DelegatePlugin(parent, 3, nil)
	require.NoError(t, err)
	serviceInput.Name = "revoked"
	serviceInput.Scopes = nil
	_, err = svc.CreateAPIKey(revoked, serviceInput)
	require.Error(t, err)
	require.Equal(t, 3, db.APIKey.Query().CountX(setup))
	principal, _ := authz.GetPrincipal(ctx)
	require.True(t, principal.IsPlugin())
	require.False(t, authz.IsBypassActive(ctx))
}
