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

const minimaxDefaultBaseURL = "https://www.minimaxi.com"

// minimaxResponse matches the Minimax token plan remains API response.
type minimaxResponse struct {
	ModelRemains []minimaxModelRemain `json:"model_remains"`
	BaseResp     minimaxBaseResp      `json:"base_resp"`
}

type minimaxBaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type minimaxModelRemain struct {
	ModelName                       string `json:"model_name"`
	StartTime                       int64  `json:"start_time"`
	EndTime                         int64  `json:"end_time"`
	RemainsTime                     int64  `json:"remains_time"`
	CurrentIntervalStatus           int    `json:"current_interval_status"`
	CurrentIntervalRemainingPercent int    `json:"current_interval_remaining_percent"`
	CurrentIntervalBoostPermille    int    `json:"current_interval_boost_permille"`
	WeeklyStartTime                 int64  `json:"weekly_start_time"`
	WeeklyEndTime                   int64  `json:"weekly_end_time"`
	WeeklyRemainsTime               int64  `json:"weekly_remains_time"`
	CurrentWeeklyStatus             int    `json:"current_weekly_status"`
	CurrentWeeklyRemainingPercent   int    `json:"current_weekly_remaining_percent"`
	WeeklyBoostPermille             int    `json:"weekly_boost_permille"`
}

// minimaxModelRow is the normalized per-model row stored in RawData.
type minimaxModelRow struct {
	ModelName            string  `json:"modelName"`
	IntervalUsedPercent  float64 `json:"intervalUsedPercent"`
	IntervalTotalPercent float64 `json:"intervalTotalPercent"`
	IntervalPercent      float64 `json:"intervalPercent"`
	IntervalStatus       string  `json:"intervalStatus"`
	IntervalResetAt      *string `json:"intervalResetAt,omitempty"`
	WeeklyUsedPercent    float64 `json:"weeklyUsedPercent"`
	WeeklyTotalPercent   float64 `json:"weeklyTotalPercent"`
	WeeklyPercent        float64 `json:"weeklyPercent"`
	WeeklyStatus         string  `json:"weeklyStatus"`
	WeeklyResetAt        *string `json:"weeklyResetAt,omitempty"`
	WeeklyBoostPermille  int     `json:"weeklyBoostPermille,omitempty"`
}

type MinimaxQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewMinimaxQuotaChecker(httpClient *httpclient.HttpClient) *MinimaxQuotaChecker {
	return &MinimaxQuotaChecker{httpClient: httpClient}
}

func (c *MinimaxQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey == "" && len(ch.Credentials.APIKeys) > 0 {
		apiKey = strings.TrimSpace(ch.Credentials.APIKeys[0])
	}
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	request := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(buildMinimaxQuotaURL(ch.BaseURL)).
		WithBearerToken(apiKey).
		WithHeader("Content-Type", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("minimax quota request failed: %w", err)
	}

	return parseMinimaxResponse(resp.Body)
}

func (c *MinimaxQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeMinimax || ch.Type == channel.TypeMinimaxAnthropic
}

func buildMinimaxQuotaURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return minimaxDefaultBaseURL + "/v1/token_plan/remains"
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return minimaxDefaultBaseURL + "/v1/token_plan/remains"
	}

	return fmt.Sprintf("%s://%s/v1/token_plan/remains", parsed.Scheme, parsed.Host)
}

// minimaxTotalPercent converts boost_permille to a total percent.
// e.g. 1500 → 150.0. Returns 100.0 if permille is 0 or absent.
func minimaxTotalPercent(boostPermille int) float64 {
	if boostPermille > 0 {
		return float64(boostPermille) / 10.0
	}
	return 100.0
}

// minimaxUtilization computes the utilization ratio from remaining percent.
// Formula: (100 - remaining%) / 100
// Returns a ratio where 1.0 = fully used.
func minimaxUtilization(remainingPercent int) float64 {
	used := 100.0 - float64(remainingPercent)
	if used < 0 {
		used = 0
	}
	return used / 100.0
}

// minimaxStatusForRatio determines status from the API status code and usage ratio.
// ratio is used/total (e.g. 3/150 = 0.02). Thresholds: >=1.0 = exhausted, >=0.8 = warning.
func minimaxStatusForRatio(apiStatus int, ratio float64) string {
	if apiStatus == 3 {
		return "exhausted"
	}
	if ratio >= 1.0 {
		return "exhausted"
	}
	if ratio >= WarningThresholdRatio {
		return "warning"
	}
	if apiStatus == 1 {
		return "available"
	}
	return "unknown"
}

func parseMinimaxResponse(body []byte) (QuotaData, error) {
	var response minimaxResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return QuotaData{}, fmt.Errorf("failed to parse minimax quota response: %w", err)
	}

	if response.BaseResp.StatusCode != 0 {
		return QuotaData{}, fmt.Errorf("minimax API error: %s (code %d)", response.BaseResp.StatusMsg, response.BaseResp.StatusCode)
	}

	if len(response.ModelRemains) == 0 {
		return QuotaData{}, fmt.Errorf("minimax quota response contains no model remains")
	}

	// Find the "general" model entry.
	var generalModel *minimaxModelRemain
	for i := range response.ModelRemains {
		if response.ModelRemains[i].ModelName == "general" {
			generalModel = &response.ModelRemains[i]
			break
		}
	}
	if generalModel == nil {
		return QuotaData{}, fmt.Errorf("minimax quota response contains no general model")
	}

	overallStatus := "available"
	var limits []QuotaLimitStatus
	var nextResetAt *time.Time
	rows := make([]minimaxModelRow, 0, len(response.ModelRemains))

	{
		model := generalModel

		// Interval (5h) window
		// used = 100 - remaining; bar = used / total; ratio = bar / 100.
		intervalTotalPercent := minimaxTotalPercent(model.CurrentIntervalBoostPermille)
		intervalUsedPercent := 100.0 - float64(model.CurrentIntervalRemainingPercent)
		intervalBarPercent := intervalUsedPercent / intervalTotalPercent * 100.0
		intervalRatio := intervalBarPercent / 100.0
		intervalStatus := minimaxStatusForRatio(model.CurrentIntervalStatus, intervalRatio)

		var intervalResetAt *time.Time
		if model.EndTime > 0 {
			t := time.UnixMilli(model.EndTime)
			intervalResetAt = &t
			if nextResetAt == nil || t.Before(*nextResetAt) {
				nextResetAt = &t
			}
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      intervalStatus,
			UsageRatio:  intervalRatio,
			Ready:       IsReadyStatus(intervalStatus),
			NextResetAt: intervalResetAt,
		})

		overallStatus = worseStatus(overallStatus, intervalStatus)

		// Weekly window: only when status == 1; status 2/3 means no weekly limit for this plan.
		var weeklyTotalPercent, weeklyUsedPercent, weeklyRatio, weeklyBarPercent float64
		var weeklyStatus string
		var weeklyResetAt *time.Time
		var weeklyResetAtStr *string

		if model.CurrentWeeklyStatus == 1 {
			weeklyTotalPercent = minimaxTotalPercent(model.WeeklyBoostPermille)
			weeklyUsedPercent = 100.0 - float64(model.CurrentWeeklyRemainingPercent)
			weeklyBarPercent = weeklyUsedPercent / weeklyTotalPercent * 100.0
			weeklyRatio = weeklyBarPercent / 100.0
			weeklyStatus = minimaxStatusForRatio(model.CurrentWeeklyStatus, weeklyRatio)

			if model.WeeklyEndTime > 0 {
				t := time.UnixMilli(model.WeeklyEndTime)
				weeklyResetAt = &t
				if nextResetAt == nil || t.Before(*nextResetAt) {
					nextResetAt = &t
				}
			}

			limits = append(limits, QuotaLimitStatus{
				Type:        QuotaLimitTypeToken,
				Status:      weeklyStatus,
				UsageRatio:  weeklyRatio,
				Ready:       IsReadyStatus(weeklyStatus),
				NextResetAt: weeklyResetAt,
			})

			overallStatus = worseStatus(overallStatus, weeklyStatus)

			if weeklyResetAt != nil {
				s := weeklyResetAt.Format(time.RFC3339)
				weeklyResetAtStr = &s
			}
		}

		// Build normalized row for RawData
		var intervalResetAtStr *string
		if intervalResetAt != nil {
			s := intervalResetAt.Format(time.RFC3339)
			intervalResetAtStr = &s
		}

		row := minimaxModelRow{
			ModelName:            model.ModelName,
			IntervalUsedPercent:  intervalUsedPercent,
			IntervalTotalPercent: intervalTotalPercent,
			IntervalPercent:      intervalBarPercent,
			IntervalStatus:       intervalStatus,
			IntervalResetAt:      intervalResetAtStr,
			WeeklyUsedPercent:    weeklyUsedPercent,
			WeeklyTotalPercent:   weeklyTotalPercent,
			WeeklyPercent:        weeklyBarPercent,
			WeeklyStatus:         weeklyStatus,
			WeeklyResetAt:        weeklyResetAtStr,
			WeeklyBoostPermille:  model.WeeklyBoostPermille,
		}

		rows = append(rows, row)
	}

	rawData := map[string]any{
		"rows": rows,
	}

	return QuotaData{
		Status:       overallStatus,
		ProviderType: "minimax",
		RawData:      rawData,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(overallStatus),
		Limits:       limits,
	}, nil
}

func worseStatus(a, b string) string {
	rank := map[string]int{"available": 0, "warning": 1, "exhausted": 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}
