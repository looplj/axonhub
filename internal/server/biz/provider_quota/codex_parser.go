package provider_quota

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type CodexQuotaParser struct{}

// CodexUsageResponse matches ChatGPT backend API response
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

func (p *CodexQuotaParser) ParseResponse(headers http.Header, body []byte) (QuotaData, error) {
	var response CodexUsageResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse codex usage response: %w", err)
	}

	// Determine overall status
	status := "ok"
	if response.RateLimit != nil && response.RateLimit.LimitReached != nil && *response.RateLimit.LimitReached {
		status = "limit_reached"
	} else if response.RateLimit != nil && response.RateLimit.Allowed != nil && !*response.RateLimit.Allowed {
		status = "not_allowed"
	}

	// Convert to raw data map
	rawData := map[string]interface{}{
		"plan_type": response.PlanType,
	}

	if response.RateLimit != nil {
		rawData["rate_limit"] = convertRateLimitToMap(response.RateLimit)
	}

	if response.CodeReviewRateLimit != nil {
		rawData["code_review_rate_limit"] = convertRateLimitToMap(response.CodeReviewRateLimit)
	}

	return QuotaData{
		Status:       status,
		ProviderType: "codex",
		RawData:      rawData,
	}, nil
}

func (p *CodexQuotaParser) GetProviderType() string {
	return "codex"
}

func convertRateLimitToMap(rateLimit *CodeRateLimitInfo) map[string]interface{} {
	result := make(map[string]interface{})

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

func convertWindowToMap(window *CodeUsageWindow) map[string]interface{} {
	result := make(map[string]interface{})
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
