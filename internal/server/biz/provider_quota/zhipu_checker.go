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
)

const zhipuDefaultQuotaBaseURL = "https://open.bigmodel.cn"

// zhipuQuotaResponse matches the ZhiPu/GLM quota API response.
type zhipuQuotaResponse struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Data    *zhipuQuotaData `json:"data,omitempty"`
}

type zhipuQuotaData struct {
	Level  string            `json:"level"`
	Limits []zhipuLimitEntry `json:"limits"`
}

type zhipuLimitEntry struct {
	Type          string  `json:"type"`
	Percentage    float64 `json:"percentage"`
	NextResetTime *int64  `json:"nextResetTime,omitempty"`
}

// zhipuWindowRow is the normalized per-window row stored in RawData.
type zhipuWindowRow struct {
	Window      string  `json:"window"`
	UsedPercent float64 `json:"usedPercent"`
	Status      string  `json:"status"`
	ResetAt     *string `json:"resetAt,omitempty"`
}

type ZhipuQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewZhipuQuotaChecker(httpClient *httpclient.HttpClient) *ZhipuQuotaChecker {
	return &ZhipuQuotaChecker{httpClient: httpClient}
}

// zhipuAPIKey returns the first non-empty API key configured on the channel,
// checking APIKey first then walking APIKeys and skipping blank entries.
func zhipuAPIKey(ch *ent.Channel) string {
	if apiKey := strings.TrimSpace(ch.Credentials.APIKey); apiKey != "" {
		return apiKey
	}

	for _, candidate := range ch.Credentials.APIKeys {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func (c *ZhipuQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := zhipuAPIKey(ch)
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	request := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(buildZhipuQuotaURL(ch.BaseURL)).
		WithHeader("Authorization", apiKey).
		WithHeader("Content-Type", "application/json").
		WithHeader("Accept-Language", "en-US,en").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("zhipu quota request failed: %w", err)
	}

	return parseZhipuQuotaResponse(resp.Body)
}

func (c *ZhipuQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeZhipu ||
		ch.Type == channel.TypeZhipuAnthropic
}

func buildZhipuQuotaURL(baseURL string) string {
	quotaBase := zhipuQuotaBaseFromChannelURL(baseURL)
	return quotaBase + "/api/monitor/usage/quota/limit"
}

// zhipuQuotaBaseFromChannelURL maps a channel base URL to the quota API host.
// All ZhiPu coding plan channels use open.bigmodel.cn.
func zhipuQuotaBaseFromChannelURL(baseURL string) string {
	return zhipuDefaultQuotaBaseURL
}

func parseZhipuQuotaResponse(body []byte) (QuotaData, error) {
	var response zhipuQuotaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse zhipu quota response: %w", err)
	}

	if !response.Success {
		msg := response.Msg
		if msg == "" {
			msg = fmt.Sprintf("API error code %d", response.Code)
		}
		return QuotaData{}, fmt.Errorf("zhipu API error: %s", msg)
	}

	if response.Data == nil {
		return QuotaData{}, fmt.Errorf("zhipu quota response contains no data")
	}

	// Filter for TOKENS_LIMIT entries only.
	var tokenLimits []zhipuLimitEntry
	for _, entry := range response.Data.Limits {
		if strings.EqualFold(entry.Type, "TOKENS_LIMIT") {
			tokenLimits = append(tokenLimits, entry)
		}
	}

	if len(tokenLimits) == 0 {
		return QuotaData{}, fmt.Errorf("zhipu quota response contains no TOKENS_LIMIT entries")
	}

	// The entry without nextResetTime is always the 5-hour bucket (the rolling
	// 5h window reports no reset time at 0% usage). When all entries have a
	// reset time, trust API return order: index 0 → five_hour, index 1 → weekly.
	windowNames := []string{"five_hour", "weekly_limit"}
	ordered := orderZhipuBuckets(tokenLimits)

	overallStatus := "available"
	var limits []QuotaLimitStatus
	var nextResetAt *time.Time
	rows := make([]zhipuWindowRow, 0, min(len(ordered), len(windowNames)))

	for i, entry := range ordered {
		if i >= len(windowNames) {
			break
		}

		windowName := windowNames[i]
		usedPercent := entry.Percentage
		ratio := usedPercent / 100.0
		status := zhipuStatusForRatio(ratio)

		var resetAt *time.Time
		if entry.NextResetTime != nil && *entry.NextResetTime > 0 {
			t := time.UnixMilli(*entry.NextResetTime)
			resetAt = &t
			if nextResetAt == nil || t.Before(*nextResetAt) {
				nextResetAt = &t
			}
		}

		limits = append(limits, NewTokenLimitStatus(status, ratio, resetAt))
		overallStatus = worseZhipuStatus(overallStatus, status)

		var resetAtStr *string
		if resetAt != nil {
			s := resetAt.Format(time.RFC3339)
			resetAtStr = &s
		}

		rows = append(rows, zhipuWindowRow{
			Window:      windowName,
			UsedPercent: usedPercent,
			Status:      status,
			ResetAt:     resetAtStr,
		})
	}

	rawData := map[string]any{
		"rows":  rows,
		"level": response.Data.Level,
	}

	return QuotaData{
		Status:       overallStatus,
		ProviderType: "zhipu",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(overallStatus),
		Limits:       limits,
	}, nil
}

func zhipuStatusForRatio(ratio float64) string {
	if ratio >= 1.0 {
		return "exhausted"
	}
	if ratio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

func worseZhipuStatus(a, b string) string {
	rank := map[string]int{"available": 0, "warning": 1, "exhausted": 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

// orderZhipuBuckets returns the TOKENS_LIMIT entries ordered so the 5-hour
// bucket comes first. A bucket without nextResetTime is the 5-hour bucket
// (the rolling 5h window omits reset time at 0% usage). If all buckets have
// a reset time, the API return order is trusted.
func orderZhipuBuckets(entries []zhipuLimitEntry) []zhipuLimitEntry {
	var withoutReset []zhipuLimitEntry
	var withReset []zhipuLimitEntry
	for _, e := range entries {
		if e.NextResetTime == nil || *e.NextResetTime <= 0 {
			withoutReset = append(withoutReset, e)
		} else {
			withReset = append(withReset, e)
		}
	}
	return append(withoutReset, withReset...)
}
