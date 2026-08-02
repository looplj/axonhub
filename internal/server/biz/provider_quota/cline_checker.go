package provider_quota

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	clineUsageLimitsFetchStatusComplete        = "complete"
	clineUsageLimitsFetchStatusPartial         = "partial"
	clineUsageLimitsFetchStatusUnusable        = "unusable"
	clineUsageLimitsFetchStatusUnavailable     = "unavailable"
	clineUsageLimitsFetchStatusPassUnavailable = "cline_pass_unavailable"

	clineWindowSourceOfficialUsageLimits    = "official_usage_limits"
	clineWindowSourceOfficialWindowLedger   = "cline_pass_ledger_official_window"
	clineWindowSourceOfficialNoActiveWindow = "official_no_active_window"
	clineWindowSourceUnavailable            = "unavailable"

	clineOfficialResetStateActive      clineResetState = "active"
	clineOfficialResetStateInactive    clineResetState = "inactive"
	clineOfficialResetStateUnavailable clineResetState = "unavailable"
	clineOfficialResetStateInvalid     clineResetState = "invalid"

	clineWindowStateActive      = "active"
	clineWindowStateInactive    = "inactive"
	clineWindowStateUnavailable = "unavailable"
	clineWindowStateInvalid     = "invalid"

	clineWindowBoundaryTolerance = 2 * time.Second
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
	CreatedAt       string `json:"createdAt,omitempty"`
	CostUSD         int64  `json:"costUsd,omitempty"`
	CreditsUsed     int64  `json:"creditsUsed,omitempty"`
	AIModelTypeName string `json:"aiModelTypeName,omitempty"`
}

type clineUsageLimitsData struct {
	Limits []clineUsageLimit `json:"limits,omitempty"`
}

type clineResetState string

type clineUsageLimit struct {
	Type            string
	PercentUsed     *float64
	ResetsAt        string
	ResetFieldState clineResetState
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
		raw = bytes.TrimSpace(raw)
		switch {
		case bytes.Equal(raw, []byte("null")):
			l.ResetFieldState = clineOfficialResetStateInactive
		default:
			if err := json.Unmarshal(raw, &l.ResetsAt); err != nil {
				l.ResetFieldState = clineOfficialResetStateInvalid
			} else if strings.TrimSpace(l.ResetsAt) == "" {
				l.ResetFieldState = clineOfficialResetStateInactive
			} else {
				l.ResetFieldState = clineOfficialResetStateActive
			}
		}
	} else {
		l.ResetFieldState = clineOfficialResetStateUnavailable
	}

	return nil
}

type clineOfficialWindowLimit struct {
	UsageRatio  *float64
	NextResetAt *time.Time
	ResetState  clineResetState
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
	costSource     string
	nextResetAt    *time.Time
	windowStartAt  *time.Time
	costStartAt    *time.Time
	resetSource    string
	state          string
	active         bool
	costAvailable  bool
}

type clineUsageFetchMeta struct {
	Pages                 int
	ItemsSeen             int
	ClinePassItemsSeen    int
	DirectItemsSeen       int
	UnclassifiedItemsSeen int
	InvalidTimestampItems int
	Truncated             bool
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

	officialLimits, officialMeta, err := c.fetchUsageLimits(ctx, hc, ch.BaseURL, apiKey)
	if err != nil {
		return QuotaData{}, err
	}
	if officialMeta.Status == clineUsageLimitsFetchStatusPassUnavailable {
		return buildClinePassUnavailableQuota(scope, planSummaries, balance.Data.Balance, officialMeta), nil
	}

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

type clineHTTPError struct {
	StatusCode int
}

func (e *clineHTTPError) Error() string {
	if statusText := http.StatusText(e.StatusCode); statusText != "" {
		return fmt.Sprintf("HTTP %d %s", e.StatusCode, statusText)
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

func clineHTTPStatusError(statusCode int) error {
	return &clineHTTPError{StatusCode: statusCode}
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
) (map[string]clineOfficialWindowLimit, clineUsageLimitsFetchMeta, error) {
	var response clineEnvelope[clineUsageLimitsData]
	if err := c.getJSON(ctx, hc, baseURL, clineUsageLimitsPath, nil, apiKey, &response); err != nil {
		var httpErr *clineHTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return nil, clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusPassUnavailable}, nil
		}
		return nil, clineUsageLimitsFetchMeta{Status: clineUsageLimitsFetchStatusUnavailable}, fmt.Errorf("failed to read Cline usage limits: %w", err)
	}
	limits, meta := parseClineUsageLimits(response.Data.Limits)
	if meta.Status == clineUsageLimitsFetchStatusUnusable {
		return nil, meta, fmt.Errorf("failed to read Cline usage limits: response contains no usable window data")
	}
	return limits, meta, nil
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
		for _, item := range resp.Data.Items {
			switch strings.TrimSpace(item.AIModelTypeName) {
			case "cline-pass":
				meta.ClinePassItemsSeen++
			case "":
				meta.UnclassifiedItemsSeen++
			default:
				meta.DirectItemsSeen++
			}
			if _, ok := parseClineTime(item.CreatedAt); !ok {
				meta.InvalidTimestampItems++
			}
		}
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

		if limit.ResetState == "" || limit.ResetState == clineOfficialResetStateUnavailable || limit.ResetState == clineOfficialResetStateInvalid {
			resetState := item.ResetFieldState
			if resetState == "" {
				switch {
				case strings.TrimSpace(item.ResetsAt) != "":
					resetState = clineOfficialResetStateActive
				default:
					resetState = clineOfficialResetStateUnavailable
				}
			}
			if resetState == clineOfficialResetStateInactive && (limit.UsageRatio == nil || *limit.UsageRatio > 0) {
				resetState = clineOfficialResetStateUnavailable
			}

			switch resetState {
			case clineOfficialResetStateActive:
				if resetAt, valid := parseClineTime(item.ResetsAt); valid {
					limit.NextResetAt = &resetAt
					limit.ResetState = clineOfficialResetStateActive
					meta.UsableFields++
				} else {
					limit.ResetState = clineOfficialResetStateInvalid
				}
			case clineOfficialResetStateInactive:
				limit.ResetState = clineOfficialResetStateInactive
				meta.UsableFields++
			case clineOfficialResetStateInvalid:
				limit.ResetState = clineOfficialResetStateInvalid
			default:
				limit.ResetState = clineOfficialResetStateUnavailable
			}
		}

		if limit.UsageRatio != nil || limit.ResetState != clineOfficialResetStateUnavailable {
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
		buildClineWindow(now, "last5h", 5*time.Hour, threshold.Last5HoursUsageCostUSDPerUser, items, usageFetchMeta.Truncated, officialLimits["last5h"]),
		buildClineWindow(now, "last7d", 7*24*time.Hour, threshold.Last7DaysUsageCostUSDPerUser, items, usageFetchMeta.Truncated, officialLimits["last7d"]),
		buildClineWindow(now, "last30d", 30*24*time.Hour, threshold.Last30DaysUsageCostUSDPerUser, items, usageFetchMeta.Truncated, officialLimits["last30d"]),
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
				"pages":                   usageFetchMeta.Pages,
				"items_seen":              usageFetchMeta.ItemsSeen,
				"cline_pass_items_seen":   usageFetchMeta.ClinePassItemsSeen,
				"direct_items_seen":       usageFetchMeta.DirectItemsSeen,
				"unclassified_items_seen": usageFetchMeta.UnclassifiedItemsSeen,
				"invalid_timestamp_items": usageFetchMeta.InvalidTimestampItems,
				"truncated":               usageFetchMeta.Truncated,
			},
			"usage_limits_fetch": clineUsageLimitsFetchRawData(officialMeta),
		},
	}
}

func buildClinePassUnavailableQuota(
	scope clineModelScope,
	plans []map[string]any,
	balance *int64,
	usageLimitsMeta clineUsageLimitsFetchMeta,
) QuotaData {
	status := "warning"
	statusBasis := "cline_pass_unavailable"
	switch scope {
	case clineModelScopePassOnly:
		status = "exhausted"
	case clineModelScopeMixed:
		statusBasis = "cline_pass_unavailable_mixed_pool"
	}

	return QuotaData{
		Status:       status,
		ProviderType: clineProviderType,
		Ready:        IsReadyStatus(status),
		RawData: map[string]any{
			"model_scope":        string(scope),
			"status_basis":       statusBasis,
			"pool":               "cline_pass",
			"pool_note":          "ClinePass is a separate provider; this quota applies to cline-pass/* models only.",
			"pass_state":         "unavailable",
			"balance":            clineBalanceRawData(balance),
			"plans":              plans,
			"usage_limits_fetch": clineUsageLimitsFetchRawData(usageLimitsMeta),
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
	usageTruncated bool,
	official clineOfficialWindowLimit,
) clineWindow {
	window := clineWindow{
		key:         key,
		duration:    duration,
		limitUnits:  limit,
		usageSource: clineWindowSourceUnavailable,
		costSource:  clineWindowSourceUnavailable,
		resetSource: clineWindowSourceUnavailable,
		state:       clineWindowStateUnavailable,
	}

	if official.UsageRatio != nil {
		ratio := *official.UsageRatio
		window.usageRatio = &ratio
		window.usageSource = clineWindowSourceOfficialUsageLimits
	}

	resetState := official.ResetState
	if official.NextResetAt != nil {
		resetState = clineOfficialResetStateActive
	}

	if resetState == clineOfficialResetStateInactive && window.usageRatio != nil && *window.usageRatio > 0 {
		resetState = clineOfficialResetStateUnavailable
	}

	switch resetState {
	case clineOfficialResetStateInactive:
		window.state = clineWindowStateInactive
		window.costAvailable = true
		window.costSource = clineWindowSourceOfficialNoActiveWindow
		window.resetSource = clineWindowSourceOfficialUsageLimits
		if window.usageRatio == nil {
			ratio := 0.0
			window.usageRatio = &ratio
			window.usageSource = clineWindowSourceOfficialUsageLimits
		}
		costRatio := 0.0
		window.costUsageRatio = &costRatio
		return window
	case clineOfficialResetStateInvalid:
		window.state = clineWindowStateInvalid
		return window
	case clineOfficialResetStateUnavailable, "":
		return window
	}

	if official.NextResetAt == nil {
		window.state = clineWindowStateInvalid
		return window
	}

	resetAt := *official.NextResetAt
	if !resetAt.After(now) || resetAt.After(now.Add(duration+clineWindowBoundaryTolerance)) {
		window.state = clineWindowStateInvalid
		return window
	}

	window.state = clineWindowStateActive
	window.active = true
	window.nextResetAt = &resetAt
	window.resetSource = clineWindowSourceOfficialUsageLimits
	if usageTruncated {
		return window
	}

	officialStart := resetAt.Add(-duration)
	costStart := alignClineWindowStart(officialStart, items)
	window.windowStartAt = &officialStart
	window.costStartAt = &costStart

	for _, item := range items {
		createdAt, ok := parseClineTime(item.CreatedAt)
		if !ok {
			window.costAvailable = false
			window.costSource = clineWindowSourceUnavailable
			window.usedUnits = 0
			window.creditsUsed = 0
			window.itemsCount = 0
			window.costUsageRatio = nil
			return window
		}
		if createdAt.Before(costStart) || !createdAt.Before(resetAt) {
			continue
		}

		switch strings.TrimSpace(item.AIModelTypeName) {
		case "cline-pass":
			window.itemsCount++
			window.usedUnits += item.CostUSD
			window.creditsUsed += item.CreditsUsed
		case "":
			window.costAvailable = false
			window.costSource = clineWindowSourceUnavailable
			window.usedUnits = 0
			window.creditsUsed = 0
			window.itemsCount = 0
			window.costUsageRatio = nil
			return window
		}
	}

	window.costAvailable = true
	window.costSource = clineWindowSourceOfficialWindowLedger
	if window.limitUnits > 0 {
		costRatio := float64(window.usedUnits) / float64(window.limitUnits)
		window.costUsageRatio = &costRatio
		if window.usageRatio == nil {
			window.usageRatio = &costRatio
			window.usageSource = clineWindowSourceOfficialWindowLedger
		}
	}

	return window
}

func alignClineWindowStart(expected time.Time, items []clineUsageItem) time.Time {
	aligned := expected
	bestDistance := clineWindowBoundaryTolerance + time.Nanosecond

	for _, item := range items {
		if strings.TrimSpace(item.AIModelTypeName) != "cline-pass" {
			continue
		}
		createdAt, ok := parseClineTime(item.CreatedAt)
		if !ok {
			continue
		}

		distance := createdAt.Sub(expected)
		if distance < 0 {
			distance = -distance
		}
		if distance <= clineWindowBoundaryTolerance && distance < bestDistance {
			aligned = createdAt
			bestDistance = distance
		}
	}

	return aligned
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
			"window_state":     window.state,
			"active_window":    window.active,
			"limit_cost_units": window.limitUnits,
			"usage_source":     window.usageSource,
			"reset_source":     window.resetSource,
			"cost_source":      window.costSource,
		}
		if window.costAvailable {
			entry["items_count"] = window.itemsCount
			entry["used_cost_units"] = window.usedUnits
			entry["remaining_cost_units"] = window.limitUnits - window.usedUnits
			entry["credits_used"] = window.creditsUsed
		}
		if window.usageRatio != nil {
			entry["usage_ratio"] = *window.usageRatio
			entry["usage_percent"] = *window.usageRatio * 100
		}
		if window.costUsageRatio != nil {
			entry["cost_usage_ratio"] = *window.costUsageRatio
			entry["cost_usage_percent"] = *window.costUsageRatio * 100
		}
		if window.windowStartAt != nil {
			entry["window_start_at"] = window.windowStartAt.Format(time.RFC3339Nano)
		}
		if window.costStartAt != nil && window.windowStartAt != nil && !window.costStartAt.Equal(*window.windowStartAt) {
			entry["cost_start_at"] = window.costStartAt.Format(time.RFC3339Nano)
		}
		if window.nextResetAt != nil {
			entry["next_reset_at"] = window.nextResetAt.Format(time.RFC3339Nano)
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
