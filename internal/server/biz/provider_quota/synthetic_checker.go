package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

type SyntheticUsageResponse struct {
	Subscription         *SyntheticSubscription         `json:"subscription,omitempty"`
	Search               *SyntheticSearch               `json:"search,omitempty"`
	WeeklyTokenLimit     *SyntheticWeeklyTokenLimit     `json:"weeklyTokenLimit,omitempty"`
	RollingFiveHourLimit *SyntheticRollingFiveHourLimit `json:"rollingFiveHourLimit,omitempty"`
}

type SyntheticSubscription struct {
	Limit    *int64  `json:"limit,omitempty"`
	Requests *int64  `json:"requests,omitempty"`
	RenewsAt *string `json:"renewsAt,omitempty"`
}

type SyntheticSearch struct {
	Hourly *SyntheticSearchHourly `json:"hourly,omitempty"`
}

type SyntheticSearchHourly struct {
	Limit    *int64 `json:"limit,omitempty"`
	Requests *int64 `json:"requests,omitempty"`
}

type SyntheticWeeklyTokenLimit struct {
	NextRegenAt      *string  `json:"nextRegenAt,omitempty"`
	PercentRemaining *float64 `json:"percentRemaining,omitempty"`
	MaxCredits       *string  `json:"maxCredits,omitempty"`
	RemainingCredits *string  `json:"remainingCredits,omitempty"`
}

type SyntheticRollingFiveHourLimit struct {
	NextTickAt  *string  `json:"nextTickAt,omitempty"`
	TickPercent *float64 `json:"tickPercent,omitempty"`
	Remaining   *float64 `json:"remaining,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Limited     *bool    `json:"limited,omitempty"`
}

type SyntheticQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewSyntheticQuotaChecker(httpClient *httpclient.HttpClient) *SyntheticQuotaChecker {
	return &SyntheticQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *SyntheticQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = ch.Credentials.APIKeys[0]
	}

	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	quotaURL := buildSyntheticQuotaURL(ch.BaseURL)

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{}, fmt.Errorf("quota request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return QuotaData{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(resp.Body))
	}

	return c.parseResponse(resp.Body)
}

func (c *SyntheticQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var response SyntheticUsageResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse synthetic usage response: %w", err)
	}

	normalizedStatus := "unknown"
	limits := buildSyntheticLimitStatuses(response.WeeklyTokenLimit, response.RollingFiveHourLimit)

	if len(limits) > 0 {
		normalizedStatus = "available"
		for i := range limits {
			if limits[i].Status == "exhausted" {
				normalizedStatus = "exhausted"
				break
			}
			if limits[i].Status == "warning" {
				normalizedStatus = "warning"
			}
		}
	}

	nextResetAt := findEarliestSyntheticResetAt(
		response.Subscription,
		response.WeeklyTokenLimit,
		response.RollingFiveHourLimit,
	)

	rawData := map[string]any{}

	if response.Subscription != nil {
		rawData["subscription"] = convertSyntheticSubscriptionToMap(response.Subscription)
	}

	if response.Search != nil {
		rawData["search"] = convertSyntheticSearchToMap(response.Search)
	}

	if response.WeeklyTokenLimit != nil {
		rawData["weeklyTokenLimit"] = convertSyntheticWeeklyTokenLimitToMap(response.WeeklyTokenLimit)
	}

	if response.RollingFiveHourLimit != nil {
		rawData["rollingFiveHourLimit"] = convertSyntheticRollingFiveHourLimitToMap(response.RollingFiveHourLimit)
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "synthetic",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}, nil
}

func (c *SyntheticQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	if ch.Type != channel.TypeOpenai && ch.Type != channel.TypeOpenaiResponses {
		return false
	}

	return DetectProviderFromURL(ch.BaseURL) == "synthetic"
}

func buildSyntheticQuotaURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://api.synthetic.new/v2/quotas"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme == "" && parsed.Host == "") {
		return "https://api.synthetic.new/v2/quotas"
	}

	scheme := parsed.Scheme
	if scheme == "http" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/v2/quotas", scheme, parsed.Host)
}

func buildSyntheticLimitStatuses(weekly *SyntheticWeeklyTokenLimit, fiveHour *SyntheticRollingFiveHourLimit) []QuotaLimitStatus {
	var limits []QuotaLimitStatus

	if fiveHour != nil {
		status := "available"
		usageRatio := 0.0

		if fiveHour.Limited != nil && *fiveHour.Limited {
			status = "exhausted"
			usageRatio = 1.0
		} else if fiveHour.TickPercent != nil {
			usageRatio = *fiveHour.TickPercent
			if usageRatio > WarningThresholdRatio {
				status = "warning"
			}
		}

		var resetAt *time.Time
		if fiveHour.NextTickAt != nil {
			if t, err := time.Parse(time.RFC3339, *fiveHour.NextTickAt); err == nil {
				resetAt = &t
			}
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       status != "exhausted",
			NextResetAt: resetAt,
		})
	}

	if weekly != nil {
		limits = append(limits, weeklyTokenLimitStatus(weekly))
	}

	return limits
}

func weeklyTokenLimitStatus(weekly *SyntheticWeeklyTokenLimit) QuotaLimitStatus {
	status := "available"
	usageRatio := 0.0

	if weekly.PercentRemaining != nil {
		usageRatio = 1.0 - (*weekly.PercentRemaining / 100.0)
		if usageRatio > WarningThresholdRatio {
			status = "warning"
		}
	}

	var resetAt *time.Time

	if weekly.NextRegenAt != nil {
		if t, err := time.Parse(time.RFC3339, *weekly.NextRegenAt); err == nil {
			resetAt = &t
		}
	}

	return QuotaLimitStatus{
		Type:        QuotaLimitTypeToken,
		Status:      status,
		UsageRatio:  usageRatio,
		Ready:       true,
		NextResetAt: resetAt,
	}
}

func findEarliestSyntheticResetAt(
	subscription *SyntheticSubscription,
	weeklyTokenLimit *SyntheticWeeklyTokenLimit,
	rollingFiveHourLimit *SyntheticRollingFiveHourLimit,
) *time.Time {
	var earliest *time.Time

	if subscription != nil && subscription.RenewsAt != nil {
		if t, err := time.Parse(time.RFC3339, *subscription.RenewsAt); err == nil {
			earliest = &t
		}
	}

	if weeklyTokenLimit != nil && weeklyTokenLimit.NextRegenAt != nil {
		if t, err := time.Parse(time.RFC3339, *weeklyTokenLimit.NextRegenAt); err == nil {
			if earliest == nil || t.Before(*earliest) {
				earliest = &t
			}
		}
	}

	if rollingFiveHourLimit != nil && rollingFiveHourLimit.NextTickAt != nil {
		if t, err := time.Parse(time.RFC3339, *rollingFiveHourLimit.NextTickAt); err == nil {
			if earliest == nil || t.Before(*earliest) {
				earliest = &t
			}
		}
	}

	return earliest
}

func convertSyntheticSubscriptionToMap(sub *SyntheticSubscription) map[string]any {
	result := make(map[string]any)

	if sub.Limit != nil {
		result["limit"] = *sub.Limit
	}

	if sub.Requests != nil {
		result["requests"] = *sub.Requests
	}

	if sub.RenewsAt != nil {
		result["renewsAt"] = *sub.RenewsAt
	}

	return result
}

func convertSyntheticSearchToMap(search *SyntheticSearch) map[string]any {
	result := make(map[string]any)

	if search.Hourly != nil {
		result["hourly"] = convertSyntheticSearchHourlyToMap(search.Hourly)
	}

	return result
}

func convertSyntheticSearchHourlyToMap(hourly *SyntheticSearchHourly) map[string]any {
	result := make(map[string]any)

	if hourly.Limit != nil {
		result["limit"] = *hourly.Limit
	}

	if hourly.Requests != nil {
		result["requests"] = *hourly.Requests
	}

	return result
}

func convertSyntheticWeeklyTokenLimitToMap(wtl *SyntheticWeeklyTokenLimit) map[string]any {
	result := make(map[string]any)

	if wtl.NextRegenAt != nil {
		result["nextRegenAt"] = *wtl.NextRegenAt
	}

	if wtl.PercentRemaining != nil {
		result["percentRemaining"] = *wtl.PercentRemaining
	}

	if wtl.MaxCredits != nil {
		result["maxCredits"] = *wtl.MaxCredits
	}

	if wtl.RemainingCredits != nil {
		result["remainingCredits"] = *wtl.RemainingCredits
	}

	return result
}

func convertSyntheticRollingFiveHourLimitToMap(rfhl *SyntheticRollingFiveHourLimit) map[string]any {
	result := make(map[string]any)

	if rfhl.NextTickAt != nil {
		result["nextTickAt"] = *rfhl.NextTickAt
	}

	if rfhl.TickPercent != nil {
		result["tickPercent"] = *rfhl.TickPercent
	}

	if rfhl.Remaining != nil {
		result["remaining"] = *rfhl.Remaining
	}

	if rfhl.Max != nil {
		result["max"] = *rfhl.Max
	}

	if rfhl.Limited != nil {
		result["limited"] = *rfhl.Limited
	}

	return result
}
