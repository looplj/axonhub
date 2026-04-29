package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
)

func TestOIDCService_GetProviders_IsLinked(t *testing.T) {
	svc, client := setupTestOIDCService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Configure mock providers
	svc.cfg.Providers = []OIDCProvider{
		{
			ID:        "google",
			Name:      "google",
			IssuerURL: "https://accounts.google.com",
		},
		{
			ID:        "github",
			Name:      "github",
			IssuerURL: "https://github.com",
		},
	}

	// 1. Unauthenticated request
	providers := svc.GetProviders(ctx)
	require.Len(t, providers, 2)
	require.False(t, providers[0].IsLinked)
	require.False(t, providers[1].IsLinked)

	// 2. Authenticated request, no identities
	u, err := client.User.Create().SetEmail("test@example.com").SetPassword("pw").Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithUser(ctx, u)

	providers = svc.GetProviders(ctx)
	require.Len(t, providers, 2)
	require.False(t, providers[0].IsLinked)
	require.False(t, providers[1].IsLinked)

	// 3. Authenticated request, one identity linked
	err = svc.createIdentity(ctx, u.ID, "https://accounts.google.com", "sub-1", u.Email, "google")
	require.NoError(t, err)

	providers = svc.GetProviders(ctx)
	require.Len(t, providers, 2)

	// Find google provider
	var google, github ProviderInfo

	for _, p := range providers {
		if p.ID == "google" {
			google = p
		} else if p.ID == "github" {
			github = p
		}
	}

	require.True(t, google.IsLinked)
	require.False(t, github.IsLinked)
}
