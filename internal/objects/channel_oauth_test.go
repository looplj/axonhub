package objects

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelCredentialsResolveOAuthCredentialsPrefersNonEmptyOAuth(t *testing.T) {
	// Given
	oauthCredentials := &OAuthCredentials{AccessToken: "current-access", RefreshToken: "current-refresh"}
	credentials := &ChannelCredentials{
		OAuth:  oauthCredentials,
		APIKey: `{"access_token":"legacy-access","refresh_token":"legacy-refresh"}`,
	}

	// When
	resolved, err := credentials.ResolveOAuthCredentials()

	// Then
	require.NoError(t, err)
	require.Same(t, oauthCredentials, resolved)
}

func TestChannelCredentialsResolveOAuthCredentialsFallsBackToLegacyJSON(t *testing.T) {
	// Given
	credentials := &ChannelCredentials{
		OAuth:  &OAuthCredentials{},
		APIKey: `{"access_token":"legacy-access","refresh_token":"legacy-refresh"}`,
	}

	// When
	resolved, err := credentials.ResolveOAuthCredentials()

	// Then
	require.NoError(t, err)
	require.Equal(t, "legacy-access", resolved.AccessToken)
	require.Equal(t, "legacy-refresh", resolved.RefreshToken)
}

func TestChannelCredentialsResolveOAuthCredentialsRejectsMissingCredentials(t *testing.T) {
	// Given
	credentials := &ChannelCredentials{}

	// When
	_, err := credentials.ResolveOAuthCredentials()

	// Then
	require.ErrorContains(t, err, "empty credentials")
}
