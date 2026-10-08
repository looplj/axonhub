package provider_quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	ollamaProviderType = "ollama"
	// ollamaBalanceURL is the official account balance API, authenticated with
	// the channel API key: https://docs.ollama.com/api/balance
	ollamaBalanceURL = "https://ollama.com/api/balance"
	// ollamaSettingsURL is the Plan & Billing page used when the channel only
	// has the browser session cookie.
	ollamaSettingsURL  = "https://ollama.com/settings"
	ollamaQuotaUA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	ollamaCookieName   = "__Secure-session"
	ollamaCookiePrefix = ollamaCookieName + "="
	// ollamaMinCookieLen guards against a bare name or a trivially short
	// placeholder. Only the prefix (16 chars) plus a plausible token payload
	// passes; the real encrypted session token is far longer.
	ollamaMinCookieLen = 30
)

// OllamaQuotaChecker reads Ollama Cloud quota from the balance API using the
// channel API key, or from the logged-in settings page using the browser
// session cookie stored on the channel. Only the `__Secure-session` cookie
// authenticates the Ollama Web session; everything else in the paste is
// dropped before forwarding.
type OllamaQuotaChecker struct {
	httpClient *httpclient.HttpClient
	now        func() time.Time
}

// NewOllamaQuotaChecker creates a checker with the shared HTTP client.
func NewOllamaQuotaChecker(httpClient *httpclient.HttpClient) *OllamaQuotaChecker {
	return &OllamaQuotaChecker{
		httpClient: httpClient,
		now:        time.Now,
	}
}

func ollamaAPIKey(ch *ent.Channel) string {
	for _, candidate := range ch.Credentials.GetAllAPIKeys() {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func ollamaRawCookie(ch *ent.Channel) string {
	if ch.Settings == nil || ch.Settings.ProviderQuota == nil || ch.Settings.ProviderQuota.Ollama == nil {
		return ""
	}

	return strings.TrimSpace(ch.Settings.ProviderQuota.Ollama.AuthCookie)
}

// HasOllamaQuotaCredentials reports whether the channel has the account API
// key (/api/balance, preferred) or the session cookie (settings page). Cookie
// format validation stays in CheckQuota so a malformed cookie surfaces as a
// quota error instead of the channel being silently skipped.
func HasOllamaQuotaCredentials(ch *ent.Channel) bool {
	if ch == nil {
		return false
	}

	return ollamaAPIKey(ch) != "" || ollamaRawCookie(ch) != ""
}

// SupportsChannel reports whether the channel is an Ollama variant with quota
// credentials configured.
func (c *OllamaQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	if ch.Type != channel.TypeOllama && ch.Type != channel.TypeOllamaAnthropic {
		return false
	}

	return HasOllamaQuotaCredentials(ch)
}

// CheckQuota fetches and parses Ollama Cloud quota for the channel. Non-2xx
// responses surface as errors with their status code; rejected API keys and
// expired cookies surface as invalid credentials.
func (c *OllamaQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	apiKey := ollamaAPIKey(ch)
	rawCookie := ollamaRawCookie(ch)
	if apiKey == "" && rawCookie == "" {
		return QuotaData{}, fmt.Errorf("%w: channel has no Ollama API key or quota cookie", ErrInvalidCredentials)
	}

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	if apiKey != "" {
		quota, err := c.fetchBalance(ctx, hc, apiKey)
		if err == nil {
			return quota, nil
		}
		if rawCookie == "" || !errors.Is(err, ErrInvalidCredentials) {
			return QuotaData{}, err
		}
	}

	cookie, err := NormalizeOllamaCookie(rawCookie)
	if err != nil {
		return QuotaData{}, fmt.Errorf("%w: invalid Ollama auth cookie: %w", ErrInvalidCredentials, err)
	}

	// Reject HTTPS-to-HTTP redirects so the session cookie is never forwarded
	// over a downgraded (cleartext) connection.
	return c.fetchSettings(ctx, hc.WithRejectHTTPSDowngrade(), cookie)
}

func (c *OllamaQuotaChecker) fetchBalance(ctx context.Context, hc *httpclient.HttpClient, apiKey string) (QuotaData, error) {
	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(ollamaBalanceURL).
		WithHeader("Authorization", "Bearer "+apiKey).
		WithHeader("Accept", "application/json").
		Build()

	resp, err := hc.Do(ctx, request)
	if err != nil {
		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			return QuotaData{}, ollamaBalanceStatusError(httpErr.StatusCode)
		}
		return QuotaData{}, fmt.Errorf("Ollama balance request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return QuotaData{}, ollamaBalanceStatusError(resp.StatusCode)
	}

	return c.parseBalance(resp.Body)
}

func ollamaBalanceStatusError(statusCode int) error {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return fmt.Errorf("%w: Ollama balance API returned %d", ErrInvalidCredentials, statusCode)
	}

	return fmt.Errorf("Ollama balance API returned %d", statusCode)
}

func (c *OllamaQuotaChecker) fetchSettings(ctx context.Context, hc *httpclient.HttpClient, cookie string) (QuotaData, error) {
	request := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(ollamaSettingsURL).
		WithHeader("Cookie", cookie).
		WithHeader("Accept", "text/html,application/xhtml+xml").
		WithHeader("User-Agent", ollamaQuotaUA).
		Build()

	resp, err := hc.Do(ctx, request)
	if err != nil {
		if httpErr, ok := errors.AsType[*httpclient.Error](err); ok {
			if httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden {
				return QuotaData{}, fmt.Errorf("%w: Ollama settings page returned %d (expired session cookie?)", ErrInvalidCredentials, httpErr.StatusCode)
			}
			return QuotaData{}, fmt.Errorf("Ollama settings page returned %d", httpErr.StatusCode)
		}
		return QuotaData{}, fmt.Errorf("Ollama settings request failed: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return QuotaData{}, fmt.Errorf("%w: Ollama settings page returned %d (expired session cookie?)", ErrInvalidCredentials, resp.StatusCode)
		}
		return QuotaData{}, fmt.Errorf("Ollama settings page returned %d", resp.StatusCode)
	}

	return c.parseResponse(resp.Body)
}

// NormalizeOllamaCookie canonicalizes a raw browser cookie capture into the
// exact allowlisted Cookie header used by the settings request. Accepted
// inputs are "name=value" pairs (with or without a leading "Cookie:" label) or
// a full browser cookie line. Only the `__Secure-session` cookie that
// authenticates the Ollama Web session survives; every other cookie (cf_clearance,
// aid, analytics, ...) is dropped. The `__Secure-session` cookie must be
// present with a token long enough to be a plausible encrypted session.
// Output preserves the session cookie; any other cookies are discarded.
func NormalizeOllamaCookie(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", errors.New("cookie is empty")
	}

	// Strip an optional leading "Cookie:" label (browser devtools paste).
	if idx := strings.Index(cleaned, ":"); idx >= 0 && strings.EqualFold(strings.TrimSpace(cleaned[:idx]), "cookie") {
		cleaned = strings.TrimSpace(cleaned[idx+1:])
	}

	// Reject multi-line pastes (e.g. cURL) outright.
	if strings.ContainsAny(cleaned, "\r\n") {
		return "", errors.New("cookie contains line breaks")
	}

	for part := range strings.SplitSeq(cleaned, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, value, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(name) == "" || value == "" {
			return "", errors.New("invalid cookie segment: expected name=value")
		}

		// Only the __Secure-session cookie is forwarded.
		if !strings.EqualFold(strings.TrimSpace(name), ollamaCookieName) {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) < ollamaMinCookieLen {
			return "", errors.New("Ollama cookie is too short to be a valid session")
		}
		return ollamaCookiePrefix + value, nil
	}

	return "", errors.New("no __Secure-session Ollama cookie found")
}

var (
	ollamaUsageMeterRe = regexp.MustCompile(`aria-label="([^"]+)"`)
	ollamaPercentRe    = regexp.MustCompile(`([0-9.]+)\s*%\s*used`)
	ollamaResetTimeRe  = regexp.MustCompile(`data-time="([^"]+)"`)
)

type ollamaUsageWindow struct {
	key         string
	percent     float64
	resetAt     *time.Time
	periodStart *time.Time
}

// ollamaBalanceResponse matches GET /api/balance. Monthly allowance plans
// report balance_usd of allowance_usd for the included period; legacy plans
// report session and weekly windows with remaining_percent (0..100).
type ollamaBalanceResponse struct {
	Included struct {
		BalanceUSD   *float64 `json:"balance_usd"`
		AllowanceUSD *float64 `json:"allowance_usd"`
		Period       *struct {
			From  *time.Time `json:"from"`
			Until *time.Time `json:"until"`
		} `json:"period"`
		Session *ollamaBalanceWindow `json:"session"`
		Weekly  *ollamaBalanceWindow `json:"weekly"`
	} `json:"included"`
	Purchased struct {
		BalanceUSD float64 `json:"balance_usd"`
	} `json:"purchased"`
}

type ollamaBalanceWindow struct {
	RemainingPercent *float64   `json:"remaining_percent"`
	ResetsAt         *time.Time `json:"resets_at"`
}

func (c *OllamaQuotaChecker) parseBalance(body []byte) (QuotaData, error) {
	var parsed ollamaBalanceResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return QuotaData{}, fmt.Errorf("parse Ollama balance response: %w", err)
	}

	included := parsed.Included
	var windows []ollamaUsageWindow

	if allowance := lo.FromPtr(included.AllowanceUSD); allowance > 0 {
		window := ollamaUsageWindow{
			key:     QuotaWindowMonthly,
			percent: clampOllamaPercent((1 - lo.FromPtr(included.BalanceUSD)/allowance) * 100),
		}
		if included.Period != nil {
			window.resetAt = included.Period.Until
			window.periodStart = included.Period.From
		}
		windows = append(windows, window)
	}

	for _, legacy := range []struct {
		key    string
		window *ollamaBalanceWindow
	}{
		{QuotaWindow5h, included.Session},
		{QuotaWindowWeekly, included.Weekly},
	} {
		if legacy.window == nil || legacy.window.RemainingPercent == nil {
			continue
		}
		windows = append(windows, ollamaUsageWindow{
			key:     legacy.key,
			percent: clampOllamaPercent(100 - *legacy.window.RemainingPercent),
			resetAt: legacy.window.ResetsAt,
		})
	}

	purchased := parsed.Purchased.BalanceUSD
	if len(windows) == 0 && purchased <= 0 {
		return QuotaData{}, errors.New("no Ollama quota found in balance response")
	}

	data := buildOllamaQuotaData(windows)
	if purchased > 0 {
		data.Limits = append(data.Limits, QuotaLimitStatus{
			Type:   QuotaLimitTypeToken,
			Status: "available",
			Ready:  true,
			Window: QuotaWindowCredits,
		})
		data.RawData["credits"] = map[string]any{
			"purchased_usd": purchased,
		}
	}

	// Ollama draws from purchased credits once the included allowance is spent,
	// so the overall status comes from the best usable limit.
	data.Status = ""

	return normalizeQuotaDataAt(data, c.now()), nil
}

func clampOllamaPercent(percent float64) float64 {
	return max(0, min(100, percent))
}

func (c *OllamaQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	html := string(body)

	// Each meter block is a "label ... data-time" pair. Match per window by
	// locating the header keyword, then capturing the nearest percent and reset
	// time within a bounded window after it.
	var windows []ollamaUsageWindow

	extractWindow := func(keyword, key string) {
		idx := strings.Index(html, keyword)
		if idx < 0 {
			return
		}
		end := min(idx+4000, len(html))
		chunk := html[idx:end]

		meter := ollamaUsageMeterRe.FindStringSubmatch(chunk)
		if len(meter) == 0 {
			return
		}
		pctMatch := ollamaPercentRe.FindStringSubmatch(meter[1])
		if len(pctMatch) == 0 {
			return
		}
		pct, err := strconv.ParseFloat(pctMatch[1], 64)
		if err != nil {
			return
		}

		resetMatch := ollamaResetTimeRe.FindStringSubmatch(chunk)
		var resetAt *time.Time
		if len(resetMatch) > 0 {
			if t, terr := time.Parse(time.RFC3339Nano, resetMatch[1]); terr == nil {
				tt := t
				resetAt = &tt
			}
		}

		windows = append(windows, ollamaUsageWindow{key: key, percent: pct, resetAt: resetAt})
	}

	extractWindow("Session usage", QuotaWindow5h)
	extractWindow("Weekly usage", QuotaWindowWeekly)

	if len(windows) == 0 {
		// No usage windows matched: the page may be a sign-in redirect to an
		// HTML login page (an expired cookie that returns 200 rather than 401/403)
		// or the markup has changed. A sign-in page means the session credential
		// is no longer valid, so classify it as invalid credentials so the
		// framework invalidates stale cached quota rather than retaining it with
		// backoff. Any other 200 body is a genuine parse failure (likely markup
		// change), which surfaces as a generic check error.
		if ollamaLooksLikeLoginPage(html) {
			return QuotaData{}, fmt.Errorf("%w: ollama.com/settings returned the sign-in page (expired session cookie?)", ErrInvalidCredentials)
		}
		return QuotaData{}, fmt.Errorf("no Ollama usage windows found in settings page (expired cookie or markup change)")
	}

	return buildOllamaQuotaData(windows), nil
}

func buildOllamaQuotaData(windows []ollamaUsageWindow) QuotaData {
	normalizedStatus := "available"
	var nextResetAt *time.Time
	limits := make([]QuotaLimitStatus, 0, len(windows))
	rawWindows := make(map[string]any, len(windows))

	for _, w := range windows {
		usageRatio := w.percent / 100.0
		status := normalizeOllamaWindowStatus(usageRatio)
		if quotaStatusRank(status) > quotaStatusRank(normalizedStatus) {
			normalizedStatus = status
		}

		var resetAt *time.Time
		if w.resetAt != nil {
			rc := *w.resetAt
			resetAt = &rc
			if nextResetAt == nil || rc.Before(*nextResetAt) {
				nextResetAt = &rc
			}
		}

		rawWindow := map[string]any{
			"usage_percent":     w.percent,
			"status":            status,
			"percent_remaining": 100 - w.percent,
		}
		if w.resetAt != nil {
			rawWindow["reset_time"] = w.resetAt.Format(time.RFC3339)
		}
		rawWindows[w.key] = rawWindow

		periodStart := w.periodStart
		if periodStart == nil {
			periodStart = PeriodStartFromReset(resetAt, ollamaWindowDuration(w.key))
		}

		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       IsReadyStatus(status),
			NextResetAt: resetAt,
			Window:      w.key,
			PeriodStart: periodStart,
		})
	}

	return QuotaData{
		Status:       normalizedStatus,
		ProviderType: ollamaProviderType,
		RawData: map[string]any{
			"windows": rawWindows,
		},
		NextResetAt: nextResetAt,
		Ready:       IsReadyStatus(normalizedStatus),
		Limits:      limits,
	}
}

// ollamaWindowDuration returns the fixed window length used to derive period
// start; 0 means the window label is informational only.
func ollamaWindowDuration(key string) time.Duration {
	switch key {
	case QuotaWindow5h:
		return 5 * time.Hour
	case QuotaWindowWeekly:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

func normalizeOllamaWindowStatus(usageRatio float64) string {
	if usageRatio >= 1.0 {
		return "exhausted"
	}
	if usageRatio >= WarningThresholdRatio {
		return "warning"
	}
	return "available"
}

// ollamaLooksLikeLoginPage reports whether an HTML body is the Ollama sign-in
// page rather than the logged-in settings page. When an expired session cookie
// hits ollama.com/settings, the site may return an HTTP 200 login/redirect
// page instead of 401/403. Detecting that lets the checker classify the result
// as invalid credentials so stale cached quota is invalidated rather than
// retained with backoff. The markers are stable HTML tokens of the AuthKit
// sign-in flow; a body containing none of them is treated as a genuine markup
// change (a generic parse failure), not an auth problem.
func ollamaLooksLikeLoginPage(html string) bool {
	lower := strings.ToLower(html)
	markers := []string{
		"<title>sign in",
		"sign-in",
		"signin",
		"wos-session", // session cookie reference implies a redirect/auth flow
		"log in to your account",
		"/signin",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}
