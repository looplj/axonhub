package db

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCalculateConfidenceLevel tests the CalculateConfidenceLevel function.
func TestCalculateConfidenceLevel(t *testing.T) {
	tests := []struct {
		name         string
		requestCount int
		median       float64
		want         string
	}{
		// Zero median cases
		{
			name:         "zero median with zero requests",
			requestCount: 0,
			median:       0,
			want:         "low",
		},
		{
			name:         "zero median with many requests",
			requestCount: 1000,
			median:       0,
			want:         "low",
		},
		{
			name:         "zero median with high ratio",
			requestCount: 1000,
			median:       0,
			want:         "low",
		},

		// Below minimum threshold for medium
		{
			name:         "below medium threshold with zero requests",
			requestCount: 0,
			median:       100,
			want:         "low",
		},
		{
			name:         "below medium threshold with 50 requests",
			requestCount: 50,
			median:       100,
			want:         "low",
		},
		{
			name:         "below medium threshold with 99 requests",
			requestCount: 99,
			median:       100,
			want:         "low",
		},

		// Exactly at medium threshold
		{
			name:         "exactly medium threshold",
			requestCount: 100,
			median:       100,
			want:         "medium",
		},
		{
			name:         "medium threshold with ratio 0.5",
			requestCount: 100,
			median:       200,
			want:         "medium",
		},

		// Medium confidence cases
		{
			name:         "medium confidence with ratio 0.5",
			requestCount: 100,
			median:       200,
			want:         "medium",
		},
		{
			name:         "medium confidence with ratio 0.6",
			requestCount: 120,
			median:       200,
			want:         "medium",
		},
		{
			name:         "medium confidence with ratio 0.9",
			requestCount: 180,
			median:       200,
			want:         "medium",
		},
		{
			name:         "medium confidence with ratio 1.0",
			requestCount: 200,
			median:       200,
			want:         "medium",
		},
		{
			name:         "medium confidence with ratio 1.4",
			requestCount: 280,
			median:       200,
			want:         "medium",
		},

		// Below high threshold but with high ratio
		{
			name:         "not enough requests for high despite ratio 2.0",
			requestCount: 400,
			median:       200,
			want:         "medium",
		},
		{
			name:         "not enough requests for high despite ratio 2.25",
			requestCount: 450,
			median:       200,
			want:         "medium",
		},

		// High confidence cases
		{
			name:         "high confidence with ratio 1.5 and 500 requests",
			requestCount: 500,
			median:       333.33,
			want:         "high",
		},
		{
			name:         "high confidence with ratio 2.0",
			requestCount: 600,
			median:       300,
			want:         "high",
		},
		{
			name:         "high confidence with ratio 3.0",
			requestCount: 1000,
			median:       333.33,
			want:         "high",
		},
		{
			name:         "high confidence with large request count",
			requestCount: 10000,
			median:       100,
			want:         "high",
		},

		// Edge cases
		{
			name:         "exactly at high threshold boundary",
			requestCount: 500,
			median:       333.33,
			want:         "high",
		},
		{
			name:         "just below high threshold boundary",
			requestCount: 499,
			median:       333.33,
			want:         "medium",
		},
		{
			name:         "very small median with large request count",
			requestCount: 1000,
			median:       0.001,
			want:         "high",
		},
		{
			name:         "very large median with small request count",
			requestCount: 100,
			median:       1000000,
			want:         "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateConfidenceLevel(tt.requestCount, tt.median)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBuildThroughputQuery tests the BuildThroughputQuery function.
func TestBuildThroughputQuery(t *testing.T) {
	tests := []struct {
		name                  string
		useDollarPlaceholders bool
		queryType             ThroughputQueryType
		limit                 int
		mode                  ThroughputQueryMode
		wantContains          []string
		wantNotContains       []string
	}{
		// ThroughputQueryByChannel tests
		{
			name:                  "by channel with dollar placeholders and ROW_NUMBER",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByChannel,
			limit:                 10,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"$1", "JOIN channels c ON", "se.channel_id", "channel_name", "channel_type", "ROW_NUMBER()", "LIMIT 10"},
			wantNotContains:       []string{},
		},
		{
			name:                  "by channel with question mark placeholders and ROW_NUMBER",
			useDollarPlaceholders: false,
			queryType:             ThroughputQueryByChannel,
			limit:                 10,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"?", "JOIN channels c ON", "se.channel_id", "channel_name", "channel_type", "ROW_NUMBER()", "LIMIT 10"},
			wantNotContains:       []string{"$1"},
		},
		{
			name:                  "by channel with dollar placeholders and MAX_ID",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByChannel,
			limit:                 10,
			mode:                  ThroughputModeMaxID,
			wantContains:          []string{"$1", "JOIN channels c ON", "se.channel_id", "channel_name", "channel_type", "MAX(re2.id)", "LIMIT 10"},
			wantNotContains:       []string{"ROW_NUMBER()"},
		},
		{
			name:                  "by channel with question mark placeholders and MAX_ID",
			useDollarPlaceholders: false,
			queryType:             ThroughputQueryByChannel,
			limit:                 10,
			mode:                  ThroughputModeMaxID,
			wantContains:          []string{"?", "JOIN channels c ON", "se.channel_id", "channel_name", "channel_type", "MAX(re2.id)", "LIMIT 10"},
			wantNotContains:       []string{"$1", "ROW_NUMBER()"},
		},

		// ThroughputQueryByModel tests
		{
			name:                  "by model with dollar placeholders and ROW_NUMBER",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByModel,
			limit:                 10,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"$1", "JOIN requests r ON", "JOIN models m ON", "r.model_id", "model_name", "ROW_NUMBER()", "LIMIT 10"},
			wantNotContains:       []string{},
		},
		{
			name:                  "by model with question mark placeholders and ROW_NUMBER",
			useDollarPlaceholders: false,
			queryType:             ThroughputQueryByModel,
			limit:                 10,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"?", "JOIN requests r ON", "JOIN models m ON", "r.model_id", "model_name", "ROW_NUMBER()", "LIMIT 10"},
			wantNotContains:       []string{"$1"},
		},
		{
			name:                  "by model with dollar placeholders and MAX_ID",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByModel,
			limit:                 10,
			mode:                  ThroughputModeMaxID,
			wantContains:          []string{"$1", "JOIN requests r ON", "JOIN models m ON", "r.model_id", "model_name", "MAX(re2.id)", "LIMIT 10"},
			wantNotContains:       []string{"ROW_NUMBER()"},
		},
		{
			name:                  "by model with question mark placeholders and MAX_ID",
			useDollarPlaceholders: false,
			queryType:             ThroughputQueryByModel,
			limit:                 10,
			mode:                  ThroughputModeMaxID,
			wantContains:          []string{"?", "JOIN requests r ON", "JOIN models m ON", "r.model_id", "model_name", "MAX(re2.id)", "LIMIT 10"},
			wantNotContains:       []string{"$1", "ROW_NUMBER()"},
		},

		// Limit edge cases
		{
			name:                  "zero limit defaults to 20",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByChannel,
			limit:                 0,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"LIMIT 20"},
			wantNotContains:       []string{},
		},
		{
			name:                  "negative limit defaults to 20",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByChannel,
			limit:                 -5,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"LIMIT 20"},
			wantNotContains:       []string{},
		},
		{
			name:                  "custom limit is respected",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryByChannel,
			limit:                 50,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"LIMIT 50"},
			wantNotContains:       []string{"LIMIT 20"},
		},

		// Invalid query type (should default to ByChannel)
		{
			name:                  "invalid query type defaults to ByChannel",
			useDollarPlaceholders: true,
			queryType:             ThroughputQueryType(999),
			limit:                 10,
			mode:                  ThroughputModeROW_NUMBER,
			wantContains:          []string{"JOIN channels c ON", "se.channel_id", "channel_name"},
			wantNotContains:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildThroughputQuery(tt.useDollarPlaceholders, tt.queryType, tt.limit, tt.mode)

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "query should contain %q", want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "query should not contain %q", notWant)
			}
		})
	}
}

// TestBuildThroughputQuery_SQLStructure tests that the generated SQL has proper structure.
func TestBuildThroughputQuery_SQLStructure(t *testing.T) {
	tests := []struct {
		name    string
		mode    ThroughputQueryMode
		wantCTE bool
	}{
		{
			name:    "ROW_NUMBER mode includes CTE",
			mode:    ThroughputModeROW_NUMBER,
			wantCTE: true,
		},
		{
			name:    "MAX_ID mode does not include CTE",
			mode:    ThroughputModeMaxID,
			wantCTE: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildThroughputQuery(true, ThroughputQueryByChannel, 10, tt.mode)

			if tt.wantCTE {
				assert.Contains(t, got, "WITH successful_execs AS", "ROW_NUMBER mode should use CTE")
			} else {
				assert.NotContains(t, got, "WITH successful_execs AS", "MAX_ID mode should not use CTE")
			}

			// Both modes should have these common elements
			assert.Contains(t, got, "request_executions", "should reference request_executions table")
			assert.Contains(t, got, "usage_logs", "should reference usage_logs table")
			assert.Contains(t, got, "throughput", "should calculate throughput")
			assert.Contains(t, got, "ORDER BY throughput DESC", "should order by throughput")
		})
	}
}

// TestBuildProbeStatsQuery tests the BuildProbeStatsQuery function.
func TestBuildProbeStatsQuery(t *testing.T) {
	tests := []struct {
		name                  string
		useDollarPlaceholders bool
		channelIDFilter       string
		mode                  ThroughputQueryMode
		wantContains          []string
		wantNotContains       []string
	}{
		// ROW_NUMBER mode tests
		{
			name:                  "ROW_NUMBER with dollar placeholders",
			useDollarPlaceholders: true,
			channelIDFilter:       "AND se.channel_id IN ($3, $4)",
			mode:                  ThroughputModeROW_NUMBER,
			wantContains: []string{
				"$1", "$2",
				"AND se.channel_id IN ($3, $4)",
				"ROW_NUMBER()",
				"WITH latest_execs AS",
				"se.channel_id",
				"total_count",
				"effective_latency_ms",
				"total_first_token_latency",
				"request_count",
			},
			wantNotContains: []string{"MAX(re2.id)"},
		},
		{
			name:                  "ROW_NUMBER with question mark placeholders",
			useDollarPlaceholders: false,
			channelIDFilter:       "AND se.channel_id IN (?, ?)",
			mode:                  ThroughputModeROW_NUMBER,
			wantContains: []string{
				"?", "?",
				"AND se.channel_id IN (?, ?)",
				"ROW_NUMBER()",
				"WITH latest_execs AS",
			},
			wantNotContains: []string{"$1", "$2", "MAX(re2.id)"},
		},

		// MAX_ID mode tests
		{
			name:                  "MAX_ID with dollar placeholders",
			useDollarPlaceholders: true,
			channelIDFilter:       "AND se.channel_id IN ($3, $4)",
			mode:                  ThroughputModeMaxID,
			wantContains: []string{
				"$1", "$2",
				"AND se.channel_id IN ($3, $4)",
				"MAX(re2.id)",
				"se.channel_id",
				"total_count",
				"effective_latency_ms",
				"total_first_token_latency",
				"request_count",
			},
			wantNotContains: []string{"ROW_NUMBER()", "WITH latest_execs AS"},
		},
		{
			name:                  "MAX_ID with question mark placeholders",
			useDollarPlaceholders: false,
			channelIDFilter:       "AND se.channel_id IN (?, ?)",
			mode:                  ThroughputModeMaxID,
			wantContains: []string{
				"?", "?",
				"AND se.channel_id IN (?, ?)",
				"MAX(re2.id)",
			},
			wantNotContains: []string{"$1", "$2", "ROW_NUMBER()", "WITH latest_execs AS"},
		},

		// Empty filter test
		{
			name:                  "empty channel filter",
			useDollarPlaceholders: true,
			channelIDFilter:       "",
			mode:                  ThroughputModeROW_NUMBER,
			wantContains: []string{
				"$1", "$2",
				"se.channel_id",
				"total_count",
			},
			wantNotContains: []string{},
		},

		// Complex filter test
		{
			name:                  "complex channel filter with multiple conditions",
			useDollarPlaceholders: true,
			channelIDFilter:       "AND se.channel_id = $3 AND se.created_at > $4",
			mode:                  ThroughputModeROW_NUMBER,
			wantContains: []string{
				"AND se.channel_id = $3 AND se.created_at > $4",
			},
			wantNotContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildProbeStatsQuery(tt.useDollarPlaceholders, tt.channelIDFilter, tt.mode)

			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "query should contain %q", want)
			}

			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "query should not contain %q", notWant)
			}
		})
	}
}

// TestBuildProbeStatsQuery_SQLStructure tests that the generated SQL has proper structure.
func TestBuildProbeStatsQuery_SQLStructure(t *testing.T) {
	tests := []struct {
		name    string
		mode    ThroughputQueryMode
		wantCTE bool
	}{
		{
			name:    "ROW_NUMBER mode includes CTE",
			mode:    ThroughputModeROW_NUMBER,
			wantCTE: true,
		},
		{
			name:    "MAX_ID mode does not include CTE",
			mode:    ThroughputModeMaxID,
			wantCTE: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildProbeStatsQuery(true, "AND se.channel_id = $1", tt.mode)

			if tt.wantCTE {
				assert.Contains(t, got, "WITH latest_execs AS", "ROW_NUMBER mode should use CTE")
			} else {
				assert.NotContains(t, got, "WITH latest_execs AS", "MAX_ID mode should not use CTE")
			}

			// Both modes should have these common elements
			assert.Contains(t, got, "request_executions", "should reference request_executions table")
			assert.Contains(t, got, "usage_logs", "should reference usage_logs table")
			assert.Contains(t, got, "GROUP BY se.channel_id", "should group by channel_id")
			assert.Contains(t, got, "ORDER BY se.channel_id", "should order by channel_id")
		})
	}
}

// TestBuildProbeStatsQuery_PlaceholderCount tests that placeholders are correctly placed.
func TestBuildProbeStatsQuery_PlaceholderCount(t *testing.T) {
	tests := []struct {
		name                  string
		useDollarPlaceholders bool
		channelIDFilter       string
		mode                  ThroughputQueryMode
		expectedDollarCount   int
		expectedQuestionCount int
	}{
		{
			name:                  "dollar placeholders count",
			useDollarPlaceholders: true,
			channelIDFilter:       "AND se.channel_id IN ($3, $4)",
			mode:                  ThroughputModeROW_NUMBER,
			expectedDollarCount:   4,
			expectedQuestionCount: 0,
		},
		{
			name:                  "question mark placeholders count",
			useDollarPlaceholders: false,
			channelIDFilter:       "AND se.channel_id IN (?, ?)",
			mode:                  ThroughputModeROW_NUMBER,
			expectedDollarCount:   0,
			expectedQuestionCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildProbeStatsQuery(tt.useDollarPlaceholders, tt.channelIDFilter, tt.mode)

			dollarCount := strings.Count(got, "$1") + strings.Count(got, "$2") + strings.Count(got, "$3") + strings.Count(got, "$4")
			questionCount := strings.Count(got, "?")

			assert.Equal(t, tt.expectedDollarCount, dollarCount, "dollar placeholder count should match")
			assert.Equal(t, tt.expectedQuestionCount, questionCount, "question mark placeholder count should match")
		})
	}
}

// TestThroughputQueryTypeEnum tests that the enum values are correct.
func TestThroughputQueryTypeEnum(t *testing.T) {
	assert.Equal(t, ThroughputQueryType(0), ThroughputQueryByChannel, "ThroughputQueryByChannel should be 0")
	assert.Equal(t, ThroughputQueryType(1), ThroughputQueryByModel, "ThroughputQueryByModel should be 1")
}

// TestThroughputQueryModeEnum tests that the enum values are correct.
func TestThroughputQueryModeEnum(t *testing.T) {
	assert.Equal(t, ThroughputQueryMode(0), ThroughputModeROW_NUMBER, "ThroughputModeROW_NUMBER should be 0")
	assert.Equal(t, ThroughputQueryMode(1), ThroughputModeMaxID, "ThroughputModeMaxID should be 1")
}

// TestAllowedQueryConfigs tests that the query configurations are valid.
func TestAllowedQueryConfigs(t *testing.T) {
	// Test that both query types have valid configurations
	assert.Contains(t, AllowedQueryConfigs, ThroughputQueryByChannel, "should have config for ByChannel")
	assert.Contains(t, AllowedQueryConfigs, ThroughputQueryByModel, "should have config for ByModel")

	// Test ByChannel config
	channelConfig := AllowedQueryConfigs[ThroughputQueryByChannel]
	assert.Contains(t, channelConfig.SelectColumns, "channel_id", "should include channel_id")
	assert.Contains(t, channelConfig.SelectColumns, "channel_name", "should include channel_name")
	assert.Contains(t, channelConfig.JoinClause, "channels c ON", "should join channels table")
	assert.Contains(t, channelConfig.GroupBy, "channel_id", "should group by channel_id")

	// Test ByModel config
	modelConfig := AllowedQueryConfigs[ThroughputQueryByModel]
	assert.Contains(t, modelConfig.SelectColumns, "model_id", "should include model_id")
	assert.Contains(t, modelConfig.SelectColumns, "model_name", "should include model_name")
	assert.Contains(t, modelConfig.JoinClause, "requests r ON", "should join requests table")
	assert.Contains(t, modelConfig.JoinClause, "models m ON", "should join models table")
	assert.Contains(t, modelConfig.GroupBy, "model_id", "should group by model_id")
}
