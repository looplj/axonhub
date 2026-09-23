package gql

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"golang.org/x/sync/singleflight"

	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/server/gql/qb"
)

var (
	allTimeCache        *TokenStats
	allTimeCacheTime    time.Time
	allTimeCacheMu      sync.RWMutex
	softTTL             = 1 * time.Hour
	hardTTL             = 24 * time.Hour
	allTimeRefreshGroup singleflight.Group
)

// cacheResult holds the result of a cache refresh operation.
type cacheResult struct {
	stats *TokenStats
	time  time.Time
}

// SetTokenStatsCacheTTL sets the cache TTL values for all-time token stats.
func SetTokenStatsCacheTTL(soft, hard time.Duration) {
	allTimeCacheMu.Lock()
	defer allTimeCacheMu.Unlock()
	softTTL = soft
	hardTTL = hard
}

// InvalidateAllTimeTokenStatsCache clears the all-time token stats cache.
func InvalidateAllTimeTokenStatsCache() {
	allTimeCacheMu.Lock()
	allTimeCache = nil
	allTimeCacheTime = time.Time{}
	allTimeCacheMu.Unlock()
}

// defaultPerformanceWindowDays bounds the performance queries when no range is supplied.
const defaultPerformanceWindowDays = 30

// hourlyResolutionMaxDays is the widest range served at hourly resolution. Beyond it the
// bucket count grows past what a single chart can show legibly, so the server falls back
// to daily buckets.
const hourlyResolutionMaxDays = 14

// resolutionForSpan picks the bucket granularity for a range spanning the given number of
// days. Every time series on the dashboard shares this rule so a given range always
// resolves to the same granularity across charts.
func resolutionForSpan(days int) qb.DateResolution {
	if days <= hourlyResolutionMaxDays {
		return qb.ResolutionHour
	}

	return qb.ResolutionDay
}

// performanceWindow is the resolved time range plus the bucket resolution the performance
// queries should use for it.
type performanceWindow struct {
	startLocal time.Time
	endLocal   time.Time
	resolution qb.DateResolution
}

// bucketSequence lists every bucket label in [start, end) at the given resolution. Charts
// draw a category per label, so the sequence has to be complete: omitting an empty bucket
// would pull its neighbours together and misrepresent the elapsed time. Both label formats
// sort lexicographically in time order.
func bucketSequence(start, end time.Time, resolution qb.DateResolution) []string {
	layout := "2006-01-02"
	if resolution == qb.ResolutionHour {
		layout = "2006-01-02 15:00"
	}

	labels := make([]string, 0, 64)
	for d := start; d.Before(end); {
		label := d.Format(layout)

		// Hourly buckets are stepped in absolute time rather than rebuilt from wall-clock
		// components: time.Date resolves a skipped hour backwards, so reconstructing the
		// next hour from the current one never advances across a spring-forward transition.
		// The same absolute step lands twice in the hour a fall-back transition repeats,
		// and that duplicate is collapsed so each wall-clock hour is one bucket.
		if len(labels) == 0 || labels[len(labels)-1] != label {
			labels = append(labels, label)
		}

		if resolution == qb.ResolutionHour {
			d = d.Add(time.Hour)
		} else {
			d = d.AddDate(0, 0, 1)
		}
	}

	return labels
}

// resolvePerformanceWindow turns the optional start/end dates into a local time range and
// picks a bucket resolution for it. Dates are "YYYY-MM-DD" and inclusive; the end becomes
// the next local midnight so the whole end day is covered by the half-open SQL bound.
// A relative window, when given, takes precedence over the dates.
// An unparseable or empty range falls back to the trailing default window.
func (r *queryResolver) resolvePerformanceWindow(ctx context.Context, timeWindow, startTime, endTime *string) performanceWindow {
	loc := r.systemService.TimeLocation(ctx)
	nowLocal := xtime.UTCNow().In(loc)
	todayLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc)

	window := performanceWindow{
		startLocal: todayLocal.AddDate(0, 0, -defaultPerformanceWindowDays+1),
		endLocal:   todayLocal.AddDate(0, 0, 1),
		resolution: qb.ResolutionDay,
	}

	// A relative window ends at the current instant, so endLocal is not a midnight and
	// the caller must not treat the range as covering whole days.
	if since, ok := relativeSince(timeWindow); ok {
		start := since.In(loc).Truncate(time.Hour)
		window.startLocal = start
		window.endLocal = nowLocal
		window.resolution = qb.ResolutionHour

		return window
	}

	parsedStart, okStart := parseWindowDate(startTime, loc, todayLocal)
	parsedEnd, okEnd := parseWindowDate(endTime, loc, todayLocal)
	if !okStart && !okEnd {
		return window
	}

	window.startLocal = parsedStart
	window.endLocal = parsedEnd.AddDate(0, 0, 1)

	if window.endLocal.Before(window.startLocal) {
		window.endLocal = window.startLocal.AddDate(0, 0, 1)
	}

	window.resolution = resolutionForSpan(int(window.endLocal.Sub(window.startLocal).Hours() / 24))

	return window
}

// parseWindowDate parses an optional "YYYY-MM-DD" date into local midnight, falling back to
// today when the value is absent or malformed.
func parseWindowDate(value *string, loc *time.Location, todayLocal time.Time) (time.Time, bool) {
	if value == nil || *value == "" {
		return todayLocal, false
	}

	parsed, err := time.ParseInLocation("2006-01-02", *value, loc)
	if err != nil {
		return todayLocal, false
	}

	return parsed, true
}

// placeholdersFor returns the bound placeholders for a dialect. Postgres uses numbered
// parameters, everything else positional "?" markers.
func placeholdersFor(dialectName string) (start string, end string, dollar bool) {
	if dialectName == dialect.Postgres {
		return "$1", "$2", true
	}

	return "?", "?", false
}

type scoredItem[T any] struct {
	stats      T
	confidence string
	score      int
}

func safeIntFromInt64(v int64) int {
	const (
		maxInt = int(^uint(0) >> 1)
		minInt = -maxInt - 1
	)

	if v > int64(maxInt) {
		return maxInt
	}

	if v < int64(minInt) {
		return minInt
	}

	return int(v)
}

func buildDateExpression(dialectName string, timestampCol string, offsetSeconds int, locName string, resolution qb.DateResolution) string {
	switch dialectName {
	case dialect.SQLite:
		if resolution == qb.ResolutionHour {
			return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:00', datetime(%s, 'unixepoch', '%+d seconds'))", timestampCol, offsetSeconds)
		}
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', datetime(%s, 'unixepoch', '%+d seconds'))", timestampCol, offsetSeconds)
	case dialect.MySQL:
		offsetStr := xtime.FormatUTCOffset(offsetSeconds)
		layout := "%Y-%m-%d"
		if resolution == qb.ResolutionHour {
			layout = "%Y-%m-%d %H:00"
		}
		return fmt.Sprintf("DATE_FORMAT(CONVERT_TZ(FROM_UNIXTIME(%s), '+00:00', '%s'), '%s')", timestampCol, offsetStr, layout)
	case dialect.Postgres:
		layout := "YYYY-MM-DD"
		if resolution == qb.ResolutionHour {
			layout = "YYYY-MM-DD HH24:00"
		}
		return fmt.Sprintf("to_char(to_timestamp(%s) AT TIME ZONE '%s', '%s')", timestampCol, locName, layout)
	default:
		return fmt.Sprintf("DATE(%s)", timestampCol)
	}
}

func buildProbeQuerySelects(s *sql.Selector, dateExpr string) []string {
	avgTokensCol := s.C(channelprobe.FieldAvgTokensPerSecond)
	totalRequestsCol := s.C(channelprobe.FieldTotalRequestCount)
	avgTTFTCol := s.C(channelprobe.FieldAvgTimeToFirstTokenMs)
	channelIDCol := s.C(channelprobe.FieldChannelID)

	throughputExpr := fmt.Sprintf(
		"SUM(CASE WHEN %s IS NOT NULL THEN %s * %s ELSE 0 END) / NULLIF(SUM(CASE WHEN %s IS NOT NULL THEN %s ELSE 0 END), 0)",
		avgTokensCol, avgTokensCol, totalRequestsCol, avgTokensCol, totalRequestsCol,
	)
	ttftExpr := fmt.Sprintf(
		"SUM(CASE WHEN %s IS NOT NULL THEN %s * %s ELSE 0 END) / NULLIF(SUM(CASE WHEN %s IS NOT NULL THEN %s ELSE 0 END), 0)",
		avgTTFTCol, avgTTFTCol, totalRequestsCol, avgTTFTCol, totalRequestsCol,
	)

	return []string{
		sql.As(dateExpr, "date"),
		sql.As(channelIDCol, "channel_id"),
		sql.As(sql.Sum(totalRequestsCol), "request_count"),
		sql.As(throughputExpr, "throughput"),
		sql.As(ttftExpr, "ttft_ms"),
	}
}

func calculateConfidenceAndSort[T any](
	results []T,
	getRequestCount func(T) int64,
	getThroughput func(T) float64,
	limit int,
) []scoredItem[T] {
	if len(results) == 0 {
		return nil
	}

	requestCounts := lo.Map(results, func(item T, _ int) int {
		return int(getRequestCount(item))
	})
	sort.Ints(requestCounts)

	var median float64

	mid := len(requestCounts) / 2
	if len(requestCounts)%2 == 0 {
		median = float64(requestCounts[mid-1]+requestCounts[mid]) / 2
	} else {
		median = float64(requestCounts[mid])
	}

	scoredResults := lo.Map(results, func(item T, _ int) scoredItem[T] {
		conf := qb.CalculateConfidenceLevel(int(getRequestCount(item)), median)
		score := 0

		switch conf {
		case "high":
			score = 3
		case "medium":
			score = 2
		case "low":
			score = 1
		}

		return scoredItem[T]{
			stats:      item,
			confidence: conf,
			score:      score,
		}
	})

	filtered := lo.Filter(scoredResults, func(item scoredItem[T], _ int) bool {
		return item.confidence == "high" || item.confidence == "medium"
	})

	resultsToShow := scoredResults
	if len(filtered) >= limit {
		resultsToShow = filtered
	}

	sort.Slice(resultsToShow, func(i, j int) bool {
		if resultsToShow[i].score != resultsToShow[j].score {
			return resultsToShow[i].score > resultsToShow[j].score
		}

		return getThroughput(resultsToShow[i].stats) > getThroughput(resultsToShow[j].stats)
	})

	if len(resultsToShow) > limit {
		resultsToShow = resultsToShow[:limit]
	}

	return resultsToShow
}

// getTopModelsForAPIKeys returns top 3 models by total tokens for multiple API keys in a single query
func (r *queryResolver) getTopModelsForAPIKeys(ctx context.Context, apiKeyIDs []int, input *APIKeyTokenUsageStatsInput) map[int][]*ModelTokenUsageStats {
	if len(apiKeyIDs) == 0 {
		return make(map[int][]*ModelTokenUsageStats)
	}

	query := r.client.UsageLog.Query().
		Where(usagelog.APIKeyIDIn(apiKeyIDs...))

	if input != nil {
		if input.CreatedAtGTE != nil {
			query = query.Where(usagelog.CreatedAtGTE(*input.CreatedAtGTE))
		}
		if input.CreatedAtLTE != nil {
			query = query.Where(usagelog.CreatedAtLTE(*input.CreatedAtLTE))
		}
	}

	type modelStats struct {
		APIKeyID        int    `json:"api_key_id"`
		ModelID         string `json:"model_id"`
		InputTokens     int64  `json:"input_tokens"`
		OutputTokens    int64  `json:"output_tokens"`
		CachedTokens    int64  `json:"cached_tokens"`
		ReasoningTokens int64  `json:"reasoning_tokens"`
		TotalTokens     int64  `json:"total_tokens"`
	}

	var allResults []modelStats

	err := query.Modify(func(s *sql.Selector) {
		s.Select(
			s.C(usagelog.FieldAPIKeyID),
			s.C(usagelog.FieldModelID),
			sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
			sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
			sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptCachedTokens)), "cached_tokens"),
			sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionReasoningTokens)), "reasoning_tokens"),
			// Order by billable total (input + output). Reasoning is already inside
			// completion_tokens, so adding it would double-count and bias the
			// per-API-key top-N model ranking toward reasoning-heavy models.
			sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0) + COALESCE(SUM(%s), 0)",
				s.C(usagelog.FieldPromptTokens),
				s.C(usagelog.FieldCompletionTokens)), "total_tokens"),
		).GroupBy(s.C(usagelog.FieldAPIKeyID), s.C(usagelog.FieldModelID)).
			OrderBy(sql.Desc("total_tokens"))
	}).Scan(ctx, &allResults)
	if err != nil {
		log.Warn(ctx, "failed to get top models for API keys", log.Cause(err))
		return make(map[int][]*ModelTokenUsageStats)
	}

	// Group by API key and take top 3 per key
	resultMap := make(map[int][]*ModelTokenUsageStats)
	for _, result := range allResults {
		if len(resultMap[result.APIKeyID]) < 3 {
			resultMap[result.APIKeyID] = append(resultMap[result.APIKeyID], &ModelTokenUsageStats{
				ModelID:         result.ModelID,
				InputTokens:     safeIntFromInt64(result.InputTokens),
				OutputTokens:    safeIntFromInt64(result.OutputTokens),
				CachedTokens:    safeIntFromInt64(result.CachedTokens),
				ReasoningTokens: safeIntFromInt64(result.ReasoningTokens),
			})
		}
	}

	return resultMap
}

// relativeWindowLast24Hours is the one relative window the stat queries accept. It is
// not a calendar period: the boundary is now minus 24 hours, so it never lines up with
// any preset on the filter bar.
const relativeWindowLast24Hours = "last24Hours"

// relativeSince resolves the one supported relative window, "last24Hours", against
// the current instant. Relative windows are resolved here rather than from a client
// supplied boundary: a timestamp computed on the client changes on every render and
// would defeat the query cache.
func relativeSince(timeWindow *string) (time.Time, bool) {
	if timeWindow == nil || *timeWindow != relativeWindowLast24Hours {
		return time.Time{}, false
	}

	return xtime.UTCNow().Add(-24 * time.Hour), true
}

// parseTimeWindow parses a time window string and returns the start time and a flag indicating
// if a filter should be applied. It returns the since time (zero if no filter) and applyFilter.
// Supported timeWindow values: "day", "week", "month", "last24Hours", "allTime", or empty string.
// Defaults to "allTime" behavior (no filtering) for unknown or empty values.
func (r *queryResolver) parseTimeWindow(ctx context.Context, timeWindow *string) (since time.Time, applyFilter bool) {
	loc := r.systemService.TimeLocation(ctx)
	period := xtime.GetCalendarPeriods(loc)

	if timeWindow != nil && *timeWindow != "" && *timeWindow != "allTime" {
		applyFilter = true

		switch *timeWindow {
		case relativeWindowLast24Hours:
			since, _ = relativeSince(timeWindow)
		case "day":
			since = period.Today.Start
		case "week":
			since = period.ThisWeek.Start
		case "month":
			since = period.ThisMonth.Start
		default:
			// Unknown value - default to allTime behavior (no filtering)
			applyFilter = false
		}
	}

	return since, applyFilter
}
