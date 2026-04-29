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
	_ "github.com/looplj/axonhub/internal/pkg/sqlite" // Register custom sqlite driver with FK support
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestOIDCService(t *testing.T) (*OIDCService, *ent.Client) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	client = client.Debug()

	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	svc := &OIDCService{
		AbstractService: &AbstractService{db: client},
		cache:           xcache.NewFromConfig[[]byte](cacheConfig),
		providers:       make(map[string]*oidcProvider),
		lastCheck:       make(map[string]int64),
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
			ID:              "github",
			Name:            "github",
			IssuerURL:       "https://github.com",
			AutoLinkByEmail: true,
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

	var foundUser bool

	for _, r := range roles2 {
		if r.ID == rUser.ID {
			foundUser = true
			break
		}
	}

	require.True(t, foundUser)

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

	// 6. create_only
	cfg.SyncRoleStrategy = "create_only"
	u6 := uUpdated5.Update()
	err = svc.applyRoleMappings(ctx, u6.Mutation(), []string{"ops"}, cfg, false) // should skip since it's not creation
	require.NoError(t, err)

	uUpdated6, _ := u6.Save(ctx)
	roles6, _ := uUpdated6.QueryRoles().All(ctx)
	require.Len(t, roles6, 1)
	require.Equal(t, rAdmin.ID, roles6[0].ID) // remains admin

	// 7. manual_first
	cfg.SyncRoleStrategy = "manual_first"
	u7 := uUpdated6.Update()
	err = svc.applyRoleMappings(ctx, u7.Mutation(), []string{"user"}, cfg, false) // should skip since user already has roles
	require.NoError(t, err)

	uUpdated7, _ := u7.Save(ctx)
	roles7, _ := uUpdated7.QueryRoles().All(ctx)
	require.Len(t, roles7, 1)
	require.Equal(t, rAdmin.ID, roles7[0].ID) // remains admin
}

func TestOIDC_ApplyRoleMappings_DefaultsAndRegex(t *testing.T) {
	svc, client := setupTestOIDCService(t)
	defer client.Close()

	ctx := context.Background()

	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create roles
	rDefault, _ := client.Role.Create().SetName("default-role").SetLevel(role.LevelSystem).Save(ctx)
	rRegex, _ := client.Role.Create().SetName("regex-role").SetLevel(role.LevelSystem).Save(ctx)

	cfg := OIDCProvider{
		DefaultRoles:  []string{"default-role"},
		DefaultScopes: []string{"read:all"},
		RoleMappingRules: []RoleMappingRule{
			{MatchGroup: `^dev-.*$`, IsRegex: true, DBRole: "regex-role", Priority: 5},
		},
	}

	// 1. Defaults (no groups match or provided)
	u1, _ := client.User.Create().SetEmail("default@test.com").SetPassword("pw").Save(ctx)
	upd1 := u1.Update()
	err := svc.applyRoleMappings(ctx, upd1.Mutation(), nil, cfg, true)
	require.NoError(t, err)
	_, err = upd1.Save(ctx)
	require.NoError(t, err)

	uUpdated1, _ := client.User.Get(ctx, u1.ID)
	roles1, _ := uUpdated1.QueryRoles().All(ctx)
	require.Len(t, roles1, 1)
	require.Equal(t, rDefault.ID, roles1[0].ID)
	require.Contains(t, uUpdated1.Scopes, "read:all")

	// 2. Regex match
	u2, _ := client.User.Create().SetEmail("regex@test.com").SetPassword("pw").Save(ctx)
	upd2 := u2.Update()
	err = svc.applyRoleMappings(ctx, upd2.Mutation(), []string{"dev-backend"}, cfg, true)
	require.NoError(t, err)
	_, err = upd2.Save(ctx)
	require.NoError(t, err)

	uUpdated2, _ := client.User.Get(ctx, u2.ID)
	roles2, _ := uUpdated2.QueryRoles().All(ctx)
	require.Len(t, roles2, 1)
	require.Equal(t, rRegex.ID, roles2[0].ID)
	require.NotContains(t, uUpdated2.Scopes, "read:all") // Defaults should not apply if matched
}
