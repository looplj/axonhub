package provider_quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	commandCodeProviderType        = "commandcode"
	commandCodeCreditsURL          = "https://api.commandcode.ai/internal/billing/credits" //nolint:gosec // Public endpoint URL, not a credential.
	commandCodeSubscriptionsURL    = "https://api.commandcode.ai/internal/billing/subscriptions"
	commandCodeQuotaUserAgent      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	commandCodeMaxResetEpochMillis = 1e15
)

// commandCodeCookieNames are the only cookie names forwarded to the Command
// Code billing endpoints. Everything else (analytics, Stripe, better-auth,
// raw whole-header pastes) is dropped. Lookups are case-insensitive.
var commandCodeCookieNames = map[string]struct{}{
	"__secure-commandcode_prod_.session_token": {},
	"__host-commandcode_prod_.session_token":   {},
	"commandcode_prod_.session_token":          {},
	"__secure-commandcode_prod_.session_data":  {},
	"__host-commandcode_prod_.session_data":    {},
	"commandcode_prod_.session_data":           {},
}

// commandCodePlanAllowance is the local monthly allowance table used to add a
// trusted monthly denominator.
type commandCodePlanAllowance struct {
	MonthlyUSD  float64
	FiveHourUSD float64
	WeeklyUSD   float64
}

// Team Pro has no stable plan id; it is matched by label.
var commandCodePlanAllowances = map[string]commandCodePlanAllowance{
	"individual-go":    {MonthlyUSD: 10, FiveHourUSD: 3, WeeklyUSD: 6},
	"individual-goat":  {MonthlyUSD: 70, FiveHourUSD: 14, WeeklyUSD: 35},
	"individual-pro":   {MonthlyUSD: 80, FiveHourUSD: 16, WeeklyUSD: 40},
	"individual-max":   {MonthlyUSD: 150, FiveHourUSD: 45, WeeklyUSD: 90},
	"individual-ultra": {MonthlyUSD: 300, FiveHourUSD: 90, WeeklyUSD: 180},
}

// CommandCodeQuotaChecker reads Command Code account quota from the internal
// billing endpoints using the browser session cookie stored on the channel's
// settings. It is quota-only: upstream inference uses the channel API key and
// never sees this cookie.
type CommandCodeQuotaChecker struct {
	httpClient *httpclient.HttpClient
}

func NewCommandCodeQuotaChecker(httpClient *httpclient.HttpClient) *CommandCodeQuotaChecker {
	return &CommandCodeQuotaChecker{httpClient: httpClient}
}

func (c *CommandCodeQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	if ch.Type != channel.TypeCommandcode && ch.Type != channel.TypeCommandcodeAnthropic {
		return false
	}
	if ch.Settings == nil || ch.Settings.ProviderQuota == nil || ch.Settings.ProviderQuota.CommandCode == nil {
		return false
	}

	return strings.TrimSpace(ch.Settings.ProviderQuota.CommandCode.AuthCookie) != ""
}

func (c *CommandCodeQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	if ch.Settings == nil || ch.Settings.ProviderQuota == nil || ch.Settings.ProviderQuota.CommandCode == nil {
		return QuotaData{}, fmt.Errorf("%w: channel has no Command Code quota cookie", ErrInvalidCredentials)
	}

	cookie, err := NormalizeCommandCodeCookie(ch.Settings.ProviderQuota.CommandCode.AuthCookie)
	if err != nil {
		return QuotaData{}, fmt.Errorf("%w: invalid Command Code auth cookie: %w", ErrInvalidCredentials, err)
	}

	hc := c.httpClient
	if ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	creditsBody, err := commandCodeGet(ctx, hc, commandCodeCreditsURL, cookie)
	if err != nil {
		return QuotaData{}, err
	}

	// The subscription endpoint is optional: a failure only drops plan info
	// (and with it the trusted monthly denominator), never the credits result.
	subscriptionsBody, _ := commandCodeGet(ctx, hc, commandCodeSubscriptionsURL, cookie)
	return parseCommandCodeCredits(creditsBody, subscriptionsBody)
}

func commandCodeGet(ctx context.Context, hc *httpclient.HttpClient, url, cookie string) ([]byte, error) {
	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(url).
		WithHeader("Cookie", cookie).
		WithHeader("Accept", "application/json, text/plain, */*").
		WithHeader("Accept-Language", "en-US,en;q=0.9").
		WithHeader("User-Agent", commandCodeQuotaUserAgent).
		WithHeader("Origin", "https://commandcode.ai").
		WithHeader("Referer", "https://commandcode.ai/").
		Build()

	resp, err := hc.Do(ctx, request)
	if err != nil {
		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden {
				return nil, fmt.Errorf("%w: Command Code billing API returned %d (expired session cookie?)", ErrInvalidCredentials, httpErr.StatusCode)
			}
			// 429 and other rate-limit/HTTP errors keep the standard message
			// so the generic quota-error backoff applies.
			return nil, fmt.Errorf("Command Code billing API returned %d", httpErr.StatusCode)
		}
		return nil, fmt.Errorf("Command Code billing request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("%w: Command Code billing API returned %d (expired session cookie?)", ErrInvalidCredentials, resp.StatusCode)
		}
		return nil, fmt.Errorf("Command Code billing API returned %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// NormalizeCommandCodeCookie canonicalizes a raw browser cookie capture into
// the exact allowlisted Cookie header used by the billing endpoints.
//
// Accepted inputs are "name=value" pairs (with or without a leading "Cookie:"
// label) or a full browser cookie line. Only the six namespaced
// session_token/session_data cookies survive; every other cookie (analytics,
// Stripe, better-auth, ...) is dropped. At least one session_token cookie must
// remain or an error is returned. Values must not be empty and must not
// contain control characters; cookie names must be valid RFC 6265 tokens.
// Output preserves the original cookie order.
func NormalizeCommandCodeCookie(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("cookie is empty")
	}

	// Strip an optional leading "Cookie:" label (browser devtools paste).
	if idx := strings.Index(cleaned, ":"); idx >= 0 && strings.EqualFold(strings.TrimSpace(cleaned[:idx]), "cookie") {
		cleaned = strings.TrimSpace(cleaned[idx+1:])
	}

	// Reject multi-line pastes (e.g. cURL text) outright.
	if strings.ContainsAny(cleaned, "\r\n") {
		return "", errors.New("cookie contains line breaks")
	}

	var kept []string
	seenToken := false

	for part := range strings.SplitSeq(cleaned, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, value, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if name == "" {
			return "", errors.New("invalid cookie segment: expected name=value")
		}

		lower := strings.ToLower(name)
		if _, ok := commandCodeCookieNames[lower]; !ok {
			continue
		}
		if !ok {
			return "", errors.New("invalid cookie segment: expected name=value")
		}

		// A name must be a valid RFC 6265 token: no separators, whitespace or
		// control characters.
		for _, r := range name {
			if r <= 0x20 || r >= 0x7f || strings.ContainsRune("()<>@,;:\\\"/[]?={}", r) {
				return "", errors.New("invalid cookie name")
			}
		}

		value = strings.TrimSpace(value)
		if value == "" {
			return "", errors.New("cookie has an empty value")
		}
		for _, r := range value {
			if r <= 0x20 || r == 0x7f {
				return "", fmt.Errorf("cookie %q has an invalid value", name)
			}
		}

		if strings.HasSuffix(lower, ".session_token") {
			seenToken = true
		}

		kept = append(kept, name+"="+value)
	}

	if !seenToken {
		return "", errors.New("no Command Code session_token cookie found")
	}

	return strings.Join(kept, "; "), nil
}

// parseCommandCodeCredits turns the credits body (and optional subscriptions
// body) into normalized QuotaData. The credits payload is consumed
// case-insensitively; windowLimits may be nested at the root or under credits.
func parseCommandCodeCredits(creditsBody, subscriptionsBody []byte) (QuotaData, error) {
	root, err := parseJSONObject(creditsBody)
	if err != nil {
		return QuotaData{}, fmt.Errorf("parse Command Code credits response: %w", err)
	}

	// Credits-scoped map: prefer the nested credits object when present.
	creditsObj, _ := nestedFold(root, "credits")
	if creditsObj == nil {
		creditsObj = root
	}

	// Windows map may sit at the root or under the credits object.
	windowsObj, _ := nestedFold(root, "windowLimits", "windows", "limits")
	if windowsObj == nil {
		windowsObj, _ = nestedFold(creditsObj, "windowLimits", "windows", "limits")
	}

	fiveHour := parseCommandCodeWindow(windowsObj, "five_hour", "5h", "fiveHour")
	weekly := parseCommandCodeWindow(windowsObj, "weekly")

	// Wire payload: monthlyCredits / purchasedCredits, kept as
	// camelCase doubles with no unit suffix. Older snake_case names stay
	// accepted for tolerance.
	monthlyRemaining, monthlyRemainingOK := numFold(creditsObj, "monthlyCredits", "monthlyRemainingUsd", "monthly_remaining_usd")
	monthlyLimitWire, monthlyLimitWireOK := numFold(creditsObj, "monthlyLimitUsd", "monthly_limit_usd")
	purchased, purchasedOK := numFold(creditsObj, "purchasedCredits", "purchasedCreditsUsd", "purchased_credits_usd")

	// Optional subscription enrichments (plan identity for the monthly
	// denominator). Failures degrade silently to a balance-only result.
	planID, planLabel, subStatus, currentPeriodEnd := "", "", "", ""
	if len(subscriptionsBody) > 0 {
		if subRoot, subErr := parseJSONObject(subscriptionsBody); subErr == nil {
			// Wire payload wraps the subscription object under "data"
			// ({"success":true,"data":{...}}); accept both shapes.
			subObj, ok := nestedFold(subRoot, "data", "subscription")
			if !ok {
				subObj = subRoot
			}
			planID = stringFold(subObj, "planId", "plan_id")
			planLabel = stringFold(subObj, "planLabel", "plan_label")
			subStatus = stringFold(subObj, "status", "subscription_status")
			currentPeriodEnd = stringFold(subObj, "currentPeriodEnd", "current_period_end")
		}
	}

	rawCredits := map[string]any{}
	if monthlyRemainingOK {
		rawCredits["monthly_remaining_usd"] = monthlyRemaining
	}
	if purchasedOK {
		rawCredits["purchased_credits_usd"] = purchased
	}

	raw := map[string]any{
		"plan_id":             planID,
		"plan_label":          planLabel,
		"subscription_status": subStatus,
	}
	if currentPeriodEnd != "" {
		raw["current_period_end"] = currentPeriodEnd
	}
	if len(rawCredits) > 0 {
		raw["credits"] = rawCredits
	}
	if fiveHour != nil || weekly != nil {
		windowsRaw := map[string]any{}
		if fiveHour != nil {
			windowsRaw["five_hour"] = fiveHour.raw()
		}
		if weekly != nil {
			windowsRaw["weekly"] = weekly.raw()
		}
		raw["windows"] = windowsRaw
	}

	// Trusted monthly denominator: only when the plan is in the local table,
	// the wire caps match the table exactly, and the remaining balance is
	// within the monthly allowance. Go plan / unknown plan / subscription
	// failure / price drift never guess a denominator. The raw wire limit is
	// only exposed as a structured monthly_limit_usd when it is trusted, so
	// the UI never renders an untrusted "$used / $limit" bar.
	allowance := commandCodeMatchPlan(planID, planLabel)
	trustedMonthly := allowance != nil && monthlyRemainingOK && fiveHour != nil && weekly != nil &&
		fiveHour.capUSD == allowance.FiveHourUSD && weekly.capUSD == allowance.WeeklyUSD &&
		monthlyRemaining <= allowance.MonthlyUSD
	if trustedMonthly {
		if monthlyLimitWireOK {
			rawCredits["monthly_limit_usd"] = monthlyLimitWire
		} else {
			// Production payload has no monthly limit field; the plan table
			// (matched by planId with wire caps verified above) is the
			// denominator source.
			rawCredits["monthly_limit_usd"] = allowance.MonthlyUSD
		}
	}

	statusRank := map[string]int{"available": 0, "warning": 1, "exhausted": 2, "unknown": -1}
	overall := "unknown"
	nextResetAt := earliestCommandCodeReset(fiveHour, weekly)

	limits := make([]QuotaLimitStatus, 0, 3)
	addWindow := func(window string, winLen time.Duration, ratio float64, resetAt *time.Time) {
		status := "available"
		if ratio >= 1.0 {
			status = "exhausted"
		} else if ratio >= WarningThresholdRatio {
			status = "warning"
		}
		if statusRank[status] > statusRank[overall] {
			overall = status
		}
		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeSubscriptionCycle,
			Status:      status,
			UsageRatio:  ratio,
			Ready:       IsReadyStatus(status),
			NextResetAt: resetAt,
		}.WithWindow(window, winLen))
	}

	if fiveHour != nil && weekly != nil {
		addWindow(QuotaWindow5h, 5*time.Hour, fiveHour.usageRatio(), fiveHour.resetAt)
		addWindow(QuotaWindowWeekly, 7*24*time.Hour, weekly.usageRatio(), weekly.resetAt)
		if trustedMonthly {
			used := allowance.MonthlyUSD - monthlyRemaining
			addWindow(QuotaWindowMonthly, 0, used/allowance.MonthlyUSD, nextResetAt)
		}
	} else {
		// No windows: provider is on pay-as-you-go. Available only while any
		// balance remains; top-ups only ever show the numeric balance.
		hasBalance := (monthlyRemainingOK && monthlyRemaining > 0) || (purchasedOK && purchased > 0)
		if hasBalance {
			overall = "available"
		} else {
			overall = "exhausted"
		}
		limits = append(limits, QuotaLimitStatus{
			Type:       QuotaLimitTypeSubscriptionCycle,
			Status:     overall,
			UsageRatio: 0,
			Ready:      IsReadyStatus(overall),
			Window:     QuotaWindowMonthly,
		})
	}

	return QuotaData{
		Status:       overall,
		ProviderType: commandCodeProviderType,
		RawData:      raw,
		NextResetAt:  nextResetAt,
		Ready:        IsReadyStatus(overall),
		Limits:       limits,
	}, nil
}

type commandCodeWindow struct {
	usedUSD  float64
	capUSD   float64
	usagePct float64
	resetAt  *time.Time
	hasCap   bool
	hasUsage bool
}

func parseCommandCodeWindow(obj map[string]any, keys ...string) *commandCodeWindow {
	if obj == nil {
		return nil
	}
	raw, ok := findMapFold(obj, keys...)
	if !ok {
		return nil
	}

	w := &commandCodeWindow{}
	used, usedOK := numFold(raw, "used_usd", "usedUsd", "used")
	cap, capOK := numFold(raw, "cap_usd", "capUsd", "cap", "limitUsd", "limit_usd")
	pct, pctOK := numFold(raw, "usage_percent", "usagePercent", "percent")
	w.hasUsage = usedOK || pctOK
	w.hasCap = capOK
	w.usedUSD = used
	if pctOK {
		w.usagePct = pct / 100.0
	} else if usedOK && capOK && cap > 0 {
		w.usagePct = used / cap
	}
	if capOK {
		w.capUSD = cap
	}

	if resetRaw, ok := findFoldAny(raw, "reset_time", "resetTime", "resetAt", "reset", "reset_at"); ok {
		w.resetAt = parseCommandCodeReset(resetRaw)
	}

	return w
}

func (w *commandCodeWindow) usageRatio() float64 {
	if w.usagePct > 0 {
		return w.usagePct
	}
	if w.hasCap && w.capUSD > 0 && w.hasUsage {
		return w.usedUSD / w.capUSD
	}
	return 0
}

func (w *commandCodeWindow) raw() map[string]any {
	m := map[string]any{"usage_percent": w.usageRatio() * 100}
	if w.hasCap {
		m["cap_usd"] = w.capUSD
	}
	m["used_usd"] = w.usedUSD
	if w.resetAt != nil {
		m["reset_time"] = w.resetAt.Format(time.RFC3339)
	}
	return m
}

func earliestCommandCodeReset(windows ...*commandCodeWindow) *time.Time {
	var earliest *time.Time
	for _, w := range windows {
		if w == nil || w.resetAt == nil {
			continue
		}
		if earliest == nil || w.resetAt.Before(*earliest) {
			earliest = w.resetAt
		}
	}
	return earliest
}

// parseCommandCodeReset parses a window reset value: unix seconds, unix
// milliseconds, or RFC3339. Values <= 0 or unparsable produce nil (no reset).
func parseCommandCodeReset(v any) *time.Time {
	switch val := v.(type) {
	case float64:
		return commandCodeResetFromEpoch(val)
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return commandCodeResetFromEpoch(f)
		}
		return nil
	case string:
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			return nil
		}
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return commandCodeResetFromEpoch(f)
		}
		if t, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
			return &t
		}
		return nil
	default:
		return nil
	}
}

func commandCodeResetFromEpoch(epoch float64) *time.Time {
	if epoch <= 0 || epoch > commandCodeMaxResetEpochMillis {
		return nil
	}
	if epoch >= 1e12 {
		t := time.UnixMilli(int64(epoch))
		return &t
	}
	t := time.Unix(int64(epoch), 0)
	return &t
}

func commandCodeMatchPlan(planID, planLabel string) *commandCodePlanAllowance {
	if allowance, ok := commandCodePlanAllowances[strings.TrimSpace(planID)]; ok {
		return &allowance
	}
	if strings.EqualFold(strings.TrimSpace(planLabel), "team pro") {
		return &commandCodePlanAllowance{MonthlyUSD: 40, FiveHourUSD: 12, WeeklyUSD: 24}
	}
	return nil
}

func parseJSONObject(body []byte) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, errors.New("empty JSON object")
	}
	return obj, nil
}

func findMapFold(obj map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		for k, v := range obj {
			if strings.EqualFold(k, key) {
				m, ok := v.(map[string]any)
				return m, ok
			}
		}
	}
	return nil, false
}

func nestedFold(obj map[string]any, keys ...string) (map[string]any, bool) {
	for _, key := range keys {
		for k, v := range obj {
			if strings.EqualFold(k, key) {
				if m, isMap := v.(map[string]any); isMap {
					return m, true
				}
			}
		}
	}
	return nil, false
}

func findFoldAny(obj map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		for k, v := range obj {
			if strings.EqualFold(k, key) {
				return v, true
			}
		}
	}
	return nil, false
}

func numFold(obj map[string]any, keys ...string) (float64, bool) {
	v, ok := findFoldAny(obj, keys...)
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func stringFold(obj map[string]any, keys ...string) string {
	v, ok := findFoldAny(obj, keys...)
	if !ok {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return ""
	}
}
