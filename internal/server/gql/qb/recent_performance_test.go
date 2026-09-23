package qb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The percentile seek is the only query in this family that binds a second parameter,
// so its offset marker has to be numbered after the boundary marker on Postgres.
// Reusing "$1" for both compiles but fails at execution: pgx sees one parameter where
// two are supplied, and the marker would have to be a timestamp and an integer at once.
func TestBuildRecentFirstTokenPercentileQuery_OffsetPlaceholder(t *testing.T) {
	tests := []struct {
		name              string
		mode              ThroughputQueryMode
		placeholder       string
		offsetPlaceholder string
		wantBoundaryHits  int
		wantOffset        string
	}{
		{
			name:              "postgres ROW_NUMBER numbers the offset after the single boundary",
			mode:              ThroughputModeRowNumber,
			placeholder:       "$1",
			offsetPlaceholder: "$2",
			wantBoundaryHits:  1,
			wantOffset:        "OFFSET $2",
		},
		{
			name:              "postgres MAX_ID numbers the offset after both boundaries",
			mode:              ThroughputModeMaxID,
			placeholder:       "$1",
			offsetPlaceholder: "$3",
			wantBoundaryHits:  2,
			wantOffset:        "OFFSET $3",
		},
		{
			name:              "positional dialects take the next marker in line",
			mode:              ThroughputModeMaxID,
			placeholder:       "?",
			offsetPlaceholder: "?",
			wantBoundaryHits:  2,
			wantOffset:        "OFFSET ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRecentFirstTokenPercentileQuery(tt.mode, tt.placeholder, tt.offsetPlaceholder)

			assert.Contains(t, got, tt.wantOffset)
			assert.Equal(t, tt.wantBoundaryHits, strings.Count(got, "created_at >= "+tt.placeholder))
			// The offset marker must never collide with the boundary marker on Postgres,
			// otherwise the bound integer lands on a timestamp parameter.
			if tt.placeholder != tt.offsetPlaceholder {
				assert.NotContains(t, got, "OFFSET "+tt.placeholder)
			}
			assert.Contains(t, got, "ORDER BY se.metrics_first_token_latency_ms LIMIT 1")
		})
	}
}

// The count variant shares the boundary markers with the seek but has no offset, so a
// caller that reuses one arg slice for both stays correct.
func TestBuildRecentFirstTokenSampleCountQuery_HasNoOffset(t *testing.T) {
	got := BuildRecentFirstTokenSampleCountQuery(ThroughputModeRowNumber, "$1")

	assert.NotContains(t, got, "OFFSET")
	assert.Equal(t, 1, strings.Count(got, "created_at >= $1"))
	assert.Contains(t, got, "COUNT(*) as sample_count")
}
