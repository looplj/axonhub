// Package db provides database utilities and query builders for AxonHub.
//
// This package contains shared database logic that can be used across the
// codebase without depending on generated code or GraphQL types.
package db

import (
	"fmt"
)

// ThroughputQueryType identifies the type of throughput query to build.
// This enum ensures only predefined, validated query patterns can be used.
type ThroughputQueryType int

const (
	// ThroughputQueryByChannel groups throughput statistics by channel.
	// Uses channels table for channel metadata.
	ThroughputQueryByChannel ThroughputQueryType = iota
	// ThroughputQueryByModel groups throughput statistics by model.
	// Uses requests and models tables for model metadata.
	ThroughputQueryByModel
)

// ThroughputQueryMode determines which SQL pattern to use.
type ThroughputQueryMode int

const (
	// ThroughputModeROW_NUMBER uses ROW_NUMBER() window function (preferred).
	// Requires SQLite 3.25+ (released 2018-09-15), PostgreSQL, MySQL 8.0+, TiDB.
	ThroughputModeROW_NUMBER ThroughputQueryMode = iota
	// ThroughputModeMaxID uses MAX(id) subquery approach for older SQLite compatibility.
	ThroughputModeMaxID
)

// QueryFragmentConfig holds the SQL fragments for a specific query type.
// These fragments are predefined constants and never accept user input.
type QueryFragmentConfig struct {
	SelectColumns string
	JoinClause    string
	GroupBy       string
}

// AllowedQueryConfigs maps each ThroughputQueryType to its validated SQL fragments.
// This allowlist ensures only safe, pre-approved SQL patterns can be executed.
var AllowedQueryConfigs = map[ThroughputQueryType]QueryFragmentConfig{
	ThroughputQueryByChannel: {
		SelectColumns: "se.channel_id,\n    c.name as channel_name,\n    c.type as channel_type,",
		JoinClause:    "JOIN channels c ON se.channel_id = c.id",
		GroupBy:       "se.channel_id, c.name, c.type",
	},
	ThroughputQueryByModel: {
		SelectColumns: "r.model_id,\n    m.name as model_name,",
		JoinClause:    "JOIN requests r ON se.request_id = r.id\nJOIN models m ON r.model_id = m.model_id",
		GroupBy:       "r.model_id, m.name",
	},
}

// BuildThroughputQuery constructs a SQL query for throughput statistics.
//
// SECURITY NOTE: This function uses ThroughputQueryType enum instead of raw SQL strings.
// The SQL fragments are retrieved from a predefined allowlist (AllowedQueryConfigs),
// eliminating the risk of SQL injection from user input. Only the queryType parameter
// determines which SQL pattern is used, and limit is validated as a positive integer.
//
// COMPATIBILITY NOTE:
//   - ROW_NUMBER mode requires SQLite 3.25+ (released 2018-09-15).
//   - All supported database dialects (PostgreSQL, MySQL 8.0+, TiDB, SQLite 3.25+)
//     support the ROW_NUMBER() window function.
//   - For older SQLite versions, use ThroughputModeMaxID.
//
// Parameters:
//   - useDollarPlaceholders: if true, uses $1, $2, etc. for PostgreSQL; otherwise uses ?
//   - queryType: determines which SQL pattern (by channel or by model)
//   - limit: maximum number of results to return
//   - mode: which SQL pattern to use (ROW_NUMBER or MAX_ID)
//
// Returns: SQL query string with placeholders for the since timestamp
func BuildThroughputQuery(useDollarPlaceholders bool, queryType ThroughputQueryType, limit int, mode ThroughputQueryMode) string {
	// Validate that limit is positive to prevent malformed queries
	if limit <= 0 {
		limit = 20 // Default fallback
	}

	// Retrieve the validated query configuration from the allowlist
	config, ok := AllowedQueryConfigs[queryType]
	if !ok {
		// This should never happen with proper enum usage, but return a safe default
		// that will result in an empty result set rather than a malformed query
		config = AllowedQueryConfigs[ThroughputQueryByChannel]
	}

	placeholder := "$1"
	if !useDollarPlaceholders {
		placeholder = "?"
	}

	if mode == ThroughputModeMaxID {
		return buildMaxIDQuery(placeholder, config, limit)
	}

	return buildRowNumberQuery(placeholder, config, limit)
}

// buildRowNumberQuery constructs a SQL query using ROW_NUMBER() window function.
// This is the preferred approach for all modern database systems.
func buildRowNumberQuery(placeholder string, config QueryFragmentConfig, limit int) string {
	return `
WITH successful_execs AS (
    SELECT
        request_id,
        channel_id,
        metrics_latency_ms,
        metrics_first_token_latency_ms,
        stream,
        ROW_NUMBER() OVER (PARTITION BY request_id ORDER BY created_at DESC) as rn
    FROM request_executions
    WHERE status = 'completed' AND metrics_latency_ms > 0 AND created_at >= ` + placeholder + `
)
SELECT
    ` + config.SelectColumns + `
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,
    SUM(se.metrics_latency_ms) as latency_ms,
    COUNT(DISTINCT se.request_id) as request_count,
    CASE
        WHEN SUM(CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                 THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                      THEN 0
                      ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
                 ELSE se.metrics_latency_ms END) > 0
        THEN SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) * 1000.0
             / SUM(CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                   THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                        THEN 0
                        ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
                   ELSE se.metrics_latency_ms END)
        ELSE 0
    END as throughput
FROM successful_execs se
JOIN usage_logs ul ON se.request_id = ul.request_id
` + config.JoinClause + `
WHERE se.rn = 1
GROUP BY ` + config.GroupBy + `
ORDER BY throughput DESC
LIMIT ` + fmt.Sprintf("%d", limit)
}

// buildMaxIDQuery constructs a SQL query using MAX(id) subquery for SQLite compatibility.
// This approach works on older SQLite versions that don't support window functions.
func buildMaxIDQuery(placeholder string, config QueryFragmentConfig, limit int) string {
	// For MAX_ID mode, we need to adjust the query to not use the CTE pattern
	// and instead use a correlated subquery to get the latest execution per request

	return `
SELECT
    ` + config.SelectColumns + `
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,
    SUM(se.metrics_latency_ms) as latency_ms,
    COUNT(DISTINCT se.request_id) as request_count,
    CASE
        WHEN SUM(CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                 THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                      THEN 0
                      ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
                 ELSE se.metrics_latency_ms END) > 0
        THEN SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) * 1000.0
             / SUM(CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                   THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                        THEN 0
                        ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
                   ELSE se.metrics_latency_ms END)
        ELSE 0
    END as throughput
FROM request_executions se
JOIN usage_logs ul ON se.request_id = ul.request_id
` + config.JoinClause + `
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
    )
GROUP BY ` + config.GroupBy + `
ORDER BY throughput DESC
LIMIT ` + fmt.Sprintf("%d", limit)
}

// CalculateConfidenceLevel determines the confidence level based on request count and median.
//
// The confidence level indicates how reliable the throughput measurement is:
//   - "high": High confidence (sufficient sample size and above-median requests)
//   - "medium": Medium confidence (moderate sample size)
//   - "low": Low confidence (insufficient sample size)
//
// Parameters:
//   - requestCount: the number of requests for this item
//   - median: the median request count across all items
//
// Returns: "high", "medium", or "low"
func CalculateConfidenceLevel(requestCount int, median float64) string {
	// When median is 0, we cannot calculate a meaningful ratio (requestCount/median),
	// so we default to low confidence since we lack sufficient data for reliable inference.
	if median == 0 {
		return "low"
	}

	// Absolute minimum request thresholds for confidence levels
	// These ensure items with very few requests are always low confidence
	const minRequestsForMedium = 100
	const minRequestsForHigh = 500

	if requestCount < minRequestsForMedium {
		return "low"
	}

	ratio := float64(requestCount) / median
	if ratio >= 1.5 && requestCount >= minRequestsForHigh {
		return "high"
	}
	if ratio >= 0.5 {
		return "medium"
	}
	return "low"
}

// BuildProbeStatsQuery builds a specialized query for channel probe statistics.
// This query returns additional columns needed for probe metrics:
//   - total_count: total number of executions
//   - effective_latency_ms: sum of effective latency (excluding first token for streaming)
//   - total_first_token_latency: sum of first token latency
//
// Unlike BuildThroughputQuery, this does not join with channels table and returns
// raw metrics needed for probe calculations rather than throughput rankings.
//
// Parameters:
//
//   - useDollarPlaceholders: if true, uses $1, $2, etc. for PostgreSQL
//
//   - channelIDFilter: SQL fragment for channel ID filtering (e.g., "AND se.channel_id IN ($3, $4)")
//
//     SECURITY WARNING: channelIDFilter is directly concatenated into the SQL query.
//     The caller MUST ensure this parameter uses parameterized placeholders ($1, $2, etc.)
//     or validated values only. NEVER pass unsanitized user input directly.
//     Example safe usage: fmt.Sprintf("AND se.channel_id IN ($%d, $%d)", placeholderIndex+1, placeholderIndex+2)
//
//   - mode: which SQL pattern to use (ROW_NUMBER or MAX_ID)
//
// Returns: SQL query string
func BuildProbeStatsQuery(useDollarPlaceholders bool, channelIDFilter string, mode ThroughputQueryMode) string {
	placeholder1, placeholder2 := "?", "?"
	if useDollarPlaceholders {
		placeholder1, placeholder2 = "$1", "$2"
	}

	if mode == ThroughputModeMaxID {
		return fmt.Sprintf(`
SELECT
    se.channel_id,
    COUNT(*) as total_count,
    SUM(CASE WHEN se.status = 'completed' THEN 1 ELSE 0 END) as success_count,
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as total_tokens,
    SUM(CASE WHEN se.status = 'completed' THEN
        CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
             THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                  THEN 0
                  ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
             ELSE se.metrics_latency_ms END
        ELSE 0 END) as effective_latency_ms,
    SUM(CASE WHEN se.status = 'completed' AND se.stream AND se.metrics_first_token_latency_ms IS NOT NULL THEN se.metrics_first_token_latency_ms ELSE 0 END) as total_first_token_latency,
    COUNT(DISTINCT se.request_id) as request_count,
    SUM(CASE WHEN se.status = 'completed' AND se.stream AND se.metrics_first_token_latency_ms IS NOT NULL THEN 1 ELSE 0 END) as streaming_request_count
FROM request_executions se
JOIN usage_logs ul ON se.request_id = ul.request_id
WHERE se.metrics_latency_ms > 0
    AND se.created_at >= %s
    AND se.created_at < %s
    AND se.id = (
        SELECT MAX(re2.id)
        FROM request_executions re2
        WHERE re2.request_id = se.request_id
            AND re2.status = 'completed'
            AND re2.metrics_latency_ms > 0
    )
    %s
GROUP BY se.channel_id
ORDER BY se.channel_id`, placeholder1, placeholder2, channelIDFilter)
	}

	// ROW_NUMBER mode
	return fmt.Sprintf(`
WITH latest_execs AS (
    SELECT
        request_id,
        channel_id,
        metrics_latency_ms,
        metrics_first_token_latency_ms,
        stream,
        status,
        ROW_NUMBER() OVER (PARTITION BY request_id ORDER BY created_at DESC) as rn
    FROM request_executions
    WHERE metrics_latency_ms > 0 AND created_at >= %s AND created_at < %s
)
SELECT
    se.channel_id,
    COUNT(*) as total_count,
    SUM(CASE WHEN se.status = 'completed' THEN 1 ELSE 0 END) as success_count,
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as total_tokens,
    SUM(CASE WHEN se.status = 'completed' THEN
        CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
             THEN CASE WHEN se.metrics_first_token_latency_ms >= se.metrics_latency_ms
                  THEN 0
                  ELSE se.metrics_latency_ms - se.metrics_first_token_latency_ms END
             ELSE se.metrics_latency_ms END
        ELSE 0 END) as effective_latency_ms,
    SUM(CASE WHEN se.status = 'completed' AND se.stream AND se.metrics_first_token_latency_ms IS NOT NULL THEN se.metrics_first_token_latency_ms ELSE 0 END) as total_first_token_latency,
    COUNT(DISTINCT se.request_id) as request_count,
    SUM(CASE WHEN se.status = 'completed' AND se.stream AND se.metrics_first_token_latency_ms IS NOT NULL THEN 1 ELSE 0 END) as streaming_request_count
FROM latest_execs se
JOIN usage_logs ul ON se.request_id = ul.request_id
WHERE se.rn = 1
    %s
GROUP BY se.channel_id
ORDER BY se.channel_id`, placeholder1, placeholder2, channelIDFilter)
}
