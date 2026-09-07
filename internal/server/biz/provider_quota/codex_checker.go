package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
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

type CodexResetCredit struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ResetType string `json:"reset_type,omitempty"`
	GrantedAt string `json:"granted_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Title     string `json:"title,omitempty"`
}

type CodexResetCreditsResponse struct {
	Credits        []CodexResetCredit `json:"credits"`
	AvailableCount int                `json:"available_count"`
}

type CodexResetConsumeResponse struct {
	Code         string           `json:"code"`
	WindowsReset int              `json:"windows_reset"`
	Credit       CodexResetCredit `json:"credit"`
}

type CodexQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

const codexResetStatusAvailable = "available"

var _ Resetter = (*CodexQuotaChecker)(nil)

func NewCodexQuotaChecker(httpClient *httpclient.HttpClient) *CodexQuotaChecker {
	return &CodexQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *CodexQuotaChecker) Reset(ctx context.Context, ch *ent.Channel) error {
	resets, err := c.ListResets(ctx, ch)
	if err != nil {
		return err
	}

	if len(resets.Resets) == 0 {
		return fmt.Errorf("no available codex reset credit")
	}

	_, err = c.consumeResetCredit(ctx, ch, resets.Resets[0].ID)
	return err
}

func (c *CodexQuotaChecker) ListResets(ctx context.Context, ch *ent.Channel) (ResetList, error) {
	response, err := c.listResetCredits(ctx, ch)
	if err != nil {
		return ResetList{Supported: true}, err
	}

	resets := make([]Reset, 0, len(response.Credits))
	for _, credit := range response.Credits {
		if credit.Status != codexResetStatusAvailable {
			continue
		}

		resets = append(resets, Reset{
			ID:        credit.ID,
			Status:    credit.Status,
			Type:      credit.ResetType,
			GrantedAt: parseCodexResetTime(credit.GrantedAt),
			ExpiresAt: parseCodexResetTime(credit.ExpiresAt),
			Title:     credit.Title,
		})
	}

	return ResetList{
		Supported: true,
		Resets:    resets,
	}, nil
}

func (c *CodexQuotaChecker) listResetCredits(ctx context.Context, ch *ent.Channel) (*CodexResetCreditsResponse, error) {
	accessToken, accountID, err := c.extractCodexCredentials(ch)
	if err != nil {
		return nil, err
	}

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL("https://chatgpt.com/backend-api/wham/rate-limit-reset-credits").
		WithBearerToken(accessToken).
		WithHeader("ChatGPT-Account-Id", accountID).
		WithHeader("Content-Type", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return nil, fmt.Errorf("list codex reset credits failed: %w", err)
	}

	var result CodexResetCreditsResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse codex reset credits response: %w", err)
	}

	return &result, nil
}

func parseCodexResetTime(value string) *time.Time {
	if value == "" {
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}

	return &parsed
}

func (c *CodexQuotaChecker) consumeResetCredit(ctx context.Context, ch *ent.Channel, creditID string) (*CodexResetConsumeResponse, error) {
	accessToken, accountID, err := c.extractCodexCredentials(ch)
	if err != nil {
		return nil, err
	}

	httpRequest := httpclient.NewRequestBuilder().
		WithMethod("POST").
		WithURL("https://chatgpt.com/backend-api/wham/rate-limit-reset-credits/consume").
		WithBearerToken(accessToken).
		WithHeader("ChatGPT-Account-Id", accountID).
		WithHeader("Content-Type", "application/json").
		WithBody(map[string]string{
			"credit_id":         creditID,
			"redeem_request_id": uuid.NewString(),
		}).
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return nil, fmt.Errorf("consume codex reset credit failed: %w", err)
	}

	var result CodexResetConsumeResponse
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse codex reset consume response: %w", err)
	}

	return &result, nil
}

func (c *CodexQuotaChecker) extractCodexCredentials(ch *ent.Channel) (string, string, error) {
	if ch.Credentials.OAuth == nil && strings.TrimSpace(ch.Credentials.APIKey) == "" {
		return "", "", fmt.Errorf("channel has no credentials")
	}

	var accessToken string
	if ch.Credentials.OAuth != nil {
		accessToken = ch.Credentials.OAuth.AccessToken
	} else if strings.TrimSpace(ch.Credentials.APIKey) != "" {
		creds, err := oauth.ParseCredentialsJSON(ch.Credentials.APIKey)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse OAuth credentials: %w", err)
		}
		accessToken = creds.AccessToken
	}

	if accessToken == "" {
		return "", "", fmt.Errorf("OAuth missing access_token")
	}

	accountID := codex.ExtractChatGPTAccountIDFromJWT(accessToken)
	if accountID == "" {
		return "", "", fmt.Errorf("failed to extract ChatGPT account id from access token")
	}

	return accessToken, accountID, nil
}

func (c *CodexQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	accessToken, _, err := c.extractCodexCredentials(ch)
	if err != nil {
		return QuotaData{}, err
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

	now := time.Now()
	limits := make([]QuotaLimitStatus, 0, 2)
	if response.RateLimit != nil {
		rateLimitExhausted := codexRateLimitExhausted(response.RateLimit)
		windows := []struct {
			name  string
			value *CodeUsageWindow
		}{
			{name: QuotaWindowPrimary, value: response.RateLimit.PrimaryWindow},
			{name: QuotaWindowSecondary, value: response.RateLimit.SecondaryWindow},
		}

		for _, candidate := range windows {
			limit, ok := buildCodexQuotaLimit(candidate.name, candidate.value, rateLimitExhausted, now)
			if ok {
				limits = append(limits, limit)
			}
		}
	}

	normalizedStatus := codexAggregateStatus(response.RateLimit, limits)
	nextResetAt := codexEarliestFutureReset(limits, now)

	return NormalizeQuotaData(QuotaData{
		Status:       normalizedStatus,
		ProviderType: "codex",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(normalizedStatus),
		Limits:       limits,
	}), nil
}

func codexRateLimitExhausted(rateLimit *CodeRateLimitInfo) bool {
	return rateLimit.LimitReached != nil && *rateLimit.LimitReached ||
		(rateLimit.Allowed != nil && !*rateLimit.Allowed)
}

func buildCodexQuotaLimit(name string, window *CodeUsageWindow, rateLimitExhausted bool, now time.Time) (QuotaLimitStatus, bool) {
	if window == nil {
		return QuotaLimitStatus{}, false
	}

	duration, ok := codexWindowDuration(window.LimitWindowSeconds)
	if !ok || window.UsedPercent != nil && (math.IsNaN(*window.UsedPercent) || math.IsInf(*window.UsedPercent, 0) || *window.UsedPercent < 0) {
		return QuotaLimitStatus{}, false
	}

	usageRatio := 0.0
	status := "available"
	if window.UsedPercent != nil {
		usageRatio = min(*window.UsedPercent/100, 1)
		if usageRatio >= 1 {
			status = "exhausted"
		} else if usageRatio >= WarningThresholdRatio {
			status = "warning"
		}
	}

	if rateLimitExhausted {
		status = "exhausted"
		usageRatio = 1
	}

	resetAt := codexWindowResetAt(window, now)
	label := NormalizeQuotaWindowLabel(duration)
	if label == "" {
		label = name
	}

	return NewTokenLimitStatus(status, usageRatio, resetAt).WithWindow(label, duration), true
}

func codexWindowDuration(seconds *int) (time.Duration, bool) {
	if seconds == nil {
		return 0, true
	}

	const maxDurationSeconds = int64(1<<63-1) / int64(time.Second)
	if *seconds <= 0 || int64(*seconds) > maxDurationSeconds {
		return 0, false
	}

	return time.Duration(*seconds) * time.Second, true
}

func codexWindowResetAt(window *CodeUsageWindow, now time.Time) *time.Time {
	if window.ResetAt != nil && *window.ResetAt > 0 {
		resetAt := time.Unix(*window.ResetAt, 0)
		return &resetAt
	}

	if resetAfter, ok := codexWindowDuration(window.ResetAfterSeconds); ok && resetAfter > 0 {
		resetAt := now.Add(resetAfter)
		return &resetAt
	}

	return nil
}

func codexAggregateStatus(rateLimit *CodeRateLimitInfo, limits []QuotaLimitStatus) string {
	status := "unknown"
	if rateLimit != nil && rateLimit.Allowed != nil && *rateLimit.Allowed {
		status = "available"
	}
	if rateLimit != nil && codexRateLimitExhausted(rateLimit) {
		status = "exhausted"
	}

	for _, limit := range limits {
		if codexStatusRank(limit.Status) > codexStatusRank(status) {
			status = limit.Status
		}
	}

	return status
}

func codexStatusRank(status string) int {
	switch status {
	case "exhausted":
		return 3
	case "warning":
		return 2
	case "available":
		return 1
	default:
		return 0
	}
}

func codexEarliestFutureReset(limits []QuotaLimitStatus, now time.Time) *time.Time {
	var earliest *time.Time
	for _, limit := range limits {
		if limit.NextResetAt == nil || !limit.NextResetAt.After(now) {
			continue
		}
		if earliest == nil || limit.NextResetAt.Before(*earliest) {
			resetAt := *limit.NextResetAt
			earliest = &resetAt
		}
	}

	return earliest
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
