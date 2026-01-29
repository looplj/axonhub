package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestFailoverStrategy_Simulation(t *testing.T) {
	ctx := context.Background()
	modelID := "gpt-4"

	// Setup metrics provider (not used by Weight+Random but kept for structure)
	metricsProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}

	// Setup LoadBalancer with Weight + Random strategies
	// This combination is used in the 'failover' load balancer strategy in orchestrator.go
	policyProvider := &mockRetryPolicyProvider{
		policy: &biz.RetryPolicy{
			Enabled:              true,
			MaxChannelRetries:    3,
			LoadBalancerStrategy: biz.LoadBalancerStrategyFailover,
		},
	}
	selectionTracker := &mockSelectionTracker{selections: make(map[int]int)}

	lb := NewLoadBalancer(policyProvider, selectionTracker,
		NewWeightStrategy(),
		NewRandomStrategy(),
	)

	// Create 3 channels with different weights
	// Ch1: Weight 100
	// Ch2: Weight 50
	// Ch3: Weight 10
	channels := []*biz.Channel{
		{
			Channel: &ent.Channel{
				ID:             1,
				Name:           "channel-1",
				OrderingWeight: 100,
			},
		},
		{
			Channel: &ent.Channel{
				ID:             2,
				Name:           "channel-2",
				OrderingWeight: 50,
			},
		},
		{
			Channel: &ent.Channel{
				ID:             3,
				Name:           "channel-3",
				OrderingWeight: 10,
			},
		},
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: channels[0]},
		{Channel: channels[1]},
		{Channel: channels[2]},
	}

	// Helper to select all sorted candidates
	selectAll := func() []*ChannelModelsCandidate {
		return lb.Sort(ctx, candidates, modelID)
	}

	// 1. Initial state: all channels are healthy
	// WeightStrategy will sort by OrderingWeight.
	// RandomStrategy adds a tiny bit to break ties (not applicable here).
	// Ch1 score: 100
	// Ch2 score: 50
	// Ch3 score: 10

	sorted := selectAll()
	assert.Len(t, sorted, 3)
	assert.Equal(t, 1, sorted[0].Channel.ID)
	assert.Equal(t, 2, sorted[1].Channel.ID)
	assert.Equal(t, 3, sorted[2].Channel.ID)

	// 2. Simulate failures for channel-1
	// In 'failover' strategy (Weight + Random), failures do NOT affect sorting
	// because there is no ErrorAwareStrategy.
	// The failover happens at the pipeline level by trying the next candidate.

	// Recording failures...
	m1 := &biz.AggregatedMetrics{}
	m1.ConsecutiveFailures = 5
	metricsProvider.metrics[1] = m1

	// Sort again - should still be the same order!
	sorted = selectAll()
	assert.Equal(t, 1, sorted[0].Channel.ID, "Failover strategy should still prioritize Ch1 despite failures (it's not error-aware)")
	assert.Equal(t, 2, sorted[1].Channel.ID)
	assert.Equal(t, 3, sorted[2].Channel.ID)

	// 3. Verify that the orchestrator's failover would work by picking the next in list
	// This is what the 'failover' strategy means in this context:
	// a static priority list that the pipeline iterates through.
	assert.Equal(t, "channel-1", sorted[0].Channel.Name)
	assert.Equal(t, "channel-2", sorted[1].Channel.Name)
	assert.Equal(t, "channel-3", sorted[2].Channel.Name)
}
