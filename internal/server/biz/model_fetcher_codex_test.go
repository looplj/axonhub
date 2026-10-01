package biz

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func TestModelFetcher_CodexCatalog(t *testing.T) {
	for _, tt := range []struct {
		name     string
		status   int
		body     string
		fallback bool
	}{
		{"visible catalog", 200, `{"models":[{"slug":"new-sol","visibility":"list","priority":2,"supported_in_api":false},{"slug":"hidden","visibility":"hide","priority":0},{"slug":"first-astra","visibility":"list","priority":1},{"slug":"new-sol","visibility":"list","priority":3}]}`, false},
		{"unauthorized", 401, `{}`, true},
		{"malformed", 200, `{`, true},
		{"empty", 200, `{"models":[]}`, true},
		{"hidden only", 200, `{"models":[{"slug":"hidden","visibility":"hide"}]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "https://chatgpt.com/backend-api/codex/models", req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
				require.NotEmpty(t, req.URL.Query().Get("client_version"))
				require.Equal(t, "Bearer synthetic-token", req.Header.Get("Authorization"))
				require.Equal(t, req.URL.Query().Get("client_version"), req.Header.Get("Version"))
				require.NotEmpty(t, req.Header.Get(codex.BetaFeaturesHeader))
				return &http.Response{StatusCode: tt.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tt.body))}, nil
			})})
			fetcher := NewModelFetcher(client, nil)
			key := `{"access_token":"synthetic-token","token_type":"bearer","expires_at":"2099-01-01T00:00:00Z"}`
			result, err := fetcher.FetchModels(t.Context(), FetchModelsInput{
				ChannelType: channel.TypeCodex.String(), BaseURL: "https://chatgpt.com/backend-api/codex#", APIKey: &key,
			})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, tt.fallback, result.Fallback)
			require.Nil(t, result.Error)
			if tt.fallback {
				require.Len(t, result.Models, len(codex.DefaultModels()))
			} else {
				require.Equal(t, []ModelIdentify{{ID: "first-astra"}, {ID: "new-sol"}}, result.Models)
			}
		})
	}
}

func TestModelFetcher_CodexStoredCredentialReplacement(t *testing.T) {
	for _, mode := range []string{"replace-oauth", "replace-api-key", "mismatch", "stored-api-key", "proxy"} {
		t.Run(mode, func(t *testing.T) {
			ctx := authz.WithSystemBypass(t.Context(), "test-model-fetch")
			db := enttest.NewEntClient(t, "sqlite3", "file:"+url.QueryEscape(t.Name())+"?mode=memory&_fk=0")
			defer db.Close()
			credentials := objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
				AccessToken: "stored-token", ExpiresAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
			}}
			if mode == "stored-api-key" {
				credentials = objects.ChannelCredentials{APIKey: "stored-key"}
			}
			settings := &objects.ChannelSettings{HeaderOverrideOperations: []objects.OverrideOperation{
				{Op: "set", Path: "X-Catalog-Test", Value: "preserved"},
			}}
			proxyCalls := 0
			if mode == "proxy" {
				proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					proxyCalls++
					require.Equal(t, http.MethodConnect, req.Method)
					require.Equal(t, "chatgpt.com:443", req.Host)
					w.WriteHeader(http.StatusBadGateway)
				}))
				defer proxy.Close()
				settings.Proxy = &httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: proxy.URL}
			}
			ch, err := db.Channel.Create().SetName("stored-codex").SetType(channel.TypeCodex).
				SetBaseURL("https://chatgpt.com/backend-api/codex#").SetCredentials(credentials).
				SetSettings(settings).SetSupportedModels([]string{"existing"}).SetDefaultTestModel("existing").Save(ctx)
			require.NoError(t, err)
			calls := 0
			client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "preserved", req.Header.Get("X-Catalog-Test"))
				body := `{"models":[{"slug":"new-model","visibility":"list"}]}`
				if mode == "replace-api-key" {
					require.Equal(t, "https://relay.example/v1/models", req.URL.String())
					require.Equal(t, "Bearer replacement-key", req.Header.Get("Authorization"))
					body = `{"data":[{"id":"new-model"}]}`
				} else {
					require.Equal(t, "Bearer replacement-token", req.Header.Get("Authorization"))
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})})
			key := `{"access_token":"replacement-token","expires_at":"2099-01-01T00:00:00Z"}`
			input := FetchModelsInput{ChannelID: &ch.ID, ChannelType: ch.Type.String(), BaseURL: ch.BaseURL, APIKey: &key}
			if mode == "replace-api-key" || mode == "mismatch" {
				input.BaseURL = "https://relay.example/v1"
				key = "replacement-key"
			}
			if mode == "mismatch" {
				input.APIKey = nil
			}
			fetcher := NewModelFetcher(client, &ChannelService{AbstractService: &AbstractService{db: db}, httpClient: client})
			result, err := fetcher.FetchModels(ctx, input)
			require.NoError(t, err)
			switch mode {
			case "mismatch":
				require.NotNil(t, result.Error)
				require.Zero(t, calls)
			case "proxy":
				require.Equal(t, 1, proxyCalls)
				require.Zero(t, calls)
				require.True(t, result.Fallback)
			default:
				require.Nil(t, result.Error)
				require.Equal(t, 1, calls)
				require.False(t, result.Fallback)
				require.Equal(t, []ModelIdentify{{ID: "new-model"}}, result.Models)
			}
			saved, err := db.Channel.Get(ctx, ch.ID)
			require.NoError(t, err)
			require.Equal(t, credentials, saved.Credentials)
		})
	}
}

func TestModelFetcher_CodexRelayModels(t *testing.T) {
	calls := 0
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "https://relay.example/v1/models", req.URL.String())
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"relay-model"}]}`))}, nil
	})})
	key := "synthetic-api-key"
	result, err := NewModelFetcher(client, nil).FetchModels(t.Context(), FetchModelsInput{
		ChannelType: channel.TypeCodex.String(), BaseURL: "https://relay.example/v1", APIKey: &key,
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Equal(t, []ModelIdentify{{ID: "relay-model"}}, result.Models)
	require.False(t, result.Fallback)
}
