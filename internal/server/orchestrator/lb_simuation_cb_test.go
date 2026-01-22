package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

type mockRetryPolicyProvider struct {
	policy *biz.RetryPolicy
}

func (m *mockRetryPolicyProvider) RetryPolicyOrDefault(ctx context.Context) *biz.RetryPolicy {
	return m.policy
}

type mockSelectionTracker struct {
	selections map[int]int
}

func (m *mockSelectionTracker) IncrementChannelSelection(channelID int) {
	if m.selections == nil {
		m.selections = make(map[int]int)
	}

	m.selections[channelID]++
}

func TestModelAwareCircuitBreakerStrategy_Simulation(t *testing.T) {
	ctx := context.Background()
	modelID := "gpt-4"

	// Setup circuit breaker
	cb := biz.NewModelCircuitBreaker()

	// Setup LoadBalancer with Weight + CB strategies
	policyProvider := &mockRetryPolicyProvider{
		policy: &biz.RetryPolicy{
			Enabled:              true,
			MaxChannelRetries:    3,
			LoadBalancerStrategy: biz.LoadBalancerStrategyCircuitBreaker,
		},
	}
	selectionTracker := &mockSelectionTracker{selections: make(map[int]int)}

	lb := NewLoadBalancer(policyProvider, selectionTracker,
		NewWeightStrategy(),
		NewModelAwareCircuitBreakerStrategy(cb),
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

	// 1. Initial state: all channels are Closed (healthy)
	// WeightStrategy: Ch1(100) > Ch2(50) > Ch3(10)
	// CBStrategy: All Closed (weight 1.0)
	// Since WeightStrategy is first, Ch1 should always be selected if it has significantly higher weight.
	// Wait, in LoadBalancer.Sort:
	// totalScore += strategy.Score(ctx, c.Channel)
	// WeightStrategy maxScore is 100.
	// CBStrategy maxScore is 200.
	// So CB has more weight in the total score!

	counts := make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Initial distribution (all healthy): %v", counts)
	// Ch1 score: weight(100/100)*100 + cb(1.0)*200 = 100 + 200 = 300
	// Ch2 score: weight(50/100)*100 + cb(1.0)*200 = 50 + 200 = 250
	// Ch3 score: weight(10/100)*100 + cb(1.0)*200 = 10 + 200 = 210
	// Ch1 should be selected every time because it always has the highest score.
	assert.Equal(t, 1000, counts[1], "Channel 1 should be selected every time due to highest weight when all are healthy")

	// 2. Simulate failures for channel-1 until it hits Half-Open
	// HalfOpenThreshold is 3 by default. HalfOpenWeight is 0.3.
	for range 3 {
		cb.RecordError(ctx, 1, modelID)
	}

	stats1 := cb.GetModelCircuitBreakerStats(ctx, 1, modelID)
	assert.Equal(t, biz.StateHalfOpen, stats1.State)

	// Ch1 score: weight(1.0)*100 + cb(0.3)*200 = 100 + 60 = 160
	// Ch2 score: weight(0.5)*100 + cb(1.0)*200 = 50 + 200 = 250
	// Ch3 score: weight(0.1)*100 + cb(1.0)*200 = 10 + 200 = 210
	// Now Ch2 should be the best!

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution after Ch1 half-open: %v", counts)
	assert.Equal(t, 0, counts[1], "Channel 1 should not be selected when half-open and others are healthy")
	assert.Equal(t, 1000, counts[2], "Channel 2 should be selected now")

	// 3. Simulate failures for Ch2 until it also hits Half-Open
	for range 3 {
		cb.RecordError(ctx, 2, modelID)
	}

	// Ch1 score: 160 (half-open)
	// Ch2 score: weight(0.5)*100 + cb(0.3)*200 = 50 + 60 = 110 (half-open)
	// Ch3 score: 210 (healthy)
	// Now Ch3 should be the best!

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution after Ch1, Ch2 half-open: %v", counts)
	assert.Equal(t, 1000, counts[3], "Channel 3 should be selected now")

	// 4. Simulate Ch1 recovery
	cb.RecordSuccess(ctx, 1, modelID)

	// Ch1 score: 300 (healthy)
	// Ch2 score: 110 (half-open)
	// Ch3 score: 210 (healthy)
	// Ch1 should be back to top!

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution after Ch1 recovery: %v", counts)
	assert.Equal(t, 1000, counts[1], "Channel 1 should be back to top")

	// 5. Simulate all channels Open
	for _, ch := range channels {
		for range 5 {
			cb.RecordError(ctx, ch.ID, modelID)
		}
	}

	// All Open, CB scores are 0 (or 0.01 for probing if time passed, but here we don't wait)
	// WeightStrategy will decide.
	// Ch1 score: 100 + 0 = 100
	// Ch2 score: 50 + 0 = 50
	// Ch3 score: 10 + 0 = 10

	counts = make(map[int]int)

	for range 1000 {
		ch := selectBest()
		counts[ch.ID]++
	}

	t.Logf("Distribution with all channels open: %v", counts)
	assert.Equal(t, 1000, counts[1], "Channel 1 should be selected by weight when all are open")
}

func TestModelAwareCircuitBreakerStrategy_EqualWeightDistribution(t *testing.T) {
	ctx := context.Background()
	modelID := "gpt-4"

	cb := biz.NewModelCircuitBreaker()
	policyProvider := &mockRetryPolicyProvider{
		policy: &biz.RetryPolicy{
			Enabled:              true,
			LoadBalancerStrategy: biz.LoadBalancerStrategyCircuitBreaker,
		},
	}
	lb := NewLoadBalancer(policyProvider, nil,
		NewWeightStrategy(),
		NewModelAwareCircuitBreakerStrategy(cb),
	)

	// Three channels with EQUAL weight
	channels := []*biz.Channel{
		{Channel: &ent.Channel{ID: 1, Name: "ch1", OrderingWeight: 100}},
		{Channel: &ent.Channel{ID: 2, Name: "ch2", OrderingWeight: 100}},
		{Channel: &ent.Channel{ID: 3, Name: "ch3", OrderingWeight: 100}},
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: channels[0]},
		{Channel: channels[1]},
		{Channel: channels[2]},
	}

	counts := make(map[int]int)

	for range 1000 {
		sorted := lb.Sort(ctx, candidates, modelID)
		counts[sorted[0].Channel.ID]++
	}

	t.Logf("Equal weight distribution: %v", counts)

	for _, ch := range channels {
		assert.Greater(t, counts[ch.ID], 200, "Channel %d should have roughly 1/3 of traffic", ch.ID)
	}
}
