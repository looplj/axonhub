package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/oidcidentity"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	_ "github.com/looplj/axonhub/internal/pkg/sqlite" // Register custom sqlite driver with FK support
)

func setupTestOIDCService(t *testing.T) (*OIDCService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	client = client.Debug()

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	svc := &OIDCService{
		db:        client,
		cache:     xcache.NewFromConfig[[]byte](cacheConfig),
		providers: make(map[string]*oidcProvider),
		lastCheck: make(map[string]int64),
	}

	return svc, client
}

func TestResolveUser_AccountFirstAndMultipleOIDC(t *testing.T) {
	svc, client := setupTestOIDCService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	p := &oidcProvider{
		config: OIDCProvider{
			ID:         "google",
			Name:       "google",
			IssuerURL:  "https://accounts.google.com",
			JITEnabled: true,
		},
	}

	// 1. Test JIT Creation: Create user then link
	email := "new-user@example.com"
	subject := "sub-1"
	u1, err := svc.resolveUser(ctx, p, subject, email, true, "New User", "", "", "", nil)
	require.NoError(t, err)
	require.NotNil(t, u1)
	require.Equal(t, email, u1.Email)

	// Verify identity created
	id1, err := client.OIDCIdentity.Query().Where(oidcidentity.Subject(subject)).WithUser().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, u1.ID, id1.UserID)

	// 2. Test Account First (Matching by Email): Existing user, new OIDC provider
	p2 := &oidcProvider{
		config: OIDCProvider{
			ID:        "github",
			Name:      "github",
			IssuerURL: "https://github.com",
		},
	}
	subject2 := "sub-github-1"
	// resolveUser should find u1 by email and link github identity
	u2, err := svc.resolveUser(ctx, p2, subject2, email, true, "GitHub Name", "", "", "", nil)
	require.NoError(t, err)
	require.Equal(t, u1.ID, u2.ID)

	// Verify both identities exist for the same user
	identities, err := client.OIDCIdentity.Query().Where(oidcidentity.UserID(u1.ID)).All(ctx)
	require.NoError(t, err)
	require.Len(t, identities, 2)

	// 3. Test JIT Disabled: Fail if account not found
	p3 := &oidcProvider{
		config: OIDCProvider{
			ID:         "limited",
			Name:       "limited",
			IssuerURL:  "https://limited.com",
			JITEnabled: false,
		},
	}
	_, err = svc.resolveUser(ctx, p3, "sub-3", "unknown@example.com", true, "", "", "", "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestResolveUser_CascadeDelete(t *testing.T) {
	_, client := setupTestOIDCService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create user
	u, err := client.User.Create().SetEmail("delete@example.com").SetPassword("pw").Save(ctx)
	require.NoError(t, err)

	// Create identity
	_, err = client.OIDCIdentity.Create().
		SetUserID(u.ID).
		SetIssuer("issuer").
		SetSubject("sub").
		SetEmail(u.Email).
		Save(ctx)
	require.NoError(t, err)

	// In production, deleting the user would cascade delete the OIDCIdentity.
	// In tests, foreign keys are disabled (migrate.WithForeignKeys(false)),
	// so we must manually clean up the identity first.
	_, err = client.OIDCIdentity.Delete().Where(oidcidentity.UserID(u.ID)).Exec(schematype.SkipSoftDelete(ctx))
	require.NoError(t, err)

	// Physically delete user using SkipSoftDelete to bypass soft-delete mixin
	ctxPhysical := schematype.SkipSoftDelete(ctx)
	err = client.User.DeleteOne(u).Exec(ctxPhysical)
	require.NoError(t, err)

	// Verify identity is gone
	exists, err := client.OIDCIdentity.Query().Where(oidcidentity.UserID(u.ID)).Exist(ctxPhysical)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestOIDC_ExtractGroups(t *testing.T) {
	svc, client := setupTestOIDCService(t)
	defer client.Close()

	p := &oidcProvider{
		config: OIDCProvider{
			GroupClaims: []string{"org_roles", "groups"},
			GroupParser: GroupParserConfig{
				RegexReplacePattern: `^prefix_`,
				RegexReplaceWith:    "",
				CaseSensitive:       false,
			},
		},
	}

	raw := map[string]any{
		"org_roles": []string{"prefix_Admin", "prefix_user"},
		"groups":    "prefix_OPS-TEAM",
		"other":     "should_not_be_included",
	}

	groups := svc.extractGroups(raw, p)
	require.Len(t, groups, 3)
	require.Contains(t, groups, "admin")
	require.Contains(t, groups, "user")
	require.Contains(t, groups, "ops-team")
}

func TestOIDC_ApplyRoleMappings_SyncStrategies(t *testing.T) {
	svc, client := setupTestOIDCService(t)
	defer client.Close()
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create system roles
	rAdmin, err := client.Role.Create().SetName("admin").SetLevel(role.LevelSystem).Save(ctx)
	require.NoError(t, err)
	rUser, err := client.Role.Create().SetName("user").SetLevel(role.LevelSystem).Save(ctx)
	require.NoError(t, err)
	rOps, err := client.Role.Create().SetName("ops").SetLevel(role.LevelSystem).Save(ctx)
	require.NoError(t, err)

	// Create user
	u, err := client.User.Create().SetEmail("sync@test.com").SetPassword("pw").Save(ctx)
	require.NoError(t, err)

	cfg := OIDCProvider{
		RoleMappingRules: []RoleMappingRule{
			{MatchGroup: "admin-*", DBRole: "admin", Priority: 10},
			{MatchGroup: "ops", DBRole: "ops", Priority: 5},
			{MatchGroup: "user", DBRole: "user", Priority: 1},
			{MatchGroup: "owner", DBRole: "system:owner", Priority: 1},
		},
	}

	// 1. merge
	cfg.SyncRoleStrategy = "merge"
	u1 := u.Update()
	err = svc.applyRoleMappings(ctx, u1.Mutation(), []string{"admin-ops-team"}, cfg, false)
	require.NoError(t, err)
	uUpdated1, _ := u1.Save(ctx)
	roles1, _ := uUpdated1.QueryRoles().All(ctx)
	require.Len(t, roles1, 1)
	require.Equal(t, rAdmin.ID, roles1[0].ID)

	// 2. merge again (additive)
	u2 := uUpdated1.Update()
	err = svc.applyRoleMappings(ctx, u2.Mutation(), []string{"user"}, cfg, false)
	require.NoError(t, err)
	uUpdated2, _ := u2.Save(ctx)
	roles2, _ := uUpdated2.QueryRoles().All(ctx)
	require.Len(t, roles2, 2) // Should have both admin and user

	// 3. always (clear and replace)
	cfg.SyncRoleStrategy = "always"
	u3 := uUpdated2.Update()
	err = svc.applyRoleMappings(ctx, u3.Mutation(), []string{"ops"}, cfg, false)
	require.NoError(t, err)
	uUpdated3, _ := u3.Save(ctx)
	roles3, _ := uUpdated3.QueryRoles().All(ctx)
	require.Len(t, roles3, 1)
	require.Equal(t, rOps.ID, roles3[0].ID)

	// 4. owner assignment
	u4 := uUpdated3.Update()
	err = svc.applyRoleMappings(ctx, u4.Mutation(), []string{"owner"}, cfg, false)
	require.NoError(t, err)
	uUpdated4, _ := u4.Save(ctx)
	require.True(t, uUpdated4.IsOwner)

	// 5. precedence
	cfg.RolePrecedenceMode = "highest"
	u5 := uUpdated4.Update()
	err = svc.applyRoleMappings(ctx, u5.Mutation(), []string{"admin-ops", "user"}, cfg, false) // admin uses "admin-*"
	require.NoError(t, err)
	uUpdated5, _ := u5.Save(ctx)
	roles5, _ := uUpdated5.QueryRoles().All(ctx)
	require.Len(t, roles5, 1)
	require.Equal(t, rAdmin.ID, roles5[0].ID) // admin has priority 10
}
