package gql

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xtime"
	"github.com/looplj/axonhub/internal/server/gql/qb"
)

// recentWindow is how far back the pulse strip's two "last 24 hours" cards reach.
const recentWindow = 24 * time.Hour

// placeholderFor returns the bound-parameter marker for the dialect behind sqlDB.
// Postgres numbers its parameters, everything else uses positional markers.
func placeholderFor(sqlDB *sql.Driver) string {
	if sqlDB.Dialect() == dialect.Postgres {
		return "$1"
	}

	return "?"
}

// nearestRankOffset converts a sample size and a whole percent into the zero-based row
// index of the nearest-rank percentile: the value at that index is greater than or equal
// to at least ceil(n*percent/100) of the samples. Nearest rank rather than interpolation
// because there is no portable percentile aggregate across SQLite, MySQL and Postgres, and
// these numbers are read as "at most n% of requests were slower than this". The percent is
// an integer so the rank stays exact; 100*0.9 drifts in float64.
func nearestRankOffset(n, percent int) int {
	if n <= 0 {
		return 0
	}

	return (n*percent+99)/100 - 1
}

// firstTokenPercentileSeek returns the first-token latency at the given nearest-rank
// offset, or nil when the window holds no streaming execution at that offset.
func (r *queryResolver) firstTokenPercentileSeek(
	ctx context.Context,
	sqlDB *sql.Driver,
	queryMode qb.ThroughputQueryMode,
	placeholder string,
	since time.Time,
	offset int,
) (*int, error) {
	// MAX(id) mode binds the boundary twice, once in the outer WHERE and once in the
	// correlated subquery; ROW_NUMBER mode only needs it in the CTE. The offset is last.
	args := []any{since}
	if queryMode == qb.ThroughputModeMaxID {
		args = []any{since, since}
	}

	rows, err := sqlDB.DB().QueryContext(
		ctx,
		qb.BuildRecentFirstTokenPercentileQuery(queryMode, placeholder, placeholder),
		append(args, offset)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query first token latency percentile: %w", err)
	}

	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, nil
	}

	var value int
	if err := rows.Scan(&value); err != nil {
		return nil, fmt.Errorf("failed to scan first token latency percentile: %w", err)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating first token latency percentile: %w", err)
	}

	return &value, nil
}

// last24HoursPerformance reports the trailing 24 hours of throughput and first-token
// latency. Every field is nil when the window holds no completed generation: an empty
// install and an idle one look the same, and neither is a measured zero.
func (r *queryResolver) last24HoursPerformance(ctx context.Context) *RecentPerformanceStats {
	stats := &RecentPerformanceStats{}

	sqlDB, ok := r.client.Driver().(*sql.Driver)
	if !ok {
		log.Warn(ctx, "failed to get underlying SQL driver for recent performance")

		return stats
	}

	// The throughput leaderboards drop to the MAX(id) pattern off Postgres rather than
	// trust window functions there, and the pulse strip has to stay comparable to them.
	queryMode := qb.ThroughputModeRowNumber
	if sqlDB.Dialect() != dialect.Postgres {
		queryMode = qb.ThroughputModeMaxID
	}

	placeholder := placeholderFor(sqlDB)
	since := xtime.UTCNow().Add(-recentWindow)

	// MAX(id) mode binds the boundary twice, once in the outer WHERE and once in the
	// correlated subquery; ROW_NUMBER mode only needs it in the CTE.
	totalsArgs := []any{since, since}
	if queryMode == qb.ThroughputModeRowNumber {
		totalsArgs = []any{since}
	}

	rows, err := sqlDB.DB().QueryContext(ctx, qb.BuildRecentThroughputTotalsQuery(queryMode, placeholder), totalsArgs...)
	if err != nil {
		log.Warn(ctx, "failed to query recent throughput totals", log.Cause(err))

		return stats
	}

	var (
		completionTokens   int64
		effectiveLatencyMs int64
	)

	if rows.Next() {
		if err := rows.Scan(&completionTokens, &effectiveLatencyMs); err != nil {
			_ = rows.Close()

			log.Warn(ctx, "failed to scan recent throughput totals", log.Cause(err))

			return stats
		}
	}

	if err := rows.Err(); err != nil {
		_ = rows.Close()

		log.Warn(ctx, "error iterating recent throughput totals", log.Cause(err))

		return stats
	}

	_ = rows.Close()

	// Divided in Go so a window with no generation time reports "no measurement" instead
	// of a zero rate, which would read as "measured, and infinitely slow".
	if effectiveLatencyMs > 0 {
		throughput := float64(completionTokens) * 1000.0 / float64(effectiveLatencyMs)
		stats.Throughput = &throughput
	}

	sampleRows, err := sqlDB.DB().QueryContext(ctx, qb.BuildRecentFirstTokenSampleCountQuery(queryMode, placeholder), totalsArgs...)
	if err != nil {
		log.Warn(ctx, "failed to count recent first token samples", log.Cause(err))

		return stats
	}

	var sampleCount int
	if sampleRows.Next() {
		if err := sampleRows.Scan(&sampleCount); err != nil {
			_ = sampleRows.Close()

			log.Warn(ctx, "failed to scan recent first token samples", log.Cause(err))

			return stats
		}
	}

	_ = sampleRows.Close()

	if sampleCount == 0 {
		return stats
	}

	if p50, err := r.firstTokenPercentileSeek(ctx, sqlDB, queryMode, placeholder, since, nearestRankOffset(sampleCount, 50)); err != nil {
		log.Warn(ctx, "failed to read recent first token p50", log.Cause(err))
	} else {
		stats.FirstTokenP50Ms = p50
	}

	if p90, err := r.firstTokenPercentileSeek(ctx, sqlDB, queryMode, placeholder, since, nearestRankOffset(sampleCount, 90)); err != nil {
		log.Warn(ctx, "failed to read recent first token p90", log.Cause(err))
	} else {
		stats.FirstTokenP90Ms = p90
	}

	return stats
}

// last24HoursExecutionStats counts the terminal request executions of the trailing 24
// hours. Success rate cannot come from usage_logs, which has no status column; the CASE
// expressions are the ones queryExecutionSuccessCounts uses, so the two cannot drift
// apart.
func (r *queryResolver) last24HoursExecutionStats(ctx context.Context) *ExecutionOutcomeStats {
	stats := &ExecutionOutcomeStats{}

	var results []struct {
		SuccessCount int `json:"success_count"`
		FailedCount  int `json:"failed_count"`
	}

	err := r.client.RequestExecution.Query().
		Where(requestexecution.CreatedAtGTE(xtime.UTCNow().Add(-recentWindow))).
		Modify(func(s *sql.Selector) {
			s.Select(
				// SUM over an empty table returns NULL rather than 0, so zero it
				// explicitly to match the other aggregates in this package.
				sql.As("COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END), 0)", "success_count"),
				sql.As("COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)", "failed_count"),
			)
		}).
		Scan(ctx, &results)
	if err != nil {
		log.Warn(ctx, "failed to count recent executions", log.Cause(err))

		return stats
	}

	if len(results) > 0 {
		stats.Succeeded = results[0].SuccessCount
		stats.Failed = results[0].FailedCount
	}

	if total := stats.Succeeded + stats.Failed; total > 0 {
		stats.SuccessRate = float64(stats.Succeeded) / float64(total) * 100
	}

	return stats
}
