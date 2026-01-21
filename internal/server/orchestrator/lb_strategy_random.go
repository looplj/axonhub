package orchestrator

import (
	"context"
	"math/rand/v2"

	"github.com/looplj/axonhub/internal/server/biz"
)

// RandomStrategy adds a small random score to break ties between channels
// with identical scores from other strategies.
type RandomStrategy struct {
	// min is the minimum random score (default: 0)
	min float64
	// max is the maximum random score (default: 0.5)
	max float64
}

// NewRandomStrategy creates a new random strategy.
func NewRandomStrategy() *RandomStrategy {
	return &RandomStrategy{
		min: 0,
		max: 0.5,
	}
}

// Score returns a random score between min and max.
func (s *RandomStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	//nolint:gosec // Checked.
	return s.min + rand.Float64()*(s.max-s.min)
}

// ScoreWithDebug returns a random score with detailed debug information.
func (s *RandomStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	score := s.Score(ctx, channel)

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details: map[string]any{
			"min":   s.min,
			"max":   s.max,
			"score": score,
		},
	}
}

// Name returns the strategy name.
func (s *RandomStrategy) Name() string {
	return "Random"
}
