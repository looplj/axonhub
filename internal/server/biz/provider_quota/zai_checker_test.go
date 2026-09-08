package provider_quota

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestZai_CheckQuota_HappyPath(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://api.z.ai/api/monitor/usage/quota/limit", req.URL.String())
			require.Equal(t, "zai-key", req.Header.Get("Authorization"))
			require.Equal(t, "en-US,en", req.Header.Get("Accept-Language"))

			body := `{
				"success": true,
				"code": 200,
				"data": {
					"level": "standard",
					"limits": [
						{"type": "TOKENS_LIMIT", "percentage": 25.5, "nextResetTime": 1784322000000},
						{"type": "TOKENS_LIMIT", "percentage": 10.0, "nextResetTime": 1784476800000},
						{"type": "TIME_LIMIT", "percentage": 50.0}
					]
				}
			}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZaiQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZai,
		BaseURL:     "https://api.z.ai/api/paas/v4",
		Credentials: objects.ChannelCredentials{APIKey: "zai-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, "zai", quota.ProviderType)
	require.Len(t, quota.Limits, 2)

	rows, ok := quota.RawData["rows"].([]zhipuWindowRow)
	require.True(t, ok)
	require.Len(t, rows, 2)
	require.Equal(t, "five_hour", rows[0].Window)
	require.Equal(t, "weekly_limit", rows[1].Window)
	require.Equal(t, "standard", quota.RawData["level"])
}

func TestZai_CheckQuota_AnthropicChannelType(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "https://api.z.ai/api/monitor/usage/quota/limit", req.URL.String())

			body := `{"success":true,"code":200,"data":{"level":"basic","limits":[{"type":"TOKENS_LIMIT","percentage":5.0}]}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZaiQuotaChecker(httpClient)
	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZaiAnthropic,
		BaseURL:     "https://api.z.ai/api/anthropic",
		Credentials: objects.ChannelCredentials{APIKey: "zai-key"},
	})
	require.NoError(t, err)
	require.Equal(t, "zai", quota.ProviderType)
	require.Len(t, quota.Limits, 1)
}

func TestZai_CheckQuota_APIError(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"success": false, "code": 1001, "msg": "Authentication parameter not received in Header, unable to authenticate"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewZaiQuotaChecker(httpClient)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type:        channel.TypeZai,
		Credentials: objects.ChannelCredentials{APIKey: "bad-key"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Authentication parameter not received")
}

func TestZai_CheckQuota_MissingAPIKey(t *testing.T) {
	checker := NewZaiQuotaChecker(nil)
	_, err := checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeZai})
	require.ErrorContains(t, err, "channel has no API key")
}

func TestZai_SupportsChannel(t *testing.T) {
	checker := NewZaiQuotaChecker(nil)

	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZai}))
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZaiAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZhipu}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeZhipuAnthropic}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeOpenai}))
}
