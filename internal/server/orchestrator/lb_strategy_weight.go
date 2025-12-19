package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// WeightStrategy prioritizes channels based on their ordering weight.
// Higher weight = higher priority.
type WeightStrategy struct {
	// maxScore is the maximum score this strategy can assign (default: 100)
	maxScore float64
}

// NewWeightStrategy creates a new weight-based strategy.
func NewWeightStrategy() *WeightStrategy {
	return &WeightStrategy{
		maxScore: 100.0,
	}
}

// Score returns a score based on the channel's ordering weight.
// Score is normalized to 0-maxScore range.
// Production path without debug logging.
func (s *WeightStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Weight is typically 0-100, normalize to 0-maxScore
	weight := float64(channel.OrderingWeight)
	if weight < 0 {
		weight = 0
	}

	// Assume max weight is 100, scale accordingly
	score := (weight / 100.0) * s.maxScore

	return score
}

// ScoreWithDebug returns a score with detailed debug information.
// Debug path with comprehensive logging.
func (s *WeightStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	// Weight is typically 0-100, normalize to 0-maxScore
	weight := float64(channel.OrderingWeight)
	details := map[string]interface{}{
		"ordering_weight": weight,
		"max_score":       s.maxScore,
	}

	if weight < 0 {
		log.Info(ctx, "WeightStrategy: channel has negative weight, clamping to 0",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("weight", weight),
		)

		details["clamped"] = true
		details["original_weight"] = weight
		weight = 0
	}

	// Assume max weight is 100, scale accordingly
	score := (weight / 100.0) * s.maxScore
	details["calculated_score"] = score

	log.Info(ctx, "WeightStrategy: calculated score",
		log.Int("channel_id", channel.ID),
		log.String("channel_name", channel.Name),
		log.Float64("ordering_weight", weight),
		log.Float64("max_score", s.maxScore),
		log.Float64("score", score),
	)

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
	}
}

// Name returns the strategy name.
func (s *WeightStrategy) Name() string {
	return "Weight"
}
