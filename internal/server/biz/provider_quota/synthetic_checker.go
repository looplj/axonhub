package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

type SyntheticUsageResponse struct {
	Subscription         *SyntheticSubscription         `json:"subscription,omitempty"`
	Search               *SyntheticSearch               `json:"search,omitempty"`
	FreeToolCalls        *SyntheticFreeToolCalls        `json:"freeToolCalls,omitempty"`
	WeeklyTokenLimit     *SyntheticWeeklyTokenLimit     `json:"weeklyTokenLimit,omitempty"`
	PlanWeeklyLimit      *SyntheticPlanWeeklyLimit      `json:"planWeeklyLimit,omitempty"`
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
	Limit    *int64  `json:"limit,omitempty"`
	Requests *int64  `json:"requests,omitempty"`
	RenewsAt *string `json:"renewsAt,omitempty"`
}

type SyntheticFreeToolCalls struct {
	Limit    *int64  `json:"limit,omitempty"`
	Requests *int64  `json:"requests,omitempty"`
	RenewsAt *string `json:"renewsAt,omitempty"`
}

type SyntheticWeeklyTokenLimit struct {
	NextRegenAt      *string  `json:"nextRegenAt,omitempty"`
	PercentRemaining *float64 `json:"percentRemaining,omitempty"`
	MaxCredits       *string  `json:"maxCredits,omitempty"`
	RemainingCredits *string  `json:"remainingCredits,omitempty"`
}

// SyntheticPlanWeeklyLimit is the weekly pool shape reported by big/pro
// new-version plans: a percentage and the next regeneration tick, with no
// credit amounts.
type SyntheticPlanWeeklyLimit struct {
	NextRegenAt      *string  `json:"nextRegenAt,omitempty"`
	PercentRemaining *float64 `json:"percentRemaining,omitempty"`
	NextRegenPercent *float64 `json:"nextRegenPercent,omitempty"`
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

	weekly := response.weeklyLimit()

	normalizedStatus := "unknown"
	limits := buildSyntheticLimitStatuses(weekly, response.RollingFiveHourLimit)

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

	// The search counter is displayed but never gates the channel: it is
	// appended after the blocking limits and stays out of the escalation above.
	if response.Search != nil {
		if searchLimit, ok := searchHourlyLimitStatus(response.Search.Hourly); ok {
			limits = append(limits, searchLimit)
		}
	}

	nextResetAt := findEarliestSyntheticResetAt(
		response.Subscription,
		weekly,
		response.RollingFiveHourLimit,
	)

	rawData := map[string]any{}

	if response.Subscription != nil {
		rawData["subscription"] = convertSyntheticSubscriptionToMap(response.Subscription)
	}

	if response.Search != nil {
		rawData["search"] = convertSyntheticSearchToMap(response.Search)
	}

	if response.FreeToolCalls != nil {
		rawData["freeToolCalls"] = convertSyntheticFreeToolCallsToMap(response.FreeToolCalls)
	}

	if response.WeeklyTokenLimit != nil {
		rawData["weeklyTokenLimit"] = convertSyntheticWeeklyTokenLimitToMap(response.WeeklyTokenLimit)
	}

	if response.PlanWeeklyLimit != nil {
		rawData["planWeeklyLimit"] = convertSyntheticPlanWeeklyLimitToMap(response.PlanWeeklyLimit)
	}

	if response.RollingFiveHourLimit != nil {
		rawData["rollingFiveHourLimit"] = convertSyntheticRollingFiveHourLimitToMap(response.RollingFiveHourLimit)
	}

	return NormalizeQuotaData(QuotaData{
		Status:       normalizedStatus,
		ProviderType: "synthetic",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}), nil
}

// weeklyLimit folds the two weekly pool shapes into the carrier the limit
// builder and the reset aggregation already use. Regular new-version plans
// report weeklyTokenLimit with credit amounts; big/pro plans report
// planWeeklyLimit with percentages only. When both are present planWeeklyLimit
// wins the shared fields and the credits still come from weeklyTokenLimit.
func (r *SyntheticUsageResponse) weeklyLimit() *SyntheticWeeklyTokenLimit {
	if r.WeeklyTokenLimit == nil && r.PlanWeeklyLimit == nil {
		return nil
	}

	merged := SyntheticWeeklyTokenLimit{}
	if r.WeeklyTokenLimit != nil {
		merged = *r.WeeklyTokenLimit
	}

	if plan := r.PlanWeeklyLimit; plan != nil {
		if plan.NextRegenAt != nil {
			merged.NextRegenAt = plan.NextRegenAt
		}
		if plan.PercentRemaining != nil {
			merged.PercentRemaining = plan.PercentRemaining
		}
	}

	return &merged
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
			if usageRatio >= WarningThresholdRatio {
				status = "warning"
			}
		}

		var resetAt *time.Time
		if fiveHour.NextTickAt != nil {
			if t, err := time.Parse(time.RFC3339, *fiveHour.NextTickAt); err == nil {
				resetAt = &t
			}
		}

		// The rolling window's NextTickAt is a regeneration tick, not the end of
		// a fixed window, so no period start can be derived from it.
		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       IsReadyStatus(status),
			NextResetAt: resetAt,
			Window:      QuotaWindow5h,
		})
	}

	if weekly != nil {
		limits = append(limits, weeklyTokenLimitStatus(weekly))
	}

	return limits
}

// syntheticSearchHourlyWindow labels Synthetic's hourly search-request quota.
// Like antigravity's gemini_5h windows it names the counter, not a bare period.
const syntheticSearchHourlyWindow = "search_hourly"

// searchHourlyLimitStatus converts the hourly search-request counter into a
// limit the quota surfaces list. It is never blocking: RequestModality never
// returns QuotaLimitTypeSearch, the limit carries no AvailabilityGroup, and
// normalization.go keeps display-only limits out of the channel status and
// reset. It carries no period money either — search is not billed in money and
// the plan's credits are reported on the weekly limit.
func searchHourlyLimitStatus(hourly *SyntheticSearchHourly) (QuotaLimitStatus, bool) {
	if hourly == nil || hourly.Limit == nil || hourly.Requests == nil || *hourly.Limit <= 0 {
		return QuotaLimitStatus{}, false
	}

	usageRatio := float64(*hourly.Requests) / float64(*hourly.Limit)

	status := "available"
	if usageRatio >= 1.0 {
		status = "exhausted"
	} else if usageRatio >= WarningThresholdRatio {
		status = "warning"
	}

	var resetAt *time.Time
	if hourly.RenewsAt != nil {
		if t, err := time.Parse(time.RFC3339, *hourly.RenewsAt); err == nil {
			resetAt = &t
		}
	}

	return QuotaLimitStatus{
		Type:        QuotaLimitTypeSearch,
		Status:      status,
		UsageRatio:  usageRatio,
		Ready:       IsReadyStatus(status),
		NextResetAt: resetAt,
		Window:      syntheticSearchHourlyWindow,
	}, true
}

func weeklyTokenLimitStatus(weekly *SyntheticWeeklyTokenLimit) QuotaLimitStatus {
	status := "available"
	usageRatio := 0.0

	if weekly.PercentRemaining != nil {
		usageRatio = 1.0 - (*weekly.PercentRemaining / 100.0)

		if usageRatio >= 1.0 {
			status = "exhausted"
		} else if usageRatio >= WarningThresholdRatio {
			status = "warning"
		}
	}

	var resetAt *time.Time

	if weekly.NextRegenAt != nil {
		if t, err := time.Parse(time.RFC3339, *weekly.NextRegenAt); err == nil {
			resetAt = &t
		}
	}

	// NextRegenAt marks the next incremental regeneration of the weekly pool
	// rather than a hard window boundary, so it cannot anchor a period start.
	limit := QuotaLimitStatus{
		Type:        QuotaLimitTypeToken,
		Status:      status,
		UsageRatio:  usageRatio,
		Ready:       IsReadyStatus(status),
		NextResetAt: resetAt,
		Window:      QuotaWindowWeekly,
	}

	// Synthetic bills this pool in its own credits and regenerates it in ticks
	// instead of resetting on a window boundary, so the provider's figures are
	// the only honest period money: maxCredits is what the window is worth,
	// maxCredits-remainingCredits what it has spent. provider_quota_cost.go
	// keeps these for "synthetic" instead of pricing the period from usage logs.
	if maxCredits, ok := parseSyntheticCredits(weekly.MaxCredits); ok && maxCredits > 0 {
		if remainingCredits, ok := parseSyntheticCredits(weekly.RemainingCredits); ok {
			// Two finite, individually valid amounts can still overflow the
			// subtraction (e.g. max 1e308 with remaining -1e308); the derived
			// money must stay finite or the quota payload stops being
			// JSON-serializable.
			usedCredits := maxCredits - remainingCredits
			if !math.IsInf(usedCredits, 0) {
				cost := max(usedCredits, 0.0)

				limit.PeriodCost = &cost
				limit.PeriodQuota = &maxCredits
			}
		}
	}

	return limit
}

// parseSyntheticCredits reads the "$10.50" amounts Synthetic reports for the
// weekly credit pool.
func parseSyntheticCredits(value *string) (float64, bool) {
	if value == nil {
		return 0, false
	}

	trimmed := strings.TrimPrefix(strings.TrimSpace(*value), "$")
	if trimmed == "" {
		return 0, false
	}

	credits, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || math.IsNaN(credits) || math.IsInf(credits, 0) {
		return 0, false
	}

	return credits, true
}

func findEarliestSyntheticResetAt(
	subscription *SyntheticSubscription,
	weeklyTokenLimit *SyntheticWeeklyTokenLimit,
	rollingFiveHourLimit *SyntheticRollingFiveHourLimit,
) *time.Time {
	// The search and free-tool-call counters renew hourly/daily, so reporting
	// them as the channel reset would mask the plan quota the status is driven
	// by. (nextCheckAt comes from the status, not from limit resets, so nothing
	// starts polling more often.)
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

	if hourly.RenewsAt != nil {
		result["renewsAt"] = *hourly.RenewsAt
	}

	return result
}

func convertSyntheticFreeToolCallsToMap(ftc *SyntheticFreeToolCalls) map[string]any {
	result := make(map[string]any)

	if ftc.Limit != nil {
		result["limit"] = *ftc.Limit
	}

	if ftc.Requests != nil {
		result["requests"] = *ftc.Requests
	}

	if ftc.RenewsAt != nil {
		result["renewsAt"] = *ftc.RenewsAt
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

func convertSyntheticPlanWeeklyLimitToMap(pwl *SyntheticPlanWeeklyLimit) map[string]any {
	result := make(map[string]any)

	if pwl.NextRegenAt != nil {
		result["nextRegenAt"] = *pwl.NextRegenAt
	}

	if pwl.PercentRemaining != nil {
		result["percentRemaining"] = *pwl.PercentRemaining
	}

	if pwl.NextRegenPercent != nil {
		result["nextRegenPercent"] = *pwl.NextRegenPercent
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
