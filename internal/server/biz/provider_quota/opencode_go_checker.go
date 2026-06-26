package provider_quota

import (
	"context"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm/httpclient"
)

const (
	opencodeGoProviderType = "opencode_go"
	opencodeGoDashboardURL = "https://opencode.ai/workspace/%s/go"
	opencodeGoUserAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Gecko/20100101 Firefox/148.0"
)

var (
	opencodeGoNumberPattern = `(-?\d+(?:\.\d+)?)`

	opencodeGoRollingPctFirst   = regexp.MustCompile(`rollingUsage:\$R\[\d+\]=\{[^}]*usagePercent:` + opencodeGoNumberPattern + `[^}]*resetInSec:` + opencodeGoNumberPattern + `[^}]*\}`)
	opencodeGoRollingResetFirst = regexp.MustCompile(`rollingUsage:\$R\[\d+\]=\{[^}]*resetInSec:` + opencodeGoNumberPattern + `[^}]*usagePercent:` + opencodeGoNumberPattern + `[^}]*\}`)
	opencodeGoWeeklyPctFirst    = regexp.MustCompile(`weeklyUsage:\$R\[\d+\]=\{[^}]*usagePercent:` + opencodeGoNumberPattern + `[^}]*resetInSec:` + opencodeGoNumberPattern + `[^}]*\}`)
	opencodeGoWeeklyResetFirst  = regexp.MustCompile(`weeklyUsage:\$R\[\d+\]=\{[^}]*resetInSec:` + opencodeGoNumberPattern + `[^}]*usagePercent:` + opencodeGoNumberPattern + `[^}]*\}`)
	opencodeGoMonthlyPctFirst   = regexp.MustCompile(`monthlyUsage:\$R\[\d+\]=\{[^}]*usagePercent:` + opencodeGoNumberPattern + `[^}]*resetInSec:` + opencodeGoNumberPattern + `[^}]*\}`)
	opencodeGoMonthlyResetFirst = regexp.MustCompile(`monthlyUsage:\$R\[\d+\]=\{[^}]*resetInSec:` + opencodeGoNumberPattern + `[^}]*usagePercent:` + opencodeGoNumberPattern + `[^}]*\}`)

	opencodeGoDataSlotItemRe  = regexp.MustCompile(`data-slot="usage-item"`)
	opencodeGoDataSlotLabelRe = regexp.MustCompile(`data-slot="usage-label"[^>]*>([\s\S]*?)</span>`)
	opencodeGoDataSlotValueRe = regexp.MustCompile(`data-slot="usage-value"[^>]*>[\s\S]*?` + opencodeGoNumberPattern)
	opencodeGoDataSlotResetRe = regexp.MustCompile(`data-slot="(reset-time|reset-now)"[^>]*>`)

	opencodeGoResetPrefixRe = regexp.MustCompile(`(?i)resets?\s*in\s*`)
	opencodeGoHTMLTagRe     = regexp.MustCompile(`<[^>]+>`)

	// opencodeGoDurationUnits maps human-readable duration units to seconds, scanned
	// largest-first so a value like "1 hour 30 minutes" accumulates correctly.
	opencodeGoDurationUnits = []struct {
		re      *regexp.Regexp
		seconds float64
	}{
		{regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:days?|d)`), 86400},
		{regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:hours?|h)`), 3600},
		{regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:minutes?|mins?|m)`), 60},
		{regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:seconds?|secs?|s)`), 1},
	}
)

type OpenCodeGoUsageWindow struct {
	UsagePercent float64
	ResetInSec   float64
}

type OpenCodeGoQuotaChecker struct {
	httpClient *httpclient.HttpClient
	now        func() time.Time
}

func NewOpenCodeGoQuotaChecker(httpClient *httpclient.HttpClient) *OpenCodeGoQuotaChecker {
	return &OpenCodeGoQuotaChecker{
		httpClient: httpClient,
		now:        time.Now,
	}
}

func (c *OpenCodeGoQuotaChecker) CheckQuota(ctx context.Context, ch *ent.Channel) (QuotaData, error) {
	workspaceID, authCookie, err := c.extractQuotaCredentials(ch)
	if err != nil {
		return QuotaData{}, err
	}

	quotaURL := fmt.Sprintf(opencodeGoDashboardURL, url.PathEscape(workspaceID))
	httpRequest := httpclient.NewRequestBuilder().
		WithMethod(http.MethodGet).
		WithURL(quotaURL).
		WithHeader("User-Agent", opencodeGoUserAgent).
		WithHeader("Accept", "text/html").
		WithHeader("Cookie", "auth="+authCookie).
		Build()

	hc := c.httpClient
	if ch.Settings != nil && ch.Settings.Proxy != nil {
		hc = c.httpClient.WithProxy(ch.Settings.Proxy)
	}

	resp, err := hc.Do(ctx, httpRequest)
	if err != nil {
		return QuotaData{}, fmt.Errorf("opencode go dashboard request failed: %w", err)
	}

	return c.parseResponse(resp.Body)
}

func (c *OpenCodeGoQuotaChecker) SupportsChannel(ch *ent.Channel) bool {
	return ch.Type == channel.TypeOpencodeGo || ch.Type == channel.TypeOpencodeGoAnthropic
}

func (c *OpenCodeGoQuotaChecker) extractQuotaCredentials(ch *ent.Channel) (string, string, error) {
	var workspaceID, authCookie string
	if ch.Settings != nil && ch.Settings.ProviderQuota != nil && ch.Settings.ProviderQuota.OpencodeGo != nil {
		cfg := ch.Settings.ProviderQuota.OpencodeGo
		workspaceID = strings.TrimSpace(cfg.WorkspaceID)
		authCookie = strings.TrimSpace(cfg.AuthCookie)
	}

	authCookie = normalizeOpenCodeGoAuthCookie(authCookie)

	switch {
	case workspaceID == "" && authCookie == "":
		return "", "", fmt.Errorf("missing OpenCode Go workspace id and auth cookie")
	case workspaceID == "":
		return "", "", fmt.Errorf("missing OpenCode Go workspace id")
	case authCookie == "":
		return "", "", fmt.Errorf("missing OpenCode Go auth cookie")
	default:
		return workspaceID, authCookie, nil
	}
}

func normalizeOpenCodeGoAuthCookie(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	raw = strings.TrimPrefix(raw, "Cookie:")
	raw = strings.TrimSpace(raw)

	for part := range strings.SplitSeq(raw, ";") {
		part = strings.TrimSpace(part)
		if after, ok := strings.CutPrefix(part, "auth="); ok {
			return strings.TrimSpace(after)
		}
	}

	return strings.TrimPrefix(raw, "auth=")
}

func (c *OpenCodeGoQuotaChecker) parseResponse(body []byte) (QuotaData, error) {
	htmlText := string(body)
	windows := parseOpenCodeGoWindows(htmlText)
	if len(windows) == 0 {
		return QuotaData{}, fmt.Errorf("could not parse OpenCode Go dashboard usage windows")
	}

	now := c.now()
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

		resetAt := now.Add(time.Duration(window.ResetInSec) * time.Second)
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

		resetAtCopy := resetAt
		limits = append(limits, QuotaLimitStatus{
			Type:        QuotaLimitTypeToken,
			Status:      status,
			UsageRatio:  usageRatio,
			Ready:       IsReadyStatus(status),
			NextResetAt: &resetAtCopy,
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

func parseOpenCodeGoWindows(htmlText string) map[string]OpenCodeGoUsageWindow {
	windows := map[string]OpenCodeGoUsageWindow{}

	if rolling, ok := parseOpenCodeGoHydrationWindow(htmlText, opencodeGoRollingPctFirst, opencodeGoRollingResetFirst); ok {
		windows["rolling"] = rolling
	}
	if weekly, ok := parseOpenCodeGoHydrationWindow(htmlText, opencodeGoWeeklyPctFirst, opencodeGoWeeklyResetFirst); ok {
		windows["weekly"] = weekly
	}
	if monthly, ok := parseOpenCodeGoHydrationWindow(htmlText, opencodeGoMonthlyPctFirst, opencodeGoMonthlyResetFirst); ok {
		windows["monthly"] = monthly
	}

	if len(windows) > 0 {
		return windows
	}

	return parseOpenCodeGoDataSlotWindows(htmlText)
}

func parseOpenCodeGoHydrationWindow(htmlText string, pctFirst *regexp.Regexp, resetFirst *regexp.Regexp) (OpenCodeGoUsageWindow, bool) {
	if matches := pctFirst.FindStringSubmatch(htmlText); len(matches) == 3 {
		usagePercent, usageOK := parseOpenCodeGoFloat(matches[1])
		resetInSec, resetOK := parseOpenCodeGoFloat(matches[2])
		if usageOK && resetOK {
			return OpenCodeGoUsageWindow{UsagePercent: usagePercent, ResetInSec: resetInSec}, true
		}
	}

	if matches := resetFirst.FindStringSubmatch(htmlText); len(matches) == 3 {
		resetInSec, resetOK := parseOpenCodeGoFloat(matches[1])
		usagePercent, usageOK := parseOpenCodeGoFloat(matches[2])
		if usageOK && resetOK {
			return OpenCodeGoUsageWindow{UsagePercent: usagePercent, ResetInSec: resetInSec}, true
		}
	}

	return OpenCodeGoUsageWindow{}, false
}

func parseOpenCodeGoDataSlotWindows(htmlText string) map[string]OpenCodeGoUsageWindow {
	windows := map[string]OpenCodeGoUsageWindow{}
	items := opencodeGoDataSlotItemRe.Split(htmlText, -1)

	for i := 1; i < len(items); i++ {
		item := items[i]

		labelMatches := opencodeGoDataSlotLabelRe.FindStringSubmatch(item)
		valueMatches := opencodeGoDataSlotValueRe.FindStringSubmatch(item)
		resetKind, resetText, resetOK := parseOpenCodeGoDataSlotReset(item)
		if len(labelMatches) < 2 || len(valueMatches) < 2 || !resetOK {
			continue
		}

		windowKey := openCodeGoWindowKeyFromLabel(cleanOpenCodeGoHTMLText(labelMatches[1]))
		if windowKey == "" {
			continue
		}

		usagePercent, usageOK := parseOpenCodeGoFloat(valueMatches[1])
		if !usageOK {
			continue
		}

		resetInSec := 0.0
		if resetKind != "reset-now" {
			parsedResetInSec, ok := parseOpenCodeGoHumanDuration(cleanOpenCodeGoResetText(resetText))
			if !ok {
				continue
			}
			resetInSec = parsedResetInSec
		}

		windows[windowKey] = OpenCodeGoUsageWindow{
			UsagePercent: usagePercent,
			ResetInSec:   resetInSec,
		}
	}

	return windows
}

func parseOpenCodeGoDataSlotReset(item string) (string, string, bool) {
	matches := opencodeGoDataSlotResetRe.FindStringSubmatchIndex(item)
	if len(matches) < 4 {
		return "", "", false
	}

	kind := item[matches[2]:matches[3]]
	text := item[matches[1]:]
	if end := strings.LastIndex(text, "</span>"); end >= 0 {
		text = text[:end]
	}
	return kind, text, true
}

func openCodeGoWindowKeyFromLabel(label string) string {
	normalized := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.Contains(normalized, "rolling"):
		return "rolling"
	case strings.Contains(normalized, "weekly"):
		return "weekly"
	case strings.Contains(normalized, "monthly"):
		return "monthly"
	default:
		return ""
	}
}

func cleanOpenCodeGoResetText(text string) string {
	text = cleanOpenCodeGoHTMLText(text)
	text = opencodeGoResetPrefixRe.ReplaceAllString(text, "")
	return strings.Join(strings.Fields(text), " ")
}

func cleanOpenCodeGoHTMLText(text string) string {
	text = strings.ReplaceAll(text, "<!--$-->", "")
	text = strings.ReplaceAll(text, "<!--/-->", "")
	text = opencodeGoHTMLTagRe.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func parseOpenCodeGoHumanDuration(text string) (float64, bool) {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return 0, false
	}
	if normalized == "now" || normalized == "reset now" || normalized == "resets now" {
		return 0, true
	}

	var total float64
	matched := false
	for _, unit := range opencodeGoDurationUnits {
		if matches := unit.re.FindStringSubmatch(normalized); len(matches) == 2 {
			value, ok := parseOpenCodeGoFloat(matches[1])
			if !ok {
				continue
			}
			total += value * unit.seconds
			matched = true
		}
	}

	return total, matched
}

func parseOpenCodeGoFloat(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || !isFiniteOpenCodeGoNumber(value) {
		return 0, false
	}
	return value, true
}

func isFiniteOpenCodeGoNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
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
