package gql

import (
	"fmt"
)

// ChannelStats represents aggregated stats for a channel.
type ChannelStats struct {
	ChannelID    int
	ChannelName  string
	ChannelType  string
	TokensCount  int64
	LatencyMs    int64
	RequestCount int64
	Throughput   float64
}

// ModelStats represents aggregated stats for a model.
type ModelStats struct {
	ModelID      string
	ModelName    string
	TokensCount  int64
	LatencyMs    int64
	RequestCount int64
	Throughput   float64
}

func buildThroughputQuery(useDollarPlaceholders bool, selectColumns, joinClause, groupBy string, limit int) string {
	placeholder := "$1"
	if !useDollarPlaceholders {
		placeholder = "?"
	}

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
    ` + selectColumns + `
    SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) as tokens_count,
    SUM(se.metrics_latency_ms) as latency_ms,
    COUNT(DISTINCT se.request_id) as request_count,
    CASE
        WHEN SUM(CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                 THEN se.metrics_latency_ms - se.metrics_first_token_latency_ms
                 ELSE se.metrics_latency_ms END) > 0
        THEN SUM(ul.completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)) * 1000.0
             / SUM(CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL
                   THEN se.metrics_latency_ms - se.metrics_first_token_latency_ms
                   ELSE se.metrics_latency_ms END)
        ELSE 0
    END as throughput
FROM successful_execs se
JOIN usage_logs ul ON se.request_id = ul.request_id
` + joinClause + `
WHERE se.rn = 1
GROUP BY ` + groupBy + `
ORDER BY throughput DESC
LIMIT ` + fmt.Sprintf("%d", limit)
}

func calculateMinRequests(totalItems int, avgRequests float64) int {
	if totalItems <= 5 {
		return 1 // Show everything for small datasets
	}
	if totalItems <= 10 {
		return 5 // Minimum 5 requests
	}
	minReq := int(avgRequests * 0.10) // 10% of average
	if minReq < 10 {
		return 10
	}
	if minReq > 100 {
		return 100
	}
	return minReq
}

func calculateConfidenceLevel(requestCount int, median float64) string {
	// When median is 0, we cannot calculate a meaningful ratio (requestCount/median),
	// so we default to low confidence since we lack sufficient data for reliable inference.
	if median == 0 {
		return "low"
	}
	ratio := float64(requestCount) / median
	if ratio >= 2.0 {
		return "high"
	}
	if ratio >= 0.5 {
		return "medium"
	}
	return "low"
}

// buildChannelJoinClause builds the channel join clause with soft-delete filter.
func buildChannelJoinClause() string {
	return "JOIN channels c ON se.channel_id = c.id AND c.deleted_at = 0"
}

// buildModelJoinClause builds the model join clause with soft-delete filter.
func buildModelJoinClause() string {
	return "JOIN requests r ON se.request_id = r.id\nJOIN models m ON r.model_id = m.model_id AND m.deleted_at = 0"
}

// buildSelectColumnsForChannels returns the SELECT columns for channels query.
func buildSelectColumnsForChannels() string {
	return "se.channel_id,\n    c.name as channel_name,\n    c.type as channel_type,"
}

// buildSelectColumnsForModels returns the SELECT columns for models query.
func buildSelectColumnsForModels() string {
	return "r.model_id,\n    m.name as model_name,"
}

// buildGroupByForChannels returns the GROUP BY clause for channels query.
func buildGroupByForChannels() string {
	return "se.channel_id, c.name, c.type"
}

// buildGroupByForModels returns the GROUP BY clause for models query.
func buildGroupByForModels() string {
	return "r.model_id, m.name"
}
