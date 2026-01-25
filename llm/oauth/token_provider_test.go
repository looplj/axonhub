package oauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestTokenProviderExchangeValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	provider := NewTokenProvider(TokenProviderParams{})

	_, err := provider.Exchange(ctx, ExchangeParams{})
	require.EqualError(t, err, "http client is nil")

	provider = NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
		OAuthUrls:  OAuthUrls{},
	})
	_, err = provider.Exchange(ctx, ExchangeParams{})
	require.EqualError(t, err, "token URL is empty")

	provider = NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClient(),
		OAuthUrls:  OAuthUrls{TokenUrl: "http://example.com/token"},
	})

	_, err = provider.Exchange(ctx, ExchangeParams{})
	require.EqualError(t, err, "code is empty")

	_, err = provider.Exchange(ctx, ExchangeParams{Code: "code"})
	require.EqualError(t, err, "code_verifier is empty")

	_, err = provider.Exchange(ctx, ExchangeParams{Code: "code", CodeVerifier: "verifier"})
	require.EqualError(t, err, "client_id is empty")

	_, err = provider.Exchange(ctx, ExchangeParams{Code: "code", CodeVerifier: "verifier", ClientID: "client"})
	require.EqualError(t, err, "redirect_uri is empty")
}

func TestTokenProviderExchangeSuccess(t *testing.T) {
	t.Parallel()

	var gotUA string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/token", r.URL.Path)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))

		gotUA = r.Header.Get("User-Agent")

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "authorization_code", form.Get("grant_type"))
		require.Equal(t, "client-1", form.Get("client_id"))
		require.Equal(t, "code-1", form.Get("code"))
		require.Equal(t, "https://example.com/callback", form.Get("redirect_uri"))
		require.Equal(t, "verifier-1", form.Get("code_verifier"))

		resp := TokenResponse{
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			TokenType:    "Bearer",
			Scope:        "openid profile",
			ExpiresIn:    3600,
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL + "/token"},
		UserAgent:  "axonhub-test",
	})

	ctx := context.Background()
	start := time.Now()
	creds, err := provider.Exchange(ctx, ExchangeParams{
		Code:         "code-1",
		CodeVerifier: "verifier-1",
		ClientID:     "client-1",
		RedirectURI:  "https://example.com/callback",
	})
	require.NoError(t, err)
	require.Equal(t, "axonhub-test", gotUA)
	require.Equal(t, "client-1", creds.ClientID)
	require.Equal(t, "access-1", creds.AccessToken)
	require.Equal(t, "refresh-1", creds.RefreshToken)
	require.Equal(t, "Bearer", creds.TokenType)
	require.Equal(t, []string{"openid", "profile"}, creds.Scopes)
	require.True(t, creds.ExpiresAt.After(start))
	require.True(t, creds.ExpiresAt.Before(start.Add(2*time.Hour)))

	// ensure provider cached credentials
	got, err := provider.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, creds, got)
}

func TestTokenProviderExchangeErrorResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errResp := TokenError{Error: "invalid_grant", ErrorDescription: "bad code"}
		b, _ := json.Marshal(errResp)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL + "/token"},
	})

	_, err := provider.Exchange(context.Background(), ExchangeParams{
		Code:         "code-1",
		CodeVerifier: "verifier-1",
		ClientID:     "client-1",
		RedirectURI:  "https://example.com/callback",
	})
	require.EqualError(t, err, "token exchange failed: invalid_grant - bad code")
}

func TestTokenProviderGetRefreshFlowSingleflight(t *testing.T) {
	t.Skip("skipping unstable test")
	t.Parallel()

	var calls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		form, err := url.ParseQuery(string(body))
		require.NoError(t, err)
		require.Equal(t, "refresh_token", form.Get("grant_type"))
		require.Equal(t, "client-1", form.Get("client_id"))
		require.Equal(t, "refresh-1", form.Get("refresh_token"))

		resp := TokenResponse{
			AccessToken:  "access-2",
			RefreshToken: "refresh-2",
			TokenType:    "Bearer",
			Scope:        "scope-a scope-b",
			ExpiresIn:    120,
		}
		b, err := json.Marshal(resp)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	t.Cleanup(server.Close)

	var refreshed atomic.Int32

	provider := NewTokenProvider(TokenProviderParams{
		HTTPClient: httpclient.NewHttpClientWithClient(server.Client()),
		OAuthUrls:  OAuthUrls{TokenUrl: server.URL + "/token"},
		Credentials: &OAuthCredentials{
			ClientID:     "client-1",
			AccessToken:  "access-1",
			RefreshToken: "refresh-1",
			ExpiresAt:    time.Now().Add(-10 * time.Minute),
		},
		OnRefreshed: func(ctx context.Context, refreshedCreds *OAuthCredentials) error {
			refreshed.Add(1)
			require.Equal(t, "access-2", refreshedCreds.AccessToken)

			return nil
		},
	})

	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(2)

	var (
		r1, r2 *OAuthCredentials
		e1, e2 error
	)

	go func() {
		defer wg.Done()

		r1, e1 = provider.Get(ctx)
	}()
	go func() {
		defer wg.Done()

		r2, e2 = provider.Get(ctx)
	}()

	wg.Wait()

	require.NoError(t, e1)
	require.NoError(t, e2)
	require.Equal(t, "access-2", r1.AccessToken)
	require.Equal(t, "access-2", r2.AccessToken)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, int32(1), refreshed.Load())
}

func TestTokenProviderRefreshValidation(t *testing.T) {
	t.Parallel()

	provider := NewTokenProvider(TokenProviderParams{})

	_, err := provider.refresh(context.Background(), nil)
	require.EqualError(t, err, "nil credentials")

	_, err = provider.refresh(context.Background(), &OAuthCredentials{})
	require.EqualError(t, err, "refresh_token is empty")

	_, err = provider.refresh(context.Background(), &OAuthCredentials{RefreshToken: "refresh"})
	require.EqualError(t, err, "token URL is empty")
}
