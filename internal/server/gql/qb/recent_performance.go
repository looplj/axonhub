package qb

import "fmt"

// recentThroughputCoreSQL is the effective generation time of one completed execution:
// the full latency for non-streaming calls, and the latency minus the first-token wait
// for streaming ones. It is copied verbatim from throughputCalculationSQL so the pulse
// strip's tok/s and the leaderboards below it are the same measurement, not two similar
// ones.
const recentThroughputCoreSQL = `CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                 THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                      THEN 0
                      ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
                 ELSE se.metrics_latency_ms END`

// BuildRecentThroughputTotalsQuery returns the window's summed completion tokens and
// summed effective generation time. Both are returned raw and divided in Go so the
// zero-denominator case can be reported as "no measurement" rather than a zero rate.
//
// A request counts once, through its latest in-window completed execution, matching the
// per-request deduplication the throughput leaderboards apply.
func BuildRecentThroughputTotalsQuery(mode ThroughputQueryMode, placeholder string) string {
	if mode == ThroughputModeMaxID {
		return `SELECT
    COALESCE(SUM(ul.completion_tokens), 0) as completion_tokens,
    COALESCE(SUM(` + recentThroughputCoreSQL + `), 0) as effective_latency_ms
FROM request_executions se
JOIN usage_logs ul ON se.request_id = ul.request_id
WHERE se.status = 'completed'
    AND se.metrics_latency_ms > 0
    AND se.created_at >= ` + placeholder + `
    AND se.id = (
        SELECT MAX(re2.id)
        FROM request_executions re2
        WHERE re2.request_id = se.request_id
            AND re2.status = 'completed'
            AND re2.metrics_latency_ms > 0
            AND re2.created_at >= ` + placeholder + `
    )`
	}

	return `WITH latest_execs AS (
    SELECT request_id, metrics_latency_ms, metrics_first_token_latency_ms, stream,
           ROW_NUMBER() OVER (PARTITION BY request_id ORDER BY created_at DESC, id DESC) as rn
    FROM request_executions
    WHERE status = 'completed' AND metrics_latency_ms > 0 AND created_at >= ` + placeholder + `
)
SELECT
    COALESCE(SUM(ul.completion_tokens), 0) as completion_tokens,
    COALESCE(SUM(` + recentThroughputCoreSQL + `), 0) as effective_latency_ms
FROM latest_execs se
JOIN usage_logs ul ON se.request_id = ul.request_id
WHERE se.rn = 1`
}

// BuildRecentFirstTokenSampleCountQuery counts the streaming executions in the window
// that reported a first-token latency. This is the sample size the percentile offsets
// are computed against, so it must use the same deduplication as the seek below.
func BuildRecentFirstTokenSampleCountQuery(mode ThroughputQueryMode, placeholder string) string {
	return buildRecentFirstTokenQuery(mode, placeholder, "", false)
}

// BuildRecentFirstTokenPercentileQuery seeks the value at the given nearest-rank offset
// over the window's first-token latencies. The offset is bound, never interpolated, so
// it stays an integer in the SQL even for an empty sample.
func BuildRecentFirstTokenPercentileQuery(mode ThroughputQueryMode, placeholder, offsetPlaceholder string) string {
	return buildRecentFirstTokenQuery(mode, placeholder, offsetPlaceholder, true)
}

func buildRecentFirstTokenQuery(mode ThroughputQueryMode, placeholder, offsetPlaceholder string, seek bool) string {
	selectColumns := "COUNT(*) as sample_count"
	orderAndLimit := ""

	if seek {
		selectColumns = "se.metrics_first_token_latency_ms"
		orderAndLimit = " ORDER BY se.metrics_first_token_latency_ms LIMIT 1 OFFSET " + offsetPlaceholder
	}

	if mode == ThroughputModeMaxID {
		return fmt.Sprintf(`SELECT %s
FROM request_executions se
WHERE se.status = 'completed'
    AND se.stream
    AND se.metrics_first_token_latency_ms IS NOT NULL
    AND se.created_at >= %s
    AND se.id = (
        SELECT MAX(re2.id)
        FROM request_executions re2
        WHERE re2.request_id = se.request_id
            AND re2.status = 'completed'
            AND re2.created_at >= %s
    )%s`, selectColumns, placeholder, placeholder, orderAndLimit)
	}

	return fmt.Sprintf(`WITH latest_execs AS (
    SELECT request_id, metrics_first_token_latency_ms, stream,
           ROW_NUMBER() OVER (PARTITION BY request_id ORDER BY created_at DESC, id DESC) as rn
    FROM request_executions
    WHERE status = 'completed' AND created_at >= %s
)
SELECT %s
FROM latest_execs se
WHERE se.rn = 1
    AND se.stream
    AND se.metrics_first_token_latency_ms IS NOT NULL%s`, placeholder, selectColumns, orderAndLimit)
}
