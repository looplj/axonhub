package main

import (
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/cmd/axonhub-desktop/locales"
)

const (
	adminNextPath = "/"
	panelBlank    = "—"
)

// requestAdminPath opens the global detail page because the desktop lists
// requests across projects, including projects outside the owner's memberships.
// The id is escaped as a single path segment, keeping a gid's slashes inside it.
func requestAdminPath(id string) string {
	return "/requests/" + url.PathEscape(id)
}

// nextSelectedID keeps the current row when it is still in the snapshot
// and otherwise selects the first loaded row.
func nextSelectedID(requests []panelRequest, selectedID string) string {
	if _, ok := findRequest(requests, selectedID); ok {
		return selectedID
	}
	first, ok := lo.First(requests)
	if !ok {
		return ""
	}
	return first.ID
}

// cardAnimates reports whether the routing wires move. They move only
// while the selected row is in progress.
func cardAnimates(requests []panelRequest, selectedID string) bool {
	request, ok := findRequest(requests, selectedID)
	return ok && request.InProgress
}

func findRequest(requests []panelRequest, id string) (panelRequest, bool) {
	if id == "" {
		return panelRequest{}, false
	}
	return lo.Find(requests, func(item panelRequest) bool {
		return item.ID == id
	})
}

func panelLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return panelBlank
	}
	return value
}

// statusTone is the color family a status or number is drawn in.
type statusTone int

const (
	toneMuted statusTone = iota
	toneSuccess
	toneActive
	toneDanger
)

// panelStat is one cell of the panel's today strip.
type panelStat struct {
	Label string
	Value string
	Tone  statusTone
}

// panelStats is today's requests, tokens in and out, cached tokens, and
// failures. Failures turn red once there is one.
func panelStats(snapshot panelSnapshot) []panelStat {
	failed := panelStat{Label: locales.Failed(), Value: formatCompact(int64(snapshot.Failures))}
	if snapshot.Failures > 0 {
		failed.Tone = toneDanger
	}
	return []panelStat{
		{Label: locales.Today(), Value: formatCompact(int64(snapshot.RequestsToday))},
		{Label: locales.Tokens(), Value: formatCompact(tokenTotal(snapshot))},
		{Label: locales.Cached(), Value: formatCompact(int64(snapshot.CachedTokens))},
		failed,
	}
}

func tokenTotal(snapshot panelSnapshot) int64 {
	return int64(snapshot.InputTokens) + int64(snapshot.OutputTokens)
}

// requestResult is the right side of a recent-request row: Primary on
// top, Secondary under it.
type requestResult struct {
	Channel   string
	Primary   string
	Secondary string
	Tone      statusTone
}

// presentRequest builds the row's channel and result. Completed rows lead
// with tokens, then latency and tok/s. Failed and canceled lead with a red
// status and omit tok/s. An in-progress row shows only its status.
func presentRequest(request panelRequest) requestResult {
	result := requestResult{Channel: panelLabel(request.Channel)}
	var metrics []string
	switch {
	case request.InProgress:
		result.Primary = lo.CoalesceOrEmpty(statusText(strings.TrimSpace(request.Status)), locales.InProgressLower())
		result.Tone = toneActive
		return result
	case request.Status == statusFailed || request.Status == statusCanceled:
		result.Primary = statusText(request.Status)
		result.Tone = toneDanger
		metrics = requestMetrics(request, false)
	case request.Status == statusCompleted:
		result.Tone = toneSuccess
		metrics = requestMetrics(request, true)
	default:
		result.Primary = strings.TrimSpace(request.Status)
		metrics = requestMetrics(request, false)
	}
	if result.Primary == "" && len(metrics) > 0 {
		result.Primary, metrics = metrics[0], metrics[1:]
	}
	result.Secondary = strings.Join(metrics, " · ")
	return result
}

// routingStatus is the chip on the routing card for the selected row.
func routingStatus(request panelRequest, selected bool) (string, statusTone) {
	switch {
	case !selected:
		return locales.Waiting(), toneMuted
	case request.InProgress:
		return locales.InProgress(), toneActive
	case request.Status == statusCompleted:
		return locales.Completed(), toneSuccess
	case request.Status == statusFailed:
		return locales.Failed(), toneDanger
	case request.Status == statusCanceled:
		return locales.Canceled(), toneDanger
	default:
		return panelLabel(statusText(request.Status)), toneMuted
	}
}

// routingSummary is the routing card's line under the wire: what the
// selected request used, or that it is still running.
func routingSummary(request panelRequest) string {
	if request.InProgress {
		return locales.InProgressLower()
	}
	return strings.Join(requestMetrics(request, request.Status == statusCompleted), " · ")
}

func requestMetrics(request panelRequest, rate bool) []string {
	var tokens, latency, perSecond string
	if n, ok := requestTokenCount(request); ok {
		tokens = locales.TokenCount(formatCompact(n))
	}
	if request.LatencyMs != nil {
		latency = formatLatency(*request.LatencyMs)
	}
	if rate && request.TokensPerSecond != nil {
		perSecond = locales.TokensPerSecond(formatCompact(int64(*request.TokensPerSecond)))
	}
	return lo.Compact([]string{tokens, latency, perSecond})
}

func requestTokenCount(request panelRequest) (int64, bool) {
	if request.TotalTokens != nil {
		return int64(*request.TotalTokens), true
	}
	if request.PromptTokens == nil && request.CompletionTokens == nil {
		return 0, false
	}
	return int64(lo.FromPtr(request.PromptTokens)) + int64(lo.FromPtr(request.CompletionTokens)), true
}

func formatCompact(n int64) string {
	if n < 0 {
		return "-" + formatCompact(-n)
	}
	unit, suffix := int64(1), ""
	switch {
	case n >= 1_000_000_000:
		unit, suffix = 1_000_000_000, "b"
	case n >= 1_000_000:
		unit, suffix = 1_000_000, "m"
	case n >= 1000:
		unit, suffix = 1000, "k"
	default:
		return strconv.FormatInt(n, 10)
	}
	tenths := int64(math.Round(float64(n) / float64(unit) * 10))
	if tenths >= 10_000 && unit < 1_000_000_000 {
		return formatCompact(unit * 1000)
	}
	text := strconv.FormatInt(tenths/10, 10)
	if tenths%10 != 0 {
		text += "." + strconv.FormatInt(tenths%10, 10)
	}
	return text + suffix
}

func formatLatency(ms int) string {
	if ms < 0 {
		ms = 0
	}
	if ms < 1000 {
		return locales.Milliseconds(strconv.Itoa(ms))
	}
	tenths := int64(math.Round(float64(ms) / 100))
	text := strconv.FormatInt(tenths/10, 10)
	if tenths%10 != 0 {
		text += "." + strconv.FormatInt(tenths%10, 10)
	}
	return locales.Seconds(text)
}

// statusText is a request status in the language the app shows. A status
// the panel does not name stays as the server sent it.
func statusText(status string) string {
	switch status {
	case statusProcessing:
		return locales.Processing()
	case statusFailed:
		return locales.StatusFailed()
	case statusCanceled:
		return locales.StatusCanceled()
	case statusCompleted:
		return locales.Completed()
	default:
		return strings.TrimSpace(status)
	}
}
