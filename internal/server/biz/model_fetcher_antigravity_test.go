package biz

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
)

type antigravityCatalogTransport func(*http.Request) (*http.Response, error)

// RoundTrip intercepts OAuth and catalog requests without contacting Google.
func (f antigravityCatalogTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// TestFetchModelsAntigravityOAuthOnlyChannel checks keyless account discovery and endpoint-change protection.
func TestFetchModelsAntigravityOAuthOnlyChannel(t *testing.T) {
	for _, tc := range []struct {
		name, refreshToken string
		expired            bool
	}{
		{"valid saved OAuth", "saved-refresh", false},
		{"valid access token only", "", false},
		{"expired saved OAuth", "saved-refresh", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:antigravity_catalog_oauth_only?mode=memory&_fk=0")
			defer client.Close()
			ctx := authz.WithSystemBypass(t.Context(), "test")
			expiresAt := time.Now().Add(time.Hour).UTC()
			if tc.expired {
				expiresAt = time.Now().Add(-time.Hour).UTC()
			}
			ch, err := client.Channel.Create().
				SetName("oauth-only-account").SetType(channel.TypeAntigravity).
				SetBaseURL(antigravity.EndpointDaily).
				SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "saved-access", RefreshToken: tc.refreshToken, ExpiresAt: expiresAt}}).
				SetSupportedModels([]string{"old-model"}).SetDefaultTestModel("old-model").Save(ctx)
			require.NoError(t, err)
			refreshCalls, catalogCalls := 0, 0
			transport := antigravityCatalogTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == antigravity.TokenURL {
					refreshCalls++
					require.NoError(t, r.ParseForm())
					require.Equal(t, "saved-refresh", r.Form.Get("refresh_token"))
					return antigravityCatalogResponse(r, 200, `{"access_token":"fresh-access","expires_in":3600,"token_type":"Bearer"}`), nil
				}
				catalogCalls++
				accessToken := "saved-access"
				if tc.expired {
					accessToken = "fresh-access"
				}
				require.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
				return antigravityCatalogResponse(r, 200, `{"models":{"oauth-account-model":{}}}`), nil
			})
			fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}), &ChannelService{AbstractService: &AbstractService{db: client}})
			input := FetchModelsInput{ChannelType: "antigravity", BaseURL: ch.BaseURL, ChannelID: &ch.ID}
			result, err := fetcher.FetchModels(ctx, input)
			require.NoError(t, err)
			require.Nil(t, result.Error)
			require.Equal(t, []ModelIdentify{{ID: "oauth-account-model"}}, result.Models)
			if tc.expired {
				require.Equal(t, 1, refreshCalls)
			} else {
				require.Zero(t, refreshCalls)
			}
			require.Equal(t, 1, catalogCalls)
			input.BaseURL = "https://changed.example"
			result, err = fetcher.FetchModels(ctx, input)
			require.NoError(t, err)
			require.NotNil(t, result.Error)
			require.Equal(t, 1, catalogCalls, "an edited endpoint must not reuse saved OAuth")
		})
	}
}

// TestFetchModelsAntigravitySavedCredentialsAndRefreshPersistence checks verified token reuse and storage.
func TestFetchModelsAntigravitySavedCredentialsAndRefreshPersistence(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:antigravity_catalog_persistence?mode=memory&_fk=0")
	defer client.Close()
	ctx := authz.WithSystemBypass(context.Background(), "test")
	ch, err := client.Channel.Create().
		SetName("antigravity-catalog").
		SetType(channel.TypeAntigravity).
		SetBaseURL(antigravity.EndpointDaily).
		SetCredentials(objects.ChannelCredentials{APIKey: "stored-refresh|stored-project"}).
		SetSupportedModels([]string{"old-model"}).
		SetDefaultTestModel("old-model").
		Save(ctx)
	require.NoError(t, err)
	refreshCalls := 0
	catalogCalls := 0
	transport := antigravityCatalogTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == antigravity.TokenURL {
			refreshCalls++
			require.NoError(t, r.ParseForm())
			require.Equal(t, "stored-refresh", r.Form.Get("refresh_token"))
			return antigravityCatalogResponse(r, 200, `{"access_token":"saved-access","refresh_token":"rotated-refresh","expires_in":3600,"token_type":"Bearer"}`), nil
		}
		catalogCalls++
		require.Equal(t, "Bearer saved-access", r.Header.Get("Authorization"))
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "stored-project", body["project"])
		return antigravityCatalogResponse(r, 200, `{"models":{"new-account-model":{}}}`), nil
	})
	fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}), &ChannelService{AbstractService: &AbstractService{db: client}})
	input := FetchModelsInput{ChannelType: "antigravity", BaseURL: ch.BaseURL, ChannelID: &ch.ID}
	for range 2 {
		result, err := fetcher.FetchModels(ctx, input)
		require.NoError(t, err)
		require.Nil(t, result.Error)
		require.Equal(t, []ModelIdentify{{ID: "new-account-model"}}, result.Models)
	}
	require.Equal(t, 1, refreshCalls, "the persisted access token should be reused")
	require.Equal(t, 2, catalogCalls)
	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, "rotated-refresh|stored-project", updated.Credentials.APIKey)
	require.Equal(t, "saved-access", updated.Credentials.OAuth.AccessToken)

	input.BaseURL = "https://changed.example"
	result, err := fetcher.FetchModels(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result.Error)
	require.Empty(t, result.Models)
	require.Equal(t, 2, catalogCalls, "changed input must not silently reuse saved credentials")
}

// antigravityCatalogResponse constructs a JSON response for the intercepted request.
func antigravityCatalogResponse(r *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}

// TestFetchModelsAntigravityAccountCatalog checks client identity and preservation of public effort variants.
func TestFetchModelsAntigravityAccountCatalog(t *testing.T) {
	key := `{"access_token":"test-access","refresh_token":"test-refresh","expires_at":"2099-01-01T00:00:00Z","project_id":"test-project"}`
	calls := 0
	transport := antigravityCatalogTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, antigravity.EndpointProd+"/v1internal:fetchAvailableModels", r.URL.String())
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer test-access", r.Header.Get("Authorization"))
		expectedHeaders := http.Header{}
		antigravity.SetClientHeaders(expectedHeaders)
		for name, values := range expectedHeaders {
			require.Equal(t, values, r.Header.Values(name), name)
		}
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "test-project", body["project"])
		return antigravityCatalogResponse(r, 200, `{"models":{"claude-sonnet-5-5-high":{},"claude-opus-5-5-low":{},"claude-opus-5-5-medium":{},"claude-opus-5-5-high":{},"hidden":{"isInternal":true}," ":{}}}`), nil
	})
	fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}), nil)
	result, err := fetcher.FetchModels(t.Context(), FetchModelsInput{ChannelType: "antigravity", BaseURL: antigravity.EndpointDaily, APIKey: &key})
	require.NoError(t, err)
	require.Nil(t, result.Error)
	require.False(t, result.Fallback)
	// Effort suffixes are upstream IDs; preserve each variant rather than inventing a bare ID.
	require.Equal(t, []ModelIdentify{{ID: "claude-opus-5-5-high"}, {ID: "claude-opus-5-5-low"}, {ID: "claude-opus-5-5-medium"}, {ID: "claude-sonnet-5-5-high"}}, result.Models)
	require.Equal(t, 1, calls)
	require.Nil(t, fetcher.getDefaultModelsByType(t.Context(), channel.TypeAntigravity))
}

// TestFetchModelsAntigravityLegacyRefresh checks refreshToken|projectID credential compatibility.
func TestFetchModelsAntigravityLegacyRefresh(t *testing.T) {
	key := "test-refresh|test-project"
	calls := 0
	transport := antigravityCatalogTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			require.Equal(t, antigravity.TokenURL, r.URL.String())
			require.NoError(t, r.ParseForm())
			require.Equal(t, "test-refresh", r.Form.Get("refresh_token"))
			require.Equal(t, antigravity.ClientID, r.Form.Get("client_id"))
			require.Equal(t, antigravity.GetUserAgent(), r.Header.Get("User-Agent"))
			return antigravityCatalogResponse(r, 200, `{"access_token":"fresh-access","expires_in":3600,"token_type":"Bearer"}`), nil
		}
		require.Equal(t, "Bearer fresh-access", r.Header.Get("Authorization"))
		return antigravityCatalogResponse(r, 200, `{"models":{"account-model":{}}}`), nil
	})
	fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}), nil)
	result, err := fetcher.FetchModels(t.Context(), FetchModelsInput{ChannelType: "antigravity", APIKey: &key})
	require.NoError(t, err)
	require.Nil(t, result.Error)
	require.Equal(t, []ModelIdentify{{ID: "account-model"}}, result.Models)
	require.Equal(t, 2, calls)
}

// TestFetchModelsAntigravityFailuresDoNotReturnStaticModels ensures discovery failures remain visible.
func TestFetchModelsAntigravityFailuresDoNotReturnStaticModels(t *testing.T) {
	for _, tc := range []struct {
		name, key, body string
		status          int
	}{
		{"missing credentials", "", "", 200},
		{"invalid credentials", "{broken", "", 200},
		{"forbidden", `{"access_token":"access","expires_at":"2099-01-01T00:00:00Z"}`, `{"error":"forbidden"}`, 403},
		{"empty catalog", `{"access_token":"access","expires_at":"2099-01-01T00:00:00Z"}`, `{"models":{}}`, 200},
		{"invalid response", `{"access_token":"access","expires_at":"2099-01-01T00:00:00Z"}`, `{broken`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := antigravityCatalogTransport(func(r *http.Request) (*http.Response, error) {
				return antigravityCatalogResponse(r, tc.status, tc.body), nil
			})
			fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}), nil)
			result, err := fetcher.FetchModels(t.Context(), FetchModelsInput{ChannelType: "antigravity", APIKey: &tc.key})
			require.NoError(t, err)
			require.NotNil(t, result.Error)
			require.Empty(t, result.Models)
			require.False(t, result.Fallback)
		})
	}
}

// TestFetchModelsAntigravityMismatchedSavedOAuth isolates the APIKey account from unverified cached tokens.
func TestFetchModelsAntigravityMismatchedSavedOAuth(t *testing.T) {
	for _, tc := range []struct {
		name, savedRefresh string
		expiresAt          time.Time
	}{
		{"valid token for another account", "old-refresh", time.Now().Add(time.Hour).UTC()},
		{"expired token for another account", "old-refresh", time.Now().Add(-time.Hour).UTC()},
		{"unverifiable saved token", "", time.Now().Add(time.Hour).UTC()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := enttest.NewEntClient(t, "sqlite3", "file:antigravity_catalog_mismatch?mode=memory&_fk=0")
			defer client.Close()
			ctx := authz.WithSystemBypass(t.Context(), "test")
			original := objects.ChannelCredentials{
				APIKey: "new-refresh|new-project",
				OAuth:  &objects.OAuthCredentials{AccessToken: "old-access", RefreshToken: tc.savedRefresh, ExpiresAt: tc.expiresAt},
			}
			ch, err := client.Channel.Create().
				SetName("mismatched-account").SetType(channel.TypeAntigravity).
				SetBaseURL(antigravity.EndpointDaily).SetCredentials(original).
				SetSupportedModels([]string{"old-model"}).SetDefaultTestModel("old-model").Save(ctx)
			require.NoError(t, err)
			refreshCalls, catalogCalls := 0, 0
			transport := antigravityCatalogTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == antigravity.TokenURL {
					refreshCalls++
					require.NoError(t, r.ParseForm())
					require.Equal(t, "new-refresh", r.Form.Get("refresh_token"))
					return antigravityCatalogResponse(r, 200, `{"access_token":"new-access","refresh_token":"new-rotated-refresh","expires_in":3600,"token_type":"Bearer"}`), nil
				}
				catalogCalls++
				require.Equal(t, "Bearer new-access", r.Header.Get("Authorization"))
				var body map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "new-project", body["project"])
				return antigravityCatalogResponse(r, 200, `{"models":{"new-account-model":{}}}`), nil
			})
			fetcher := NewModelFetcher(httpclient.NewHttpClientWithClient(&http.Client{Transport: transport}), &ChannelService{AbstractService: &AbstractService{db: client}})
			result, err := fetcher.FetchModels(ctx, FetchModelsInput{ChannelType: "antigravity", BaseURL: ch.BaseURL, ChannelID: &ch.ID})
			require.NoError(t, err)
			require.Nil(t, result.Error)
			require.Equal(t, []ModelIdentify{{ID: "new-account-model"}}, result.Models)
			require.Equal(t, 1, refreshCalls)
			require.Equal(t, 1, catalogCalls)
			updated, err := client.Channel.Get(ctx, ch.ID)
			require.NoError(t, err)
			require.Equal(t, original, updated.Credentials, "unverified OAuth pairs must not be overwritten by discovery")
		})
	}
}
