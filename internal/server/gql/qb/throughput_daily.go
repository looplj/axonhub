// Package qb provides database utilities and query builders for AxonHub.
package qb

import (
	"fmt"
)

// DailyThroughputQueryType identifies the type of daily throughput query to build.
type DailyThroughputQueryType int

const (
	// DailyThroughputByChannel groups daily throughput by channel.
	DailyThroughputByChannel DailyThroughputQueryType = iota
	// DailyThroughputByModel groups daily throughput by model.
	DailyThroughputByModel
)

// DailyQueryFragmentConfig holds the SQL fragments for daily throughput queries.
type DailyQueryFragmentConfig struct {
	IDColumn      string
	NameColumn    string
	JoinClause    string
	GroupByFields string
}

// AllowedDailyQueryConfigs maps each DailyThroughputQueryType to its SQL fragments.
var AllowedDailyQueryConfigs = map[DailyThroughputQueryType]DailyQueryFragmentConfig{
	DailyThroughputByChannel: {
		IDColumn:      "se.channel_id",
		NameColumn:    "c.name as channel_name",
		JoinClause:    "JOIN channels c ON se.channel_id = c.id",
		GroupByFields: "date, se.channel_id, c.name",
	},
	DailyThroughputByModel: {
		IDColumn:      "r.model_id",
		NameColumn:    "m.name as model_name",
		JoinClause:    "JOIN requests r ON se.request_id = r.id\nJOIN models m ON r.model_id = m.model_id",
		GroupByFields: "date, r.model_id, m.name",
	},
}

// DateFormatDialect holds dialect-specific date formatting expressions.
type DateFormatDialect struct {
	// SQLite format string using strftime
	SQLite string
	// MySQL format using DATE_FORMAT and CONVERT_TZ
	MySQL string
	// Postgres format using to_char and AT TIME ZONE
	Postgres string
}

// getDateExpression returns the dialect-specific date expression for grouping by day.
// The dateExpr should include the column reference (e.g., "se.created_at").
func getDateExpression(dialect string, dateExpr string, timezone string, offsetSeconds int) string {
	switch dialect {
	case "sqlite3", "sqlite":
		// SQLite: strftime('%Y-%m-%d', datetime(substr(created_at, 1, 19), 'offset seconds'))
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', datetime(substr(%s, 1, 19), '%+d seconds'))", dateExpr, offsetSeconds)
	case "mysql":
		// MySQL: DATE_FORMAT(CONVERT_TZ(created_at, '+00:00', timezone), '%Y-%m-%d')
		return fmt.Sprintf("DATE_FORMAT(CONVERT_TZ(%s, '+00:00', '%s'), '%%Y-%%m-%%d')", dateExpr, timezone)
	case "postgres", "postgresql":
		// PostgreSQL: to_char(created_at AT TIME ZONE 'timezone', 'YYYY-MM-DD')
		return fmt.Sprintf("to_char(%s AT TIME ZONE '%s', 'YYYY-MM-DD')", dateExpr, timezone)
	default:
		// Fallback: try standard DATE() function
		return fmt.Sprintf("DATE(%s)", dateExpr)
	}
}

// BuildDailyModelThroughputQuery constructs a SQL query for daily model throughput statistics.
//
// Parameters:
//   - dialect: database dialect ("postgres", "mysql", "sqlite3")
//   - timezone: timezone string for date conversion (e.g., "America/New_York")
//   - offsetSeconds: timezone offset in seconds
//   - limit: maximum number of results per day
//   - mode: which SQL pattern to use (ROW_NUMBER or MAX_ID)
//
// Returns: SQL query string with placeholders for the since timestamp
func BuildDailyModelThroughputQuery(dialect string, timezone string, offsetSeconds int, limit int, mode ThroughputQueryMode) string {
	return buildDailyThroughputQuery(dialect, timezone, offsetSeconds, DailyThroughputByModel, limit, mode)
}

// BuildDailyChannelThroughputQuery constructs a SQL query for daily channel throughput statistics.
//
// Parameters:
//   - dialect: database dialect ("postgres", "mysql", "sqlite3")
//   - timezone: database timezone string for date conversion
//   - offsetSeconds: timezone offset in seconds
//   - limit: maximum number of results per day
//   - mode: which SQL pattern to use (ROW_NUMBER or MAX_ID)
//
// Returns: SQL query string with placeholders for the since timestamp
func BuildDailyChannelThroughputQuery(dialect string, timezone string, offsetSeconds int, limit int, mode ThroughputQueryMode) string {
	return buildDailyThroughputQuery(dialect, timezone, offsetSeconds, DailyThroughputByChannel, limit, mode)
}

// buildDailyThroughputQuery constructs the actual SQL query for daily throughput.
func buildDailyThroughputQuery(dialect string, timezone string, offsetSeconds int, queryType DailyThroughputQueryType, limit int, mode ThroughputQueryMode) string {
	// Validate limit
	if limit <= 0 {
		limit = 10 // Default for daily queries
	}

	// Get query config
	config, ok := AllowedDailyQueryConfigs[queryType]
	if !ok {
		config = AllowedDailyQueryConfigs[DailyThroughputByModel]
	}

	// Get date expression based on dialect
	dateExpr := getDateExpression(dialect, "se.created_at", timezone, offsetSeconds)

	if mode == ThroughputModeMaxID {
		return buildDailyMaxIDQuery(dateExpr, config, limit)
	}

	return buildDailyRowNumberQuery(dateExpr, config, limit)
}

// buildDailyRowNumberQuery constructs a daily throughput query using ROW_NUMBER().
func buildDailyRowNumberQuery(dateExpr string, config DailyQueryFragmentConfig, limit int) string {
	return fmt.Sprintf(`
WITH successful_execs AS (
    SELECT
        request_id,
        channel_id,
        metrics_latency_ms,
        metrics_first_token_latency_ms,
        stream,
        created_at,
        ROW_NUMBER() OVER (PARTITION BY request_id ORDER BY created_at DESC) as rn
    FROM request_executions
    WHERE status = 'completed' AND metrics_latency_ms > 0
)
SELECT
    %s as date,
    %s as id,
    %s,
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,
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
%s
WHERE se.rn = 1
GROUP BY %s
ORDER BY date DESC, throughput DESC
LIMIT %d`, dateExpr, config.IDColumn, config.NameColumn, config.JoinClause, config.GroupByFields, limit)
}

// buildDailyMaxIDQuery constructs a daily throughput query using MAX(id) subquery.
func buildDailyMaxIDQuery(dateExpr string, config DailyQueryFragmentConfig, limit int) string {
	return fmt.Sprintf(`
SELECT
    %s as date,
    %s as id,
    %s,
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,
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
%s
WHERE se.status = 'completed'
    AND se.metrics_latency_ms > 0
    AND se.id = (
        SELECT MAX(re2.id)
        FROM request_executions re2
        WHERE re2.request_id = se.request_id
            AND re2.status = 'completed'
            AND re2.metrics_latency_ms > 0
    )
GROUP BY %s
ORDER BY date DESC, throughput DESC
LIMIT %d`, dateExpr, config.IDColumn, config.NameColumn, config.JoinClause, config.GroupByFields, limit)
}
