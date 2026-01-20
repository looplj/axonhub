package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// OrderedStrategy prioritizes channels based on their ordering weight.
// Higher weight = higher priority.
// This strategy is designed for use cases where you want to always use
// Channel A first, and only use Channel B if Channel A fails.
type OrderedStrategy struct {
	// baseScore is the base score used in calculation (default: 1000000)
	baseScore float64
}

// NewOrderedStrategy creates a new ordered strategy.
func NewOrderedStrategy() *OrderedStrategy {
	return &OrderedStrategy{
		baseScore: 1000000.0, // Large base ensures all channels get positive scores
	}
}

// Score returns a score based on the channel's ordering weight.
// Higher weight = higher score = higher priority.
// Production path without debug logging.
func (s *OrderedStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Direct relationship: higher weight gets higher score
	// This ensures channels with higher weight are used first
	// Formula: baseScore + orderingWeight
	orderingWeight := float64(channel.OrderingWeight)

	score := s.baseScore + orderingWeight

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// ScoreWithDebug returns a score with detailed debug information.
// Debug path with comprehensive logging.
func (s *OrderedStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	orderingWeight := float64(channel.OrderingWeight)

	// Calculate score: higher weight = higher score
	score := s.baseScore + orderingWeight

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	details := map[string]any{
		"ordering_weight":  orderingWeight,
		"base_score":       s.baseScore,
		"calculated_score": score,
	}

	log.Info(ctx, "OrderedStrategy: calculated score based on ordering weight",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Int("ordering_weight", channel.OrderingWeight),
		log.Float64("score", score),
	)

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *OrderedStrategy) Name() string {
	return "Ordered"
}
