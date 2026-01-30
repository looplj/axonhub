package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

// fakeChannelMetricsProvider is a fake implementation that tracks request counts dynamically.
// It allows simulating request distribution by incrementing request counts.
type fakeChannelMetricsProvider struct {
	metrics map[int]*biz.AggregatedMetrics
}

func newFakeChannelMetricsProvider() *fakeChannelMetricsProvider {
	return &fakeChannelMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
}

func (f *fakeChannelMetricsProvider) GetChannelMetrics(_ context.Context, channelID int) (*biz.AggregatedMetrics, error) {
	if m, ok := f.metrics[channelID]; ok {
		return m, nil
	}

	return &biz.AggregatedMetrics{}, nil
}

func (f *fakeChannelMetricsProvider) IncrementRequestCount(channelID int) {
	if _, ok := f.metrics[channelID]; !ok {
		f.metrics[channelID] = &biz.AggregatedMetrics{}
	}

	f.metrics[channelID].RequestCount++
	now := time.Now()
	f.metrics[channelID].LastSelectedAt = &now
}

func (f *fakeChannelMetricsProvider) GetRequestCount(channelID int) int64 {
	if m, ok := f.metrics[channelID]; ok {
		return m.RequestCount
	}

	return 0
}

// getNormalizedCount calculates the normalized request count for a channel.
// normalizedCount = requestCount / (weight / 100.0).
func getNormalizedCount(requestCount int64, weight int) float64 {
	weightFactor := float64(weight) / 100.0
	if weightFactor <= 0 {
		weightFactor = 0.01
	}

	return float64(requestCount) / weightFactor
}

func TestWeightRoundRobinStrategy_WithFakeProvider_Distribution(t *testing.T) {
	ctx := context.Background()

	// Create channels with weights: 80, 50, 20, 10 (total 160)
	// Expected distribution (ideal):
	// - weight=80: 80/160 = 50.0%
	// - weight=50: 50/160 = 31.25%
	// - weight=20: 20/160 = 12.5%
	// - weight=10: 10/160 = 6.25%
	weights := []int{80, 50, 20, 10}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	channels := make([]*biz.Channel, len(weights))
	for i, w := range weights {
		channels[i] = &biz.Channel{
			Channel: &ent.Channel{
				ID:             i + 1,
				Name:           fmt.Sprintf("channel-%d", i+1),
				OrderingWeight: w,
			},
		}
	}

	fakeProvider := newFakeChannelMetricsProvider()
	strategy := NewWeightRoundRobinStrategy(fakeProvider)

	// Simulate 1000 requests (all successful)
	const totalRequests = 1000

	requestCounts := make(map[int]int) // channelID -> request count

	for range int(totalRequests) {
		// Find the channel with the highest score
		// When scores are equal (or very close), prefer the channel with the lowest normalized count
		// This ensures proportional distribution based on weights
		var bestChannel *biz.Channel

		bestScore := -1.0
		bestNormalizedCount := float64(1e18)

		for _, ch := range channels {
			score := strategy.Score(ctx, ch)
			normalizedCount := getNormalizedCount(fakeProvider.GetRequestCount(ch.ID), ch.OrderingWeight)

			// Use score as primary criterion, normalized count as tie-breaker
			// Consider scores "equal" if they differ by less than 0.001
			if score > bestScore+0.001 {
				bestScore = score
				bestChannel = ch
				bestNormalizedCount = normalizedCount
			} else if score >= bestScore-0.001 && normalizedCount < bestNormalizedCount {
				// Scores are approximately equal, prefer lower normalized count
				bestScore = score
				bestChannel = ch
				bestNormalizedCount = normalizedCount
			}
		}

		// Record the request (assuming success)
		requestCounts[bestChannel.ID]++
		fakeProvider.IncrementRequestCount(bestChannel.ID)
	}

	// Calculate and report distribution
	t.Logf("Request distribution after %d requests:", totalRequests)
	t.Logf("%-15s %-10s %-15s %-15s %-15s", "Channel", "Weight", "Requests", "Actual %", "Expected %")
	t.Logf("%-15s %-10s %-15s %-15s %-15s", "-------", "------", "--------", "--------", "----------")

	for i, ch := range channels {
		count := requestCounts[ch.ID]
		actualPercent := float64(count) / float64(totalRequests) * 100
		expectedPercent := float64(weights[i]) / float64(totalWeight) * 100

		t.Logf("%-15s %-10d %-15d %-15.2f %-15.2f",
			ch.Name, weights[i], count, actualPercent, expectedPercent)

		// Verify distribution is within 2% tolerance
		assert.InDelta(t, expectedPercent, actualPercent, 2.0,
			"Channel %s (weight=%d) should receive approximately %.2f%% of requests, got %.2f%%",
			ch.Name, weights[i], expectedPercent, actualPercent)
	}

	// Verify total requests
	totalCounted := 0
	for _, count := range requestCounts {
		totalCounted += count
	}

	assert.Equal(t, totalRequests, totalCounted, "Total requests should match")

	// Verify each channel received at least some requests
	for _, ch := range channels {
		assert.Greater(t, requestCounts[ch.ID], 0,
			"Channel %s should receive at least one request", ch.Name)
	}
}
