package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestOrderedStrategy_Name(t *testing.T) {
	strategy := NewOrderedStrategy()
	assert.Equal(t, "Ordered", strategy.Name())
}

func TestOrderedStrategy_Score(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	tests := []struct {
		name           string
		orderingWeight int
	}{
		{
			name:           "zero weight channel",
			orderingWeight: 0,
		},
		{
			name:           "low weight channel",
			orderingWeight: 10,
		},
		{
			name:           "medium weight channel",
			orderingWeight: 50,
		},
		{
			name:           "high weight channel",
			orderingWeight: 100,
		},
		{
			name:           "very high weight channel",
			orderingWeight: 1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tt.orderingWeight,
				},
			}

			score := strategy.Score(ctx, channel)
			// Score should be baseScore (1000000) + orderingWeight
			expectedScore := 10000.0 + float64(tt.orderingWeight)
			assert.Equal(t, expectedScore, score)
		})
	}
}

func TestOrderedStrategy_Score_HigherWeightGetsHigherScore(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	// Create channels with different weights
	channel1 := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "low-weight", OrderingWeight: 10},
	}
	channel2 := &biz.Channel{
		Channel: &ent.Channel{ID: 2, Name: "medium-weight", OrderingWeight: 50},
	}
	channel3 := &biz.Channel{
		Channel: &ent.Channel{ID: 3, Name: "high-weight", OrderingWeight: 100},
	}

	score1 := strategy.Score(ctx, channel1)
	score2 := strategy.Score(ctx, channel2)
	score3 := strategy.Score(ctx, channel3)

	// Higher weight should always get higher score
	assert.Greater(t, score3, score2, "Channel with weight=100 should have higher score than weight=50")
	assert.Greater(t, score2, score1, "Channel with weight=50 should have higher score than weight=10")
	assert.Greater(t, score3, score1, "Channel with weight=100 should have higher score than weight=10")
}

func TestOrderedStrategy_Score_NegativeWeight(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	tests := []struct {
		name           string
		orderingWeight int
	}{
		{
			name:           "negative weight",
			orderingWeight: -100,
		},
		{
			name:           "large negative weight",
			orderingWeight: -500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tt.orderingWeight,
				},
			}

			score := strategy.Score(ctx, channel)
			// Score should be baseScore + orderingWeight
			expectedScore := 10000.0 + float64(tt.orderingWeight)
			assert.Equal(t, expectedScore, score)
			assert.Greater(t, score, 0.0, "Score should still be positive even with negative weight")
		})
	}
}

func TestOrderedStrategy_Score_ClampingToZero(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	// Test with extremely negative weight that should trigger clamping
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             1,
			Name:      "test",
			OrderingWeight: -20000,
		},
	}

	score := strategy.Score(ctx, channel)
	// Score should be clamped to 0
	assert.Equal(t, 0.0, score, "Score should be clamped to 0 for extremely negative weight")
}

func TestOrderedStrategy_Score_VeryHighWeight(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	// Test with very high weight
	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             1,
			Name:           "test",
			OrderingWeight: 999999,
		},
	}

	score := strategy.Score(ctx, channel)
	// Score should be baseScore + weight
	expectedScore := 10000.0 + 999999.0
	assert.Equal(t, expectedScore, score, "Score should handle very high weights")
	assert.Greater(t, score, 0.0, "Score should be positive")
}

func TestOrderedStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	testCases := []struct {
		name           string
		orderingWeight int
	}{
		{"zero weight", 0},
		{"low weight", 10},
		{"medium weight", 50},
		{"high weight", 1000},
		{"negative weight", -10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tc.orderingWeight,
				},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for orderingWeight=%d", tc.orderingWeight)
		})
	}
}

func TestOrderedStrategy_ScoreWithDebug_Details(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             5,
			Name:           "test",
			OrderingWeight: 75,
		},
	}

	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, "Ordered", strategyScore.StrategyName)
	assert.Equal(t, score, strategyScore.Score)
	assert.NotNil(t, strategyScore.Details)
	assert.Contains(t, strategyScore.Details, "ordering_weight")
	assert.Contains(t, strategyScore.Details, "base_score")
	assert.Contains(t, strategyScore.Details, "calculated_score")
}

// TestOrderedStrategy_StrictOrdering verifies that the strategy enforces strict ordering
// where higher weight channels are always preferred over lower weight channels.
func TestOrderedStrategy_StrictOrdering(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	// Simulate multiple channels with different weights
	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "channel-a", OrderingWeight: 50}},
		{Channel: &ent.Channel{ID: 2, Name: "channel-b", OrderingWeight: 100}},
		{Channel: &ent.Channel{ID: 3, Name: "channel-c", OrderingWeight: 25}},
		{Channel: &ent.Channel{ID: 4, Name: "channel-d", OrderingWeight: 75}},
		{Channel: &ent.Channel{ID: 5, Name: "channel-e", OrderingWeight: 10}},
	}

	// Calculate scores for all channels
	type channelScore struct {
		name   string
		weight int
		score  float64
	}

	scores := make([]channelScore, len(channels))
	for i, ch := range channels {
		scores[i] = channelScore{
			name:   ch.Name,
			weight: ch.OrderingWeight,
			score:  strategy.Score(ctx, ch),
		}
	}

	// Verify that channels are ordered by weight (higher weight = higher score)
	// Channel with weight=100 should have highest score, weight=10 should have lowest
	for i := 0; i < len(scores); i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[i].weight > scores[j].weight {
				assert.Greater(t, scores[i].score, scores[j].score,
					"Channel %s (weight=%d) should have higher score than %s (weight=%d)",
					scores[i].name, scores[i].weight, scores[j].name, scores[j].weight)
			} else if scores[i].weight < scores[j].weight {
				assert.Less(t, scores[i].score, scores[j].score,
					"Channel %s (weight=%d) should have lower score than %s (weight=%d)",
					scores[i].name, scores[i].weight, scores[j].name, scores[j].weight)
			}
		}
	}
}

// TestOrderedStrategy_PositiveScores verifies that all reasonable weights get positive scores.
func TestOrderedStrategy_PositiveScores(t *testing.T) {
	ctx := context.Background()
	strategy := NewOrderedStrategy()

	// Test a range of reasonable weights
	testWeights := []int{0, 10, 100, 1000, 10000, 100000, 500000, 999999}

	for _, weight := range testWeights {
		channel := &biz.Channel{
			Channel: &ent.Channel{ID: 1, Name: "test", OrderingWeight: weight},
		}

		score := strategy.Score(ctx, channel)
		assert.Greater(t, score, 0.0, "Channel with weight=%d should have positive score", weight)
	}
}
