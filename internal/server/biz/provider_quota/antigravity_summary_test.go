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
)

const antigravitySummaryFixture = `{"response":{"groups":[
 {"displayName":"Gemini Models","buckets":[
  {"bucketId":"gemini-weekly","displayName":"Weekly Limit","remaining":{"remainingFraction":0.82},"resetTime":"2099-09-11T08:00:00Z"},
  {"bucketId":"gemini-5h","displayName":"Five Hour Limit","remaining":{"remainingFraction":0.91},"resetTime":"2099-09-04T13:00:00Z"}]},
 {"displayName":"Claude and GPT models","buckets":[
  {"bucketId":"3p-weekly","remaining":{"case":"remainingFraction","value":0.64}},
  {"bucketId":"3p-5h","remainingFraction":0.73}]}]}}`

// TestAntigravitySummaryWindows verifies independent weekly and session values for both shared pools.
func TestAntigravitySummaryWindows(t *testing.T) {
	quota, err := parseAntigravityQuotaSummary([]byte(antigravitySummaryFixture))
	require.NoError(t, err)
	require.Len(t, quota.Limits, 4)
	for i, expected := range []struct {
		window string
		usage  float64
	}{
		{"gemini_5h", 0.09}, {"gemini_7d", 0.18}, {"claude_gpt_5h", 0.27}, {"claude_gpt_7d", 0.36},
	} {
		require.Equal(t, expected.window, quota.Limits[i].Window)
		require.InDelta(t, expected.usage, quota.Limits[i].UsageRatio, 1e-9)
	}
	require.Equal(t, 5*time.Hour, quota.Limits[0].NextResetAt.Sub(*quota.Limits[0].PeriodStart))
	require.Equal(t, 7*24*time.Hour, quota.Limits[1].NextResetAt.Sub(*quota.Limits[1].PeriodStart))
	require.NotNil(t, quota.RawData["response"], "preserve the provider summary for diagnosis")
}

// TestAntigravitySummaryWindowAliases covers the cadence names the provider has used across builds.
func TestAntigravitySummaryWindowAliases(t *testing.T) {
	for _, tc := range []struct {
		window, id, displayName, expectedWindow string
		expectedDuration                        time.Duration
	}{
		{"", "gemini-weekly", "Weekly Limit", "7d", 7 * 24 * time.Hour},
		{"", "3p-weekly", "", "7d", 7 * 24 * time.Hour},
		{"7d", "", "", "7d", 7 * 24 * time.Hour},
		{"", "gemini-5h", "Five Hour Limit", "5h", 5 * time.Hour},
		{"", "gemini-5h limit", "", "5h", 5 * time.Hour},
		{"", "gemini_session", "session", "5h", 5 * time.Hour},
		{"", "gemini-session-history", "", "5h", 5 * time.Hour},
		{"5-hour", "", "", "5h", 5 * time.Hour},
		{"", "gemini-5-hour", "", "5h", 5 * time.Hour},
		{"five hour limit", "", "", "5h", 5 * time.Hour},
		{"", "gemini-7-day", "", "7d", 7 * 24 * time.Hour},
		{"", "gemini-15h", "", "", 0},
		{"", "gemini-24h", "", "", 0},
		{"", "gemini-biweekly", "", "", 0},
		{"", "gemini-bi-weekly", "", "", 0},
		{"", "gemini-semi-weekly", "", "", 0},
		{"", "gemini-15-hour", "", "", 0},
		{"", "gemini-monthly", "Monthly Limit", "", 0},
		{"", "", "", "", 0},
	} {
		window, duration := antigravitySummaryWindow(tc.window, tc.id, tc.displayName)
		require.Equal(t, tc.expectedWindow, window, "id=%q window=%q", tc.id, tc.window)
		require.Equal(t, tc.expectedDuration, duration, "id=%q window=%q", tc.id, tc.window)
	}
}

// TestAntigravitySummaryDoesNotInventWindows excludes unknown, disabled and unmeasured buckets.
func TestAntigravitySummaryDoesNotInventWindows(t *testing.T) {
	body := `{"groups":[{"displayName":"Gemini Models","buckets":[
 {"bucketId":"gemini-weekly","remainingFraction":0},
 {"bucketId":"gemini-5h","disabled":true,"remainingFraction":1},
 {"bucketId":"gemini-mystery","remainingFraction":1},
 {"bucketId":"gemini-5h"},
 {"bucketId":"gemini-5h","remainingFraction":-0.1}]}]}`
	quota, err := parseAntigravityQuotaSummary([]byte(body))
	require.NoError(t, err)
	require.Len(t, quota.Limits, 1)
	require.Equal(t, "gemini_7d", quota.Limits[0].Window)
	require.Equal(t, "exhausted", quota.Status)
	_, err = parseAntigravityQuotaSummary([]byte(`{"groups":[{"displayName":"Gemini Models","buckets":[{"bucketId":"gemini-5h"}]}]}`))
	require.Error(t, err)
}

// TestAntigravityLegacyPoolLimits collapses effort variants conservatively without fabricating weekly data.
func TestAntigravityLegacyPoolLimits(t *testing.T) {
	body := `{"models":{
 "claude-opus-5-5-low":{"quotaInfo":{"remainingFraction":1}},
 "claude-opus-5-5-medium":{"quotaInfo":{"remainingFraction":0.4}},
 "claude-sonnet-5-5-high":{"quotaInfo":{"remainingFraction":0.7}},
 "gemini-3.8-flash-low":{"quotaInfo":{"remainingFraction":0.8}},
 "gemini-3.8-flash-high":{"quotaInfo":{"remainingFraction":0.3}}}}`
	quota, err := parseAntigravityQuota([]byte(body))
	require.NoError(t, err)
	require.Len(t, quota.Limits, 2)
	require.Equal(t, "gemini_5h", quota.Limits[0].Window)
	require.InDelta(t, 0.7, quota.Limits[0].UsageRatio, 1e-9)
	require.Equal(t, "claude_gpt_5h", quota.Limits[1].Window)
	require.InDelta(t, 0.6, quota.Limits[1].UsageRatio, 1e-9)
	require.Len(t, quota.RawData["models"], 5)
}

// TestAntigravitySummaryRequest prefers the summary and only falls back for unavailable summary APIs.
func TestAntigravitySummaryRequest(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		expectedCalls  int
		expectedLimits int
	}{
		{"summary", 200, 1, 4}, {"legacy", 404, 2, 1}, {"unauthorized", 403, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				require.Equal(t, "Bearer access", r.Header.Get("Authorization"))
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Equal(t, "project", body["project"])
				status, payload := tc.status, antigravitySummaryFixture
				if calls == 1 {
					require.Equal(t, antigravityQuotaSummaryURL, r.URL.String())
				} else {
					require.Equal(t, antigravityQuotaURL, r.URL.String())
					status, payload = 200, `{"models":{"gemini":{"quotaInfo":{"remainingFraction":0.5}}}}`
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload))}, nil
			})})
			quota, err := NewAntigravityQuotaChecker(client).CheckQuota(t.Context(), &ent.Channel{
				Type:        channel.TypeAntigravity,
				Credentials: objects.ChannelCredentials{APIKey: "refresh|project", OAuth: &objects.OAuthCredentials{AccessToken: "access", ExpiresAt: time.Now().Add(time.Hour)}},
			})
			if tc.status == 403 {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, quota.Limits, tc.expectedLimits)
			}
			require.Equal(t, tc.expectedCalls, calls)
		})
	}
}
