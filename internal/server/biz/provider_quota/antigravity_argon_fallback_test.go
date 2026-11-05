package provider_quota

import (
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

// TestAntigravityLegacyArgonFallback preserves Argon quotas when the summary
// endpoint is unsupported or has no usable windows, including Argon-only accounts.
func TestAntigravityLegacyArgonFallback(t *testing.T) {
	for _, summaryStatus := range []int{http.StatusNotFound, http.StatusNotImplemented, http.StatusOK} {
		for _, tc := range []struct {
			name    string
			models  string
			windows []string
			usage   float64
			status  string
		}{
			{"argon_only", `"gemini-argon":{"quotaInfo":{"remainingFraction":0.6}}`, []string{"argon_5h"}, 0.4, "available"},
			{"argon_exhausted", `"gemini-argon":{"quotaInfo":{"remainingFraction":0}}`, []string{"argon_5h"}, 1, "exhausted"},
			{"only_argon_available", `"gemini-pro":{"quotaInfo":{"remainingFraction":0}},"claude-sonnet":{"quotaInfo":{"remainingFraction":0}},"gemini-argon":{"quotaInfo":{"remainingFraction":0.6}}`, []string{"gemini_5h", "claude_gpt_5h", "argon_5h"}, 0.4, "available"},
			{"mixed_pools", `"gemini-pro":{"quotaInfo":{"remainingFraction":0.8}},"claude-sonnet":{"quotaInfo":{"remainingFraction":0.7}},"gemini-argon-low":{"quotaInfo":{"remainingFraction":0.9}},"gemini-argon-high":{"quotaInfo":{"remainingFraction":0.2}}`, []string{"gemini_5h", "claude_gpt_5h", "argon_5h"}, 0.8, "warning"},
		} {
			t.Run(http.StatusText(summaryStatus)+"/"+tc.name, func(t *testing.T) {
				var urls []string
				client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					urls = append(urls, r.URL.String())
					status, body := http.StatusOK, `{"models":{`+tc.models+`}}`
					if len(urls) == 1 {
						status, body = summaryStatus, `{"groups":[]}`
					}
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
				})})
				quota, err := NewAntigravityQuotaChecker(client).CheckQuota(t.Context(), &ent.Channel{
					Type:        channel.TypeAntigravity,
					Credentials: objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{AccessToken: "access", ExpiresAt: time.Now().Add(time.Hour)}},
				})
				require.NoError(t, err)
				require.Equal(t, []string{antigravityQuotaSummaryURL, antigravityQuotaURL}, urls)
				require.Len(t, quota.Limits, len(tc.windows))
				for i, window := range tc.windows {
					require.Equal(t, window, quota.Limits[i].Window)
				}
				argon := quota.Limits[len(quota.Limits)-1]
				require.Equal(t, "antigravity_pool:argon", argon.AvailabilityGroup)
				require.InDelta(t, tc.usage, argon.UsageRatio, 1e-9)
				require.Equal(t, tc.status, argon.Status)
				require.Equal(t, tc.name != "argon_exhausted", quota.Ready)
			})
		}
	}
}
