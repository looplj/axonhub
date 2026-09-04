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
	"github.com/looplj/axonhub/llm/transformer/xai/subscription"
)

func TestXAISubscriptionQuotaChecker_CheckQuota_merges_billing_windows(t *testing.T) {
	// Given
	checker := NewXAISubscriptionQuotaChecker(newXAIBillingTestClient(t, "access-token",
		`{"config":{"currentPeriod":{"type":"WEEKLY","end":"2026-07-16T03:25:00Z"},"creditUsagePercent":82.5}}`,
		`{"config":{"monthlyLimit":{"val":15000},"used":{"val":7500},"billingPeriodEnd":"2026-08-01T00:00:00Z"}}`,
	))
	ch := &ent.Channel{
		Type:    channel.TypeXaiSubscription,
		BaseURL: subscription.DefaultBaseURL,
		Credentials: objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
			AccessToken: "access-token",
		}},
	}

	// When
	result, err := checker.CheckQuota(t.Context(), ch)

	// Then
	require.NoError(t, err)
	require.Equal(t, "warning", result.Status)
	require.Equal(t, "xai_subscription", result.ProviderType)
	require.True(t, result.Ready)
	require.Len(t, result.Limits, 2)
	require.Equal(t, "warning", result.Limits[0].Status)
	require.Equal(t, QuotaWindowWeekly, result.Limits[0].Window)
	require.InDelta(t, 0.825, result.Limits[0].UsageRatio, 1e-9)
	require.Equal(t, time.Date(2026, 7, 16, 3, 25, 0, 0, time.UTC), *result.Limits[0].NextResetAt)
	require.Equal(t, "available", result.Limits[1].Status)
	require.Equal(t, QuotaWindowMonthly, result.Limits[1].Window)
	require.InDelta(t, 0.5, result.Limits[1].UsageRatio, 1e-9)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), *result.Limits[1].NextResetAt)
	require.Equal(t, time.Date(2026, 7, 16, 3, 25, 0, 0, time.UTC), *result.NextResetAt)
	require.Equal(t, "SuperGrok", result.RawData["plan_type"])
	require.NotNil(t, result.RawData["billing"])
}

func TestXAISubscriptionQuotaChecker_CheckQuota_accepts_legacy_oauth_json(t *testing.T) {
	// Given
	checker := NewXAISubscriptionQuotaChecker(newXAIBillingTestClient(t, "legacy-access-token",
		`{"config":{"creditUsagePercent":10}}`,
		`{"config":{"monthlyLimit":{"val":15000},"used":{"val":1500}}}`,
	))
	ch := &ent.Channel{
		Type:    channel.TypeXaiSubscription,
		BaseURL: subscription.DefaultBaseURL,
		Credentials: objects.ChannelCredentials{APIKey: `{
			"access_token":"legacy-access-token",
			"refresh_token":"legacy-refresh-token",
			"client_id":"synthetic-client"
		}`},
	}

	// When
	result, err := checker.CheckQuota(t.Context(), ch)

	// Then
	require.NoError(t, err)
	require.Equal(t, "available", result.Status)
}

func TestXAISubscriptionQuotaChecker_CheckQuota_prefers_JWT_tier(t *testing.T) {
	// Given
	const heavyAccessToken = "header.eyJ0aWVyIjo1fQ.signature"
	checker := NewXAISubscriptionQuotaChecker(newXAIBillingTestClient(t, heavyAccessToken,
		`{"config":{"creditUsagePercent":10}}`,
		`{"config":{"monthlyLimit":{"val":15000},"used":{"val":1500}}}`,
	))
	ch := &ent.Channel{
		Type: channel.TypeXaiSubscription,
		Credentials: objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
			AccessToken: heavyAccessToken,
		}},
	}

	// When
	result, err := checker.CheckQuota(t.Context(), ch)

	// Then
	require.NoError(t, err)
	require.Equal(t, "SuperGrok Heavy", result.RawData["plan_type"])
}

func TestXAISubscriptionQuotaChecker_SupportsChannel_only_subscription_type(t *testing.T) {
	// Given
	checker := NewXAISubscriptionQuotaChecker(httpclient.NewHttpClient())

	// When / Then
	require.True(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeXaiSubscription}))
	require.False(t, checker.SupportsChannel(&ent.Channel{Type: channel.TypeXai}))
}

func TestXAISubscriptionQuotaData_does_not_invent_missing_windows(t *testing.T) {
	// Given
	summary := subscription.BillingSummary{}

	// When
	result := xaiBillingQuotaData(summary)

	// Then
	require.Equal(t, "unknown", result.Status)
	require.False(t, result.Ready)
	require.Empty(t, result.Limits)
}

func TestXAISubscriptionQuotaData_labels_monthly_only_window(t *testing.T) {
	result := xaiBillingQuotaData(subscription.BillingSummary{
		Monthly: &subscription.BillingWindow{UsagePercent: 50},
	})

	require.Len(t, result.Limits, 1)
	require.Equal(t, QuotaWindowMonthly, result.Limits[0].Window)
}

func newXAIBillingTestClient(t *testing.T, accessToken, weeklyBody, monthlyBody string) *httpclient.HttpClient {
	t.Helper()
	return httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer "+accessToken, request.Header.Get("Authorization"))
		require.Equal(t, subscription.CLITokenAuth, request.Header.Get(subscription.CLITokenAuthHeader))
		require.Equal(t, subscription.CLIClientVersion, request.Header.Get(subscription.CLIClientVersionHeader))

		body := monthlyBody
		if request.URL.String() == subscription.BillingWeeklyURL {
			body = weeklyBody
		} else {
			require.Equal(t, subscription.BillingMonthlyURL, request.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})})
}
