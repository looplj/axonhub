package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
)

// CodexUsageResponse matches ChatGPT backend API response.
type CodexUsageResponse struct {
	PlanType            string             `json:"plan_type,omitempty"`
	RateLimit           *CodeRateLimitInfo `json:"rate_limit,omitempty"`
	CodeReviewRateLimit *CodeRateLimitInfo `json:"code_review_rate_limit,omitempty"`
}

type CodeRateLimitInfo struct {
	Allowed         *bool            `json:"allowed,omitempty"`
	LimitReached    *bool            `json:"limit_reached,omitempty"`
	PrimaryWindow   *CodeUsageWindow `json:"primary_window,omitempty"`
	SecondaryWindow *CodeUsageWindow `json:"secondary_window,omitempty"`
}

type CodeUsageWindow struct {
	UsedPercent        *float64 `json:"used_percent,omitempty"`
	ResetAt            *int64   `json:"reset_at,omitempty"`
	ResetAfterSeconds  *int     `json:"reset_after_seconds,omitempty"`
	LimitWindowSeconds *int     `json:"limit_window_seconds,omitempty"`
}

type CodexQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewCodexQuotaChecker(httpClient *httpclient.HttpClient) *CodexQuotaChecker {
	return &CodexQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *CodexQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	// Extract OAuth credentials
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
		return QuotaData{}, fmt.Errorf("OAuth missing access_token")
	}

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL("https://chatgpt.com/backend-api/wham/usage").
		WithBearerToken(accessToken).
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

func (c *CodexQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var response CodexUsageResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse codex usage response: %w", err)
	}

	// Normalize status
	normalizedStatus := "unknown"

	var (
		nextResetAt              *time.Time
		primaryWindowUsedPercent *float64
	)

	if response.RateLimit != nil {
		if response.RateLimit.LimitReached != nil && *response.RateLimit.LimitReached {
			normalizedStatus = "exhausted"
		} else if response.RateLimit.Allowed != nil && !*response.RateLimit.Allowed {
			normalizedStatus = "exhausted"
		} else {
			normalizedStatus = "available"

			// Check for warning state (primary window utilization >= 80%)
			if response.RateLimit.PrimaryWindow != nil && response.RateLimit.PrimaryWindow.UsedPercent != nil {
				primaryWindowUsedPercent = response.RateLimit.PrimaryWindow.UsedPercent
				if *primaryWindowUsedPercent >= 80.0 {
					normalizedStatus = "warning"
				}
			}

			// Extract next reset from primary window
			if response.RateLimit.PrimaryWindow != nil && response.RateLimit.PrimaryWindow.ResetAt != nil && *response.RateLimit.PrimaryWindow.ResetAt > 0 {
				t := time.Unix(*response.RateLimit.PrimaryWindow.ResetAt, 0)
				nextResetAt = &t
			}
		}
	}

	// Convert to raw data map
	rawData := map[string]any{
		"plan_type": response.PlanType,
	}

	if response.RateLimit != nil {
		rawData["rate_limit"] = convertRateLimitToMap(response.RateLimit)
	}

	if response.CodeReviewRateLimit != nil {
		rawData["code_review_rate_limit"] = convertRateLimitToMap(response.CodeReviewRateLimit)
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "codex",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        normalizedStatus == "available" || normalizedStatus == "warning",
	}, nil
}

func (c *CodexQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeCodex
}

func convertRateLimitToMap(rateLimit *CodeRateLimitInfo) map[string]any {
	result := make(map[string]any)

	if rateLimit.Allowed != nil {
		result["allowed"] = *rateLimit.Allowed
	}

	if rateLimit.LimitReached != nil {
		result["limit_reached"] = *rateLimit.LimitReached
	}

	if rateLimit.PrimaryWindow != nil {
		result["primary_window"] = convertWindowToMap(rateLimit.PrimaryWindow)
	}

	if rateLimit.SecondaryWindow != nil {
		result["secondary_window"] = convertWindowToMap(rateLimit.SecondaryWindow)
	}

	return result
}

func convertWindowToMap(window *CodeUsageWindow) map[string]any {
	result := make(map[string]any)
	if window.UsedPercent != nil {
		result["used_percent"] = *window.UsedPercent
	}

	if window.ResetAt != nil {
		result["reset_at"] = *window.ResetAt
	}

	if window.ResetAfterSeconds != nil {
		result["reset_after_seconds"] = *window.ResetAfterSeconds
	}

	if window.LimitWindowSeconds != nil {
		result["limit_window_seconds"] = *window.LimitWindowSeconds
	}

	return result
}
