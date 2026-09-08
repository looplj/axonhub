package provider_quota

import (
	"context"
	"fmt"
	"io"
	"math"
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

func quotaCheckerFixture(checker string) (QuotaData, error) {
	switch checker {
	case "claudecode":
		return NewClaudeCodeQuotaChecker(nil).parseResponse(http.Header{
			"Anthropic-Ratelimit-Unified-Status":               {"allowed"},
			"Anthropic-Ratelimit-Unified-Representative-Claim": {"five_hour"},
			"Anthropic-Ratelimit-Unified-5h-Utilization":       {"0.25"},
			"Anthropic-Ratelimit-Unified-5h-Reset":             {"4102444800"},
			"Anthropic-Ratelimit-Unified-5h-Status":            {"allowed"},
			"Anthropic-Ratelimit-Unified-7d-Utilization":       {"0.50"},
			"Anthropic-Ratelimit-Unified-7d-Reset":             {"4103049600"},
			"Anthropic-Ratelimit-Unified-7d-Status":            {"allowed"},
		})
	case "codex":
		return NewCodexQuotaChecker(nil).parseResponse([]byte(`{
			"rate_limit":{"primary_window":{"used_percent":25,"limit_window_seconds":18000,"reset_at":4102444800},"secondary_window":{"used_percent":50,"limit_window_seconds":604800,"reset_at":4103049600}}
		}`))
	case "antigravity":
		return parseAntigravityQuota([]byte(`{"models":{"gemini":{"quotaInfo":{"remainingFraction":0.75,"resetTime":"2099-01-01T00:00:00Z"}},"claude":{"quotaInfo":{"remainingFraction":0.5,"resetTime":"2099-01-02T00:00:00Z"}}}}`))
	case "xai_subscription":
		summary, err := subscription.ParseBillingResponses(
			[]byte(`{"config":{"currentPeriod":{"end":"2099-01-01T00:00:00Z"},"creditUsagePercent":25}}`),
			[]byte(`{"config":{"monthlyLimit":{"val":10000},"used":{"val":2500},"billingPeriodEnd":"2099-02-01T00:00:00Z"}}`),
		)
		if err != nil {
			return QuotaData{}, err
		}
		return xaiBillingQuotaData(summary), nil
	case "github_copilot":
		client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/copilot_internal/user" {
				return nil, fmt.Errorf("unexpected Copilot fixture URL %q", req.URL.String())
			}
			body := `{"copilot_plan":"individual","quota_reset_date_utc":"2099-02-01T00:00:00Z","limited_user_quotas":{"premium_interactions":8},"monthly_quotas":{"premium_interactions":10},"quota_snapshots":{"chat":{"percent_remaining":75}}}`
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})})
		return NewGithubCopilotQuotaChecker(client).CheckQuota(context.Background(), &ent.Channel{Credentials: objects.ChannelCredentials{APIKey: "fixture-token"}})
	case "nanogpt":
		return NewNanoGPTQuotaChecker(nil).parseResponse([]byte(`{"dailyImages":{"used":1,"remaining":9,"percentUsed":0.1,"resetAt":4102444800000},"dailyInputTokens":{"used":10,"remaining":90,"percentUsed":0.1,"resetAt":4102444800000},"weeklyInputTokens":{"used":20,"remaining":80,"percentUsed":0.2,"resetAt":4103049600000}}`))
	case "zenmux":
		return parseZenmuxQuotaResponse([]byte(`{"success":true,"data":{"account_status":"healthy","quota_5_hour":{"usage_percentage":0.25,"resets_at":"2099-01-01T00:00:00Z"},"quota_7_day":{"usage_percentage":0.5,"resets_at":"2099-01-08T00:00:00Z"},"quota_monthly":{"max_flows":1000,"max_value_usd":50}}}`))
	case "cline":
		client := httpclient.NewHttpClientWithClient(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body string
			switch req.URL.Path {
			case "/api/v1/users/me":
				body = `{"data":{"id":"fixture-user"}}`
			case "/api/v1/plans":
				body = `{"data":[{"type":"pro","interval":"month","isActive":true,"entitlements":{"cline_pass":{"enabled":true,"inferenceCapThreshold":{"last5HoursUsageCostUSDPerUser":100,"last7daysUsageCostUSDPerUser":100,"last30daysUsageCostUSDPerUser":100}}}}]}`
			case "/api/v1/users/fixture-user/balance":
				body = `{"data":{"balance":0}}`
			case "/api/v1/users/me/plan/usage-limits":
				body = `{"data":{"limits":[{"type":"five_hour","percentUsed":25,"resetsAt":"2099-01-01T05:00:00Z"},{"type":"weekly","percentUsed":25,"resetsAt":"2099-01-08T00:00:00Z"},{"type":"monthly","percentUsed":25,"resetsAt":"2099-02-01T00:00:00Z"}]}}`
			case "/api/v1/users/fixture-user/usages":
				body = `{"data":{"items":[]}}`
			default:
				return nil, fmt.Errorf("unexpected Cline fixture URL %q", req.URL.String())
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
		})})
		checker := NewClineQuotaChecker(client)
		checker.now = func() time.Time { return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) }
		return checker.CheckQuota(context.Background(), &ent.Channel{Type: channel.TypeCline, BaseURL: "https://api.cline.bot", Credentials: objects.ChannelCredentials{APIKey: "fixture-token"}})
	case "wafer":
		return NewWaferQuotaChecker(nil).parseResponse([]byte(`{"current_period_used_percent":25,"window_start":"2099-01-01T00:00:00Z","window_end":"2099-02-01T00:00:00Z","remaining_included_requests":75}`))
	case "synthetic":
		return NewSyntheticQuotaChecker(nil).parseResponse([]byte(`{"weeklyTokenLimit":{"percentRemaining":75,"nextRegenAt":"2099-01-08T00:00:00Z"},"rollingFiveHourLimit":{"tickPercent":25,"nextTickAt":"2099-01-01T05:00:00Z"}}`))
	case "neuralwatt":
		return NewNeuralWattQuotaChecker(nil).parseResponse([]byte(`{"subscription":{"kwh_included":100,"kwh_used":25,"kwh_remaining":75,"kwh_reset_date":"2099-02-01T00:00:00Z"}}`))
	case "apertis":
		return NewApertisQuotaChecker(nil).parseResponse([]byte(`{"is_subscriber":true,"subscription":{"status":"active","cycle_quota_limit":1000,"cycle_quota_used":250,"cycle_quota_remaining":750,"cycle_start":"2099-01-01T00:00:00Z","cycle_end":"2099-02-01T00:00:00Z"}}`))
	case "opencode_go":
		return (&OpenCodeGoQuotaChecker{now: func() time.Time { return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) }}).parseResponse([]byte(`{"usage":{"rolling":{"percent":25,"resetsAt":"2099-01-01T05:00:00Z"},"weekly":{"percent":50,"resetsAt":"2099-01-08T00:00:00Z"},"monthly":{"percent":10,"resetsAt":"2099-02-01T00:00:00Z"}}}`))
	case "kimi_code":
		return parseKimiCodeUsageResponse([]byte(`{"usage":{"used":25,"limit":100,"resetAt":"2099-01-08T00:00:00Z"},"limits":[{"window":"5h","used":10,"limit":100,"resetAt":"2099-01-01T05:00:00Z"}]}`))
	case "minimax":
		return parseMinimaxResponse([]byte(`{"base_resp":{"status_code":0},"model_remains":[{"model_name":"general","start_time":4070908800000,"end_time":4070926800000,"current_interval_status":1,"current_interval_remaining_percent":75,"current_interval_boost_permille":1000,"weekly_start_time":4070908800000,"weekly_end_time":4071513600000,"current_weekly_status":1,"current_weekly_remaining_percent":50,"weekly_boost_permille":1000}]}`))
	case "zhipu":
		return parseZhipuQuotaResponse([]byte(`{"success":true,"data":{"limits":[{"type":"TOKENS_LIMIT","percentage":25,"nextResetTime":4102444800000},{"type":"TOKENS_LIMIT","percentage":50,"nextResetTime":4103049600000}]}}`))
	case "zai":
		return parseZaiQuotaResponse([]byte(`{"success":true,"data":{"limits":[{"type":"TOKENS_LIMIT","percentage":25,"nextResetTime":4102444800000},{"type":"TOKENS_LIMIT","percentage":50,"nextResetTime":4103049600000}]}}`))
	case "charm_hyper":
		return NewCharmHyperQuotaChecker(nil).parseResponse([]byte(`{"balance":75}`))
	case "commandcode":
		return parseCommandCodeCredits([]byte(`{"windowLimits":{"five_hour":{"usage_percent":25,"cap_usd":10,"resetAt":"2099-01-01T05:00:00Z"},"weekly":{"usage_percent":50,"cap_usd":20,"resetAt":"2099-01-08T00:00:00Z"}}}`), nil)
	default:
		return QuotaData{}, fmt.Errorf("unknown quota checker fixture %q", checker)
	}
}

func assertNormalizedLimitContract(t *testing.T, quota QuotaData) {
	t.Helper()
	identities := make(map[string]struct{}, len(quota.Limits))
	for _, limit := range quota.Limits {
		require.NotEmpty(t, limit.Type)
		require.NotEmpty(t, limit.Window)
		require.False(t, math.IsNaN(limit.UsageRatio))
		require.False(t, math.IsInf(limit.UsageRatio, 0))
		require.GreaterOrEqual(t, limit.UsageRatio, 0.0)
		require.LessOrEqual(t, limit.UsageRatio, 1.0)
		require.Contains(t, []string{"available", "warning", "exhausted", "unknown"}, limit.Status)
		require.Equal(t, IsReadyStatus(limit.Status), limit.Ready)
		identity := string(limit.Type) + "\x00" + limit.Window
		require.NotContains(t, identities, identity)
		identities[identity] = struct{}{}
		if limit.PeriodStart != nil {
			require.NotNil(t, limit.NextResetAt)
			require.True(t, limit.PeriodStart.Before(*limit.NextResetAt))
		}
	}
	require.Equal(t, IsReadyStatus(quota.Status), quota.Ready)
}

func normalizedLimitIdentities(quota QuotaData) []string {
	identities := make([]string, 0, len(quota.Limits))
	for _, limit := range quota.Limits {
		identities = append(identities, string(limit.Type)+"/"+limit.Window)
	}
	return identities
}
