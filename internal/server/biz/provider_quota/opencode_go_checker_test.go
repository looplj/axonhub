package provider_quota

import (
	"context"
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
)

func TestOpenCodeGo_CheckQuota_Hydration(t *testing.T) {
	httpClient := httpclient.NewHttpClientWithClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, http.MethodGet, req.Method)
			require.Equal(t, "https://opencode.ai/workspace/wk_123/go", req.URL.String())
			require.Equal(t, "text/html", req.Header.Get("Accept"))
			require.Equal(t, opencodeGoUserAgent, req.Header.Get("User-Agent"))
			require.Equal(t, "auth=cookie-abc", req.Header.Get("Cookie"))

			body := `<html><script>
				rollingUsage:$R[10]={usagePercent:7.5,resetInSec:18000}
				weeklyUsage:$R[11]={usagePercent:2,resetInSec:540000}
				monthlyUsage:$R[12]={usagePercent:16,resetInSec:2480000}
			</script></html>`

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	})

	checker := NewOpenCodeGoQuotaChecker(httpClient)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	quota, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeOpencodeGo,
		Settings: &objects.ChannelSettings{
			ProviderQuota: &objects.ChannelProviderQuotaSettings{
				OpencodeGo: &objects.OpenCodeGoQuotaSettings{
					WorkspaceID: "wk_123",
					AuthCookie:  "Cookie: other=1; auth=cookie-abc; theme=dark",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "available", quota.Status)
	require.True(t, quota.Ready)
	require.Equal(t, opencodeGoProviderType, quota.ProviderType)
	require.Len(t, quota.Limits, 3)
	require.Equal(t, now.Add(5*time.Hour), *quota.NextResetAt)

	windows, ok := quota.RawData["windows"].(map[string]any)
	require.True(t, ok)
	rolling, ok := windows["rolling"].(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 7.5, rolling["usage_percent"], 0.001)
	require.InDelta(t, 92.5, rolling["percent_remaining"], 0.001)
	require.Equal(t, "available", rolling["status"])
	require.Equal(t, now.Add(5*time.Hour).Format(time.RFC3339), rolling["reset_time"])
}

func TestOpenCodeGo_CheckQuota_DataSlotFallback(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(nil)
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	checker.now = func() time.Time { return now }

	body := []byte(`
		<div data-slot="usage-item">
			<span data-slot="usage-label" class="label">Rolling Usage</span>
			<span data-slot="usage-value" class="value"><!--$-->82<!--/-->%</span>
			<span data-slot="reset-time"><!--$-->Resets in <span>1 hour</span> 30 minutes<!--/--></span>
		</div>
		<div data-slot="usage-item">
			<span data-slot="usage-label">Weekly Usage</span>
			<span data-slot="usage-value">12%</span>
			<span data-slot="reset-time">Resets in 2d 3h</span>
		</div>
	`)

	quota, err := checker.parseResponse(body)
	require.NoError(t, err)
	require.Equal(t, "warning", quota.Status)
	require.True(t, quota.Ready)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, now.Add(90*time.Minute), *quota.NextResetAt)

	windows, ok := quota.RawData["windows"].(map[string]any)
	require.True(t, ok)
	rolling, ok := windows["rolling"].(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 82, rolling["usage_percent"], 0.001)
	require.InDelta(t, 5400, rolling["reset_in_seconds"], 0.001)
	require.Equal(t, "warning", rolling["status"])
}

func TestOpenCodeGo_CheckQuota_MissingCredentials(t *testing.T) {
	checker := NewOpenCodeGoQuotaChecker(nil)

	_, err := checker.CheckQuota(context.Background(), &ent.Channel{
		Type: channel.TypeOpencodeGo,
	})
	require.ErrorContains(t, err, "missing OpenCode Go workspace id and auth cookie")
}
