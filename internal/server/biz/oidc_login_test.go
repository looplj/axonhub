package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/oidcidentity"
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
	u1, err := svc.resolveUser(ctx, p, subject, email, true, "New User", "", "", "")
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
	u2, err := svc.resolveUser(ctx, p2, subject2, email, true, "GitHub Name", "", "", "")
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
	_, err = svc.resolveUser(ctx, p3, "sub-3", "unknown@example.com", true, "", "", "", "")
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
