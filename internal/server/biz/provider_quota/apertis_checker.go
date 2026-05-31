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

// ApertisBillingCreditsResponse represents the response from the Apertis billing credits API.
// The API returns PAYG credit balances and subscription quota details.
type ApertisBillingCreditsResponse struct {
	Object       string               `json:"object"`
	IsSubscriber bool                 `json:"is_subscriber"`
	Payg         *ApertisPayg         `json:"payg"`
	Subscription *ApertisSubscription `json:"subscription,omitempty"`
}

// ApertisPayg represents PAYG (Pay-As-You-Go) credit information.
type ApertisPayg struct {
	AccountCredits float64     `json:"account_credits"`
	TokenUsed      float64     `json:"token_used"`
	TokenTotal     interface{} `json:"token_total"`     // Can be float64 or string "unlimited"
	TokenRemaining interface{} `json:"token_remaining"` // Can be float64 or string "unlimited"
	TokenIsUnlimited bool      `json:"token_is_unlimited"`
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
		schemeHost = "https://api.apertis.ai"
	}

	parsed, err := url.Parse(schemeHost)
	if err != nil {
		return "https://api.apertis.ai/v1/dashboard/billing/credits"
	}

	scheme := parsed.Scheme
	if scheme == "" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/v1/dashboard/billing/credits", scheme, parsed.Host)
}

// determineApertisStatus determines the overall quota status based on the response.
func determineApertisStatus(resp *ApertisBillingCreditsResponse) string {
	// Check subscription status first
	if resp.Subscription != nil {
		// If subscription is suspended or cancelled, it's exhausted
		if strings.EqualFold(resp.Subscription.Status, "suspended") ||
			strings.EqualFold(resp.Subscription.Status, "cancelled") {
			return "exhausted"
		}

		// If subscription cycle quota is exhausted
		if resp.Subscription.CycleQuotaRemaining <= 0 {
			return "exhausted"
		}
	}

	// Check PAYG account credits
	if resp.Payg != nil {
		if resp.Payg.AccountCredits <= 0 {
			// If not a subscriber, or subscriber without fallback, it's exhausted
			if !resp.IsSubscriber {
				return "exhausted"
			}
			// If subscriber with fallback, check if fallback is also exhausted
			if resp.Subscription != nil && !resp.Subscription.PaygFallbackEnabled {
				return "exhausted"
			}
		}

		// Check for warning state - token usage or subscription cycle usage >= 80%
		if !resp.Payg.TokenIsUnlimited {
			// Check token usage ratio
			if total, ok := toFloat64(resp.Payg.TokenTotal); ok {
				if total > 0 {
					usageRatio := resp.Payg.TokenUsed / total
					if usageRatio >= WarningThresholdRatio {
						return "warning"
					}
				}
			}
		}

		// Check subscription cycle usage ratio for warning
		if resp.Subscription != nil && resp.Subscription.CycleQuotaLimit > 0 {
			usageRatio := float64(resp.Subscription.CycleQuotaUsed) / float64(resp.Subscription.CycleQuotaLimit)
			if usageRatio >= WarningThresholdRatio {
				return "warning"
			}
		}
	}

	return "available"
}

// buildApertisLimits builds the limit status list from the response.
func buildApertisLimits(resp *ApertisBillingCreditsResponse, nextResetAt *time.Time) []QuotaLimitStatus {
	var limits []QuotaLimitStatus

	// Token limit (from PAYG)
	if resp.Payg != nil {
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
				} else if usageRatio >= WarningThresholdRatio {
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

	// Subscription cycle limit (if subscriber)
	if resp.IsSubscriber && resp.Subscription != nil {
		var subStatus string
		usageRatio := 0.0

		if resp.Subscription.CycleQuotaLimit > 0 {
			usageRatio = float64(resp.Subscription.CycleQuotaUsed) / float64(resp.Subscription.CycleQuotaLimit)
			if resp.Subscription.CycleQuotaRemaining <= 0 {
				subStatus = "exhausted"
			} else if usageRatio >= WarningThresholdRatio {
				subStatus = "warning"
			} else {
				subStatus = "available"
			}
		} else {
			subStatus = "unknown"
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken, // Using token as general quota type
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
		rawData["payg"] = map[string]any{
			"account_credits":    resp.Payg.AccountCredits,
			"token_used":         resp.Payg.TokenUsed,
			"token_total":        resp.Payg.TokenTotal,
			"token_remaining":    resp.Payg.TokenRemaining,
			"token_is_unlimited": resp.Payg.TokenIsUnlimited,
		}
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

// toFloat64 attempts to convert an interface{} to float64.
func toFloat64(v interface{}) (float64, bool) {
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