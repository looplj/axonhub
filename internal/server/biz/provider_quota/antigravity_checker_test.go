package provider_quota

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/antigravity"
)

func TestAntigravityQuotaChecker_CheckQuota(t *testing.T) {
	resetAt := "2026-09-04T08:00:00Z"
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, antigravityQuotaURL, request.URL.String())
		require.Equal(t, "Bearer access-token", request.Header.Get("Authorization"))
		require.Equal(t, "antigravity", request.Header.Get("X-Client-Name"))
		require.Equal(t, antigravity.GetVersion(), request.Header.Get("X-Client-Version"))
		var requestBody map[string]any
		require.NoError(t, json.NewDecoder(request.Body).Decode(&requestBody))
		require.Equal(t, "project-id", requestBody["project"])
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"models": {
					"gemini-3-pro": {"displayName":"Gemini 3 Pro","quotaInfo":{"remainingFraction":0.75,"resetTime":"` + resetAt + `"}},
					"claude-sonnet": {"displayName":"Claude Sonnet","quotaInfo":{"remainingFraction":0.1,"resetTime":"` + resetAt + `"}},
					"internal": {"isInternal":true,"quotaInfo":{"remainingFraction":0}}
				}
			}`)),
		}, nil
	})})
	checker := NewAntigravityQuotaChecker(httpClient)
	channelEntity := &ent.Channel{
		Type: channel.TypeAntigravity,
		Credentials: objects.ChannelCredentials{
			APIKey: "refresh-token|project-id",
			OAuth: &objects.OAuthCredentials{
				AccessToken: "access-token",
				ExpiresAt:   time.Now().Add(time.Hour),
			},
		},
	}

	quota, err := checker.CheckQuota(t.Context(), channelEntity)

	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.Equal(t, "antigravity", quota.ProviderType)
	require.True(t, quota.Ready)
	require.Empty(t, quota.Limits, "per-model limits must not exhaust the entire channel")
	require.Len(t, quota.RawData["models"], 2)
	models := quota.RawData["models"].(map[string]any)
	require.Equal(t, "warning", models["claude-sonnet"].(map[string]any)["status"])
	require.Equal(t, time.Date(2026, 9, 4, 8, 0, 0, 0, time.UTC), *quota.NextResetAt)
	require.True(t, checker.SupportsChannel(channelEntity))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeGemini}))
}

func TestAntigravityQuotaChecker_CheckQuotaRefreshesLegacyCredentials(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"models":{"gemini":{"quotaInfo":{"remainingFraction":0.5}}}}`
		if request.URL.String() == antigravity.TokenURL {
			require.NoError(t, request.ParseForm())
			require.Equal(t, "refresh_token", request.Form.Get("grant_type"))
			require.Equal(t, "refresh-token", request.Form.Get("refresh_token"))
			body = `{"access_token":"fresh-access-token","expires_in":3600,"token_type":"Bearer"}`
		} else {
			require.Equal(t, antigravityQuotaURL, request.URL.String())
			require.Equal(t, "Bearer fresh-access-token", request.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})})
	checker := NewAntigravityQuotaChecker(httpClient)

	quota, err := checker.CheckQuota(t.Context(), &ent.Channel{
		Type:        channel.TypeAntigravity,
		Credentials: objects.ChannelCredentials{APIKey: "refresh-token|project-id"},
	})

	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
}
