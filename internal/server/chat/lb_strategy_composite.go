package chat

import (
	"context"

	"github.com/looplj/axonhub/internal/server/biz"
)

// CompositeStrategy combines multiple strategies with configurable weights.
type CompositeStrategy struct {
	strategies []compositeStrategyWeight
}

type compositeStrategyWeight struct {
	strategy LoadBalanceStrategy
	weight   float64
}

// NewCompositeStrategy creates a new composite strategy.
func NewCompositeStrategy(strategies ...LoadBalanceStrategy) *CompositeStrategy {
	weighted := make([]compositeStrategyWeight, len(strategies))
	for i, s := range strategies {
		weighted[i] = compositeStrategyWeight{
			strategy: s,
			weight:   1.0, // Default weight
		}
	}

	return &CompositeStrategy{
		strategies: weighted,
	}
}

// WithWeights sets custom weights for the strategies.
// weights slice should match the order of strategies.
func (c *CompositeStrategy) WithWeights(weights ...float64) *CompositeStrategy {
	for i, w := range weights {
		if i < len(c.strategies) {
			c.strategies[i].weight = w
		}
	}

	return c
}

// Score combines all strategy scores with their weights.
// Production path without debug logging.
func (c *CompositeStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	totalScore := 0.0

	for _, ws := range c.strategies {
		score := ws.strategy.Score(ctx, channel)
		totalScore += score * ws.weight
	}

	return totalScore
}

// ScoreWithDebug combines all strategy scores with detailed debug information.
// Debug path with comprehensive logging.
func (c *CompositeStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	totalScore := 0.0
	details := map[string]any{}

	strategies := make([]map[string]any, 0, len(c.strategies))

	for _, ws := range c.strategies {
		score, strategyScore := ws.strategy.ScoreWithDebug(ctx, channel)
		weightedScore := score * ws.weight
		totalScore += weightedScore

		strategy := map[string]any{
			"name":           strategyScore.StrategyName,
			"score":          score,
			"weight":         ws.weight,
			"weighted_score": weightedScore,
			"details":        strategyScore.Details,
		}
		strategies = append(strategies, strategy)
	}

	details["strategies"] = strategies
	details["total_score"] = totalScore

	return totalScore, StrategyScore{
		StrategyName: c.Name(),
		Score:        totalScore,
		Details:      details,
	}
}

// Name returns the strategy name.
func (c *CompositeStrategy) Name() string {
	return "Composite"
}
