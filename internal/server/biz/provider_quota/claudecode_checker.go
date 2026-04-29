package provider_quota

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
)

type ClaudeCodeQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewClaudeCodeQuotaChecker(httpClient *httpclient.HttpClient) *ClaudeCodeQuotaChecker {
	return &ClaudeCodeQuotaChecker{httpClient: httpClient}
}

func (c *ClaudeCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	// Verify credentials
	if ch.Credentials.OAuth == nil && strings.TrimSpace(ch.Credentials.APIKey) == "" {
		return QuotaData{}, fmt.Errorf("channel has no credentials")
	}

	// Parse OAuth credentials from apiKey JSON
	var accessToken string
	if ch.Credentials.OAuth != nil {
		accessToken = ch.Credentials.OAuth.AccessToken
	} else if strings.TrimSpace(ch.Credentials.APIKey) != "" {
		creds, err := oauth.ParseCredentialsJSON(ch.Credentials.APIKey)
		if err != nil {
			return QuotaData{}, fmt.Errorf("failed to parse OAuth credentials: %w", err)
		}

		accessToken = creds.AccessToken
	}

	if accessToken == "" {
		return QuotaData{}, fmt.Errorf("channel credentials missing access token")
	}

	// Build HTTP request using Bearer auth like ClaudeCode transformers
	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("POST").
		WithURL(getEndpointURL(ch.BaseURL)).
		WithAuth(&httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: accessToken,
		}).
		WithHeader("anthropic-beta", claudecode.ClaudeCodeBetaHeader).
		WithHeader("anthropic-version", claudecode.ClaudeCodeVersionHeader).
		WithHeader("anthropic-dangerous-direct-browser-access", claudecode.ClaudeCodeBrowserAccessHeader).
		WithHeader("x-app", claudecode.ClaudeCodeAppHeader).
		WithHeader("content-type", "application/json").
		WithBody(map[string]any{
			"model": claudecode.ClaudeCodeQuotaCheckModel,
			"messages": []map[string]any{
				{
					"role":    "user",
					"content": "limit",
				},
			},
			"max_tokens": 1,
		}).
		Build()

	// Use proxy-configured HTTP client if available
	httpClient := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		httpClient = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	httpResponse, err := httpClient.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{}, fmt.Errorf("HTTP request failed: %w", err)
	}

	// Check HTTP status
	if httpResponse.StatusCode != http.StatusOK {
		return QuotaData{}, fmt.Errorf("HTTP %d: %s", httpResponse.StatusCode, string(httpResponse.Body))
	}

	return c.parseResponse(httpResponse.Headers)
}

func (c *ClaudeCodeQuotaChecker) parseResponse(headers http.Header) (QuotaData, error) {
	// Guard clause - early return if no quota headers
	if headers.Get("Anthropic-Ratelimit-Unified-Status") == "" {
		return QuotaData{}, fmt.Errorf("missing quota headers")
	}

	unifiedStatus := headers.Get("Anthropic-Ratelimit-Unified-Status")
	representativeClaim := headers.Get("Anthropic-Ratelimit-Unified-Representative-Claim")

	// Parse window data
	windows := map[string]any{
		"5h": map[string]any{
			"status":      headers.Get("Anthropic-Ratelimit-Unified-5h-Status"),
			"reset":       parseUnixTimestamp(headers.Get("Anthropic-Ratelimit-Unified-5h-Reset")),
			"utilization": parseFloat(headers.Get("Anthropic-Ratelimit-Unified-5h-Utilization")),
		},
		"7d": map[string]any{
			"status":      headers.Get("Anthropic-Ratelimit-Unified-7d-Status"),
			"reset":       parseUnixTimestamp(headers.Get("Anthropic-Ratelimit-Unified-7d-Reset")),
			"utilization": parseFloat(headers.Get("Anthropic-Ratelimit-Unified-7d-Utilization")),
		},
		"overage": map[string]any{
			"status":      headers.Get("Anthropic-Ratelimit-Unified-Overage-Status"),
			"reset":       parseUnixTimestamp(headers.Get("Anthropic-Ratelimit-Unified-Overage-Reset")),
			"utilization": parseFloat(headers.Get("Anthropic-Ratelimit-Unified-Overage-Utilization")),
		},
	}

	rawData := map[string]any{
		"unified_status":       unifiedStatus,
		"windows":              windows,
		"representative_claim": representativeClaim,
		"fallback":             headers.Get("Anthropic-Ratelimit-Unified-Fallback"),
		"fallback_percentage":  parseFloat(headers.Get("Anthropic-Ratelimit-Unified-Fallback-Percentage")),
		"reset":                parseUnixTimestamp(headers.Get("Anthropic-Ratelimit-Unified-Reset")),
	}

	// Normalize status: allowed -> available, throttled/rejected -> exhausted
	var normalizedStatus string

	switch unifiedStatus {
	case "allowed":
		normalizedStatus = "available"
	case "throttled", "rejected":
		normalizedStatus = "exhausted"
	default:
		normalizedStatus = "unknown"
	}

	// Check for warning state (utilization >= 80% on any window)
	if normalizedStatus == "available" {
		fiveHourUtilization := parseFloat(headers.Get("Anthropic-Ratelimit-Unified-5h-Utilization"))

		sevenDayUtilization := parseFloat(headers.Get("Anthropic-Ratelimit-Unified-7d-Utilization"))
		if fiveHourUtilization >= WarningThresholdRatio || sevenDayUtilization >= WarningThresholdRatio {
			normalizedStatus = "warning"
		}
	}

	// Extract next reset time based on representative claim
	// Map representative claim to window key: "five_hour" -> "5h", "seven_day" -> "7d"
	windowKey := representativeClaim
	switch representativeClaim {
	case "five_hour":
		windowKey = "5h"
	case "seven_day":
		windowKey = "7d"
	}

	var nextResetAt *time.Time

	if resetWindow, ok := windows[windowKey].(map[string]any); ok {
		if resetTs, exists := resetWindow["reset"].(int64); exists && resetTs > 0 {
			t := time.Unix(resetTs, 0)
			nextResetAt = &t
		}
	}

	limits := []QuotaLimitStatus{
		c.buildTokenLimit("5h", headers),
		c.buildTokenLimit("7d", headers),
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "claudecode",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}, nil
}

func (c *ClaudeCodeQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeClaudecode
}

func (c *ClaudeCodeQuotaChecker) buildTokenLimit(windowKey string, headers http.Header) QuotaLimitStatus {
	var utilizationKey, resetKey string
	switch windowKey {
	case "5h":
		utilizationKey = "Anthropic-Ratelimit-Unified-5h-Utilization"
		resetKey = "Anthropic-Ratelimit-Unified-5h-Reset"
	case "7d":
		utilizationKey = "Anthropic-Ratelimit-Unified-7d-Utilization"
		resetKey = "Anthropic-Ratelimit-Unified-7d-Reset"
	}

	utilization := parseFloat(headers.Get(utilizationKey))
	resetTs := parseUnixTimestamp(headers.Get(resetKey))

	status := "available"
	if utilization >= 1.0 {
		status = "exhausted"
	} else if utilization >= WarningThresholdRatio {
		status = "warning"
	}

	var nextReset *time.Time
	if resetTs > 0 {
		t := time.Unix(resetTs, 0)
		nextReset = &t
	}

	return QuotaLimitStatus{
		Type:        QuotaLimitTypeToken,
		Status:      status,
		UsageRatio:  utilization,
		Ready:       status != "exhausted",
		NextResetAt: nextReset,
	}
}

func getEndpointURL(baseURL string) string {
	if baseURL == "" {
		return "https://api.anthropic.com/v1/messages"
	}

	baseURL = strings.TrimSuffix(baseURL, "/")

	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/messages"
	}

	return baseURL + "/v1/messages"
}

func parseUnixTimestamp(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
