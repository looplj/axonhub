package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestWeightStrategy_Score(t *testing.T) {
	ctx := context.Background()
	strategy := NewWeightStrategy()

	tests := []struct {
		name        string
		weight      int
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "zero weight",
			weight:      0,
			expectedMin: 0,
			expectedMax: 0,
		},
		{
			name:        "low weight",
			weight:      25,
			expectedMin: 24,
			expectedMax: 26,
		},
		{
			name:        "medium weight",
			weight:      50,
			expectedMin: 49,
			expectedMax: 51,
		},
		{
			name:        "high weight",
			weight:      100,
			expectedMin: 99,
			expectedMax: 101,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tt.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)
		})
	}
}

func TestWeightStrategy_Name(t *testing.T) {
	strategy := NewWeightStrategy()
	assert.Equal(t, "Weight", strategy.Name())
}

func TestWeightStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()
	strategy := NewWeightStrategy()

	testCases := []struct {
		name   string
		weight int
	}{
		{"zero weight", 0},
		{"low weight", 25},
		{"medium weight", 50},
		{"high weight", 100},
		{"negative weight", -10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tc.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for weight=%d", tc.weight)
		})
	}
}
