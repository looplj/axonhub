package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
)

func TestRandomStrategy_Score(t *testing.T) {
	ctx := context.Background()
	strategy := NewRandomStrategy()

	channel := &biz.Channel{}

	// Run multiple times to see if we get different values
	scores := make(map[float64]bool)

	for range 100 {
		score := strategy.Score(ctx, channel)
		require.GreaterOrEqual(t, score, float64(0))
		require.LessOrEqual(t, score, 0.5)
		scores[score] = true
	}

	// It's extremely unlikely to get the same float64 100 times
	require.Greater(t, len(scores), 1, "Should produce random scores")
}

func TestRandomStrategy_ScoreWithDebug(t *testing.T) {
	ctx := context.Background()
	strategy := NewRandomStrategy()

	channel := &biz.Channel{}

	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	require.Equal(t, "Random", strategyScore.StrategyName)
	require.Equal(t, score, strategyScore.Score)
	require.Equal(t, float64(0), strategyScore.Details["min"])
	require.Equal(t, 0.5, strategyScore.Details["max"])
	require.Equal(t, score, strategyScore.Details["score"])
}

func TestRandomStrategy_Name(t *testing.T) {
	strategy := NewRandomStrategy()
	require.Equal(t, "Random", strategy.Name())
}
