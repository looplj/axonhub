package provider_quota

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	clineProviderType        = "cline"
	clinePassModelPrefix     = "cline-pass/"
	clineQuotaDefaultBaseURL = "https://api.cline.bot"
	clineUsagePageLimit      = 100
	clineMaxUsagePages       = 100
	clineCostUnitsPerUSD     = int64(100_000_000)
	clineMaxResponseBodySize = 1 << 20
	clineUsageLimitsPath     = "/api/v1/users/me/plan/usage-limits"

	clineUsageLimitTypeFiveHour = "five_hour"
	clineUsageLimitTypeWeekly   = "weekly"
	clineUsageLimitTypeMonthly  = "monthly"

	clineUsageLimitsFetchStatusComplete    = "complete"
	clineUsageLimitsFetchStatusPartial     = "partial"
	clineUsageLimitsFetchStatusUnusable    = "unusable"
	clineUsageLimitsFetchStatusUnavailable = "unavailable"

	clineWindowSourceOfficialUsageLimits = "official_usage_limits"
	clineWindowSourceEstimatedCost       = "estimated_from_cost"
	clineWindowSourceEstimatedUsage      = "estimated_from_usage_window"
	clineWindowSourceUnavailable         = "unavailable"
)

type ClineQuotaChecker struct {
	httpClient *httpclient.HttpClient
	now        func() time.Time
}

type clineEnvelope[T any] struct {
	Data T `json:"data"`
}

type clineMeData struct {
	ID            string `json:"id"`
	Organizations []any  `json:"organizations,omitempty"`
}

type clineBalanceData struct {
	Balance *int64 `json:"balance,omitempty"`
}

type clinePlansResponse []clinePlan

type clinePlan struct {
	Type         string            `json:"type,omitempty"`
	Interval     string            `json:"interval,omitempty"`
	IsActive     bool              `json:"isActive,omitempty"`
	Entitlements clineEntitlements `json:"entitlements,omitzero"`
}

type clineEntitlements struct {
	ClinePass *clinePassEntitlement `json:"cline_pass,omitempty"`
}

type clinePassEntitlement struct {
	Enabled               bool                        `json:"enabled,omitempty"`
	InferenceCapThreshold *clineInferenceCapThreshold `json:"inferenceCapThreshold,omitempty"`
}

type clineInferenceCapThreshold struct {
	Last5HoursUsageCostUSDPerUser int64 `json:"last5HoursUsageCostUSDPerUser,omitempty"`
	Last7DaysUsageCostUSDPerUser  int64 `json:"last7daysUsageCostUSDPerUser,omitempty"`
	Last30DaysUsageCostUSDPerUser int64 `json:"last30daysUsageCostUSDPerUser,omitempty"`
}

type clineUsagesData struct {
	Items     []clineUsageItem `json:"items,omitempty"`
	NextToken string           `json:"nextToken,omitempty"`
}

type clineUsageItem struct {
	CreatedAt   string `json:"createdAt,omitempty"`
	CostUSD     int64  `json:"costUsd,omitempty"`
	CreditsUsed int64  `json:"creditsUsed,omitempty"`
}

type clineUsageLimitsData struct {
	Limits []clineUsageLimit `json:"limits,omitempty"`
}

type clineUsageLimit struct {
	Type        string
	PercentUsed *float64
	ResetsAt    string
}

func (l *clineUsageLimit) UnmarshalJSON(data []byte) error {
	*l = clineUsageLimit{}

	data = bytes.TrimSpace(data)
	if !json.Valid(data) {
		return fmt.Errorf("invalid Cline usage limit JSON")
	}
	if len(data) == 0 || data[0] != '{' {
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("failed to parse Cline usage limit: %w", err)
	}

	if raw, ok := fields["type"]; ok {
		_ = json.Unmarshal(raw, &l.Type)
	}
	if raw, ok := fields["percentUsed"]; ok {
		var value *float64
		if err := json.Unmarshal(raw, &value); err == nil {
			l.PercentUsed = value
		}
	}
	if raw, ok := fields["resetsAt"]; ok {
		_ = json.Unmarshal(raw, &l.ResetsAt)
	}

	return nil
}

type clineOfficialWindowLimit struct {
	UsageRatio  *float64
	NextResetAt *time.Time
}

type clineUsageLimitsFetchMeta struct {
	Status            string
	EntriesSeen       int
	RecognizedEntries int
	UsableWindows     int
	UsableFields      int
}

type clineWindow struct {
	key            string
	duration       time.Duration
	limitUnits     int64
	usedUnits      int64
	creditsUsed    int64
	itemsCount     int
	usageRatio     *float64
	costUsageRatio *float64
	usageSource    string
	nextResetAt    *time.Time
	resetSource    string
}

type clineUsageFetchMeta struct {
	Pages     int
	ItemsSeen int
	Truncated bool
}

type clineModelScope string

const (
	clineModelScopePassOnly clineModelScope = "cline_pass_only"
	clineModelScopeMixed    clineModelScope = "mixed"
	clineModelScopeDirect   clineModelScope = "direct_only"
	clineModelScopeUnknown  clineModelScope = "unknown"
)

func NewClineQuotaChecker(httpClient *httpclient.HttpClient) *ClineQuotaChecker {
	return &ClineQuotaChecker{
		httpClient: httpClient,
		now:        time.Now,
	}
}

func (c *ClineQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeCline
}

func (c *ClineQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := clineAPIKey(ch)
	if apiKey == "" {
		return QuotaData{}, fmt.Errorf("channel has no API key")
	}

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	var me clineEnvelope[clineMeData]
	if err := c.getJSON(ctx, hc, ch.BaseURL, "/api/v1/users/me", nil, apiKey, &me); err != nil {
		return QuotaData{}, fmt.Errorf("failed to read Cline user identity: %w", err)
	}
	if strings.TrimSpace(me.Data.ID) == "" {
		return QuotaData{}, fmt.Errorf("Cline user identity response missing id")
	}

	var plans clineEnvelope[clinePlansResponse]
	if err := c.getJSON(ctx, hc, ch.BaseURL, "/api/v1/plans", nil, apiKey, &plans); err != nil {
		return QuotaData{}, fmt.Errorf("failed to read Cline plans: %w", err)
	}
	threshold, planSummaries, hasClinePass := selectClinePassThreshold(plans.Data)

	var balance clineEnvelope[clineBalanceData]
	balancePath := "/api/v1/users/" + url.PathEscape(me.Data.ID) + "/balance"
	if err := c.getJSON(ctx, hc, ch.BaseURL, balancePath, nil, apiKey, &balance); err != nil {
		return QuotaData{}, fmt.Errorf("failed to read Cline balance: %w", err)
	}

	scope := classifyClineModelScope(ch)
	if scope == clineModelScopeDirect {
		return buildClineDirectOnlyQuota(balance.Data.Balance, planSummaries), nil
	}
	if !hasClinePass {
		return QuotaData{}, fmt.Errorf("Cline plans response does not include an active ClinePass threshold")
	}

	officialLimits, officialMeta := c.fetchUsageLimits(ctx, hc, ch.BaseURL, apiKey)

	items, fetchMeta, err := c.fetchUsageItems(ctx, hc, ch.BaseURL, me.Data.ID, apiKey)
	if err != nil {
		return QuotaData{}, err
	}

	return buildClineQuotaData(
		c.now(),
		scope,
		threshold,
		planSummaries,
		balance.Data.Balance,
		items,
		fetchMeta,
		officialLimits,
		officialMeta,
	), nil
}

func (c *ClineQuotaChecker) getJSON(ctx context.Context, hc *httpclient.HttpClient, baseURL, path string, query url.Values, apiKey string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, buildClineQuotaURL(baseURL, path, query), nil)
	if err != nil {
		return fmt.Errorf("failed to create Cline quota request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "axonhub/1.0")

	resp, err := clineNativeHTTPClient(hc).Do(req)
	if err != nil {
		return fmt.Errorf("Cline quota request failed during transport")
	}
	if resp == nil {
		return fmt.Errorf("Cline quota request failed during transport")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, clineMaxResponseBodySize))
		return clineHTTPStatusError(resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, clineMaxResponseBodySize))
	if err != nil {
		return fmt.Errorf("failed to read Cline quota response")
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("failed to parse Cline quota response")
	}

	return nil
}

func clineNativeHTTPClient(hc *httpclient.HttpClient) *http.Client {
	if hc == nil || hc.GetNativeClient() == nil {
		return http.DefaultClient
	}
	return hc.GetNativeClient()
}

func clineHTTPStatusError(statusCode int) error {
	if statusText := http.StatusText(statusCode); statusText != "" {
		return fmt.Errorf("HTTP %d %s", statusCode, statusText)
	}
	return fmt.Errorf("HTTP %d", statusCode)
}

func clineAPIKey(ch *ent.Channel) string {
	apiKey := strings.TrimSpace(ch.Credentials.APIKey)
	if apiKey != "" {
		return apiKey
	}

	for _, candidate := range ch.Credentials.APIKeys {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func buildClineQuotaURL(baseURL, path string, query url.Values) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = clineQuotaDefaultBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		parsed, _ = url.Parse(clineQuotaDefaultBaseURL)
	}

	scheme := parsed.Scheme
	if scheme == "" || scheme == "http" {
		scheme = "https"
	}

	host := parsed.Host
	if host == "" {
		fallback, _ := url.Parse(clineQuotaDefaultBaseURL)
		scheme = fallback.Scheme
		host = fallback.Host
	}

	result := url.URL{Scheme: scheme, Host: host, Path: path}
	if len(query) > 0 {
		result.RawQuery = query.Encode()
	}

	return result.String()
}

func classifyClineModelScope(ch *ent.Channel) clineModelScope {
	models := append([]string{}, ch.SupportedModels...)
	models = append(models, ch.ManualModels...)
	if len(models) == 0 {
		return clineModelScopeUnknown
	}

	hasPass := false
	hasDirect := false

	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if strings.HasPrefix(model, clinePassModelPrefix) {
			hasPass = true
		} else {
			hasDirect = true
		}
	}

	switch {
	case hasPass && hasDirect:
		return clineModelScopeMixed
	case hasPass:
		return clineModelScopePassOnly
	case hasDirect:
		return clineModelScopeDirect
	default:
		return clineModelScopeUnknown
	}
}

func selectClinePassThreshold(plans []clinePlan) (clineInferenceCapThreshold, []map[string]any, bool) {
	var selected clineInferenceCapThreshold
	var summaries []map[string]any
	found := false

	for _, plan := range plans {
		pass := plan.Entitlements.ClinePass
		if pass == nil || !pass.Enabled || pass.InferenceCapThreshold == nil || !plan.IsActive {
			continue
		}

		summaries = append(summaries, map[string]any{
			"type":     plan.Type,
			"interval": plan.Interval,
		})

		if !found {
			selected = *pass.InferenceCapThreshold
			found = true
		}
	}

	return selected, summaries, found
}

func (c *ClineQuotaChecker) fetchUsageLimits(
	ctx context.Context,
	hc *httpclient.HttpClient,
	baseURL string,
	apiKey string,
) (map[string]clineOfficialWindowLimit, clineUsageLimitsFetchMeta) {
	var response clineEnvelope[clineUsageLimitsData]
	if err := c.getJSON(ctx, hc, baseURL, clineUsageLimitsPath, nil, apiKey, &response); err != nil {
		return nil, clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusUnavailable}
	}
	return parseClineUsageLimits(response.Data.Limits)
}

func (c *ClineQuotaChecker) fetchUsageItems(ctx context.Context, hc *httpclient.HttpClient, baseURL, userID, apiKey string) ([]clineUsageItem, clineUsageFetchMeta, error) {
	var items []clineUsageItem
	meta := clineUsageFetchMeta{}
	cursor := ""
	cutoff := c.now().Add(-30 * 24 * time.Hour)
	path := "/api/v1/users/" + url.PathEscape(userID) + "/usages"

	for range clineMaxUsagePages {
		query := url.Values{}
		query.Set("limit", fmt.Sprintf("%d", clineUsagePageLimit))
		if cursor != "" {
			query.Set("cursor", cursor)
		}

		var resp clineEnvelope[clineUsagesData]
		if err := c.getJSON(ctx, hc, baseURL, path, query, apiKey, &resp); err != nil {
			return nil, meta, fmt.Errorf("failed to read Cline usages: %w", err)
		}

		meta.Pages++
		meta.ItemsSeen += len(resp.Data.Items)
		items = append(items, resp.Data.Items...)

		oldest := oldestClineUsageTime(resp.Data.Items)
		cursor = strings.TrimSpace(resp.Data.NextToken)
		if cursor == "" || len(resp.Data.Items) == 0 || (oldest != nil && oldest.Before(cutoff)) {
			return items, meta, nil
		}
	}

	meta.Truncated = true
	return items, meta, nil
}

func oldestClineUsageTime(items []clineUsageItem) *time.Time {
	var oldest *time.Time

	for _, item := range items {
		parsed, ok := parseClineTime(item.CreatedAt)
		if !ok {
			continue
		}
		if oldest == nil || parsed.Before(*oldest) {
			oldest = &parsed
		}
	}

	return oldest
}

func parseClineTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, false
	}

	return parsed, true
}

func parseClineUsageLimits(items []clineUsageLimit) (map[string]clineOfficialWindowLimit, clineUsageLimitsFetchMeta) {
	limits := make(map[string]clineOfficialWindowLimit, 3)
	meta := clineUsageLimitsFetchMeta{
		Status:      clineUsageLimitsFetchStatusUnusable,
		EntriesSeen: len(items),
	}

	for _, item := range items {
		key, ok := clineUsageLimitWindowKey(item.Type)
		if !ok {
			continue
		}
		meta.RecognizedEntries++

		limit := limits[key]
		if limit.UsageRatio == nil && item.PercentUsed != nil {
			ratio := *item.PercentUsed / 100
			if ratio < 0 {
				ratio = 0
			}
			if ratio > 1 {
				ratio = 1
			}
			limit.UsageRatio = &ratio
			meta.UsableFields++
		}

		if limit.NextResetAt == nil {
			if resetAt, valid := parseClineTime(item.ResetsAt); valid {
				limit.NextResetAt = &resetAt
				meta.UsableFields++
			}
		}

		if limit.UsageRatio != nil || limit.NextResetAt != nil {
			limits[key] = limit
		}
	}

	meta.UsableWindows = len(limits)
	switch {
	case meta.UsableFields == 0:
		meta.Status = clineUsageLimitsFetchStatusUnusable
	case meta.UsableWindows == 3 && meta.UsableFields == 6:
		meta.Status = clineUsageLimitsFetchStatusComplete
	default:
		meta.Status = clineUsageLimitsFetchStatusPartial
	}

	return limits, meta
}

func clineUsageLimitWindowKey(value string) (string, bool) {
	switch strings.TrimSpace(value) {
	case clineUsageLimitTypeFiveHour:
		return "last5h", true
	case clineUsageLimitTypeWeekly:
		return "last7d", true
	case clineUsageLimitTypeMonthly:
		return "last30d", true
	default:
		return "", false
	}
}

func buildClineQuotaData(
	now time.Time,
	scope clineModelScope,
	threshold clineInferenceCapThreshold,
	plans []map[string]any,
	balance *int64,
	items []clineUsageItem,
	usageFetchMeta clineUsageFetchMeta,
	officialLimits map[string]clineOfficialWindowLimit,
	officialMeta clineUsageLimitsFetchMeta,
) QuotaData {
	windows := []clineWindow{
		buildClineWindow(now, "last5h", 5*time.Hour, threshold.Last5HoursUsageCostUSDPerUser, items, officialLimits["last5h"]),
		buildClineWindow(now, "last7d", 7*24*time.Hour, threshold.Last7DaysUsageCostUSDPerUser, items, officialLimits["last7d"]),
		buildClineWindow(now, "last30d", 30*24*time.Hour, threshold.Last30DaysUsageCostUSDPerUser, items, officialLimits["last30d"]),
	}

	passStatus := worstClineStatus(windows)
	status := passStatus
	statusBasis := "cline_pass_windows"
	if scope != clineModelScopePassOnly && passStatus == "exhausted" {
		status = "warning"
		statusBasis = "mixed_pool_pass_exhausted"
	}

	return QuotaData{
		Status:       status,
		ProviderType: clineProviderType,
		Ready:        IsReadyStatus(status),
		NextResetAt:  earliestClineWindowReset(windows),
		Limits:       clineLimitStatuses(windows, scope == clineModelScopePassOnly),
		RawData: map[string]any{
			"model_scope":  string(scope),
			"status_basis": statusBasis,
			"pool":         "cline_pass",
			"pool_note":    "ClinePass is a separate provider; this quota applies to cline-pass/* models only.",
			"cost_scale":   clineCostUnitsPerUSD,
			"balance":      clineBalanceRawData(balance),
			"plans":        plans,
			"windows":      clineWindowsRawData(windows),
			"usage_fetch": map[string]any{
				"pages":      usageFetchMeta.Pages,
				"items_seen": usageFetchMeta.ItemsSeen,
				"truncated":  usageFetchMeta.Truncated,
			},
			"usage_limits_fetch": clineUsageLimitsFetchRawData(officialMeta),
		},
	}
}

func buildClineDirectOnlyQuota(balance *int64, plans []map[string]any) QuotaData {
	return QuotaData{
		Status:       "available",
		ProviderType: clineProviderType,
		Ready:        true,
		RawData: map[string]any{
			"model_scope":  string(clineModelScopeDirect),
			"status_basis": "direct_credit_balance_informational",
			"pool":         "direct_credit",
			"pool_note":    "Cline (usage-billing) credits balance is informational until exact pay-as-you-go exhaustion semantics are verified.",
			"balance":      clineBalanceRawData(balance),
			"plans":        plans,
		},
	}
}

func buildClineWindow(
	now time.Time,
	key string,
	duration time.Duration,
	limit int64,
	items []clineUsageItem,
	official clineOfficialWindowLimit,
) clineWindow {
	window := clineWindow{
		key:         key,
		duration:    duration,
		limitUnits:  limit,
		usageSource: clineWindowSourceUnavailable,
		resetSource: clineWindowSourceUnavailable,
	}
	start := now.Add(-duration)
	var earliest *time.Time

	for _, item := range items {
		createdAt, ok := parseClineTime(item.CreatedAt)
		if !ok || createdAt.Before(start) {
			continue
		}

		window.itemsCount++
		window.usedUnits += item.CostUSD
		window.creditsUsed += item.CreditsUsed
		if earliest == nil || createdAt.Before(*earliest) {
			earliest = &createdAt
		}
	}

	if window.limitUnits > 0 {
		costRatio := float64(window.usedUnits) / float64(window.limitUnits)
		window.costUsageRatio = &costRatio
		window.usageRatio = &costRatio
		window.usageSource = clineWindowSourceEstimatedCost
	}
	if earliest != nil {
		resetAt := earliest.Add(duration)
		window.nextResetAt = &resetAt
		window.resetSource = clineWindowSourceEstimatedUsage
	}
	if official.UsageRatio != nil {
		ratio := *official.UsageRatio
		window.usageRatio = &ratio
		window.usageSource = clineWindowSourceOfficialUsageLimits
	}
	if official.NextResetAt != nil && official.NextResetAt.After(now) {
		resetAt := *official.NextResetAt
		window.nextResetAt = &resetAt
		window.resetSource = clineWindowSourceOfficialUsageLimits
	}

	return window
}

func clineWindowStatus(window clineWindow) string {
	if window.usageRatio == nil {
		return "unknown"
	}

	ratio := *window.usageRatio
	if ratio >= 1.0 {
		return "exhausted"
	}
	if ratio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

func worstClineStatus(windows []clineWindow) string {
	status := "unknown"
	for _, window := range windows {
		status = worseQuotaStatus(status, clineWindowStatus(window))
	}
	return status
}

func worseQuotaStatus(a, b string) string {
	rank := map[string]int{"unknown": -1, "available": 0, "warning": 1, "exhausted": 2}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func clineLimitStatuses(windows []clineWindow, allowExhausted bool) []QuotaLimitStatus {
	limits := make([]QuotaLimitStatus, 0, len(windows))

	for _, window := range windows {
		usageRatio := 0.0
		if window.usageRatio != nil {
			usageRatio = *window.usageRatio
		}

		status := clineWindowStatus(window)
		if !allowExhausted && status == "exhausted" {
			status = "warning"
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       IsReadyStatus(status),
			NextResetAt: window.nextResetAt,
		})
	}

	return limits
}

func earliestClineWindowReset(windows []clineWindow) *time.Time {
	var earliest *time.Time

	for _, window := range windows {
		if window.nextResetAt == nil {
			continue
		}
		if earliest == nil || window.nextResetAt.Before(*earliest) {
			reset := *window.nextResetAt
			earliest = &reset
		}
	}

	return earliest
}

func clineWindowsRawData(windows []clineWindow) map[string]any {
	result := make(map[string]any, len(windows))

	for _, window := range windows {
		entry := map[string]any{
			"items_count":          window.itemsCount,
			"used_cost_units":      window.usedUnits,
			"limit_cost_units":     window.limitUnits,
			"remaining_cost_units": window.limitUnits - window.usedUnits,
			"credits_used":         window.creditsUsed,
			"usage_source":         window.usageSource,
			"reset_source":         window.resetSource,
		}
		if window.usageRatio != nil {
			entry["usage_ratio"] = *window.usageRatio
			entry["usage_percent"] = *window.usageRatio * 100
		}
		if window.costUsageRatio != nil {
			entry["cost_usage_ratio"] = *window.costUsageRatio
			entry["cost_usage_percent"] = *window.costUsageRatio * 100
		}
		if window.nextResetAt != nil {
			entry["next_reset_at"] = window.nextResetAt.Format(time.RFC3339)
		}
		result[window.key] = entry
	}

	return result
}

func clineUsageLimitsFetchRawData(meta clineUsageLimitsFetchMeta) map[string]any {
	return map[string]any{
		"status":             meta.Status,
		"entries_seen":       meta.EntriesSeen,
		"recognized_entries": meta.RecognizedEntries,
		"usable_windows":     meta.UsableWindows,
		"usable_fields":      meta.UsableFields,
	}
}

func clineBalanceRawData(balance *int64) map[string]any {
	result := map[string]any{
		"unit_note": "Cline API response field name is balance; AxonHub displays it using Cline's Cline credits terminology.",
	}
	if balance != nil {
		result["raw_balance"] = *balance
	}
	return result
}
