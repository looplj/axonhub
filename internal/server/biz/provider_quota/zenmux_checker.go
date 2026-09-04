package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const zenmuxSubscriptionDetailPath = "/api/v1/management/subscription/detail"

type zenmuxSubscriptionResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message,omitempty"`
	Data    *zenmuxSubscriptionData `json:"data,omitempty"`
}

type zenmuxSubscriptionData struct {
	Plan          zenmuxPlan         `json:"plan"`
	AccountStatus string             `json:"account_status"`
	Quota5Hour    zenmuxQuotaWindow  `json:"quota_5_hour"`
	Quota7Day     zenmuxQuotaWindow  `json:"quota_7_day"`
	QuotaMonthly  zenmuxMonthlyQuota `json:"quota_monthly"`
}

type zenmuxPlan struct {
	Tier      string  `json:"tier"`
	AmountUSD float64 `json:"amount_usd"`
	Interval  string  `json:"interval"`
	ExpiresAt string  `json:"expires_at"`
}

type zenmuxQuotaWindow struct {
	UsagePercentage float64 `json:"usage_percentage"`
	ResetsAt        *string `json:"resets_at"`
	MaxFlows        float64 `json:"max_flows"`
	UsedFlows       float64 `json:"used_flows"`
	RemainingFlows  float64 `json:"remaining_flows"`
	UsedValueUSD    float64 `json:"used_value_usd"`
	MaxValueUSD     float64 `json:"max_value_usd"`
}

type zenmuxMonthlyQuota struct {
	MaxFlows    float64 `json:"max_flows"`
	MaxValueUSD float64 `json:"max_value_usd"`
}

type ZenmuxQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewZenmuxQuotaChecker(httpClient *httpclient.HttpClient) *ZenmuxQuotaChecker {
	return &ZenmuxQuotaChecker{httpClient: httpClient}
}

func (c *ZenmuxQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	managementKey := strings.TrimSpace(ch.Credentials.ManagementAPIKey)
	if managementKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no management API key")
	}

	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(ZenmuxDefaultBaseURL + zenmuxSubscriptionDetailPath).
		WithBearerToken(managementKey).
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}
	hc = hc.WithRejectHTTPSDowngrade()

	response, err := hc.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("zenmux quota request failed: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return QuotaData{}, fmt.Errorf("zenmux quota request returned HTTP %d", response.StatusCode)
	}

	return parseZenmuxQuotaResponse(response.Body)
}

func (c *ZenmuxQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	switch ch.Type {
	case channel.TypeZenmux,
		channel.TypeZenmuxResponses,
		channel.TypeZenmuxAnthropic,
		channel.TypeZenmuxGemini:
		return true
	default:
		return false
	}
}

func parseZenmuxQuotaResponse(body []byte) (QuotaData, error) {
	var response zenmuxSubscriptionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse zenmux quota response: %w", err)
	}
	if !response.Success {
		if response.Message != "" {
			return QuotaData{}, fmt.Errorf("zenmux API error: %s", response.Message)
		}
		return QuotaData{}, fmt.Errorf("zenmux API returned success=false")
	}
	if response.Data == nil {
		return QuotaData{}, fmt.Errorf("zenmux quota response contains no data")
	}

	fiveHourReset, err := parseZenmuxResetAt(response.Data.Quota5Hour.ResetsAt)
	if err != nil {
		return QuotaData{}, fmt.Errorf("parse zenmux 5-hour reset: %w", err)
	}
	sevenDayReset, err := parseZenmuxResetAt(response.Data.Quota7Day.ResetsAt)
	if err != nil {
		return QuotaData{}, fmt.Errorf("parse zenmux 7-day reset: %w", err)
	}

	fiveHourStatus := zenmuxWindowStatus(response.Data.Quota5Hour.UsagePercentage)
	sevenDayStatus := zenmuxWindowStatus(response.Data.Quota7Day.UsagePercentage)
	overallStatus := worstZenmuxStatus(
		zenmuxAccountStatus(response.Data.AccountStatus),
		fiveHourStatus,
		sevenDayStatus,
	)

	nextResetAt := fiveHourReset
	if sevenDayReset != nil && (nextResetAt == nil || sevenDayReset.Before(*nextResetAt)) {
		nextResetAt = sevenDayReset
	}

	return QuotaData{
		Status:       overallStatus,
		ProviderType: "zenmux",
		RawData: map[string]any{
			"plan":           response.Data.Plan,
			"quota_monthly":  response.Data.QuotaMonthly,
			"account_status": response.Data.AccountStatus,
		},
		NextResetAt: nextResetAt,
		Ready:       IsReadyStatus(overallStatus),
		Limits: []QuotaLimitStatus{
			NewTokenLimitStatus(fiveHourStatus, response.Data.Quota5Hour.UsagePercentage, fiveHourReset).
				WithWindow(QuotaWindow5h, 5*time.Hour),
			NewTokenLimitStatus(sevenDayStatus, response.Data.Quota7Day.UsagePercentage, sevenDayReset).
				WithWindow(QuotaWindow7d, 7*24*time.Hour),
		},
	}, nil
}

func parseZenmuxResetAt(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return nil, fmt.Errorf("invalid reset timestamp %q: %w", *value, err)
	}
	return &parsed, nil
}

func zenmuxWindowStatus(usageRatio float64) string {
	if usageRatio >= 1 {
		return "exhausted"
	}
	if usageRatio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

func zenmuxAccountStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "healthy", "monitored":
		return "available"
	case "abusive":
		return "warning"
	case "suspended", "banned":
		return "exhausted"
	default:
		return "unknown"
	}
}

func worstZenmuxStatus(statuses ...string) string {
	worst := "available"
	for _, status := range statuses {
		if zenmuxStatusRank(status) > zenmuxStatusRank(worst) {
			worst = status
		}
	}
	return worst
}

func zenmuxStatusRank(status string) int {
	switch status {
	case "available":
		return 0
	case "warning":
		return 1
	case "exhausted":
		return 2
	default:
		return 3
	}
}
