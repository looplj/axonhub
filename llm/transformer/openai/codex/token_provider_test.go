package codex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

func TestTokenProvider_RefreshInvokesCallback(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"token_type":"bearer","scope":"openid offline_access"}`))
	}))
	t.Cleanup(tokenServer.Close)

	creds := &oauth.OAuthCredentials{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ClientID:     ClientID,
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}

	var called int

	p := oauth.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: creds,
		HTTPClient:  httpclient.NewHttpClient(),
		OAuthUrls: oauth.OAuthUrls{
			AuthorizeUrl: AuthorizeURL,
			TokenUrl:     tokenServer.URL,
		},
		OnRefreshed: func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
			called++

			require.Equal(t, "new-access", refreshed.AccessToken)
			require.Equal(t, "new-refresh", refreshed.RefreshToken)
			require.NotZero(t, refreshed.ExpiresAt)
			require.Equal(t, "bearer", refreshed.TokenType)
			require.Contains(t, refreshed.Scopes, "openid")

			return nil
		},
	})

	got, err := p.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "new-access", got.AccessToken)
	require.Equal(t, 1, called)
}

func TestTokenProvider_UnexpiredDoesNotInvokeCallback(t *testing.T) {
	creds := &oauth.OAuthCredentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ClientID:     ClientID,
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	var called int

	p := oauth.NewTokenProvider(oauth.TokenProviderParams{
		Credentials: creds,
		HTTPClient:  httpclient.NewHttpClient(),
		OAuthUrls:   DefaultTokenURLs,
		OnRefreshed: func(ctx context.Context, refreshed *oauth.OAuthCredentials) error {
			called++
			return nil
		},
	})

	got, err := p.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "access", got.AccessToken)
	require.Equal(t, 0, called)
}
