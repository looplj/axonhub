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

const warningUsedPercent = 80

type WaferUsageResponse struct {
	Endpoint                   *string  `json:"endpoint,omitempty"`
	BillingModel               *string  `json:"billing_model,omitempty"`
	PlanTier                   *string  `json:"plan_tier,omitempty"`
	WindowStart                *string  `json:"window_start,omitempty"`
	WindowEnd                  *string  `json:"window_end,omitempty"`
	RequestCount               *int64   `json:"request_count,omitempty"`
	IncludedRequestLimit       *int64   `json:"included_request_limit,omitempty"`
	IncludedRequestCount       *int64   `json:"included_request_count,omitempty"`
	RemainingIncludedRequests  *int64   `json:"remaining_included_requests,omitempty"`
	OverageRequestCount        *int64   `json:"overage_request_count,omitempty"`
	CurrentPeriodUsedPercent   *float64 `json:"current_period_used_percent,omitempty"`
	InputTokens                *int64   `json:"input_tokens,omitempty"`
	OutputTokens               *int64   `json:"output_tokens,omitempty"`
	TotalTokens                *int64   `json:"total_tokens,omitempty"`
}

type WaferQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewWaferQuotaChecker(httpClient *httpclient.HttpClient) *WaferQuotaChecker {
	return &WaferQuotaChecker{
		httpClient: httpClient,
	}
}

func (c *WaferQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = ch.Credentials.APIKeys[0]
	}

	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	quotaURL := buildWaferQuotaURL(ch.BaseURL)

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

func (c *WaferQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var response WaferUsageResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse wafer usage response: %w", err)
	}

	normalizedStatus := "unknown"

	if response.CurrentPeriodUsedPercent != nil {
		pct := *response.CurrentPeriodUsedPercent
		if pct < warningUsedPercent {
			normalizedStatus = "available"
		} else {
			normalizedStatus = "warning"
		}
	}

	if response.RemainingIncludedRequests != nil &&
		*response.RemainingIncludedRequests <= 0 {
		normalizedStatus = "exhausted"
	}

	var nextResetAt *time.Time
	if response.WindowEnd != nil {
		if t, err := time.Parse(time.RFC3339, *response.WindowEnd); err == nil {
			nextResetAt = &t
		}
	}

	rawData := map[string]any{}

	if response.Endpoint != nil {
		rawData["endpoint"] = *response.Endpoint
	}
	if response.BillingModel != nil {
		rawData["billing_model"] = *response.BillingModel
	}
	if response.PlanTier != nil {
		rawData["plan_tier"] = *response.PlanTier
	}
	if response.WindowStart != nil {
		rawData["window_start"] = *response.WindowStart
	}
	if response.WindowEnd != nil {
		rawData["window_end"] = *response.WindowEnd
	}
	if response.RequestCount != nil {
		rawData["request_count"] = *response.RequestCount
	}
	if response.IncludedRequestLimit != nil {
		rawData["included_request_limit"] = *response.IncludedRequestLimit
	}
	if response.IncludedRequestCount != nil {
		rawData["included_request_count"] = *response.IncludedRequestCount
	}
	if response.RemainingIncludedRequests != nil {
		rawData["remaining_included_requests"] = *response.RemainingIncludedRequests
	}
	if response.OverageRequestCount != nil {
		rawData["overage_request_count"] = *response.OverageRequestCount
	}
	if response.CurrentPeriodUsedPercent != nil {
		rawData["current_period_used_percent"] = *response.CurrentPeriodUsedPercent
	}
	if response.InputTokens != nil {
		rawData["input_tokens"] = *response.InputTokens
	}
	if response.OutputTokens != nil {
		rawData["output_tokens"] = *response.OutputTokens
	}
	if response.TotalTokens != nil {
		rawData["total_tokens"] = *response.TotalTokens
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: "wafer",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        normalizedStatus == "available" || normalizedStatus == "warning",
	}, nil
}

func (c *WaferQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	if ch.Type != channel.TypeOpenai && ch.Type != channel.TypeOpenaiResponses {
		return false
	}

	return DetectProviderFromURL(ch.BaseURL) == "wafer"
}

func buildWaferQuotaURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "https://pass.wafer.ai/v1/inference/quota"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme == "" && parsed.Host == "") {
		return "https://pass.wafer.ai/v1/inference/quota"
	}

	scheme := parsed.Scheme
	if scheme == "http" {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s/v1/inference/quota", scheme, parsed.Host)
}
