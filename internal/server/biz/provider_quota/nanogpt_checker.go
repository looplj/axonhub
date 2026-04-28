package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

// NanoGPTUsageResponse matches the NanoGPT subscription usage API response.
type NanoGPTUsageResponse struct {
	Active            *bool               `json:"active,omitempty"`
	Provider          *string             `json:"provider,omitempty"`
	ProviderStatus    *string             `json:"providerStatus,omitempty"`
	Limits            *NanoGPTLimits      `json:"limits,omitempty"`
	AllowOverage      *bool               `json:"allowOverage,omitempty"`
	Period            *NanoGPTPeriod      `json:"period,omitempty"`
	DailyImages       *NanoGPTQuotaWindow `json:"dailyImages,omitempty"`
	DailyInputTokens  *NanoGPTQuotaWindow `json:"dailyInputTokens,omitempty"`
	WeeklyInputTokens *NanoGPTQuotaWindow `json:"weeklyInputTokens,omitempty"`
	State             *string             `json:"state,omitempty"`
	GraceUntil        *string             `json:"graceUntil,omitempty"`
}

// NanoGPTLimits represents the quota limits from the NanoGPT API.
type NanoGPTLimits struct {
	WeeklyInputTokens *int64 `json:"weeklyInputTokens,omitempty"`
	DailyInputTokens  *int64 `json:"dailyInputTokens,omitempty"`
	DailyImages       *int64 `json:"dailyImages,omitempty"`
}

// NanoGPTQuotaWindow represents a single usage window from the NanoGPT API.
type NanoGPTQuotaWindow struct {
	Used        *int64   `json:"used,omitempty"`
	Remaining   *int64   `json:"remaining,omitempty"`
	PercentUsed *float64 `json:"percentUsed,omitempty"`
	ResetAt     *int64   `json:"resetAt,omitempty"`
}

// NanoGPTPeriod represents the subscription period from the NanoGPT API.
type NanoGPTPeriod struct {
	CurrentPeriodEnd *string `json:"currentPeriodEnd,omitempty"`
}

type NanoGPTQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewNanoGPTQuotaChecker(httpClient *httpclient.HttpClient) *NanoGPTQuotaChecker {
	return &NanoGPTQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *NanoGPTQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	// Extract API key: prefer APIKey field, then first from APIKeys
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = ch.Credentials.APIKeys[0]
	}

	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	// Build quota URL from channel base URL scheme+host + path
	quotaURL := buildNanoGPTQuotaURL(ch.BaseURL)

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaURL).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	// Use proxy-configured HTTP client if available
	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{}, fmt.Errorf("quota request failed: %w", err)
	}

	return c.parseResponse(resp.Body)
}

func (c *NanoGPTQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var response NanoGPTUsageResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse nanogpt usage response: %w", err)
	}

	// Map state to normalized status
	normalizedStatus := "unknown"

	if response.State != nil {
		switch *response.State {
		case "active":
			normalizedStatus = "available"
		case "grace":
			normalizedStatus = "warning"
		case "inactive":
			normalizedStatus = "exhausted"
		}
	}

	limits := buildNanoGPTLimitStatuses(response.DailyImages, response.DailyInputTokens, response.WeeklyInputTokens)

	// Check for warning state: any window with percentUsed >= WarningThresholdRatio
	if normalizedStatus == "available" {
		for i := range limits {
			if limits[i].Status == "warning" || limits[i].Status == "exhausted" {
				normalizedStatus = "warning"
				break
			}
		}
	}

	// Calculate NextResetAt from earliest resetAt across all windows
	nextResetAt := findEarliestResetAt(response.DailyImages, response.DailyInputTokens, response.WeeklyInputTokens)

	// During grace period, windows typically have no resetAt;
	// fall back to graceUntil as the relevant deadline.
	if nextResetAt == nil && response.GraceUntil != nil {
		if t, err := time.Parse(time.RFC3339, *response.GraceUntil); err == nil {
			nextResetAt = &t
		}
	}

	// Build raw data map with all non-nil windows
	rawData := map[string]any{}

	if response.Active != nil {
		rawData["active"] = *response.Active
	}

	if response.Provider != nil {
		rawData["provider"] = *response.Provider
	}

	if response.ProviderStatus != nil {
		rawData["providerStatus"] = *response.ProviderStatus
	}

	if response.State != nil {
		rawData["state"] = *response.State
	}

	if response.AllowOverage != nil {
		rawData["allowOverage"] = *response.AllowOverage
	}

	if response.Limits != nil {
		rawData["limits"] = convertNanoGPTLimitsToMap(response.Limits)
	}

	if response.Period != nil {
		rawData["period"] = convertNanoGPTPeriodToMap(response.Period)
	}

	windows := map[string]any{}

	if response.DailyImages != nil {
		windows["dailyImages"] = convertNanoGPTWindowToMap(response.DailyImages)
	}

	if response.DailyInputTokens != nil {
		windows["dailyInputTokens"] = convertNanoGPTWindowToMap(response.DailyInputTokens)
	}

	if response.WeeklyInputTokens != nil {
		windows["weeklyInputTokens"] = convertNanoGPTWindowToMap(response.WeeklyInputTokens)
	}

	if len(windows) > 0 {
		rawData["windows"] = windows
	}

	if response.GraceUntil != nil {
		rawData["graceUntil"] = *response.GraceUntil
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "nanogpt",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}, nil
}

func (c *NanoGPTQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeNanogpt || ch.Type == channel.TypeNanogptResponses
}

// buildNanoGPTQuotaURL derives the quota URL from the channel base URL.
// It extracts scheme+host and appends the quota path.
// Falls back to https://nano-gpt.com if base URL is empty or invalid.
func buildNanoGPTQuotaURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://nano-gpt.com/api/subscription/v1/usage"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme == "" && parsed.Host == "") {
		return "https://nano-gpt.com/api/subscription/v1/usage"
	}

	scheme := parsed.Scheme
	if scheme == "http" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/api/subscription/v1/usage", scheme, parsed.Host)
}

func buildNanoGPTLimitStatuses(imageWindow, dailyTokenWindow, weeklyTokenWindow *NanoGPTQuotaWindow) []QuotaLimitStatus {
	typed := []struct {
		window   *NanoGPTQuotaWindow
		limitType QuotaLimitType
	}{
		{window: imageWindow, limitType: QuotaLimitTypeImage},
		{window: dailyTokenWindow, limitType: QuotaLimitTypeToken},
		{window: weeklyTokenWindow, limitType: QuotaLimitTypeToken},
	}

	var limits []QuotaLimitStatus

	for _, w := range typed {
		if w.window == nil {
			continue
		}

		status := "available"
		usageRatio := 0.0

		if w.window.PercentUsed != nil {
			usageRatio = *w.window.PercentUsed
		}

		if w.window.Remaining != nil && *w.window.Remaining <= 0 {
			status = "exhausted"
			usageRatio = 1.0
		} else if usageRatio >= 1.0 {
			status = "exhausted"
		} else if usageRatio >= WarningThresholdRatio {
			status = "warning"
		}

		var resetAt *time.Time
		if w.window.ResetAt != nil && *w.window.ResetAt > 0 {
			t := time.UnixMilli(*w.window.ResetAt)
			resetAt = &t
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        w.limitType,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       status != "exhausted",
			NextResetAt: resetAt,
		})
	}

	return limits
}

// findEarliestResetAt returns the earliest resetAt time from all non-nil windows.
// resetAt is a millisecond epoch timestamp.
func findEarliestResetAt(windows ...*NanoGPTQuotaWindow) *time.Time {
	var earliest *time.Time

	for _, w := range windows {
		if w == nil || w.ResetAt == nil || *w.ResetAt <= 0 {
			continue
		}

		// Convert millisecond epoch to time.Time
		t := time.UnixMilli(*w.ResetAt)

		if earliest == nil || t.Before(*earliest) {
			earliest = &t
		}
	}

	return earliest
}

func convertNanoGPTLimitsToMap(limits *NanoGPTLimits) map[string]any {
	result := make(map[string]any)

	if limits.WeeklyInputTokens != nil {
		result["weeklyInputTokens"] = *limits.WeeklyInputTokens
	}

	if limits.DailyInputTokens != nil {
		result["dailyInputTokens"] = *limits.DailyInputTokens
	}

	if limits.DailyImages != nil {
		result["dailyImages"] = *limits.DailyImages
	}

	return result
}

func convertNanoGPTPeriodToMap(period *NanoGPTPeriod) map[string]any {
	result := make(map[string]any)

	if period.CurrentPeriodEnd != nil {
		result["currentPeriodEnd"] = *period.CurrentPeriodEnd
	}

	return result
}

func convertNanoGPTWindowToMap(window *NanoGPTQuotaWindow) map[string]any {
	result := make(map[string]any)

	if window.Used != nil {
		result["used"] = *window.Used
	}

	if window.Remaining != nil {
		result["remaining"] = *window.Remaining
	}

	if window.PercentUsed != nil {
		result["percentUsed"] = *window.PercentUsed
	}

	if window.ResetAt != nil {
		result["resetAt"] = *window.ResetAt
	}

	return result
}
