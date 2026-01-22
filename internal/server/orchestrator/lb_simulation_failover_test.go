package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestErrorAwareStrategy_FailoverSimulation(t *testing.T) {
	ctx := context.Background()
	modelID := "gpt-4"

	// Setup metrics provider
	metricsProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}

	// Setup LoadBalancer with Weight + ErrorAware strategies
	// This combination is used in the 'adaptive' load balancer strategy
	policyProvider := &mockRetryPolicyProvider{
		policy: &biz.RetryPolicy{
			Enabled:              true,
			MaxChannelRetries:    3,
			LoadBalancerStrategy: biz.LoadBalancerStrategyAdaptive,
		},
	}
	selectionTracker := &mockSelectionTracker{selections: make(map[int]int)}

	lb := NewLoadBalancer(policyProvider, selectionTracker,
		NewWeightStrategy(),
		NewErrorAwareStrategy(metricsProvider),
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

	// Helper to select best channel using LB.Sort
	selectBest := func() *biz.Channel {
		sorted := lb.Sort(ctx, candidates, modelID)
		if len(sorted) > 0 {
			return sorted[0].Channel
		}

		return nil
	}

	// Helper to record failure for a channel
	recordFailure := func(channelID int) {
		m, ok := metricsProvider.metrics[channelID]
		if !ok {
			m = &biz.AggregatedMetrics{}
			metricsProvider.metrics[channelID] = m
		}

		m.ConsecutiveFailures++
		now := time.Now()
		m.LastFailureAt = &now
		m.RequestCount++
		m.FailureCount++
	}

	// Helper to record success for a channel
	recordSuccess := func(channelID int) {
		m, ok := metricsProvider.metrics[channelID]
		if !ok {
			m = &biz.AggregatedMetrics{}
			metricsProvider.metrics[channelID] = m
		}

		m.ConsecutiveFailures = 0
		now := time.Now()
		m.LastSuccessAt = &now
		m.LastFailureAt = nil // Clear failure for full recovery in test
		m.RequestCount++
		m.SuccessCount++
	}

	// 1. Initial state: all channels are healthy
	// WeightStrategy maxScore is 100.
	// ErrorAwareStrategy maxScore is 200.
	// Ch1 score: weight(100/100)*100 + error(200) = 300
	// Ch2 score: weight(50/100)*100 + error(200) = 250
	// Ch3 score: weight(10/100)*100 + error(200) = 210
	// Note: WeightStrategy normalizes weights relative to max weight (100).

	counts := make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Initial distribution (all healthy): %v", counts)
	assert.Equal(t, 1000, counts[1], "Channel 1 should be selected every time due to highest weight when all are healthy")

	// 2. Simulate failures for channel-1
	// Each consecutive failure subtracts 50 (default penaltyPerConsecutiveFailure)
	// Plus recent failure penalty up to 100.
	// After 2 failures:
	// Ch1 error score: 200 - (2 * 50) - 100 (recent) = 0
	// Ch1 total score: 100 (weight) + 0 = 100
	// Ch2 total score: 50 + 200 = 250
	// Ch3 total score: 10 + 200 = 210
	// Now Ch2 should be selected!

	recordFailure(1)
	recordFailure(1)

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution after Ch1 failures: %v", counts)
	assert.Equal(t, 0, counts[1], "Channel 1 should not be selected after failures")
	assert.Equal(t, 1000, counts[2], "Channel 2 should be selected now")

	// 3. Simulate failures for Ch2
	// After 2 failures for Ch2:
	// Ch2 score: 50 (weight) + 0 (error) = 50
	// Ch3 score: 10 (weight) + 200 (error) = 210
	// Now Ch3 should be selected!
	recordFailure(2)
	recordFailure(2)

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution after Ch1, Ch2 failures: %v", counts)
	assert.Equal(t, 1000, counts[3], "Channel 3 should be selected now")

	// 4. Simulate Ch1 recovery
	recordSuccess(1)
	// Ch1 score: 100 (weight) + 200 (error) = 300
	// Ch1 should be back to top!
	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution after Ch1 recovery: %v", counts)
	assert.Equal(t, 1000, counts[1], "Channel 1 should be back to top after recovery")

	// 5. Simulate all channels failing
	for _, ch := range channels {
		recordFailure(ch.ID)
		recordFailure(ch.ID)
		recordFailure(ch.ID)
	}

	// All channels have roughly the same penalty.
	// Ch1: 100 + 0 = 100
	// Ch2: 50 + 0 = 50
	// Ch3: 10 + 0 = 10
	// Ch1 should win again by weight.

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution with all channels failing: %v", counts)
	assert.Equal(t, 1000, counts[1], "Channel 1 should be selected by weight when all are failing equally")
}
