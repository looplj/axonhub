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

// betterStatusRank maps status strings to their priority ranks.
var betterStatusRank = map[string]int{
	"available": 3,
	"warning":   2,
	"exhausted": 1,
	"unknown":   0,
}

// ApertisBillingCreditsResponse represents the response from the Apertis billing credits API.
// The API returns PAYG credit balances and subscription quota details.
type ApertisBillingCreditsResponse struct {
	Object       string               `json:"object"`
	IsSubscriber bool                 `json:"is_subscriber"`
	Payg         *ApertisPayg         `json:"payg"`
	Subscription *ApertisSubscription `json:"subscription,omitempty"`
}

// ApertisPayg represents PAYG (Pay-As-You-Go) credit information.
// All token_* fields are in USD. token_total and token_remaining can be the
// string "unlimited" when token_is_unlimited is true.
type ApertisPayg struct {
	AccountCredits       float64  `json:"account_credits"`
	TokenUsed            float64  `json:"token_used"`
	TokenTotal           any      `json:"token_total"`     // Can be float64 or string "unlimited"
	TokenRemaining       any      `json:"token_remaining"` // Can be float64 or string "unlimited"
	TokenIsUnlimited     bool     `json:"token_is_unlimited"`
	TokenMonthlyLimitUSD *float64 `json:"token_monthly_limit_usd,omitempty"` // Monthly spending limit for this token in USD
	TokenMonthlyUsedUSD  *float64 `json:"token_monthly_used_usd,omitempty"`  // Spending by this token in current month in USD
	MonthlyResetDay      *int     `json:"monthly_reset_day,omitempty"`       // Day of month when the monthly counter resets
}

// ApertisSubscription represents subscription quota information.
type ApertisSubscription struct {
	PlanType            string   `json:"plan_type"`
	Status              string   `json:"status"`
	CycleQuotaLimit     int      `json:"cycle_quota_limit"`
	CycleQuotaUsed      int      `json:"cycle_quota_used"`
	CycleQuotaRemaining int      `json:"cycle_quota_remaining"`
	CycleStart          string   `json:"cycle_start"`
	CycleEnd            string   `json:"cycle_end"`
	PaygFallbackEnabled bool     `json:"payg_fallback_enabled"`
	PaygSpentUSD        *float64 `json:"payg_spent_usd,omitempty"`
	PaygLimitUSD        *float64 `json:"payg_limit_usd,omitempty"`
}

// ApertisQuotaChecker checks quota status for Apertis provider.
type ApertisQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

// NewApertisQuotaChecker creates a new Apertis quota checker.
func NewApertisQuotaChecker(httpClient *httpclient.HttpClient) *ApertisQuotaChecker {
	return &ApertisQuotaChecker{
		httpClient: httpClient,
	}
}

// CheckQuota makes a request to the Apertis billing credits endpoint and returns normalized quota data.
func (c *ApertisQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = ch.Credentials.APIKeys[0]
	}
	if apiKey == "" {
		return QuotaData{
			Status:       "unknown",
			ProviderType: "apertis",
			Ready:        false,
			RawData:      map[string]any{"error": "missing API key"},
		}, nil
	}

	quotaURL := buildApertisQuotaURL(ch.BaseURL)

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	resp, err := c.httpClient.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{
			Status:       "unknown",
			ProviderType: "apertis",
			Ready:        false,
			RawData:      map[string]any{"error": fmt.Sprintf("request failed: %v", err)},
		}, err
	}

	if resp.StatusCode != http.StatusOK {
		return QuotaData{
			Status:       "unknown",
			ProviderType: "apertis",
			Ready:        false,
			RawData:      map[string]any{"error": fmt.Sprintf("HTTP %d", resp.StatusCode)},
		}, fmt.Errorf("apertis API returned status %d", resp.StatusCode)
	}

	return c.parseResponse(resp.Body)
}

// parseResponse parses the Apertis billing credits response and returns normalized QuotaData.
func (c *ApertisQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var resp ApertisBillingCreditsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return QuotaData{
			Status:       "unknown",
			ProviderType: "apertis",
			Ready:        false,
			RawData:      map[string]any{"error": fmt.Sprintf("failed to parse JSON: %v", err)},
		}, err
	}

	quotaData := QuotaData{
		ProviderType: "apertis",
		RawData:      convertApertisResponseToMap(resp),
	}

	// Determine status
	status := determineApertisStatus(&resp)
	quotaData.Status = status
	quotaData.Ready = IsReadyStatus(status)

	// Determine reset time
	var nextResetAt *time.Time
	if resp.IsSubscriber && resp.Subscription != nil && resp.Subscription.CycleEnd != "" {
		if resetTime, err := time.Parse(time.RFC3339, resp.Subscription.CycleEnd); err == nil {
			nextResetAt = &resetTime
			quotaData.NextResetAt = nextResetAt
		}
	}

	// Build limits
	quotaData.Limits = buildApertisLimits(&resp, nextResetAt)

	return quotaData, nil
}

// SupportsChannel returns true if the channel is OpenAI-compatible (used by Apertis).
func (c *ApertisQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeOpenai || ch.Type == channel.TypeOpenaiResponses
}

// buildApertisQuotaURL builds the URL for the Apertis billing credits endpoint.
func buildApertisQuotaURL(baseURL string) string {
	schemeHost := strings.TrimSpace(baseURL)
	if schemeHost == "" {
		schemeHost = ApertisDefaultBaseURL
	}

	parsed, err := url.Parse(schemeHost)
	if err != nil {
		return ApertisDefaultBaseURL + "/v1/dashboard/billing/credits"
	}

	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/v1/dashboard/billing/credits", scheme, parsed.Host)
}

// determineApertisStatus determines the overall quota status based on the response.
//
// Priority order for status determination:
//  1. If subscription is active with remaining cycle quota → check if high usage (warning)
//  2. If subscription cycle quota is exhausted BUT PAYG fallback is enabled with credits → available/warning
//  3. If subscription is suspended/canceled → fall through to PAYG check
//  4. If PAYG account credits > 0 → available/warning based on token usage
//  5. Otherwise → exhausted
func determineApertisStatus(resp *ApertisBillingCreditsResponse) string {
	bestStatus := "exhausted"

	// --- Subscription path ---
	if resp.Subscription != nil {
		subStatus := determineSubscriptionStatus(resp.Subscription)
		bestStatus = betterStatus(bestStatus, subStatus)
	}

	// --- PAYG path (always checked) ---
	// PAYG is the fallback whenever subscription quota is unavailable.
	// For subscribers, PAYG is only reachable when payg_fallback_enabled is true
	// or when the subscription is suspended/canceled (no cycle quota left).
	// For non-subscribers, PAYG is the only path.
	if resp.Payg != nil {
		shouldCheckPAYG := !resp.IsSubscriber
		if resp.Subscription != nil {
			// PAYG is available when fallback is enabled, or when
			// the subscription itself is exhausted (suspended/canceled/cycle used up)
			if resp.Subscription.PaygFallbackEnabled {
				shouldCheckPAYG = true
			}
			if strings.EqualFold(resp.Subscription.Status, "suspended") || strings.EqualFold(resp.Subscription.Status, "cancelled") { //nolint:misspell // API domain value
				shouldCheckPAYG = true
			}
			if resp.Subscription.CycleQuotaRemaining <= 0 {
				shouldCheckPAYG = true
			}
		}
		if shouldCheckPAYG {
			paygStatus := determinePaygStatus(resp.Payg)
			bestStatus = betterStatus(bestStatus, paygStatus)
		}
	}

	return bestStatus
}

// determineSubscriptionStatus returns the status based on subscription state alone.
func determineSubscriptionStatus(sub *ApertisSubscription) string {
	if strings.EqualFold(sub.Status, "suspended") || strings.EqualFold(sub.Status, "cancelled") { //nolint:misspell // API domain value
		return "exhausted"
	}

	if sub.CycleQuotaLimit > 0 {
		usageRatio := float64(sub.CycleQuotaUsed) / float64(sub.CycleQuotaLimit)
		if sub.CycleQuotaRemaining <= 0 {
			return "exhausted"
		}
		if usageRatio > WarningThresholdRatio {
			return "warning"
		}
	}

	return "available"
}

// determinePaygStatus returns the status based on PAYG credits.
func determinePaygStatus(payg *ApertisPayg) string {
	// Unlimited token is always available regardless of account_credits.
	if payg.TokenIsUnlimited {
		return "available"
	}

	if payg.AccountCredits <= 0 {
		return "exhausted"
	}

	// Check token-level usage ratio for warning
	if total, ok := toFloat64(payg.TokenTotal); ok && total > 0 {
		usageRatio := payg.TokenUsed / total
		if usageRatio > WarningThresholdRatio {
			return "warning"
		}
	}

	return "available"
}

// betterStatus returns the more permissive of two statuses.
// Order: available > warning > exhausted > unknown.
func betterStatus(a, b string) string {
	if betterStatusRank[b] > betterStatusRank[a] {
		return b
	}
	return a
}

func buildApertisLimits(resp *ApertisBillingCreditsResponse, nextResetAt *time.Time) []QuotaLimitStatus {
	var limits []QuotaLimitStatus
	// PAYG token limit — skip only when an active subscription exists with
	// remaining quota and no PAYG fallback is needed. When the subscription
	// cycle is exhausted or PAYG fallback is enabled, PAYG serves as the
	// active funding source and should be included.
	if resp.Payg != nil {
		// Skip PAYG limit only when subscription is active with remaining quota
		// and PAYG fallback is not enabled. Match the same conditions used in
		// determineApertisStatus for when PAYG should be considered.
		shouldSkipPAYG := resp.IsSubscriber &&
			resp.Subscription != nil &&
			!resp.Subscription.PaygFallbackEnabled &&
			resp.Subscription.Status == "active" &&
			resp.Subscription.CycleQuotaRemaining > 0
		if !shouldSkipPAYG {
			var tokenStatus string
			var usageRatio float64
			if resp.Payg.TokenIsUnlimited {
				tokenStatus = "available"
				usageRatio = 0
			} else {
				total, ok := toFloat64(resp.Payg.TokenTotal)
				if ok && total > 0 {
					usageRatio = resp.Payg.TokenUsed / total
					if resp.Payg.TokenUsed >= total {
						tokenStatus = "exhausted"
					} else if usageRatio > WarningThresholdRatio {
						tokenStatus = "warning"
					} else {
						tokenStatus = "available"
					}
				} else {
					// Can't determine usage ratio
					tokenStatus = "unknown"
				}
			}
			limits = append(limits, QuotaLimitStatus{
				Type:        QuotaLimitTypeToken,
				Status:      tokenStatus,
				UsageRatio:  usageRatio,
				Ready:       IsReadyStatus(tokenStatus),
				NextResetAt: nextResetAt,
			})
		}
	}

	// Subscription cycle limit (if subscriber).
	// Uses a distinct QuotaLimitTypeSubscriptionCycle so that EffectiveStatus
	// does not merge subscription-cycle and PAYG-token limits under
	// OR-semantics (available if EITHER source has quota).
	if resp.IsSubscriber && resp.Subscription != nil {
		var subStatus string
		usageRatio := 0.0

		// Suspended/canceled subscriptions have no usable cycle quota.
		if strings.EqualFold(resp.Subscription.Status, "suspended") || strings.EqualFold(resp.Subscription.Status, "cancelled") { //nolint:misspell // API domain value
			subStatus = "exhausted"
		} else if resp.Subscription.CycleQuotaLimit > 0 {
			usageRatio = float64(resp.Subscription.CycleQuotaUsed) / float64(resp.Subscription.CycleQuotaLimit)
			if resp.Subscription.CycleQuotaRemaining <= 0 {
				subStatus = "exhausted"
			} else if usageRatio > WarningThresholdRatio {
				subStatus = "warning"
			} else {
				subStatus = "available"
			}
		} else {
			subStatus = "unknown"
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeSubscriptionCycle,
			Status:      subStatus,
			UsageRatio:  usageRatio,
			Ready:       IsReadyStatus(subStatus),
			NextResetAt: nextResetAt,
		})
	}

	// If no limits were created, add an unknown one
	if len(limits) == 0 {
		limits = append(limits, QuotaLimitStatus{
			Type:       QuotaLimitTypeToken,
			Status:     "unknown",
			UsageRatio: 0,
			Ready:      false,
		})
	}

	return limits
}

// convertApertisResponseToMap converts the response to a map for RawData storage.
func convertApertisResponseToMap(resp ApertisBillingCreditsResponse) map[string]any {
	rawData := map[string]any{
		"is_subscriber": resp.IsSubscriber,
	}

	if resp.Payg != nil {
		paygMap := map[string]any{
			"account_credits":    resp.Payg.AccountCredits,
			"token_used":         resp.Payg.TokenUsed,
			"token_total":        resp.Payg.TokenTotal,
			"token_remaining":    resp.Payg.TokenRemaining,
			"token_is_unlimited": resp.Payg.TokenIsUnlimited,
		}
		if resp.Payg.TokenMonthlyLimitUSD != nil {
			paygMap["token_monthly_limit_usd"] = *resp.Payg.TokenMonthlyLimitUSD
		}
		if resp.Payg.TokenMonthlyUsedUSD != nil {
			paygMap["token_monthly_used_usd"] = *resp.Payg.TokenMonthlyUsedUSD
		}
		if resp.Payg.MonthlyResetDay != nil {
			paygMap["monthly_reset_day"] = *resp.Payg.MonthlyResetDay
		}
		rawData["payg"] = paygMap
	}

	if resp.Subscription != nil {
		subMap := map[string]any{
			"plan_type":             resp.Subscription.PlanType,
			"status":                resp.Subscription.Status,
			"cycle_quota_limit":     resp.Subscription.CycleQuotaLimit,
			"cycle_quota_used":      resp.Subscription.CycleQuotaUsed,
			"cycle_quota_remaining": resp.Subscription.CycleQuotaRemaining,
			"cycle_start":           resp.Subscription.CycleStart,
			"cycle_end":             resp.Subscription.CycleEnd,
			"payg_fallback_enabled": resp.Subscription.PaygFallbackEnabled,
		}
		if resp.Subscription.PaygSpentUSD != nil {
			subMap["payg_spent_usd"] = *resp.Subscription.PaygSpentUSD
		}
		if resp.Subscription.PaygLimitUSD != nil {
			subMap["payg_limit_usd"] = *resp.Subscription.PaygLimitUSD
		}
		rawData["subscription"] = subMap
	}

	return rawData
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		// Handle "unlimited" string
		if val == "unlimited" {
			return 0, false
		}
		// Try to parse numeric strings
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f, true
		}
		return 0, false
	default:
		return 0, false
	}
}
