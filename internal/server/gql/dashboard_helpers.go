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

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelprobe"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/server/biz"
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

func buildDateExpression(dialectName string, timestampCol string, offsetSeconds int, locName string) string {
	switch dialectName {
	case dialect.SQLite:
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', datetime(%s, 'unixepoch', '%+d seconds'))", timestampCol, offsetSeconds)
	case dialect.MySQL:
		offsetStr := xtime.FormatUTCOffset(offsetSeconds)
		return fmt.Sprintf("DATE_FORMAT(CONVERT_TZ(FROM_UNIXTIME(%s), '+00:00', '%s'), '%%Y-%%m-%%d')", timestampCol, offsetStr)
	case dialect.Postgres:
		return fmt.Sprintf("to_char(to_timestamp(%s) AT TIME ZONE '%s', 'YYYY-MM-DD')", timestampCol, locName)
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

// parseTimeWindow parses a time window string and returns the start time and a flag indicating
// if a filter should be applied. It returns the since time (zero if no filter) and applyFilter.
// Supported timeWindow values: "day", "week", "month", "allTime", or empty string.
// Defaults to "allTime" behavior (no filtering) for unknown or empty values.
func (r *queryResolver) parseTimeWindow(ctx context.Context, timeWindow *string) (since time.Time, applyFilter bool) {
	loc := r.systemService.TimeLocation(ctx)
	period := xtime.GetCalendarPeriods(loc)

	if timeWindow != nil && *timeWindow != "" && *timeWindow != "allTime" {
		applyFilter = true

		switch *timeWindow {
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

func (r *queryResolver) dailyRequestStats(ctx context.Context) ([]*DailyRequestStats, error) {
	daysCount := 30

	loc := r.systemService.TimeLocation(ctx)
	nowUTC := xtime.UTCNow()
	nowLocal := nowUTC.In(loc)
	startDateLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysCount+1)
	startDateUTC := startDateLocal.UTC()
	_, offsetSeconds := nowLocal.Zone()

	// Use GROUP BY aggregation for efficient database-level computation
	type dailyStats struct {
		Date   string  `json:"date"`
		Count  int     `json:"total_count"`
		Tokens int     `json:"total_tokens"`
		Cost   float64 `json:"total_cost"`
	}

	var results []dailyStats

	// Use raw SQL for complex GROUP BY with conditional counting
	err := r.client.UsageLog.Query().
		Where(
			usagelog.CreatedAtGTE(startDateUTC),
			usagelog.CreatedAtLT(nowUTC),
		).
		Modify(func(s *sql.Selector) {
			// Build a dialect-specific date expression that returns a string 'YYYY-MM-DD'
			var dateExpr string
			// Use qualified column name to avoid ambiguity when joining
			createdAtCol := s.C(usagelog.FieldCreatedAt)

			switch s.Dialect() {
			case dialect.SQLite:
				dateExpr = fmt.Sprintf("strftime('%%Y-%%m-%%d', datetime(substr(%s, 1, 19), '%+d seconds'))", createdAtCol, offsetSeconds)
			case dialect.MySQL:
				offsetStr := xtime.FormatUTCOffset(offsetSeconds)
				dateExpr = fmt.Sprintf("DATE_FORMAT(CONVERT_TZ(%s, '+00:00', '%s'), '%%Y-%%m-%%d')", createdAtCol, offsetStr)
			case dialect.Postgres:
				dateExpr = fmt.Sprintf("to_char(%s AT TIME ZONE '%s', 'YYYY-MM-DD')", createdAtCol, loc.String())
			default:
				// Fallback to ANSI-ish cast; many DBs accept this, but not guaranteed
				dateExpr = fmt.Sprintf("DATE(%s)", createdAtCol)
			}

			s.Select(
				sql.As(dateExpr, "date"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "total_count"),
				sql.As(sql.Sum(s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "total_cost"),
			).
				GroupBy(dateExpr).
				OrderBy("date")
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily request stats: %w", err)
	}

	// Create a map for fast lookup of aggregated data
	statsMap := lo.SliceToMap(results, func(item dailyStats) (string, dailyStats) {
		return item.Date, item
	})

	// Build complete response with zero values for missing dates
	response := make([]*DailyRequestStats, 0, daysCount)

	for i := range daysCount {
		date := startDateLocal.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")

		if stats, exists := statsMap[dateStr]; exists {
			// Use aggregated data from database
			response = append(response, &DailyRequestStats{
				Date:   dateStr,
				Count:  stats.Count,
				Tokens: stats.Tokens,
				Cost:   stats.Cost,
			})
		} else {
			// Fill missing dates with zero values
			response = append(response, &DailyRequestStats{
				Date:   dateStr,
				Count:  0,
				Tokens: 0,
				Cost:   0,
			})
		}
	}

	return response, nil
}

func normalizeChannelSuccessRatesParams(timeWindow *string, limit *int) (*string, *int) {
	window := "day"
	if timeWindow != nil && *timeWindow != "" {
		switch *timeWindow {
		case "day", "week", "month":
			window = *timeWindow
		default:
			// Unknown windows are all-time in parseTimeWindow; canonicalize
			// them so distinct spellings share one cache key.
			window = "allTime"
		}
	}

	limitCount := 0
	if limit != nil && *limit > 0 {
		limitCount = *limit
	}

	return &window, &limitCount
}

func (r *queryResolver) channelSuccessRates(ctx context.Context, timeWindow *string, limit *int) ([]*ChannelSuccessRate, error) {
	// Parse time window, default to "day"
	if timeWindow == nil || *timeWindow == "" {
		defaultWindow := "day"
		timeWindow = &defaultWindow
	}
	since, applyFilter := r.parseTimeWindow(ctx, timeWindow)

	// Handle limit parameter (0 means no limit)
	limitCount := 0
	if limit != nil && *limit > 0 {
		limitCount = *limit
	}

	type channelExecutionStats struct {
		ChannelID    int `json:"channel_id"`
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}

	var results []channelExecutionStats

	// Step 1: Get success/failure counts from request_execution
	err := r.client.RequestExecution.Query().
		Modify(func(s *sql.Selector) {
			s.Select(
				requestexecution.FieldChannelID,
				sql.As("SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END)", "success_count"),
				sql.As("SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END)", "failed_count"),
			).
				Where(sql.NotNull(requestexecution.FieldChannelID))

			// Apply time filter
			if applyFilter {
				s.Where(sql.GTE(s.C(requestexecution.FieldCreatedAt), since))
			}

			s.GroupBy(requestexecution.FieldChannelID)
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel execution stats: %w", err)
	}

	if len(results) == 0 {
		return []*ChannelSuccessRate{}, nil
	}

	// Build response with success rate calculation
	var response []*ChannelSuccessRate

	for _, result := range results {
		totalCount := result.SuccessCount + result.FailedCount

		var successRate float64
		if totalCount > 0 {
			successRate = float64(result.SuccessCount) / float64(totalCount) * 100
		}

		response = append(response, &ChannelSuccessRate{
			ChannelID:       objects.GUID{Type: "Channel", ID: result.ChannelID},
			ChannelName:     "",
			ChannelType:     "",
			ChannelDisabled: false,
			SuccessCount:    result.SuccessCount,
			FailedCount:     result.FailedCount,
			TotalCount:      totalCount,
			SuccessRate:     successRate,
		})
	}

	// Order by total count (descending)
	sort.Slice(response, func(i, j int) bool {
		return response[i].TotalCount > response[j].TotalCount
	})

	// Apply limit
	if limitCount > 0 && len(response) > limitCount {
		response = response[:limitCount]
	}

	// Get channel details for the top channels (including soft-deleted channels)
	channelIDs := lo.Map(response, func(item *ChannelSuccessRate, _ int) int {
		return item.ChannelID.ID
	})

	ctx = schematype.SkipSoftDelete(ctx)

	channels, err := r.client.Channel.Query().
		Where(channel.IDIn(channelIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel details: %w", err)
	}

	channelMap := lo.SliceToMap(channels, func(ch *ent.Channel) (int, *ent.Channel) {
		return ch.ID, ch
	})

	// Fill in channel details
	for _, item := range response {
		if ch, exists := channelMap[item.ChannelID.ID]; exists {
			item.ChannelName = ch.Name
			item.ChannelType = string(ch.Type)
			item.ChannelDisabled = ch.Status != "enabled"
		}
	}

	return response, nil
}

func (r *queryResolver) fastestChannels(ctx context.Context, input FastestChannelsInput) ([]*FastestChannel, error) {
	// Validate and set default limit
	if input.Limit == nil || *input.Limit <= 0 {
		input.Limit = new(int)
		*input.Limit = 5
	}
	if *input.Limit > 100 {
		*input.Limit = 100
	}

	// Parse time window using calendar periods (like Token Statistics)
	loc := r.systemService.TimeLocation(ctx)
	period := xtime.GetCalendarPeriods(loc)

	var since time.Time
	switch input.TimeWindow {
	case "day":
		since = period.Today.Start
	case "week":
		since = period.ThisWeek.Start
	case "month":
		since = period.ThisMonth.Start
	default:
		since = period.Today.Start // Default to day
	}

	// Query structure for aggregation

	type channelStats struct {
		ChannelID    int     `json:"channel_id"`
		ChannelName  string  `json:"channel_name"`
		ChannelType  string  `json:"channel_type"`
		TokensCount  int64   `json:"tokens_count"`
		LatencyMs    int64   `json:"latency_ms"`
		RequestCount int64   `json:"request_count"`
		Throughput   float64 `json:"throughput"`
	}
	var results []channelStats
	dbDriver := r.client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	// Detect dialect to use appropriate placeholder syntax
	// PostgreSQL uses $1, $2, etc. while SQLite uses ? placeholders
	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	queryMode := qb.ThroughputModeRowNumber
	if dialectName == dialect.Postgres {
		queryMode = qb.ThroughputModeDistinctOn
	}

	// PostgreSQL uses DISTINCT ON for this ranking; other dialects keep
	// ROW_NUMBER, which avoids the correlated MAX(id) scan on SQLite.
	// Fetch more items than needed to allow confidence-based filtering
	sqlLimit := max(*input.Limit*4, 20)
	query := qb.BuildThroughputQuery(
		useDollarPlaceholders,
		qb.ThroughputQueryByChannel,
		sqlLimit,
		queryMode,
	)

	// Use UTC for the time parameter to match the timezone of the created_at column.
	// This assumes created_at is stored in UTC, which is consistent with the application's timezone handling.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	// The window start is the sole query argument, bound once for the
	// selected query mode (ROW_NUMBER or DISTINCT ON).
	queryArgs := []any{since.UTC()}

	rows, err := sqlDB.DB().QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fastest channels: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var stat channelStats
		if err := rows.Scan(
			&stat.ChannelID,
			&stat.ChannelName,
			&stat.ChannelType,
			&stat.TokensCount,
			&stat.LatencyMs,
			&stat.RequestCount,
			&stat.Throughput,
		); err != nil {
			return nil, fmt.Errorf("failed to scan channel stats: %w", err)
		}
		results = append(results, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(results) == 0 {
		return []*FastestChannel{}, nil
	}

	resultsToShow := calculateConfidenceAndSort(results,
		func(item channelStats) int64 { return item.RequestCount },
		func(item channelStats) float64 { return item.Throughput },
		*input.Limit,
	)

	// Build response with confidence levels
	return lo.Map(resultsToShow, func(item scoredItem[channelStats], _ int) *FastestChannel {
		return &FastestChannel{
			ChannelID:       objects.GUID{Type: "Channel", ID: item.stats.ChannelID},
			ChannelName:     item.stats.ChannelName,
			ChannelType:     item.stats.ChannelType,
			Throughput:      item.stats.Throughput,
			TokensCount:     safeIntFromInt64(item.stats.TokensCount),
			LatencyMs:       safeIntFromInt64(item.stats.LatencyMs),
			RequestCount:    safeIntFromInt64(item.stats.RequestCount),
			ConfidenceLevel: item.confidence,
		}
	}), nil
}

func (r *queryResolver) fastestModels(ctx context.Context, input FastestChannelsInput) ([]*FastestModel, error) {
	// Validate and set default limit
	if input.Limit == nil || *input.Limit <= 0 {
		input.Limit = new(int)
		*input.Limit = 5
	}
	if *input.Limit > 100 {
		*input.Limit = 100
	}

	// Parse time window using calendar periods (like Token Statistics)
	loc := r.systemService.TimeLocation(ctx)
	period := xtime.GetCalendarPeriods(loc)

	var since time.Time
	switch input.TimeWindow {
	case "day":
		since = period.Today.Start
	case "week":
		since = period.ThisWeek.Start
	case "month":
		since = period.ThisMonth.Start
	default:
		since = period.Today.Start // Default to day
	}

	// Query structure for aggregation

	type modelStats struct {
		ModelID      string  `json:"model_id"`
		ModelName    string  `json:"model_name"`
		TokensCount  int64   `json:"tokens_count"`
		LatencyMs    int64   `json:"latency_ms"`
		RequestCount int64   `json:"request_count"`
		Throughput   float64 `json:"throughput"`
	}
	var results []modelStats
	dbDriver := r.client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	// Detect dialect to use appropriate placeholder syntax
	// PostgreSQL uses $1, $2, etc. while SQLite uses ? placeholders
	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	// PostgreSQL uses DISTINCT ON for this ranking; other dialects keep
	// ROW_NUMBER, which avoids the correlated MAX(id) scan on SQLite.
	queryMode := qb.ThroughputModeRowNumber
	if dialectName == dialect.Postgres {
		queryMode = qb.ThroughputModeDistinctOn
	}

	// Fetch more items than needed to allow confidence-based filtering
	sqlLimit := max(*input.Limit*4, 20)

	query := qb.BuildThroughputQuery(
		useDollarPlaceholders,
		qb.ThroughputQueryByModel,
		sqlLimit,
		queryMode,
	)

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}
	// The window start is the sole query argument, bound once for the
	// selected query mode (ROW_NUMBER or DISTINCT ON).
	queryArgs := []any{since.UTC()}

	rows, err := sqlDB.DB().QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query fastest models: %w", err)
	}

	defer func() { _ = rows.Close() }()

	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var stat modelStats
		if err := rows.Scan(
			&stat.ModelID,
			&stat.ModelName,
			&stat.TokensCount,
			&stat.LatencyMs,
			&stat.RequestCount,
			&stat.Throughput,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model stats: %w", err)
		}
		results = append(results, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	if len(results) == 0 {
		return []*FastestModel{}, nil
	}

	resultsToShow := calculateConfidenceAndSort(results,
		func(item modelStats) int64 { return item.RequestCount },
		func(item modelStats) float64 { return item.Throughput },
		*input.Limit,
	)

	// Build response with confidence levels
	return lo.Map(resultsToShow, func(item scoredItem[modelStats], _ int) *FastestModel {
		return &FastestModel{
			ModelID:         item.stats.ModelID,
			ModelName:       item.stats.ModelName,
			Throughput:      item.stats.Throughput,
			TokensCount:     safeIntFromInt64(item.stats.TokensCount),
			LatencyMs:       safeIntFromInt64(item.stats.LatencyMs),
			RequestCount:    safeIntFromInt64(item.stats.RequestCount),
			ConfidenceLevel: item.confidence,
		}
	}), nil
}

func (r *queryResolver) modelPerformanceStats(ctx context.Context) ([]*ModelPerformanceStat, error) {
	// Add 30-second timeout to prevent long-running queries
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	daysCount := 30

	loc := r.systemService.TimeLocation(ctx)
	nowUTC := xtime.UTCNow()
	nowLocal := nowUTC.In(loc)
	startDateLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysCount+1)
	startDateUTC := startDateLocal.UTC()
	_, offsetSeconds := nowLocal.Zone()

	dbDriver := r.client.Driver()
	sqlDB, ok := dbDriver.(*sql.Driver)
	if !ok {
		return nil, fmt.Errorf("failed to get underlying SQL driver")
	}

	dialectName := sqlDB.Dialect()
	useDollarPlaceholders := dialectName == dialect.Postgres

	placeholder := "?"
	if useDollarPlaceholders {
		placeholder = "$1"
	}

	// The bundled SQLite driver is far newer than 3.25, so window functions are
	// available. ROW_NUMBER avoids the correlated MAX(id) scan that made these
	// aggregates roughly three times slower on SQLite.
	queryMode := qb.ThroughputModeRowNumber

	// Use shared query builder for daily performance stats
	query := qb.BuildDailyPerformanceStatsQuery(
		dialectName,
		loc.String(),
		offsetSeconds,
		qb.DailyThroughputByModel,
		placeholder,
		queryMode,
	)

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context canceled: %w", err)
	}

	rows, err := sqlDB.DB().QueryContext(ctx, query, startDateUTC)
	if err != nil {
		return nil, fmt.Errorf("failed to query model performance stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type rawStat struct {
		Date         string
		ModelID      string
		TokensCount  int64
		LatencyMs    int64
		FirstTokenMs *float64
		RequestCount int64
		Throughput   *float64
	}

	// ModelStatsBucket holds aggregated statistics for a single model.
	// This is used internally to group performance stats before ranking.
	type ModelStatsBucket struct {
		totalRequests int64
		results       []*ModelPerformanceStat
	}

	var rawResults []rawStat
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context canceled: %w", err)
		}

		var stat rawStat
		if err := rows.Scan(
			&stat.Date,
			&stat.ModelID,
			&stat.TokensCount,
			&stat.LatencyMs,
			&stat.FirstTokenMs,
			&stat.RequestCount,
			&stat.Throughput,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model performance stats: %w", err)
		}
		rawResults = append(rawResults, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	modelStats := make(map[string]*ModelStatsBucket)

	for _, raw := range rawResults {
		var ttftMs *float64
		if raw.FirstTokenMs != nil && *raw.FirstTokenMs > 0 {
			ttftMs = raw.FirstTokenMs
		}

		stat := &ModelPerformanceStat{
			Date:         raw.Date,
			ModelID:      raw.ModelID,
			Throughput:   raw.Throughput,
			TtftMs:       ttftMs,
			RequestCount: safeIntFromInt64(raw.RequestCount),
		}

		if modelStats[raw.ModelID] == nil {
			modelStats[raw.ModelID] = &ModelStatsBucket{}
		}
		modelStats[raw.ModelID].totalRequests += raw.RequestCount
		modelStats[raw.ModelID].results = append(modelStats[raw.ModelID].results, stat)
	}

	type modelInfo struct {
		modelID      string
		requestCount int64
	}

	modelInfos := lo.MapToSlice(modelStats, func(modelID string, stats *ModelStatsBucket) modelInfo {
		return modelInfo{
			modelID:      modelID,
			requestCount: stats.totalRequests,
		}
	})

	// Use requestCount for both count and value since modelInfo doesn't include throughput.
	// This ranks models by request volume rather than performance metrics.
	topModels := calculateConfidenceAndSort(modelInfos, func(m modelInfo) int64 { return m.requestCount }, func(m modelInfo) float64 { return float64(m.requestCount) }, topPerformersLimit)

	statsResults := make([]*ModelPerformanceStat, 0)
	for _, item := range topModels {
		statsResults = append(statsResults, modelStats[item.stats.modelID].results...)
	}

	return statsResults, nil
}

func (r *queryResolver) channelPerformanceStats(ctx context.Context) ([]*ChannelPerformanceStat, error) {
	var cancel context.CancelFunc

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	daysCount := 30

	loc := r.systemService.TimeLocation(ctx)
	nowUTC := xtime.UTCNow()
	nowLocal := nowUTC.In(loc)
	startDateLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysCount+1)
	startTimestamp := startDateLocal.UTC().Unix()
	_, offsetSeconds := nowLocal.Zone()

	probeResults, err := r.queryChannelProbeStats(ctx, startTimestamp, loc.String(), offsetSeconds)
	if err != nil {
		return nil, err
	}

	if len(probeResults) == 0 {
		return r.buildChannelPerformanceStatsFromExecutions(ctx, startDateLocal, offsetSeconds, daysCount)
	}

	statsMap := aggregateProbeStats(probeResults)
	topChannelIDs := getTopChannelIDs(statsMap, topPerformersLimit)
	filterStatsByTopChannels(statsMap, topChannelIDs)

	channelIDs := extractChannelIDsFromStats(statsMap)
	channelNames, err := r.fetchChannelNames(ctx, channelIDs)
	if err != nil {
		return nil, err
	}

	return buildChannelPerformanceResponse(statsMap, channelNames, startDateLocal, daysCount), nil
}

// channelModelStat is the per (channel_id, model_id) aggregate assembled from the
// execution and usage sub-queries before channel names are resolved.
type channelModelStat struct {
	ChannelID    int      `json:"channel_id"`
	ModelID      string   `json:"model_id"`
	RequestCount int      `json:"request_count"`
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	TotalTokens  int64    `json:"total_tokens"`
	TotalCost    float64  `json:"total_cost"`
	SuccessCount int      `json:"success_count"`
	FailedCount  int      `json:"failed_count"`
	Throughput   *float64 `json:"throughput"`
	TtftMs       *float64 `json:"ttft_ms"`
}

// executionStat is the per (channel_id, model_id) request/success/failed aggregate
// sourced from request_executions — the source of truth for the success-rate
// denominator because it records every attempt (including failures, which never
// write a usage_log).
type executionStat struct {
	ChannelID    int    `json:"channel_id"`
	ModelID      string `json:"model_id"`
	RequestCount int    `json:"request_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
}

// usageStat is the per (channel_id, model_id) token/cost/throughput/ttft aggregate
// sourced from usage_log, joined to the latest execution per request/channel pair.
type usageStat struct {
	ChannelID    int      `json:"channel_id"`
	ModelID      string   `json:"model_id"`
	RequestCount int      `json:"request_count"`
	SuccessCount int      `json:"success_count"`
	FailedCount  int      `json:"failed_count"`
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	TotalTokens  int64    `json:"total_tokens"`
	TotalCost    float64  `json:"total_cost"`
	Throughput   *float64 `json:"throughput"`
	TtftMs       *float64 `json:"ttft_ms"`
}

func (r *queryResolver) channelModelPerformanceStats(ctx context.Context, timeWindow *string, limit *int) ([]*ChannelModelPerformanceStat, error) {
	since, applyFilter := r.parseTimeWindow(ctx, timeWindow)

	executionResults, err := r.queryChannelModelExecutionStats(ctx, since, applyFilter)
	if err != nil {
		return nil, err
	}
	usageResults, err := r.queryChannelModelUsageStats(ctx, since, applyFilter)
	if err != nil {
		return nil, err
	}

	results := mergeChannelModelStats(executionResults, usageResults)
	if len(results) == 0 {
		return []*ChannelModelPerformanceStat{}, nil
	}

	response, err := r.attachChannelNamesToChannelModelStats(ctx, results)
	if err != nil {
		return nil, err
	}

	limitCount := 0
	if limit != nil && *limit > 0 {
		limitCount = *limit
	}

	// Order by request count (descending) with a deterministic tie-breaker so a
	// positive limit that cuts through rows with equal request count is stable
	// across refreshes.
	sort.Slice(response, func(i, j int) bool {
		a, b := response[i], response[j]
		if a.RequestCount != b.RequestCount {
			return a.RequestCount > b.RequestCount
		}
		if a.ChannelName != b.ChannelName {
			return a.ChannelName < b.ChannelName
		}
		if a.ChannelID != b.ChannelID {
			return a.ChannelID < b.ChannelID
		}
		return a.ModelID < b.ModelID
	})
	if limitCount > 0 && len(response) > limitCount {
		response = response[:limitCount]
	}

	return response, nil
}

// queryChannelModelExecutionStats aggregates request/success/failed counts per
// (channel_id, model_id) from request_executions.
func (r *queryResolver) queryChannelModelExecutionStats(ctx context.Context, since time.Time, applyFilter bool) ([]executionStat, error) {
	var results []executionStat
	err := r.client.RequestExecution.Query().
		Modify(func(s *sql.Selector) {
			s.Select(
				sql.As(s.C(requestexecution.FieldChannelID), "channel_id"),
				sql.As(s.C(requestexecution.FieldModelID), "model_id"),
				sql.As(sql.Count(s.C(requestexecution.FieldID)), "request_count"),
				sql.As(fmt.Sprintf("SUM(CASE WHEN %s = '%s' THEN 1 ELSE 0 END)", s.C(requestexecution.FieldStatus), requestexecution.StatusCompleted), "success_count"),
				sql.As(fmt.Sprintf("SUM(CASE WHEN %s = '%s' THEN 1 ELSE 0 END)", s.C(requestexecution.FieldStatus), requestexecution.StatusFailed), "failed_count"),
			).
				Where(sql.NotNull(s.C(requestexecution.FieldChannelID))).
				Where(sql.NEQ(s.C(requestexecution.FieldModelID), ""))

			if applyFilter {
				s.Where(sql.GTE(s.C(requestexecution.FieldCreatedAt), since))
			}

			s.GroupBy(s.C(requestexecution.FieldChannelID), s.C(requestexecution.FieldModelID))
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel-model execution stats: %w", err)
	}
	return results, nil
}

// queryChannelModelUsageStats aggregates throughput/ttft/token/cost per
// (channel_id, model_id) from usage_log joined to its latest execution for
// latency/stream metrics. Throughput follows biz.PerformanceRecord.Calculate and
// queryDimensionPerformanceStats: only completion_tokens count, streaming requests
// exclude TTFT, and generation time is clamped to biz.MinLatencyMs so cache hits
// cannot contribute a zero duration.
func (r *queryResolver) queryChannelModelUsageStats(ctx context.Context, since time.Time, applyFilter bool) ([]usageStat, error) {
	var results []usageStat
	err := r.client.UsageLog.Query().
		Modify(func(s *sql.Selector) {
			executionTable := sql.Table(requestexecution.Table).As("cm_execution")
			latestExecutionTable := sql.Table(requestexecution.Table).As("latest_cm_execution")

			// LEFT join the single latest execution for each request/channel pair so
			// retried attempts cannot inflate throughput/ttft or be double-counted.
			// It is a LEFT join because requests and usage logs have independent
			// retention: when the GC deletes a request's executions before its usage
			// log, the usage row (its tokens and cost) must still be reported. A
			// missing execution simply leaves the execution columns NULL, so
			// throughput/ttft degrade to NULL while the token/cost totals survive.
			s.LeftJoin(executionTable).On(
				s.C(usagelog.FieldRequestID),
				executionTable.C(requestexecution.FieldRequestID),
			)
			s.OnP(sql.ColumnsEQ(
				s.C(usagelog.FieldChannelID),
				executionTable.C(requestexecution.FieldChannelID),
			))

			latestExecutionID := sql.Select(sql.Max(latestExecutionTable.C(requestexecution.FieldID))).
				From(latestExecutionTable).
				Where(sql.And(
					sql.ColumnsEQ(latestExecutionTable.C(requestexecution.FieldRequestID), s.C(usagelog.FieldRequestID)),
					sql.ColumnsEQ(latestExecutionTable.C(requestexecution.FieldChannelID), s.C(usagelog.FieldChannelID)),
				))
			s.Where(sql.Or(
				sql.IsNull(executionTable.C(requestexecution.FieldID)),
				sql.EQ(executionTable.C(requestexecution.FieldID), latestExecutionID),
			))

			s.Where(sql.NotNull(s.C(usagelog.FieldChannelID)))
			s.Where(sql.NEQ(s.C(usagelog.FieldModelID), ""))

			if applyFilter {
				s.Where(sql.GTE(s.C(usagelog.FieldCreatedAt), since))
			}

			generationLatency := fmt.Sprintf(
				"CASE WHEN %s AND %s IS NOT NULL THEN %s - %s ELSE %s END",
				executionTable.C(requestexecution.FieldStream),
				executionTable.C(requestexecution.FieldMetricsFirstTokenLatencyMs),
				executionTable.C(requestexecution.FieldMetricsLatencyMs),
				executionTable.C(requestexecution.FieldMetricsFirstTokenLatencyMs),
				executionTable.C(requestexecution.FieldMetricsLatencyMs),
			)
			effectiveLatency := fmt.Sprintf(
				"CASE WHEN %[1]s < %[2]d THEN %[2]d ELSE %[1]s END",
				generationLatency,
				biz.MinLatencyMs,
			)
			// Only count completion_tokens for rows that actually have a measured
			// latency; missing latency (e.g. interrupted streams) must not inflate
			// throughput by contributing tokens over a zero/NULL duration.
			throughput := fmt.Sprintf(
				"CASE WHEN SUM(%[1]s) > 0 THEN SUM(CASE WHEN %[3]s IS NOT NULL THEN %[2]s ELSE 0 END) * 1000.0 / SUM(%[1]s) ELSE NULL END",
				effectiveLatency,
				s.C(usagelog.FieldCompletionTokens),
				executionTable.C(requestexecution.FieldMetricsLatencyMs),
			)
			ttft := fmt.Sprintf(
				"AVG(CASE WHEN %s AND %s IS NOT NULL AND %s > 0 THEN %s END)",
				executionTable.C(requestexecution.FieldStream),
				executionTable.C(requestexecution.FieldMetricsFirstTokenLatencyMs),
				executionTable.C(requestexecution.FieldMetricsFirstTokenLatencyMs),
				executionTable.C(requestexecution.FieldMetricsFirstTokenLatencyMs),
			)

			s.Select(
				sql.As(s.C(usagelog.FieldChannelID), "channel_id"),
				sql.As(s.C(usagelog.FieldModelID), "model_id"),
				sql.As(sql.Count(s.C(usagelog.FieldID)), "request_count"),
				// Success/failed come from the joined execution's status, not from the
				// presence of a usage_log row: a failed or canceled stream can still
				// report usage and write a usage_log, so counting every usage row as a
				// success would overstate the success rate.
				sql.As(fmt.Sprintf("SUM(CASE WHEN %s = '%s' THEN 1 ELSE 0 END)", executionTable.C(requestexecution.FieldStatus), requestexecution.StatusCompleted), "success_count"),
				sql.As(fmt.Sprintf("SUM(CASE WHEN %s = '%s' THEN 1 ELSE 0 END)", executionTable.C(requestexecution.FieldStatus), requestexecution.StatusFailed), "failed_count"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldPromptTokens)), "input_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldCompletionTokens)), "output_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalTokens)), "total_tokens"),
				sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagelog.FieldTotalCost)), "total_cost"),
				sql.As(throughput, "throughput"),
				sql.As(ttft, "ttft_ms"),
			).
				GroupBy(s.C(usagelog.FieldChannelID), s.C(usagelog.FieldModelID))
		}).
		Scan(ctx, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel-model usage stats: %w", err)
	}
	return results, nil
}

// mergeChannelModelStats combines the execution and usage aggregates into one row
// per (channel_id, model_id). Execution stats are authoritative for
// request/success/failed counts; usage stats supply token/cost/throughput/ttft for
// pairs that produced usage data. A streaming request started just before a window
// boundary can write its usage_log just after it, so usage-only pairs (no in-window
// execution) are retained too — their success/failed counts come from the joined
// execution's status (a failed/canceled stream can still report usage), so their
// tokens and cost are not silently dropped and the success rate stays accurate.
func mergeChannelModelStats(executionResults []executionStat, usageResults []usageStat) []channelModelStat {
	usageByKey := lo.SliceToMap(usageResults, func(item usageStat) (string, usageStat) {
		return fmt.Sprintf("%d\x00%s", item.ChannelID, item.ModelID), item
	})

	executionKeys := make(map[string]struct{}, len(executionResults))
	results := make([]channelModelStat, 0, len(executionResults)+len(usageResults))
	for _, exec := range executionResults {
		key := fmt.Sprintf("%d\x00%s", exec.ChannelID, exec.ModelID)
		executionKeys[key] = struct{}{}
		stat := channelModelStat{
			ChannelID:    exec.ChannelID,
			ModelID:      exec.ModelID,
			RequestCount: exec.RequestCount,
			SuccessCount: exec.SuccessCount,
			FailedCount:  exec.FailedCount,
		}
		if usage, ok := usageByKey[key]; ok {
			stat.InputTokens = usage.InputTokens
			stat.OutputTokens = usage.OutputTokens
			stat.TotalTokens = usage.TotalTokens
			stat.TotalCost = usage.TotalCost
			stat.Throughput = usage.Throughput
			stat.TtftMs = usage.TtftMs
		}
		results = append(results, stat)
	}
	for _, usage := range usageResults {
		key := fmt.Sprintf("%d\x00%s", usage.ChannelID, usage.ModelID)
		if _, ok := executionKeys[key]; ok {
			continue
		}
		results = append(results, channelModelStat{
			ChannelID:    usage.ChannelID,
			ModelID:      usage.ModelID,
			RequestCount: usage.RequestCount,
			SuccessCount: usage.SuccessCount,
			FailedCount:  usage.FailedCount,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			TotalCost:    usage.TotalCost,
			Throughput:   usage.Throughput,
			TtftMs:       usage.TtftMs,
		})
	}

	return results
}

// attachChannelNamesToChannelModelStats resolves channel names for the merged stats,
// including soft-deleted channels so historical data stays attributable.
func (r *queryResolver) attachChannelNamesToChannelModelStats(ctx context.Context, results []channelModelStat) ([]*ChannelModelPerformanceStat, error) {
	channelIDSet := lo.SliceToMap(results, func(item channelModelStat) (int, struct{}) { return item.ChannelID, struct{}{} })
	channelIDs := lo.Keys(channelIDSet)

	ctx = schematype.SkipSoftDelete(ctx)
	channels, err := r.client.Channel.Query().
		Where(channel.IDIn(channelIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel details: %w", err)
	}

	channelNameMap := lo.SliceToMap(channels, func(ch *ent.Channel) (int, string) {
		return ch.ID, ch.Name
	})

	response := make([]*ChannelModelPerformanceStat, 0, len(results))
	for _, item := range results {
		totalCount := item.SuccessCount + item.FailedCount
		var successRate float64
		if totalCount > 0 {
			successRate = float64(item.SuccessCount) / float64(totalCount) * 100
		}

		channelName := channelNameMap[item.ChannelID]
		if channelName == "" {
			channelName = fmt.Sprintf("channel-%d", item.ChannelID)
		}

		response = append(response, &ChannelModelPerformanceStat{
			ChannelID:    fmt.Sprintf("%d", item.ChannelID),
			ChannelName:  channelName,
			ModelID:      item.ModelID,
			RequestCount: item.RequestCount,
			SuccessCount: item.SuccessCount,
			FailedCount:  item.FailedCount,
			SuccessRate:  successRate,
			Throughput:   item.Throughput,
			TtftMs:       item.TtftMs,
			TotalCost:    item.TotalCost,
			TotalTokens:  safeIntFromInt64(item.TotalTokens),
		})
	}

	return response, nil
}
