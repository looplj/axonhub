package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	opencodeGoProviderType = "opencode_go"
	opencodeGoUsageURL     = "https://opencode.ai/zen/go/v1/usage"
)

// opencodeGoUsageResponse matches the OpenCode Go usage API response:
// GET /zen/go/v1/usage with Authorization: Bearer <api key>.
type opencodeGoUsageResponse struct {
	Usage opencodeGoUsageWindows `json:"usage"`
}

type opencodeGoUsageWindows struct {
	Rolling *opencodeGoUsageWindow `json:"rolling"`
	Weekly  *opencodeGoUsageWindow `json:"weekly"`
	Monthly *opencodeGoUsageWindow `json:"monthly"`
}

type opencodeGoUsageWindow struct {
	Percent  float64         `json:"percent"`
	ResetsAt json.RawMessage `json:"resetsAt"`
	// Status is informational today: all live responses report "ok" and window
	// status is derived from Percent thresholds (same as the old dashboard
	// scraper). The raw value is surfaced as api_status for observability until
	// the API's status vocabulary is known.
	Status string `json:"status"`
}

// OpenCodeGoUsageWindow is a normalized usage window from the OpenCode Go
// usage API, at 100-based usage percent with a computed reset deadline.
type OpenCodeGoUsageWindow struct {
	UsagePercent float64
	ResetInSec   float64
	ResetAt      time.Time
	APISubStatus string
}

// OpenCodeGoQuotaChecker fetches OpenCode Go quota status from the official
// usage API using the channel's upstream API key.
type OpenCodeGoQuotaChecker struct {
	httpClient *httpclient.HttpClient
	now        func() time.Time
}

// NewOpenCodeGoQuotaChecker creates a checker with the shared HTTP client.
func NewOpenCodeGoQuotaChecker(httpClient *httpclient.HttpClient) *OpenCodeGoQuotaChecker {
	return &OpenCodeGoQuotaChecker{
		httpClient: httpClient,
		now:        time.Now,
	}
}

// opencodeGoAPIKey returns the first usable API key configured on the channel.
// It reuses ChannelCredentials.GetAllAPIKeys for the legacy-key ordering and
// OAuth exclusion, keeping only the trim-and-skip-blank selection here.
func opencodeGoAPIKey(ch *ent.Channel) string {
	for _, candidate := range ch.Credentials.GetAllAPIKeys() {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

// CheckQuota fetches and parses OpenCode Go usage for the channel. Non-2xx
// responses surface as errors from the HTTP client with their status code.
func (c *OpenCodeGoQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := opencodeGoAPIKey(ch)
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(opencodeGoUsageURL).
		WithHeader("Authorization", "Bearer "+apiKey).
		WithHeader("Accept", "application/json").
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, request)
	if err != nil {
		return QuotaData{}, fmt.Errorf("opencode go usage request failed: %w", err)
	}

	return c.parseResponse(resp.Body)
}

// SupportsChannel reports whether the channel is an OpenCode Go variant.
func (c *OpenCodeGoQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeOpencodeGo || ch.Type == channel.TypeOpencodeGoAnthropic
}

func (c *OpenCodeGoQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	var parsed opencodeGoUsageResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return QuotaData{}, fmt.Errorf("parse OpenCode Go usage response: %w", err)
	}

	now := c.now()
	windows := make(map[string]OpenCodeGoUsageWindow, 3)
	for key, window := range map[string]*opencodeGoUsageWindow{
		"rolling": parsed.Usage.Rolling,
		"weekly":  parsed.Usage.Weekly,
		"monthly": parsed.Usage.Monthly,
	} {
		if window == nil {
			continue
		}

		resetAt, ok := parseOpenCodeGoResetsAt(window.ResetsAt)
		if !ok {
			continue
		}

		resetInSec := resetAt.Sub(now).Seconds()
		if resetInSec < 0 {
			resetInSec = 0
		}

		windows[key] = OpenCodeGoUsageWindow{
			UsagePercent: window.Percent,
			ResetInSec:   resetInSec,
			ResetAt:      resetAt,
			APISubStatus: window.Status,
		}
	}

	if len(windows) == 0 {
		return QuotaData{}, fmt.Errorf("could not parse OpenCode Go usage windows")
	}

	rawWindows := make(map[string]any, len(windows))
	limits := make([]QuotaLimitStatus, 0, len(windows))
	normalizedStatus := "available"
	var nextResetAt *time.Time

	for _, key := range []string{"rolling", "weekly", "monthly"} {
		window, ok := windows[key]
		if !ok {
			continue
		}

		usageRatio := window.UsagePercent / 100.0
		status := normalizeOpenCodeGoWindowStatus(usageRatio)
		if quotaStatusRank(status) > quotaStatusRank(normalizedStatus) {
			normalizedStatus = status
		}

		resetAt := window.ResetAt
		if nextResetAt == nil || resetAt.Before(*nextResetAt) {
			nextResetAt = &resetAt
		}

		rawWindows[key] = map[string]any{
			"usage_percent":     window.UsagePercent,
			"reset_in_seconds":  window.ResetInSec,
			"reset_time":        resetAt.Format(time.RFC3339),
			"status":            status,
			"percent_remaining": 100 - window.UsagePercent,
		}
		if window.APISubStatus != "" {
			if m, ok := rawWindows[key].(map[string]any); ok {
				m["api_status"] = window.APISubStatus
			}
		}

		resetAtCopy := resetAt
		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       IsReadyStatus(status),
			NextResetAt: &resetAtCopy,
			Window:      opencodeGoWindowLabel(key),
			PeriodStart: opencodeGoPeriodStart(key, resetAt),
		})
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: opencodeGoProviderType,
		RawData: map[string]any{
			"plan_type": "go",
			"windows":   rawWindows,
		},
		NextResetAt: nextResetAt,
		Ready:       IsReadyStatus(normalizedStatus),
		Limits:      limits,
	}, nil
}

// parseOpenCodeGoResetsAt parses the resetsAt value from the usage API. The
// API returns an RFC3339 timestamp with millisecond precision (e.g.
// "2026-08-12T11:24:29.905Z"); unix seconds are accepted as a fallback.
func parseOpenCodeGoResetsAt(raw json.RawMessage) (time.Time, bool) {
	if len(raw) == 0 {
		return time.Time{}, false
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		resetAt, err := time.Parse(time.RFC3339Nano, asString)
		if err != nil {
			return time.Time{}, false
		}
		return resetAt, true
	}

	var asSeconds float64
	if err := json.Unmarshal(raw, &asSeconds); err == nil {
		// json.Unmarshal already rejects NaN/Infinity tokens, so only a sane
		// magnitude guard is needed here. 1e15 is well beyond any plausible
		// timestamp in either unit.
		if asSeconds < 0 || asSeconds > 1e15 {
			return time.Time{}, false
		}
		// >= 1e12 is epoch milliseconds (year ~33,658 in seconds); seconds
		// never legitimately reach that magnitude.
		if asSeconds >= 1e12 {
			return time.UnixMilli(int64(asSeconds)), true
		}
		return time.Unix(int64(asSeconds), 0), true
	}

	return time.Time{}, false
}

// opencodeGoWindowLabel maps an API window key onto a normalized window label.
// The API's "rolling" window is the plan's 5 hour window, which is how the rest
// of the app already labels it.
func opencodeGoWindowLabel(key string) string {
	switch key {
	case "rolling":
		return QuotaWindow5h
	case "weekly":
		return QuotaWindowWeekly
	case "monthly":
		return QuotaWindowMonthly
	default:
		return key
	}
}

// opencodeGoPeriodStart returns the start of the window that ends at resetAt.
// The API only reports the reset deadline, so the start comes from the length
// of the window the key names. The monthly window steps back a calendar month
// instead of a fixed 30 days so it lands on the actual cycle boundary.
func opencodeGoPeriodStart(key string, resetAt time.Time) *time.Time {
	switch key {
	case "rolling":
		return PeriodStartFromReset(&resetAt, 5*time.Hour)
	case "weekly":
		return PeriodStartFromReset(&resetAt, 7*24*time.Hour)
	case "monthly":
		return PeriodStartFromMonthlyReset(&resetAt)
	default:
		return nil
	}
}

func normalizeOpenCodeGoWindowStatus(usageRatio float64) string {
	if usageRatio >= 1.0 {
		return "exhausted"
	}
	if usageRatio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

func quotaStatusRank(status string) int {
	switch status {
	case "exhausted":
		return 2
	case "warning":
		return 1
	case "available":
		return 0
	default:
		return -1
	}
}
