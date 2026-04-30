package biz

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOIDCService_PKCE_Flow(t *testing.T) {
	svc, client := setupTestOIDCService(t)
	defer client.Close()

	ctx := context.Background()

	// 1. Setup provider with PKCE enabled
	providerID := "test-pkce"
	p := &oidcProvider{
		config: OIDCProvider{
			ID:         providerID,
			Name:       providerID,
			EnablePKCE: true,
		},
	}
	svc.providers[providerID] = p

	// 2. Test GetAuthorizeURL adds PKCE params
	authURLStr, state, err := svc.GetAuthorizeURL(ctx, providerID, "http://localhost:8090")
	require.NoError(t, err)
	require.NotEmpty(t, state)

	authURL, err := url.Parse(authURLStr)
	require.NoError(t, err)

	query := authURL.Query()
	require.NotEmpty(t, query.Get("code_challenge"), "code_challenge should be present in the authorize URL")
	require.Equal(t, "S256", query.Get("code_challenge_method"), "code_challenge_method should be S256")

	// Verify verifier is stored in cache
	verifierBytes, err := svc.cache.Get(ctx, "oidc_pkce:"+state)
	require.NoError(t, err)
	require.NotEmpty(t, verifierBytes, "PKCE verifier should be stored in cache")

	// 3. Test Callback PKCE verification logic

	// Case A: Missing state/verifier in cache should fail
	_, _, err = svc.Callback(ctx, providerID, "test-code", "non-existent-state", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid or expired state parameter")

	// Case B: Correct state but exchange will fail (because we didn't mock the provider fully)
	// But we can at least see it passed the PKCE check if the error is about exchange
	_, _, err = svc.Callback(ctx, providerID, "test-code", state, "")
	require.Error(t, err)
	// It should fail at exchange because p.oauth2 is empty/uninitialized for a real exchange
	require.Contains(t, err.Error(), "failed to exchange authorization code")

	// Verify verifier is consumed from cache
	verifierBytes, err = svc.cache.Get(ctx, "oidc_pkce:"+state)
	require.Error(t, err, "PKCE verifier should be consumed from cache after use")
	require.Empty(t, verifierBytes)
}
