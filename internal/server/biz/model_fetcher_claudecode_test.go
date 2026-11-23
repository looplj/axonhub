// 职责: 用模拟上游和内存数据库验证 Claude Code 目录的分页、凭据选择和预设回退。
// 关联文件:
//
//	internal/server/biz/model_fetcher_claudecode.go
//	internal/server/biz/model_fetcher.go
//	internal/server/biz/channel_model_sync.go

package biz

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func claudeCatalogResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestClaudeCodeCatalogPages(t *testing.T) {
	calls := 0
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "https://api.anthropic.com/v1/models", req.URL.Scheme+"://"+req.URL.Host+req.URL.Path)
		require.Equal(t, "1000", req.URL.Query().Get("limit"))
		require.Equal(t, "Bearer sk-ant-oat-test", req.Header.Get("Authorization"))
		require.Empty(t, req.Header.Get("X-Api-Key"))
		require.Equal(t, "2023-06-01", req.Header.Get("Anthropic-Version"))
		require.Equal(t, "oauth-2025-04-20", req.Header.Get("Anthropic-Beta"))
		if calls == 1 {
			require.Empty(t, req.URL.Query().Get("after_id"))
			return claudeCatalogResponse(200, `{"data":[{"id":"first"}],"has_more":true,"last_id":"first"}`), nil
		}
		require.Equal(t, "first", req.URL.Query().Get("after_id"))
		return claudeCatalogResponse(200, `{"data":[{"id":"first"},{"id":"new"}],"has_more":false}`), nil
	})})
	result, err := NewModelFetcher(client, nil).FetchModels(t.Context(), FetchModelsInput{ChannelType: "claudecode", BaseURL: claudeCodeAPI, APIKey: lo.ToPtr("sk-ant-oat-test")})
	require.NoError(t, err)
	require.Nil(t, result.Error)
	require.False(t, result.Fallback)
	require.Nil(t, result.FallbackReason)
	require.Equal(t, []ModelIdentify{{ID: "first"}, {ID: "new"}}, result.Models)
	require.Equal(t, 2, calls)
}

func TestClaudeCodeCatalogFallback(t *testing.T) {
	const key = `{"access_token":"fake-access"}`
	const partial = `{"data":[{"id":"partial"}],"has_more":true,"last_id":"partial"}`
	for _, tt := range []struct {
		name, baseURL, credential string
		status                    int
		bodies                    []string
		reason                    ModelCatalogFallbackReason
	}{
		{"missing credentials", claudeCodeAPI, "", 0, nil, catalogNoCredentials},
		{"invalid credentials", claudeCodeAPI, "{", 0, nil, catalogAuthorization},
		{"custom origin", "https://relay.example/v1", key, 0, nil, catalogUnavailable},
		{"unexpected path", claudeCodeAPI + "/custom", key, 0, nil, catalogUnavailable},
		{"rejected token", claudeCodeAPI, key, 401, []string{"{}"}, catalogAuthorization},
		{"insufficient scope", claudeCodeAPI, key, 403, []string{"{}"}, catalogAuthorization},
		{"rate limited", claudeCodeAPI, key, 429, []string{"{}"}, catalogUnavailable},
		{"redirect", claudeCodeAPI, key, 302, []string{"{}"}, catalogUnavailable},
		{"transport", claudeCodeAPI, key, 0, []string{""}, catalogUnavailable},
		{"empty list", claudeCodeAPI, key, 200, []string{`{"data":[],"has_more":false}`}, catalogUnavailable},
		{"missing page flag", claudeCodeAPI, key, 200, []string{`{"data":[{"id":"partial"}]}`}, catalogUnavailable},
		{"missing cursor", claudeCodeAPI, key, 200, []string{`{"data":[{"id":"partial"}],"has_more":true}`}, catalogUnavailable},
		{"invalid model", claudeCodeAPI, key, 200, []string{partial, `{"data":[{}],"has_more":false}`}, catalogUnavailable},
		{"repeated cursor", claudeCodeAPI, key, 200, []string{partial, partial}, catalogUnavailable},
		{"incomplete response", claudeCodeAPI, key, 200, []string{partial, "{"}, catalogUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				require.Less(t, calls, len(tt.bodies), "unexpected HTTP request")
				body := tt.bodies[calls]
				calls++
				if tt.status == 0 {
					return nil, errors.New("mock upstream unavailable")
				}
				resp := claudeCatalogResponse(tt.status, body)
				if tt.status == 302 {
					resp.Header.Set("Location", "https://other.example/v1/models")
				}
				return resp, nil
			})})
			result, err := NewModelFetcher(client, nil).FetchModels(t.Context(), FetchModelsInput{ChannelType: "claudecode", BaseURL: tt.baseURL, APIKey: &tt.credential})
			require.NoError(t, err)
			require.Nil(t, result.Error)
			require.True(t, result.Fallback)
			require.Equal(t, &tt.reason, result.FallbackReason)
			require.Equal(t, defaultClaudeCodeModels(), result.Models)
			require.Equal(t, len(tt.bodies), calls)
		})
	}
}

func TestClaudeCodeCatalogChannelCredentials(t *testing.T) {
	ctx := authz.WithSystemBypass(t.Context(), "test-claude-catalog")
	db := enttest.NewEntClient(t, "sqlite3", "file:"+url.QueryEscape(t.Name())+"?mode=memory&_fk=0")
	defer db.Close()
	stored := objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "account-a"}}
	ch := db.Channel.Create().SetName("saved").SetType(channel.TypeClaudecode).SetBaseURL(claudeCodeAPI).
		SetCredentials(stored).SetSupportedModels([]string{"old", "manual"}).SetManualModels([]string{"manual"}).SetDefaultTestModel("old").SaveX(ctx)
	var requests []string
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		if req.URL.Path == "/v1/oauth/token" {
			require.Equal(t, http.MethodPost, req.Method)
			return claudeCatalogResponse(200, `{"access_token":"refreshed-b","refresh_token":"next-b","expires_in":3600}`), nil
		}
		require.Equal(t, "/v1/models", req.URL.Path)
		account := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		return claudeCatalogResponse(200, fmt.Sprintf(`{"data":[{"id":%q}],"has_more":false}`, account)), nil
	})})
	svc := &ChannelService{AbstractService: &AbstractService{db: db}, httpClient: client}
	fetcher := NewModelFetcher(client, svc)
	input := FetchModelsInput{ChannelType: ch.Type.String(), BaseURL: ch.BaseURL, ChannelID: &ch.ID}
	for _, tt := range []struct {
		key     *string
		account string
	}{
		{nil, "account-a"},
		{lo.ToPtr(`{"access_token":"account-b","refresh_token":"fake-refresh","expires_at":"2000-01-01T00:00:00Z"}`), "account-b"},
		{nil, "account-a"},
	} {
		input.APIKey = tt.key
		result, err := fetcher.FetchModels(ctx, input)
		require.NoError(t, err)
		require.False(t, result.Fallback)
		require.Equal(t, []ModelIdentify{{ID: tt.account}}, result.Models)
	}
	require.Equal(t, []string{"GET /v1/models", "GET /v1/models", "GET /v1/models"}, requests)
	requests = nil
	for _, mode := range []modelFetchMode{syncModelsNow, syncModelsPeriodically} {
		result, err := svc.syncChannelModelsForChannel(ctx, ch, nil, mode)
		require.NoError(t, err)
		require.True(t, result.Fallback)
	}
	saved := db.Channel.GetX(ctx, ch.ID)
	require.Equal(t, stored, saved.Credentials)
	require.Equal(t, ch.SupportedModels, saved.SupportedModels)
	require.Equal(t, ch.ManualModels, saved.ManualModels)
	require.Empty(t, requests)

	// Legacy OAuth JSON still uses the persisted refresh path.
	legacy := objects.ChannelCredentials{APIKey: `{"access_token":"old-b","refresh_token":"fake-refresh","expires_at":"2000-01-01T00:00:00Z"}`}
	db.Channel.UpdateOneID(ch.ID).SetCredentials(legacy).SaveX(ctx)
	result, err := fetcher.FetchModels(ctx, input)
	require.NoError(t, err)
	require.Equal(t, []ModelIdentify{{ID: "refreshed-b"}}, result.Models)
	creds, err := db.Channel.GetX(ctx, ch.ID).Credentials.ResolveOAuthCredentials()
	require.NoError(t, err)
	require.Equal(t, "refreshed-b", creds.AccessToken)
}

func TestClaudeCodeCatalogDraftProxy(t *testing.T) {
	targets := make(chan string, 1)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targets <- r.Method + " " + r.Host
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("draft catalog must use its configured proxy")
		return nil, nil
	})})
	result, err := NewModelFetcher(client, nil).FetchModels(t.Context(), FetchModelsInput{
		ChannelType: "claudecode", BaseURL: claudeCodeAPI, APIKey: lo.ToPtr("sk-ant-oat-test"),
		Proxy: &httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: proxy.URL},
	})
	require.NoError(t, err)
	require.True(t, result.Fallback)
	select {
	case target := <-targets:
		require.Equal(t, "CONNECT api.anthropic.com:443", target)
	default:
		t.Fatal("proxy did not receive the catalog request")
	}
}

func TestClaudeCodeCatalogThirdParty(t *testing.T) {
	calls := 0
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "https://relay.example/v1/models", req.URL.String())
		require.Contains(t, []string{req.Header.Get("X-Api-Key"), strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")}, "synthetic-plain-key")
		return claudeCatalogResponse(200, `{"data":[{"id":"relay-model"}]}`), nil
	})})
	result, err := NewModelFetcher(client, nil).FetchModels(t.Context(), FetchModelsInput{ChannelType: "claudecode", BaseURL: "https://relay.example/v1", APIKey: lo.ToPtr("synthetic-plain-key")})
	require.NoError(t, err)
	require.Nil(t, result.Error)
	require.False(t, result.Fallback)
	require.Equal(t, []ModelIdentify{{ID: "relay-model"}}, result.Models)
	require.Equal(t, 1, calls)
}
