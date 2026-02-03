package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
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
	httpClient *http.Client
}

func NewCodexQuotaChecker() *CodexQuotaChecker {
	return &CodexQuotaChecker{
		httpClient: &http.Client{},
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

	// Extract chatgpt_account_id from access_token JWT
	// The access_token contains the account ID in the https://api.openai.com/auth claim
	accountID := codex.ExtractChatGPTAccountIDFromJWT(accessToken)
	if accountID == "" {
		return QuotaData{}, fmt.Errorf("failed to extract account ID from access_token (invalid JWT format or missing claim)")
	}

	// Build request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://chatgpt.com/backend-api/wham/usage", nil)
	if err != nil {
		return QuotaData{}, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal")
	req.Header.Set("Chatgpt-Account-Id", accountID)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return QuotaData{}, fmt.Errorf("quota request failed: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return QuotaData{}, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return QuotaData{}, fmt.Errorf("quota request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return c.parseResponse(body)
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
