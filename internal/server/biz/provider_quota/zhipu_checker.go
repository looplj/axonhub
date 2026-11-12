package provider_quota

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const zhipuDefaultQuotaBaseURL = "https://open.bigmodel.cn"

// Window identifiers of the ZhiPu-family quota API. They are part of the
// stored payload, so they must stay stable.
const (
	zhipuWindowFiveHour = "five_hour"
	zhipuWindowWeekly   = "weekly_limit"
)

// Window units reported by the CREDIT_LIMIT entries of the GLM coding plan.
const (
	zhipuCreditUnitHour = 3
	zhipuCreditUnitWeek = 6
)

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

// zhipuLimitEntry covers both payload shapes of the family API: the
// TOKENS_LIMIT entries of api.z.ai report a percentage only, while the
// CREDIT_LIMIT entries of the GLM coding plan also carry the window shape
// (unit/number) and the credit amounts.
type zhipuLimitEntry struct {
	Type          string  `json:"type"`
	Percentage    float64 `json:"percentage"`
	NextResetTime *int64  `json:"nextResetTime,omitempty"`
	Unit          *int64  `json:"unit,omitempty"`
	Number        *int64  `json:"number,omitempty"`
	Usage         *int64  `json:"usage,omitempty"`
	CurrentValue  *int64  `json:"currentValue,omitempty"`
	Remaining     *int64  `json:"remaining,omitempty"`
}

// zhipuWindowRow is the normalized per-window row stored in RawData.
type zhipuWindowRow struct {
	Window      string  `json:"window"`
	UsedPercent float64 `json:"usedPercent"`
	Status      string  `json:"status"`
	ResetAt     *string `json:"resetAt,omitempty"`
	Usage       *int64  `json:"usage,omitempty"`
	Used        *int64  `json:"used,omitempty"`
	Remaining   *int64  `json:"remaining,omitempty"`
}

// zhipuAccountSnapshot is one parsed, still account-less quota payload.
type zhipuAccountSnapshot struct {
	Level       string
	Rows        []zhipuWindowRow
	Status      string
	Ready       bool
	NextResetAt *time.Time
}

// zhipuWindowSpec names a quota window of the family API.
type zhipuWindowSpec struct {
	Name   string
	Label  string
	Length time.Duration
}

var (
	zhipuFiveHourSpec = zhipuWindowSpec{Name: zhipuWindowFiveHour, Label: QuotaWindow5h, Length: 5 * time.Hour}
	zhipuWeeklySpec   = zhipuWindowSpec{Name: zhipuWindowWeekly, Label: QuotaWindowWeekly, Length: 7 * 24 * time.Hour}
)

type ZhipuQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewZhipuQuotaChecker(httpClient *httpclient.HttpClient) *ZhipuQuotaChecker {
	return &ZhipuQuotaChecker{httpClient: httpClient}
}

func (c *ZhipuQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	return collectZhipuFamilyQuota(ctx, c.httpClient, ch, "zhipu", buildZhipuQuotaURL(ch.BaseURL))
}

func (c *ZhipuQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeZhipu ||
		ch.Type == channel.TypeZhipuAnthropic
}

// collectZhipuFamilyQuota queries the quota endpoint once per API key through
// the shared per-key collector, so every key of the channel is an independent
// quota account.
func collectZhipuFamilyQuota(
	ctx context.Context,
	httpClient *httpclient.HttpClient,
	ch *ent.Channel,
	providerType string,
	quotaURL string,
) (QuotaData, error) {
	hc := httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = httpClient.WithProxy(ch.Settings.Proxy)
	}

	return CollectAPIKeyQuotas(ctx, ch, providerType, time.Now(), func(ctx context.Context, _ *ent.Channel, apiKey string) (QuotaData, error) {
		snapshot, err := fetchZhipuAccountQuota(ctx, hc, quotaURL, apiKey)
		if err != nil {
			return QuotaData{}, err
		}

		return zhipuSnapshotQuotaData(providerType, snapshot), nil
	})
}

func fetchZhipuAccountQuota(
	ctx context.Context,
	hc *httpclient.HttpClient,
	quotaURL string,
	apiKey string,
) (zhipuAccountSnapshot, error) {
	request := httpclient.NewRequestBuilder().
		WithMethod("GET").
		WithURL(quotaURL).
		WithHeader("Authorization", apiKey).
		WithHeader("Content-Type", "application/json").
		WithHeader("Accept-Language", "en-US,en").
		Build()

	resp, err := hc.Do(ctx, request)
	if err != nil {
		return zhipuAccountSnapshot{}, fmt.Errorf("zhipu quota request failed: %w", err)
	}

	return parseZhipuAccountSnapshot(resp.Body)
}

// zhipuSnapshotQuotaData turns one parsed account snapshot into that account's
// quota result. Limits drive routing inside the account (both windows AND),
// while RawData keeps the level and the rows the UI renders, credit amounts
// included.
func zhipuSnapshotQuotaData(providerType string, snapshot zhipuAccountSnapshot) QuotaData {
	return NormalizeQuotaData(QuotaData{
		Status:       snapshot.Status,
		ProviderType: providerType,
		RawData: map[string]any{
			"level": snapshot.Level,
			"rows":  snapshot.Rows,
		},
		NextResetAt: snapshot.NextResetAt,
		Ready:       snapshot.Ready,
		Limits:      zhipuWindowLimits(snapshot.Rows),
	})
}

// zhipuWindowLimits reports one limit per window of an account, so the UI keeps
// showing both windows and routing ANDs them.
func zhipuWindowLimits(rows []zhipuWindowRow) []QuotaLimitStatus {
	limits := make([]QuotaLimitStatus, 0, len(rows))

	for _, row := range rows {
		spec, ok := zhipuWindowSpecByName(row.Window)
		if !ok {
			continue
		}

		limits = append(limits, NewTokenLimitStatus(row.Status, row.UsedPercent/100, zhipuRowResetAt(row)).
			WithWindow(spec.Label, spec.Length))
	}

	return limits
}

func zhipuRowResetAt(row zhipuWindowRow) *time.Time {
	if row.ResetAt == nil {
		return nil
	}

	resetAt, err := time.Parse(time.RFC3339, *row.ResetAt)
	if err != nil {
		return nil
	}

	return lo.ToPtr(resetAt)
}

// parseZhipuQuotaResponse parses a single-account ZhiPu payload.
func parseZhipuQuotaResponse(body []byte) (QuotaData, error) {
	return parseZhipuFamilyQuotaResponse(body, "zhipu")
}

// parseZhipuFamilyQuotaResponse parses the ZhiPu-family quota API payload
// (shared by open.bigmodel.cn and api.z.ai) under the given provider type. It
// covers the single-account shape; the checkers use collectZhipuFamilyQuota,
// which parses every key of the channel through the same snapshot.
func parseZhipuFamilyQuotaResponse(body []byte, providerType string) (QuotaData, error) {
	snapshot, err := parseZhipuAccountSnapshot(body)
	if err != nil {
		return QuotaData{}, err
	}

	return zhipuSnapshotQuotaData(providerType, snapshot), nil
}

func parseZhipuAccountSnapshot(body []byte) (zhipuAccountSnapshot, error) {
	var response zhipuQuotaResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return zhipuAccountSnapshot{}, fmt.Errorf("failed to parse zhipu quota response: %w", err)
	}

	if !response.Success {
		msg := response.Msg
		if msg == "" {
			msg = fmt.Sprintf("API error code %d", response.Code)
		}

		return zhipuAccountSnapshot{}, fmt.Errorf("zhipu API error: %s", msg)
	}

	if response.Data == nil {
		return zhipuAccountSnapshot{}, fmt.Errorf("zhipu quota response contains no data")
	}

	windows := zhipuResponseWindows(response.Data.Limits)
	if len(windows) == 0 {
		return zhipuAccountSnapshot{}, fmt.Errorf("zhipu quota response contains no TOKENS_LIMIT or CREDIT_LIMIT entries")
	}

	snapshot := zhipuAccountSnapshot{Level: response.Data.Level, Status: "available"}

	for _, window := range windows {
		row, resetAt := window.toRow()
		snapshot.Rows = append(snapshot.Rows, row)
		snapshot.Status = worseZhipuStatus(snapshot.Status, row.Status)

		if resetAt != nil && (snapshot.NextResetAt == nil || resetAt.Before(*snapshot.NextResetAt)) {
			snapshot.NextResetAt = resetAt
		}
	}

	snapshot.Ready = IsReadyStatus(snapshot.Status)

	return snapshot, nil
}

// zhipuResponseWindow is one limit entry together with the window it belongs to.
type zhipuResponseWindow struct {
	spec  zhipuWindowSpec
	entry zhipuLimitEntry
}

func (w zhipuResponseWindow) toRow() (zhipuWindowRow, *time.Time) {
	ratio := w.entry.Percentage / 100.0
	row := zhipuWindowRow{
		Window:      w.spec.Name,
		UsedPercent: w.entry.Percentage,
		Status:      zhipuStatusForRatio(ratio),
		Usage:       w.entry.Usage,
		Used:        w.entry.CurrentValue,
		Remaining:   w.entry.Remaining,
	}

	var resetAt *time.Time
	if w.entry.NextResetTime != nil && *w.entry.NextResetTime > 0 {
		reset := time.UnixMilli(*w.entry.NextResetTime)
		resetAt = &reset
		row.ResetAt = lo.ToPtr(reset.Format(time.RFC3339))
	}

	return row, resetAt
}

// zhipuResponseWindows normalizes the limit entries of one account payload.
// CREDIT_LIMIT entries (GLM coding plan) name their window through unit/number;
// TOKENS_LIMIT entries (api.z.ai) keep the historical ordering rule, where the
// entry without a reset time is the rolling 5-hour bucket. Entries whose window
// stays unknown fall back to the API order.
func zhipuResponseWindows(limits []zhipuLimitEntry) []zhipuResponseWindow {
	creditEntries := lo.Filter(limits, func(entry zhipuLimitEntry, _ int) bool {
		return strings.EqualFold(entry.Type, "CREDIT_LIMIT")
	})
	if len(creditEntries) > 0 {
		return zhipuBuildResponseWindows(creditEntries)
	}

	tokenEntries := lo.Filter(limits, func(entry zhipuLimitEntry, _ int) bool {
		return strings.EqualFold(entry.Type, "TOKENS_LIMIT")
	})

	return zhipuBuildResponseWindows(orderZhipuBuckets(tokenEntries))
}

func zhipuBuildResponseWindows(entries []zhipuLimitEntry) []zhipuResponseWindow {
	windows := make([]zhipuResponseWindow, 0, len(entries))

	for index, entry := range entries {
		spec, ok := zhipuCreditWindowSpec(entry)
		if !ok {
			spec, ok = zhipuPositionalSpec(index)
		}

		if !ok {
			continue
		}

		windows = append(windows, zhipuResponseWindow{spec: spec, entry: entry})
	}

	return windows
}

// zhipuCreditWindowSpec maps the unit/number pair of a CREDIT_LIMIT entry onto
// a known window. The GLM coding plan reports 5 hours (unit 3) and 1 week
// (unit 6); anything else falls back to the API order.
func zhipuCreditWindowSpec(entry zhipuLimitEntry) (zhipuWindowSpec, bool) {
	if entry.Unit == nil || entry.Number == nil || *entry.Number <= 0 {
		return zhipuWindowSpec{}, false
	}

	var length time.Duration

	switch *entry.Unit {
	case zhipuCreditUnitHour:
		length = time.Duration(*entry.Number) * time.Hour
	case zhipuCreditUnitWeek:
		length = time.Duration(*entry.Number) * 7 * 24 * time.Hour
	default:
		return zhipuWindowSpec{}, false
	}

	return zhipuWindowSpecForLength(length)
}

func zhipuWindowSpecForLength(length time.Duration) (zhipuWindowSpec, bool) {
	switch length {
	case zhipuFiveHourSpec.Length:
		return zhipuFiveHourSpec, true
	case zhipuWeeklySpec.Length:
		return zhipuWeeklySpec, true
	default:
		return zhipuWindowSpec{}, false
	}
}

func zhipuWindowSpecByName(name string) (zhipuWindowSpec, bool) {
	switch name {
	case zhipuWindowFiveHour:
		return zhipuFiveHourSpec, true
	case zhipuWindowWeekly:
		return zhipuWeeklySpec, true
	default:
		return zhipuWindowSpec{}, false
	}
}

func zhipuPositionalSpec(index int) (zhipuWindowSpec, bool) {
	switch index {
	case 0:
		return zhipuFiveHourSpec, true
	case 1:
		return zhipuWeeklySpec, true
	default:
		return zhipuWindowSpec{}, false
	}
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
