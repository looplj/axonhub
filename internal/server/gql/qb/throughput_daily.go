package qb

import (
	"fmt"
	"strconv"

	"github.com/looplj/axonhub/internal/pkg/xtime"
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
	NameAlias     string
	JoinClause    string
	GroupByFields string
}

// AllowedDailyQueryConfigs maps each DailyThroughputQueryType to its SQL fragments.
var AllowedDailyQueryConfigs = map[DailyThroughputQueryType]DailyQueryFragmentConfig{
	DailyThroughputByChannel: {
		IDColumn:      "se.channel_id",
		NameColumn:    "c.name as channel_name",
		NameAlias:     "channel_name",
		JoinClause:    "JOIN channels c ON se.channel_id = c.id",
		GroupByFields: "se.channel_id, c.name",
	},
	DailyThroughputByModel: {
		IDColumn:      "r.model_id",
		NameColumn:    "m.name as model_name",
		NameAlias:     "model_name",
		JoinClause:    "JOIN requests r ON se.request_id = r.id\nJOIN models m ON r.model_id = m.model_id",
		GroupByFields: "r.model_id, m.name",
	},
}

// getDateExpression returns the dialect-specific date expression for grouping by day.
// The dateExpr should include the column reference (e.g., "se.created_at").
//
// SECURITY NOTE: The timezone parameter is interpolated directly into SQL queries for
// MySQL (CONVERT_TZ) and Postgres (AT TIME ZONE). This parameter must be a trusted,
// sanitized value (e.g., from system loc.String() which returns a valid IANA timezone name
// like "America/New_York"). NEVER pass untrusted user input directly to this function.
// If you need timezone support from user input, validate against a whitelist of known
// timezones first, or use offsetSeconds as an alternative (though offsetSeconds doesn't
// handle DST transitions correctly).
func getDateExpression(dialect string, dateExpr string, timezone string, offsetSeconds int) string {
	switch dialect {
	case "sqlite3", "sqlite":
		// SQLite: strftime('%Y-%m-%d', datetime(substr(created_at, 1, 19), 'offset seconds'))
		return fmt.Sprintf("strftime('%%Y-%%m-%%d', datetime(substr(%s, 1, 19), '%+d seconds'))", dateExpr, offsetSeconds)
	case "mysql":
		// MySQL: DATE_FORMAT(CONVERT_TZ(created_at, '+00:00', timezone), '%Y-%m-%d')
		offsetStr := xtime.FormatUTCOffset(offsetSeconds)
		return fmt.Sprintf("DATE_FORMAT(CONVERT_TZ(%s, '+00:00', '%s'), '%%Y-%%m-%%d')", dateExpr, offsetStr)
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

	// Determine parameter placeholder based on dialect
	paramPlaceholder := "?"
	if dialect == "postgres" || dialect == "postgresql" {
		paramPlaceholder = "$1"
	}

	if mode == ThroughputModeMaxID {
		return buildDailyMaxIDQuery(dateExpr, config, limit, paramPlaceholder)
	}

	return buildDailyRowNumberQuery(dateExpr, config, limit, paramPlaceholder)
}

// throughputCoreSQL returns the shared CASE expression body for throughput calculations.
// This extracts the common latency calculation logic used by throughputCalculationSQL.
func throughputCoreSQL(seTable string) string {
	return fmt.Sprintf(`CASE WHEN %s.stream AND %s.metrics_first_token_latency_ms IS NOT NULL
                 THEN CASE WHEN %s.metrics_first_token_latency_ms >= %s.metrics_latency_ms
                      THEN 0
                      ELSE %s.metrics_latency_ms - %s.metrics_first_token_latency_ms END
                 ELSE %s.metrics_latency_ms END`, seTable, seTable, seTable, seTable, seTable, seTable, seTable)
}

// throughputCalculationSQL returns the SQL for calculating throughput from tokens and latency.
// This is shared across multiple query builders to ensure consistent throughput calculation.
func throughputCalculationSQL(seTable string) string {
	core := throughputCoreSQL(seTable)
	return fmt.Sprintf(`CASE
        WHEN SUM(%s) > 0
        THEN SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) * 1000.0
             / SUM(%s)
        ELSE 0
    END`, core, core)
}

// buildDailyRowNumberQuery constructs a daily throughput query using ROW_NUMBER().
// Applies date filtering and per-day limit using ROW_NUMBER() window partitioned by date.
func buildDailyRowNumberQuery(dateExpr string, config DailyQueryFragmentConfig, limit int, startDatePlaceholder string) string {
	throughputSQL := throughputCalculationSQL("se")
	return "WITH successful_execs AS (\n" +
		"    SELECT\n" +
		"        request_id,\n" +
		"        channel_id,\n" +
		"        metrics_latency_ms,\n" +
		"        metrics_first_token_latency_ms,\n" +
		"        stream,\n" +
		"        created_at,\n" +
		"        ROW_NUMBER() OVER (PARTITION BY request_id ORDER BY created_at DESC) as rn\n" +
		"    FROM request_executions\n" +
		"    WHERE status = 'completed' AND metrics_latency_ms > 0 AND created_at >= " + startDatePlaceholder + "\n" +
		"),\n" +
		"daily_stats AS (\n" +
		"    SELECT\n" +
		"        " + dateExpr + " as date,\n" +
		"        " + config.IDColumn + " as id,\n" +
		"        " + config.NameColumn + ",\n" +
		"        SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,\n" +
		"        COUNT(DISTINCT se.request_id) as request_count,\n" +
		"        " + throughputSQL + " as throughput,\n" +
		"        ROW_NUMBER() OVER (PARTITION BY " + dateExpr + " ORDER BY " + throughputSQL + " DESC) as daily_rn\n" +
		"    FROM successful_execs se\n" +
		"    JOIN usage_logs ul ON se.request_id = ul.request_id\n" +
		"    " + config.JoinClause + "\n" +
		"    WHERE se.rn = 1\n" +
		"    GROUP BY " + dateExpr + ", " + config.GroupByFields + "\n" +
		")\n" +
		"SELECT date, id, " + config.NameAlias + ", tokens_count, request_count, throughput\n" +
		"FROM daily_stats\n" +
		"WHERE daily_rn <= " + strconv.Itoa(limit) + "\n" +
		"ORDER BY date DESC, throughput DESC"
}

// buildDailyMaxIDQuery constructs a daily throughput query using MAX(id) subquery.
// Applies date filtering and per-day limit using ROW_NUMBER() window partitioned by date.
func buildDailyMaxIDQuery(dateExpr string, config DailyQueryFragmentConfig, limit int, startDatePlaceholder string) string {
	throughputSQL := throughputCalculationSQL("se")
	return "WITH ranked_execs AS (\n" +
		"    SELECT\n" +
		"        " + dateExpr + " as date,\n" +
		"        " + config.IDColumn + " as id,\n" +
		"        " + config.NameColumn + ",\n" +
		"        SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,\n" +
		"        COUNT(DISTINCT se.request_id) as request_count,\n" +
		"        " + throughputSQL + " as throughput,\n" +
		"        ROW_NUMBER() OVER (PARTITION BY " + dateExpr + " ORDER BY " + throughputSQL + " DESC) as daily_rn\n" +
		"    FROM request_executions se\n" +
		"    JOIN usage_logs ul ON se.request_id = ul.request_id\n" +
		"    " + config.JoinClause + "\n" +
		"    WHERE se.status = 'completed'\n" +
		"        AND se.metrics_latency_ms > 0\n" +
		"        AND se.created_at >= " + startDatePlaceholder + "\n" +
		"        AND se.id = (\n" +
		"            SELECT MAX(re2.id)\n" +
		"            FROM request_executions re2\n" +
		"            WHERE re2.request_id = se.request_id\n" +
		"                AND re2.status = 'completed'\n" +
		"                AND re2.metrics_latency_ms > 0\n" +
		"        )\n" +
		"    GROUP BY " + dateExpr + ", " + config.GroupByFields + "\n" +
		")\n" +
		"SELECT date, id, " + config.NameAlias + ", tokens_count, request_count, throughput\n" +
		"FROM ranked_execs\n" +
		"WHERE daily_rn <= " + strconv.Itoa(limit) + "\n" +
		"ORDER BY date DESC, throughput DESC"
}

// BuildDailyPerformanceStatsQuery constructs a SQL query for daily performance statistics
// including throughput (tokens/sec) and TTFT (time to first token in ms).
// This is used by ModelPerformanceStats to get detailed performance metrics.
//
// Parameters:
//   - dialect: database dialect ("postgres", "mysql", "sqlite3")
//   - timezone: timezone string for date conversion
//   - offsetSeconds: timezone offset in seconds
//   - queryType: DailyThroughputByModel or DailyThroughputByChannel
//   - placeholder: parameter placeholder ("?" or "$1")
//   - mode: which SQL pattern to use (ROW_NUMBER or MAX_ID)
//
// Returns: SQL query string ready for execution
func BuildDailyPerformanceStatsQuery(dialect string, timezone string, offsetSeconds int, queryType DailyThroughputQueryType, placeholder string, mode ThroughputQueryMode) string {
	config, ok := AllowedDailyQueryConfigs[queryType]
	if !ok {
		config = AllowedDailyQueryConfigs[DailyThroughputByModel]
	}

	dateExpr := getDateExpression(dialect, "se.created_at", timezone, offsetSeconds)
	throughputSQL := throughputCalculationSQL("se")

	// Only add the requests join for model queries (channel_id is already in request_executions)
	var joinRequests string
	if queryType == DailyThroughputByModel {
		joinRequests = "    JOIN requests r ON se.request_id = r.id\n"
	}

	if mode == ThroughputModeMaxID {
		return buildDailyPerformanceStatsMaxIDQuery(dateExpr, config, queryType, placeholder, joinRequests, throughputSQL)
	}

	return buildDailyPerformanceStatsRowNumberQuery(dateExpr, config, queryType, placeholder, joinRequests, throughputSQL)
}

// buildDailyPerformanceStatsRowNumberQuery constructs the ROW_NUMBER() version of the daily performance stats query.
func buildDailyPerformanceStatsRowNumberQuery(dateExpr string, config DailyQueryFragmentConfig, queryType DailyThroughputQueryType, placeholder string, joinRequests string, throughputSQL string) string {
	return "WITH successful_execs AS (\n" +
		"    SELECT\n" +
		"        se.request_id,\n" +
		"        " + config.IDColumn + ",\n" +
		"        se.metrics_latency_ms,\n" +
		"        se.metrics_first_token_latency_ms,\n" +
		"        se.stream,\n" +
		"        " + dateExpr + " as exec_date,\n" +
		"        ROW_NUMBER() OVER (PARTITION BY se.request_id ORDER BY se.created_at DESC) as rn\n" +
		"    FROM request_executions se\n" +
		joinRequests +
		"    WHERE se.status = 'completed'\n" +
		"        AND se.metrics_latency_ms > 0\n" +
		"        AND se.created_at >= " + placeholder + "\n" +
		"),\n" +
		"daily AS (\n" +
		"    SELECT\n" +
		"        exec_date as date,\n" +
		"        se." + getIDColumnName(queryType) + " as id,\n" +
		"        SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,\n" +
		"        SUM(se.metrics_latency_ms) as latency_ms,\n" +
		"        SUM(CASE\n" +
		"            WHEN se.metrics_first_token_latency_ms IS NOT NULL AND se.metrics_first_token_latency_ms > 0\n" +
		"            THEN se.metrics_first_token_latency_ms\n" +
		"            ELSE 0\n" +
		"        END) / NULLIF(SUM(CASE\n" +
		"            WHEN se.metrics_first_token_latency_ms IS NOT NULL AND se.metrics_first_token_latency_ms > 0\n" +
		"            THEN 1\n" +
		"            ELSE 0\n" +
		"        END), 0) as ttft_ms,\n" +
		"        COUNT(DISTINCT se.request_id) as request_count,\n" +
		"        " + throughputSQL + " as throughput\n" +
		"    FROM successful_execs se\n" +
		"    JOIN usage_logs ul ON se.request_id = ul.request_id\n" +
		"    WHERE se.rn = 1\n" +
		"    GROUP BY exec_date, se." + getIDColumnName(queryType) + "\n" +
		")\n" +
		"SELECT date, id, tokens_count, latency_ms, ttft_ms, request_count, throughput\n" +
		"FROM daily\n" +
		"WHERE daily.throughput IS NOT NULL\n" +
		"    AND daily.throughput > 0\n" +
		"ORDER BY date DESC, throughput DESC"
}

// buildDailyPerformanceStatsMaxIDQuery constructs the MAX(id) fallback version for older databases.
func buildDailyPerformanceStatsMaxIDQuery(dateExpr string, config DailyQueryFragmentConfig, queryType DailyThroughputQueryType, placeholder string, joinRequests string, throughputSQL string) string {
	return "WITH latest_execs AS (\n" +
		"    SELECT\n" +
		"        se.request_id,\n" +
		"        " + config.IDColumn + ",\n" +
		"        se.metrics_latency_ms,\n" +
		"        se.metrics_first_token_latency_ms,\n" +
		"        se.stream,\n" +
		"        " + dateExpr + " as exec_date\n" +
		"    FROM request_executions se\n" +
		joinRequests +
		"    WHERE se.status = 'completed'\n" +
		"        AND se.metrics_latency_ms > 0\n" +
		"        AND se.created_at >= " + placeholder + "\n" +
		"        AND se.id = (\n" +
		"            SELECT MAX(se2.id)\n" +
		"            FROM request_executions se2\n" +
		"            WHERE se2.request_id = se.request_id\n" +
		"                AND se2.status = 'completed'\n" +
		"                AND se2.metrics_latency_ms > 0\n" +
		"        )\n" +
		"),\n" +
		"daily AS (\n" +
		"    SELECT\n" +
		"        exec_date as date,\n" +
		"        se." + getIDColumnName(queryType) + " as id,\n" +
		"        SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,\n" +
		"        SUM(se.metrics_latency_ms) as latency_ms,\n" +
		"        SUM(CASE\n" +
		"            WHEN se.metrics_first_token_latency_ms IS NOT NULL AND se.metrics_first_token_latency_ms > 0\n" +
		"            THEN se.metrics_first_token_latency_ms\n" +
		"            ELSE 0\n" +
		"        END) / NULLIF(SUM(CASE\n" +
		"            WHEN se.metrics_first_token_latency_ms IS NOT NULL AND se.metrics_first_token_latency_ms > 0\n" +
		"            THEN 1\n" +
		"            ELSE 0\n" +
		"        END), 0) as ttft_ms,\n" +
		"        COUNT(DISTINCT se.request_id) as request_count,\n" +
		"        " + throughputSQL + " as throughput\n" +
		"    FROM latest_execs se\n" +
		"    JOIN usage_logs ul ON se.request_id = ul.request_id\n" +
		"    GROUP BY exec_date, se." + getIDColumnName(queryType) + "\n" +
		")\n" +
		"SELECT date, id, tokens_count, latency_ms, ttft_ms, request_count, throughput\n" +
		"FROM daily\n" +
		"WHERE daily.throughput IS NOT NULL\n" +
		"    AND daily.throughput > 0\n" +
		"ORDER BY date DESC, throughput DESC"
}

// getIDColumnName returns the appropriate ID column name for the query type.
func getIDColumnName(queryType DailyThroughputQueryType) string {
	switch queryType {
	case DailyThroughputByChannel:
		return "channel_id"
	case DailyThroughputByModel:
		return "model_id"
	default:
		return "model_id"
	}
}
