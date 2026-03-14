package qb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDateExpression(t *testing.T) {
	tests := []struct {
		name          string
		dialect       string
		dateExpr      string
		timezone      string
		offsetSeconds int
		wantContains  []string
	}{
		{
			name:          "sqlite dialect",
			dialect:       "sqlite",
			dateExpr:      "se.created_at",
			timezone:      "UTC",
			offsetSeconds: 0,
			wantContains:  []string{"strftime", "%Y-%m-%d", "se.created_at"},
		},
		{
			name:          "sqlite3 dialect",
			dialect:       "sqlite3",
			dateExpr:      "se.created_at",
			timezone:      "America/New_York",
			offsetSeconds: -14400,
			wantContains:  []string{"strftime", "%Y-%m-%d", "se.created_at", "-14400"},
		},
		{
			name:          "mysql dialect",
			dialect:       "mysql",
			dateExpr:      "se.created_at",
			timezone:      "America/New_York",
			offsetSeconds: -14400,
			wantContains:  []string{"DATE_FORMAT", "CONVERT_TZ", "-04:00", "%Y-%m-%d"},
		},
		{
			name:          "postgres dialect",
			dialect:       "postgres",
			dateExpr:      "se.created_at",
			timezone:      "UTC",
			offsetSeconds: 0,
			wantContains:  []string{"to_char", "AT TIME ZONE", "YYYY-MM-DD"},
		},
		{
			name:          "postgresql dialect",
			dialect:       "postgresql",
			dateExpr:      "se.created_at",
			timezone:      "Europe/London",
			offsetSeconds: 0,
			wantContains:  []string{"to_char", "Europe/London"},
		},
		{
			name:          "unknown dialect falls back to DATE()",
			dialect:       "oracle",
			dateExpr:      "se.created_at",
			timezone:      "UTC",
			offsetSeconds: 0,
			wantContains:  []string{"DATE(se.created_at)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getDateExpression(tt.dialect, tt.dateExpr, tt.timezone, tt.offsetSeconds)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "date expression should contain %q", want)
			}
		})
	}
}

func TestBuildDailyModelThroughputQuery(t *testing.T) {
	tests := []struct {
		name            string
		dialect         string
		timezone        string
		offsetSeconds   int
		limit           int
		mode            ThroughputQueryMode
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:          "model query with ROW_NUMBER mode postgres",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         10,
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"WITH successful_execs AS",
				"ROW_NUMBER()",
				"JOIN requests r ON",
				"JOIN models m ON",
				"r.model_id",
				"model_name",
				"to_char",
				"throughput",
				"request_count",
				"daily_rn <= 10",
				"ORDER BY date DESC, throughput DESC",
			},
			wantNotContains: []string{"MAX(re2.id)"},
		},
		{
			name:          "model query with MAX_ID mode mysql",
			dialect:       "mysql",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         10,
			mode:          ThroughputModeMaxID,
			wantContains: []string{
				"MAX(re2.id)",
				"JOIN requests r ON",
				"JOIN models m ON",
				"DATE_FORMAT",
				"throughput",
				"daily_rn <= 10",
			},
			wantNotContains: []string{"WITH successful_execs"},
		},
		{
			name:          "model query with sqlite",
			dialect:       "sqlite",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         10,
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"strftime",
				"r.model_id",
				"model_name",
			},
		},
		{
			name:          "zero limit defaults to 10",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         0,
			mode:          ThroughputModeRowNumber,
			wantContains:  []string{"daily_rn <= 10"},
		},
		{
			name:          "custom limit is respected",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         25,
			mode:          ThroughputModeRowNumber,
			wantContains:  []string{"daily_rn <= 25"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyModelThroughputQuery(tt.dialect, tt.timezone, tt.offsetSeconds, tt.limit, tt.mode)

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "query should contain %q", want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "query should not contain %q", notWant)
			}
		})
	}
}

func TestBuildDailyChannelThroughputQuery(t *testing.T) {
	tests := []struct {
		name            string
		dialect         string
		timezone        string
		offsetSeconds   int
		limit           int
		mode            ThroughputQueryMode
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:          "channel query with ROW_NUMBER mode postgres",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         10,
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"WITH successful_execs AS",
				"ROW_NUMBER()",
				"JOIN channels c ON",
				"se.channel_id",
				"channel_name",
				"to_char",
				"throughput",
				"request_count",
				"daily_rn <= 10",
			},
			wantNotContains: []string{"MAX(re2.id)", "JOIN requests r ON"},
		},
		{
			name:          "channel query with MAX_ID mode mysql",
			dialect:       "mysql",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         10,
			mode:          ThroughputModeMaxID,
			wantContains: []string{
				"MAX(re2.id)",
				"JOIN channels c ON",
				"DATE_FORMAT",
				"se.channel_id",
				"daily_rn <= 10",
			},
			wantNotContains: []string{"WITH successful_execs"},
		},
		{
			name:          "channel query with sqlite",
			dialect:       "sqlite",
			timezone:      "UTC",
			offsetSeconds: 0,
			limit:         10,
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"strftime",
				"se.channel_id",
				"channel_name",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyChannelThroughputQuery(tt.dialect, tt.timezone, tt.offsetSeconds, tt.limit, tt.mode)

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "query should contain %q", want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "query should not contain %q", notWant)
			}
		})
	}
}

func TestBuildDailyThroughputQuery_SQLStructure(t *testing.T) {
	tests := []struct {
		name            string
		mode            ThroughputQueryMode
		wantCTE         bool
		wantDateGroupBy bool
	}{
		{
			name:            "ROW_NUMBER mode includes CTE and date grouping",
			mode:            ThroughputModeRowNumber,
			wantCTE:         true,
			wantDateGroupBy: true,
		},
		{
			name:            "MAX_ID mode does not include CTE",
			mode:            ThroughputModeMaxID,
			wantCTE:         false,
			wantDateGroupBy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyModelThroughputQuery("postgres", "UTC", 0, 10, tt.mode)

			if tt.wantCTE {
				assert.Contains(t, got, "WITH successful_execs AS", "ROW_NUMBER mode should use CTE")
			} else {
				assert.NotContains(t, got, "WITH successful_execs AS", "MAX_ID mode should not use CTE")
			}

			assert.Contains(t, got, "GROUP BY", "should have GROUP BY clause")
			// For BuildDailyModelThroughputQuery, the date expression is used directly in GROUP BY
			// (e.g., "to_char(...)", "DATE_FORMAT(...)", or "strftime(...)")
			assert.Contains(t, got, "model_id", "should group by model_id")
			assert.Contains(t, got, "throughput", "should calculate throughput")
			assert.Contains(t, got, "request_count", "should count requests")
			assert.Contains(t, got, "ORDER BY date DESC, throughput DESC", "should order by date and throughput")
		})
	}
}

func TestBuildDailyThroughputQuery_TokPerSecondCalculation(t *testing.T) {
	tests := []struct {
		name         string
		mode         ThroughputQueryMode
		wantContains []string
	}{
		{
			name: "ROW_NUMBER mode has correct tok/s calculation",
			mode: ThroughputModeRowNumber,
			wantContains: []string{
				"completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)",
				"* 1000.0",
				"metrics_first_token_latency_ms",
				"metrics_latency_ms - se.metrics_first_token_latency_ms",
			},
		},
		{
			name: "MAX_ID mode has correct tok/s calculation",
			mode: ThroughputModeMaxID,
			wantContains: []string{
				"completion_tokens + COALESCE(ul.completion_reasoning_tokens, 0) + COALESCE(ul.completion_audio_tokens, 0)",
				"* 1000.0",
				"metrics_first_token_latency_ms",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyModelThroughputQuery("postgres", "UTC", 0, 10, tt.mode)

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "query should contain tok/s calculation pattern %q", want)
			}
		})
	}
}

func TestBuildDailyThroughputQuery_DateExpressions(t *testing.T) {
	tests := []struct {
		name          string
		dialect       string
		timezone      string
		offsetSeconds int
		wantExpr      string
	}{
		{
			name:          "postgres uses to_char",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			wantExpr:      "to_char(se.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')",
		},
		{
			name:          "mysql uses DATE_FORMAT",
			dialect:       "mysql",
			timezone:      "UTC",
			offsetSeconds: 0,
			wantExpr:      "DATE_FORMAT(CONVERT_TZ(se.created_at, '+00:00', '+00:00'), '%Y-%m-%d')",
		},
		{
			name:          "sqlite uses strftime",
			dialect:       "sqlite",
			timezone:      "UTC",
			offsetSeconds: 0,
			wantExpr:      "strftime('%Y-%m-%d', datetime(substr(se.created_at, 1, 19), '+0 seconds'))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyModelThroughputQuery(tt.dialect, tt.timezone, tt.offsetSeconds, 10, ThroughputModeRowNumber)

			assert.Contains(t, got, tt.wantExpr, "query should use correct date expression for %s", tt.dialect)
		})
	}
}

func TestAllowedDailyQueryConfigs(t *testing.T) {
	assert.Contains(t, AllowedDailyQueryConfigs, DailyThroughputByChannel, "should have config for ByChannel")
	assert.Contains(t, AllowedDailyQueryConfigs, DailyThroughputByModel, "should have config for ByModel")

	channelConfig := AllowedDailyQueryConfigs[DailyThroughputByChannel]
	assert.Contains(t, channelConfig.IDColumn, "channel_id", "should include channel_id")
	assert.Contains(t, channelConfig.NameColumn, "channel_name", "should include channel_name")
	assert.Contains(t, channelConfig.JoinClause, "channels c ON", "should join channels table")
	assert.Contains(t, channelConfig.GroupByFields, "channel_id", "should group by channel_id")
	// date is added separately via dateExpr in GROUP BY clauses

	modelConfig := AllowedDailyQueryConfigs[DailyThroughputByModel]
	assert.Contains(t, modelConfig.IDColumn, "model_id", "should include model_id")
	assert.Contains(t, modelConfig.NameColumn, "model_name", "should include model_name")
	assert.Contains(t, modelConfig.JoinClause, "requests r ON", "should join requests table")
	assert.Contains(t, modelConfig.JoinClause, "models m ON", "should join models table")
	assert.Contains(t, modelConfig.GroupByFields, "model_id", "should group by model_id")
	// date is added separately via dateExpr in GROUP BY clauses
}

func TestDailyThroughputQueryTypeEnum(t *testing.T) {
	assert.Equal(t, DailyThroughputQueryType(0), DailyThroughputByChannel, "DailyThroughputByChannel should be 0")
	assert.Equal(t, DailyThroughputQueryType(1), DailyThroughputByModel, "DailyThroughputByModel should be 1")
}

func TestBuildDailyThroughputQuery_StreamingLatencyHandling(t *testing.T) {
	got := BuildDailyModelThroughputQuery("postgres", "UTC", 0, 10, ThroughputModeRowNumber)

	assert.Contains(t, got, "se.stream", "should reference stream flag")
	assert.Contains(t, got, "CASE WHEN se.stream AND se.metrics_first_token_latency_ms IS NOT NULL", "should handle streaming latency")

	streamingLogic := strings.Count(got, "metrics_first_token_latency_ms >= se.metrics_latency_ms")
	assert.GreaterOrEqual(t, streamingLogic, 2, "should check for invalid first token latency")
}

func TestBuildDailyPerformanceStatsQuery(t *testing.T) {
	tests := []struct {
		name            string
		dialect         string
		timezone        string
		offsetSeconds   int
		queryType       DailyThroughputQueryType
		placeholder     string
		mode            ThroughputQueryMode
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:          "model query with postgres includes requests join",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			queryType:     DailyThroughputByModel,
			placeholder:   "$1",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"WITH successful_execs AS",
				"JOIN requests r ON se.request_id = r.id",
				"r.model_id",
				"to_char",
				"AT TIME ZONE",
				"NULLIF(",
				", 0)",
				"GROUP BY exec_date",
				"WHERE daily.throughput IS NOT NULL",
				"AND daily.throughput > 0",
				"ORDER BY date DESC, throughput DESC",
			},
			wantNotContains: []string{},
		},
		{
			name:          "channel query with postgres excludes requests join",
			dialect:       "postgres",
			timezone:      "UTC",
			offsetSeconds: 0,
			queryType:     DailyThroughputByChannel,
			placeholder:   "$1",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"WITH successful_execs AS",
				"se.channel_id",
				"to_char",
				"AT TIME ZONE",
				"NULLIF(",
				", 0)",
				"GROUP BY exec_date",
				"WHERE daily.throughput IS NOT NULL",
				"AND daily.throughput > 0",
				"ORDER BY date DESC, throughput DESC",
			},
			wantNotContains: []string{
				"JOIN requests r ON se.request_id = r.id",
				"r.model_id",
			},
		},
		{
			name:          "model query with mysql",
			dialect:       "mysql",
			timezone:      "America/New_York",
			offsetSeconds: -14400,
			queryType:     DailyThroughputByModel,
			placeholder:   "?",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"WITH successful_execs AS",
				"JOIN requests r ON se.request_id = r.id",
				"r.model_id",
				"DATE_FORMAT",
				"CONVERT_TZ",
				"-04:00",
				"NULLIF(",
				", 0)",
				"GROUP BY exec_date",
				"WHERE daily.throughput IS NOT NULL",
				"AND daily.throughput > 0",
			},
		},
		{
			name:          "channel query with mysql",
			dialect:       "mysql",
			timezone:      "UTC",
			offsetSeconds: 0,
			queryType:     DailyThroughputByChannel,
			placeholder:   "?",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"se.channel_id",
				"DATE_FORMAT",
				"CONVERT_TZ",
				"NULLIF(",
				", 0)",
				"GROUP BY exec_date",
				"WHERE daily.throughput IS NOT NULL",
				"AND daily.throughput > 0",
			},
			wantNotContains: []string{
				"JOIN requests r ON se.request_id = r.id",
			},
		},
		{
			name:          "model query with sqlite",
			dialect:       "sqlite",
			timezone:      "UTC",
			offsetSeconds: 0,
			queryType:     DailyThroughputByModel,
			placeholder:   "?",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"WITH successful_execs AS",
				"JOIN requests r ON se.request_id = r.id",
				"r.model_id",
				"strftime",
				"NULLIF(",
				", 0)",
				"GROUP BY exec_date",
				"WHERE daily.throughput IS NOT NULL",
				"AND daily.throughput > 0",
			},
		},
		{
			name:          "channel query with sqlite3",
			dialect:       "sqlite3",
			timezone:      "UTC",
			offsetSeconds: 0,
			queryType:     DailyThroughputByChannel,
			placeholder:   "?",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"se.channel_id",
				"strftime",
				"NULLIF(",
				", 0)",
				"GROUP BY exec_date",
				"WHERE daily.throughput IS NOT NULL",
				"AND daily.throughput > 0",
			},
			wantNotContains: []string{
				"JOIN requests r ON se.request_id = r.id",
			},
		},
		{
			name:          "model query with sqlite3 and MaxID mode",
			dialect:       "sqlite3",
			timezone:      "UTC",
			offsetSeconds: 0,
			queryType:     DailyThroughputByModel,
			placeholder:   "?",
			mode:          ThroughputModeMaxID,
			wantContains: []string{
				"MAX(se2.id)",
				"latest_execs",
				"JOIN requests r ON se.request_id = r.id",
				"r.model_id",
				"strftime",
			},
			wantNotContains: []string{
				"successful_execs",
				"ROW_NUMBER() OVER (PARTITION BY request_id",
			},
		},
		{
			name:          "model query with postgresql dialect alias",
			dialect:       "postgresql",
			timezone:      "Europe/London",
			offsetSeconds: 0,
			queryType:     DailyThroughputByModel,
			placeholder:   "$1",
			mode:          ThroughputModeRowNumber,
			wantContains: []string{
				"AT TIME ZONE 'Europe/London'",
				"JOIN requests r ON se.request_id = r.id",
				"r.model_id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyPerformanceStatsQuery(tt.dialect, tt.timezone, tt.offsetSeconds, tt.queryType, tt.placeholder, tt.mode)

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "query should contain %q", want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "query should not contain %q", notWant)
			}
		})
	}
}

func TestBuildDailyPerformanceStatsQuery_TTFTCalculation(t *testing.T) {
	got := BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeRowNumber)

	assert.Contains(t, got, "NULLIF(", "should use NULLIF for TTFT denominator")
	assert.Contains(t, got, ", 0)", "should have NULLIF(..., 0) pattern")
	assert.Contains(t, got, "metrics_first_token_latency_ms", "should reference first token latency")
	assert.Contains(t, got, "SUM(CASE", "should use CASE for TTFT calculation")
}

func TestBuildDailyPerformanceStatsQuery_GroupByBehavior(t *testing.T) {
	tests := []struct {
		name      string
		queryType DailyThroughputQueryType
		wantIDCol string
	}{
		{
			name:      "model query groups by model_id",
			queryType: DailyThroughputByModel,
			wantIDCol: "model_id",
		},
		{
			name:      "channel query groups by channel_id",
			queryType: DailyThroughputByChannel,
			wantIDCol: "channel_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, tt.queryType, "$1", ThroughputModeRowNumber)

			assert.Contains(t, got, "GROUP BY exec_date", "should group by CTE column exec_date")

			assert.Contains(t, got, tt.wantIDCol, "should group by "+tt.wantIDCol)
		})
	}
}

func TestBuildDailyPerformanceStatsQuery_PostFilter(t *testing.T) {
	got := BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeRowNumber)

	assert.Contains(t, got, "WHERE daily.throughput IS NOT NULL", "should filter null throughput")
	assert.Contains(t, got, "AND daily.throughput > 0", "should filter zero/negative throughput")
}

func TestBuildDailyPerformanceStatsQuery_ConditionalJoinRequests(t *testing.T) {
	modelQuery := BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByModel, "$1", ThroughputModeRowNumber)
	assert.Contains(t, modelQuery, "JOIN requests r ON se.request_id = r.id", "model query should join requests")
	assert.Contains(t, modelQuery, "r.model_id", "model query should reference r.model_id")

	channelQuery := BuildDailyPerformanceStatsQuery("postgres", "UTC", 0, DailyThroughputByChannel, "$1", ThroughputModeRowNumber)
	assert.NotContains(t, channelQuery, "JOIN requests r ON se.request_id = r.id", "channel query should not join requests")
	assert.NotContains(t, channelQuery, "r.model_id", "channel query should not reference r.model_id")
}
