package provider_quota

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm/httpclient"
)

const zhipuDefaultQuotaBaseURL = "https://open.bigmodel.cn"

// zhipuAccountsAvailabilityGroup tags the limit of every API key of a
// multi-key channel. EvaluateQuotaRouting ORs the members of a single
// availability group, so a channel backed by several accounts stays routable
// while at least one of them still has quota, instead of being dropped as soon
// as the first account is exhausted.
const zhipuAccountsAvailabilityGroup = "zhipu_accounts"

// maxZhipuQuotaDisabledAccounts bounds how many parked (disabled) keys one
// check reports. Keys that can still serve are always checked, because their
// quota decides the channel status.
const maxZhipuQuotaDisabledAccounts = 32

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

// zhipuAccountQuota is the per-API-key snapshot stored in RawData under
// "accounts". The key itself never reaches the payload: only a short digest and
// the four trailing characters every channel UI already prints.
type zhipuAccountQuota struct {
	Ref      string           `json:"ref"`
	Suffix   string           `json:"suffix"`
	Disabled bool             `json:"disabled,omitempty"`
	Status   string           `json:"status"`
	Ready    bool             `json:"ready"`
	Level    string           `json:"level,omitempty"`
	Error    string           `json:"error,omitempty"`
	Rows     []zhipuWindowRow `json:"rows,omitempty"`
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

// collectZhipuFamilyQuota queries the quota endpoint once per API key and folds
// the answers into one channel payload. It only fails when no key could be read
// at all; a single unreachable account is recorded on that account instead.
func collectZhipuFamilyQuota(
	ctx context.Context,
	httpClient *httpclient.HttpClient,
	ch *ent.Channel,
	providerType string,
	quotaURL string,
) (QuotaData, error) {
	keys := zhipuChannelAPIKeys(ch)
	if len(keys) == 0 {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	hc := httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = httpClient.WithProxy(ch.Settings.Proxy)
	}

	disabled := zhipuDisabledKeySet(ch)

	// Serving keys decide the channel status, so every one of them is checked.
	// Parked keys are display-only and bounded, so a channel with a huge
	// disabled pool cannot stall the collection loop.
	enabledKeys := make([]string, 0, len(keys))
	disabledKeys := make([]string, 0, len(keys))

	for _, key := range keys {
		if disabled[key] {
			disabledKeys = append(disabledKeys, key)

			continue
		}

		enabledKeys = append(enabledKeys, key)
	}

	if len(disabledKeys) > maxZhipuQuotaDisabledAccounts {
		log.Warn(ctx, "zhipu quota check truncates disabled keys",
			log.Int("channel_id", ch.ID),
			log.Int("disabled_keys", len(disabledKeys)),
			log.Int("checked", maxZhipuQuotaDisabledAccounts),
		)
		disabledKeys = disabledKeys[:maxZhipuQuotaDisabledAccounts]
	}

	targets := slices.Concat(enabledKeys, disabledKeys)
	accounts := make([]zhipuAccountQuota, 0, len(targets))

	var (
		failures int
		firstErr error
	)

	for _, key := range targets {
		account := zhipuAccountQuota{
			Ref:      zhipuKeyRef(key),
			Suffix:   zhipuKeySuffix(key),
			Disabled: disabled[key],
		}

		snapshot, err := fetchZhipuAccountQuota(ctx, hc, quotaURL, key)
		if err != nil {
			failures++
			account.Status = "unknown"
			account.Error = err.Error()
			accounts = append(accounts, account)

			if firstErr == nil {
				firstErr = err
			}

			continue
		}

		account.Status = snapshot.Status
		account.Ready = snapshot.Ready
		account.Level = snapshot.Level
		account.Rows = snapshot.Rows
		accounts = append(accounts, account)
	}

	if len(accounts) == 0 || failures == len(accounts) {
		// Every key failed: keep the underlying reason (the quota service logs
		// it and stores the error code) instead of a generic summary.
		return QuotaData{}, fmt.Errorf("zhipu quota check failed for all %d keys: %w", len(keys), firstErr)
	}

	return buildZhipuQuotaData(providerType, accounts), nil
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

// buildZhipuQuotaData folds the per-account snapshots into the channel payload.
// Limits drive routing: a single-key channel keeps one limit per window (AND
// semantics), while a multi-key channel reports the binding window of every
// account inside one availability group (OR semantics across keys). RawData
// carries the per-account rows the UI renders.
func buildZhipuQuotaData(providerType string, accounts []zhipuAccountQuota) QuotaData {
	usable := lo.Filter(accounts, func(account zhipuAccountQuota, _ int) bool {
		return !account.Disabled && account.Error == "" && len(account.Rows) > 0
	})

	status, ready := zhipuAggregateStatus(usable)

	return NormalizeQuotaData(QuotaData{
		Status:       status,
		ProviderType: providerType,
		RawData: map[string]any{
			"level":    zhipuAggregateLevel(accounts),
			"rows":     zhipuWorstRows(usable),
			"accounts": accounts,
		},
		NextResetAt: zhipuEarliestReset(usable),
		Ready:       ready,
		Limits:      zhipuChannelLimits(accounts, usable),
	})
}

// zhipuAggregateStatus reports the channel status of the serving accounts: only
// a pool without a single usable account is exhausted.
func zhipuAggregateStatus(usable []zhipuAccountQuota) (string, bool) {
	if len(usable) == 0 {
		// Every key is disabled locally or unreadable, so nothing can serve.
		return "exhausted", false
	}

	status := ""
	for _, account := range usable {
		if account.Status == "exhausted" {
			continue
		}

		if status == "" {
			status = account.Status

			continue
		}

		status = worseZhipuStatus(status, account.Status)
	}

	if status == "" {
		return "exhausted", false
	}

	return status, IsReadyStatus(status)
}

// zhipuChannelLimits keeps the historical single-key shape and switches to one
// binding limit per account as soon as the channel carries several keys.
func zhipuChannelLimits(accounts, usable []zhipuAccountQuota) []QuotaLimitStatus {
	if len(accounts) == 1 {
		return zhipuWindowLimits(accounts[0])
	}

	return zhipuBindingLimits(usable)
}

// zhipuWindowLimits reports one limit per window of a single-key channel, so
// the UI keeps showing both windows and routing ANDs them.
func zhipuWindowLimits(account zhipuAccountQuota) []QuotaLimitStatus {
	limits := make([]QuotaLimitStatus, 0, len(account.Rows))

	for _, row := range account.Rows {
		spec, ok := zhipuWindowSpecByName(row.Window)
		if !ok {
			continue
		}

		limits = append(limits, NewTokenLimitStatus(row.Status, row.UsedPercent/100, zhipuRowResetAt(row)).
			WithWindow(spec.Label, spec.Length))
	}

	return limits
}

// zhipuBindingLimits reports the binding window of every usable account inside
// one availability group. The group makes the routing evaluator OR the
// accounts, while the binding window keeps the AND of the two windows inside a
// single account. Period starts stay unset: the usage-log cost behind them is
// recorded per channel, so it cannot be attributed to one account.
func zhipuBindingLimits(accounts []zhipuAccountQuota) []QuotaLimitStatus {
	limits := make([]QuotaLimitStatus, 0, len(accounts))

	for _, account := range accounts {
		row, spec, ok := zhipuBindingRow(account.Rows)
		if !ok {
			continue
		}

		limits = append(limits, QuotaLimitStatus{
			Type:              QuotaLimitTypeToken,
			Status:            row.Status,
			UsageRatio:        row.UsedPercent / 100,
			Ready:             IsReadyStatus(row.Status),
			NextResetAt:       zhipuRowResetAt(row),
			AvailabilityGroup: zhipuAccountsAvailabilityGroup,
			Window:            spec.Label,
			Account:           account.Suffix,
		})
	}

	return limits
}

// zhipuBindingRow returns the window of an account that limits it most.
func zhipuBindingRow(rows []zhipuWindowRow) (zhipuWindowRow, zhipuWindowSpec, bool) {
	var (
		binding zhipuWindowRow
		spec    zhipuWindowSpec
		found   bool
	)

	for _, row := range rows {
		candidate, ok := zhipuWindowSpecByName(row.Window)
		if !ok {
			continue
		}

		if !found || row.UsedPercent > binding.UsedPercent {
			binding = row
			spec = candidate
			found = true
		}
	}

	return binding, spec, found
}

// zhipuWorstRows reports the most used account per window; it is what the
// single-row badge and the channel list fall back to.
func zhipuWorstRows(accounts []zhipuAccountQuota) []zhipuWindowRow {
	rows := make([]zhipuWindowRow, 0, 2)

	for _, spec := range []zhipuWindowSpec{zhipuFiveHourSpec, zhipuWeeklySpec} {
		var worst *zhipuWindowRow

		for _, account := range accounts {
			for _, row := range account.Rows {
				if row.Window != spec.Name {
					continue
				}

				if worst == nil || row.UsedPercent > worst.UsedPercent {
					candidate := row
					worst = &candidate
				}
			}
		}

		if worst != nil {
			rows = append(rows, *worst)
		}
	}

	return rows
}

func zhipuAggregateLevel(accounts []zhipuAccountQuota) string {
	for _, account := range accounts {
		if account.Level != "" {
			return account.Level
		}
	}

	return ""
}

func zhipuEarliestReset(accounts []zhipuAccountQuota) *time.Time {
	var earliest *time.Time

	for _, account := range accounts {
		for _, row := range account.Rows {
			resetAt := zhipuRowResetAt(row)
			if resetAt == nil {
				continue
			}

			if earliest == nil || resetAt.Before(*earliest) {
				earliest = resetAt
			}
		}
	}

	return earliest
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
// which calls the per-account parser for every key of the channel.
func parseZhipuFamilyQuotaResponse(body []byte, providerType string) (QuotaData, error) {
	snapshot, err := parseZhipuAccountSnapshot(body)
	if err != nil {
		return QuotaData{}, err
	}

	return buildZhipuQuotaData(providerType, []zhipuAccountQuota{{
		Status: snapshot.Status,
		Ready:  snapshot.Ready,
		Level:  snapshot.Level,
		Rows:   snapshot.Rows,
	}}), nil
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

// zhipuChannelAPIKeys returns the non-empty API keys of the channel in order,
// without duplicates.
func zhipuChannelAPIKeys(ch *ent.Channel) []string {
	candidates := ch.Credentials.GetAllAPIKeys()
	keys := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))

	for _, candidate := range candidates {
		key := strings.TrimSpace(candidate)
		if key == "" {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	return keys
}

// zhipuDisabledKeySet returns the keys currently disabled on the channel.
func zhipuDisabledKeySet(ch *ent.Channel) map[string]bool {
	disabled := make(map[string]bool, len(ch.DisabledAPIKeys))

	for _, entry := range ch.DisabledAPIKeys {
		key := strings.TrimSpace(entry.Key)
		if key == "" || entry.IsExpired() {
			continue
		}

		disabled[key] = true
	}

	return disabled
}

// zhipuKeyRef is a stable, non-reversible identity of a key inside the payload.
func zhipuKeyRef(key string) string {
	digest := sha256.Sum256([]byte("zhipu:" + key))

	return hex.EncodeToString(digest[:])[:8]
}

// zhipuKeySuffix is the trailing fragment the channel UI already prints when it
// masks a key.
func zhipuKeySuffix(key string) string {
	runes := []rune(key)
	if len(runes) <= 4 {
		return key
	}

	return string(runes[len(runes)-4:])
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
