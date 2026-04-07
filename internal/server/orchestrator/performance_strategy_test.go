package orchestrator

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestPerformanceLoadBalancerStrategy(t *testing.T) {
	if biz.LoadBalancerStrategyPerformance != "performance" {
		t.Errorf("LoadBalancerStrategyPerformance = %q, want %q", biz.LoadBalancerStrategyPerformance, "performance")
	}

	strategy := &PerformanceAwareStrategy{
		maxScore:         150.0,
		historicalWeight: 0.5,
		realtimeWeight:   0.5,
	}

	var _ LoadBalanceStrategy = strategy

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test-channel"},
	}

	ctx := context.Background()
	score := strategy.Score(ctx, channel)
	if score < 0 {
		t.Errorf("Score() = %f, want >= 0", score)
	}

	debugScore, strategyScore := strategy.ScoreWithDebug(ctx, channel)
	if debugScore != score {
		t.Errorf("ScoreWithDebug() = %f, want %f", debugScore, score)
	}
	if strategyScore.StrategyName != "performance-aware" {
		t.Errorf("StrategyName = %q, want %q", strategyScore.StrategyName, "performance-aware")
	}
}
