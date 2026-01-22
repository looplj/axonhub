package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

type fakeAdaptiveMetricsProvider struct {
	windowSeconds int64
	nowMs         int64
	channels      map[int]*fakeAdaptiveChannelMetrics
}

type fakeAdaptiveSlotMetrics struct {
	requestCount int64
	successCount int64
	failureCount int64
}

type fakeAdaptiveChannelMetrics struct {
	slots              map[int64]*fakeAdaptiveSlotMetrics
	requestCount       int64
	successCount       int64
	failureCount       int64
	consecutiveFailure int64
	lastSuccessAtMs    *int64
	lastFailureAtMs    *int64
}

func newFakeAdaptiveMetricsProvider(windowSeconds int64) *fakeAdaptiveMetricsProvider {
	if windowSeconds <= 0 {
		windowSeconds = 600
	}

	return &fakeAdaptiveMetricsProvider{
		windowSeconds: windowSeconds,
		nowMs:         0,
		channels:      make(map[int]*fakeAdaptiveChannelMetrics),
	}
}

func (f *fakeAdaptiveMetricsProvider) AdvanceMs(ms int64) {
	if ms < 0 {
		return
	}

	f.nowMs += ms
}

func (f *fakeAdaptiveMetricsProvider) getOrCreateChannel(channelID int) *fakeAdaptiveChannelMetrics {
	if ch, ok := f.channels[channelID]; ok {
		return ch
	}

	ch := &fakeAdaptiveChannelMetrics{slots: make(map[int64]*fakeAdaptiveSlotMetrics)}
	f.channels[channelID] = ch

	return ch
}

func (f *fakeAdaptiveMetricsProvider) cleanupExpiredSlots(ch *fakeAdaptiveChannelMetrics) {
	if ch == nil || len(ch.slots) == 0 {
		return
	}

	currentSec := f.nowMs / 1000

	cutoffSec := currentSec - f.windowSeconds
	for ts, slot := range ch.slots {
		if ts < cutoffSec {
			ch.requestCount -= slot.requestCount
			ch.successCount -= slot.successCount
			ch.failureCount -= slot.failureCount
			delete(ch.slots, ts)
		}
	}
}

func (f *fakeAdaptiveMetricsProvider) GetChannelMetrics(_ context.Context, channelID int) (*biz.AggregatedMetrics, error) {
	ch := f.getOrCreateChannel(channelID)
	f.cleanupExpiredSlots(ch)

	var lastSuccessAt *time.Time

	if ch.lastSuccessAtMs != nil {
		deltaMs := f.nowMs - *ch.lastSuccessAtMs
		ts := time.Now().Add(-time.Duration(deltaMs) * time.Millisecond)
		lastSuccessAt = &ts
	}

	var lastFailureAt *time.Time

	if ch.lastFailureAtMs != nil {
		deltaMs := f.nowMs - *ch.lastFailureAtMs
		ts := time.Now().Add(-time.Duration(deltaMs) * time.Millisecond)
		lastFailureAt = &ts
	}

	m := &biz.AggregatedMetrics{
		LastSuccessAt: lastSuccessAt,
		LastFailureAt: lastFailureAt,
	}
	m.RequestCount = ch.requestCount
	m.SuccessCount = ch.successCount
	m.FailureCount = ch.failureCount
	m.ConsecutiveFailures = ch.consecutiveFailure

	return m, nil
}

func (f *fakeAdaptiveMetricsProvider) RecordSuccess(channelID int) {
	ch := f.getOrCreateChannel(channelID)
	f.cleanupExpiredSlots(ch)

	sec := f.nowMs / 1000

	slot, ok := ch.slots[sec]
	if !ok {
		slot = &fakeAdaptiveSlotMetrics{}
		ch.slots[sec] = slot
	}

	slot.requestCount++
	slot.successCount++
	ch.requestCount++
	ch.successCount++
	ch.consecutiveFailure = 0
	nowMs := f.nowMs
	ch.lastSuccessAtMs = &nowMs
}

func (f *fakeAdaptiveMetricsProvider) RecordFailure(channelID int) {
	ch := f.getOrCreateChannel(channelID)
	f.cleanupExpiredSlots(ch)

	sec := f.nowMs / 1000

	slot, ok := ch.slots[sec]
	if !ok {
		slot = &fakeAdaptiveSlotMetrics{}
		ch.slots[sec] = slot
	}

	slot.requestCount++
	slot.failureCount++
	ch.requestCount++
	ch.failureCount++
	ch.consecutiveFailure++
	nowMs := f.nowMs
	ch.lastFailureAtMs = &nowMs
}

type fakeTraceProvider struct {
	lastSuccessful map[int]int
}

func newFakeTraceProvider() *fakeTraceProvider {
	return &fakeTraceProvider{lastSuccessful: make(map[int]int)}
}

func (f *fakeTraceProvider) GetLastSuccessfulChannelID(_ context.Context, traceID int) (int, error) {
	return f.lastSuccessful[traceID], nil
}

func buildSimulationCandidates(weights []int) []*ChannelModelsCandidate {
	candidates := make([]*ChannelModelsCandidate, 0, len(weights))
	for i, w := range weights {
		id := i + 1
		candidates = append(candidates, &ChannelModelsCandidate{
			Channel:  &biz.Channel{Channel: &ent.Channel{ID: id, Name: fmt.Sprintf("ch-%d", id), OrderingWeight: w}},
			Priority: 0,
		})
	}

	return candidates
}

func TestAdaptiveLoadBalancer_Simulation_Healthy_DistributionByWeight(t *testing.T) {
	ctx := context.Background()
	weights := []int{80, 50, 20, 10}
	candidates := buildSimulationCandidates(weights)

	metrics := newFakeAdaptiveMetricsProvider(600)
	traceProvider := newFakeTraceProvider()
	connectionTracker := NewDefaultConnectionTracker(0)

	const totalRequests = 1000

	wrr := NewWeightRoundRobinStrategy(metrics)
	wrr.requestCountCap = int64(totalRequests) // Use large enough cap for simulation
	wrr.minScore = 0.0                         // Allow scores to drop below default floor for better distribution in simulation

	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(traceProvider),
		NewErrorAwareStrategy(metrics),
		wrr,
		NewConnectionAwareStrategy(nil, connectionTracker),
	}

	lb := NewLoadBalancer(&mockSystemService{retryPolicy: &biz.RetryPolicy{Enabled: false}}, nil, strategies...)

	const tickMs = int64(50)

	requestCounts := make(map[int]int)

	for range totalRequests {
		metrics.AdvanceMs(tickMs)

		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.Len(t, sorted, 1)

		picked := sorted[0].Channel.ID
		requestCounts[picked]++
		metrics.RecordSuccess(picked)
	}

	totalWeight := 0
	for _, w := range weights {
		totalWeight += w
	}

	for i, c := range candidates {
		channelID := c.Channel.ID
		actualPercent := float64(requestCounts[channelID]) / float64(totalRequests) * 100
		expectedPercent := float64(weights[i]) / float64(totalWeight) * 100
		require.InDelta(t, expectedPercent, actualPercent, 2.0)
	}
}

func TestAdaptiveLoadBalancer_Simulation_TraceStickyOverridesWeight(t *testing.T) {
	baseCtx := context.Background()
	trace := &ent.Trace{ID: 1}
	ctx := contexts.WithTrace(baseCtx, trace)

	weights := []int{80, 50, 20, 10}
	candidates := buildSimulationCandidates(weights)

	metrics := newFakeAdaptiveMetricsProvider(600)
	traceProvider := newFakeTraceProvider()
	traceProvider.lastSuccessful[trace.ID] = candidates[2].Channel.ID
	connectionTracker := NewDefaultConnectionTracker(0)

	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(traceProvider),
		NewErrorAwareStrategy(metrics),
		NewWeightRoundRobinStrategy(metrics),
		NewConnectionAwareStrategy(nil, connectionTracker),
	}

	lb := NewLoadBalancer(&mockSystemService{retryPolicy: &biz.RetryPolicy{Enabled: false}}, nil, strategies...)

	const tickMs = int64(50)
	for range 50 {
		metrics.AdvanceMs(tickMs)

		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.Len(t, sorted, 1)
		require.Equal(t, candidates[2].Channel.ID, sorted[0].Channel.ID)
		metrics.RecordSuccess(sorted[0].Channel.ID)
	}
}

func TestAdaptiveLoadBalancer_Simulation_ConnectionPressureCanOverrideWeight(t *testing.T) {
	ctx := context.Background()
	weights := []int{80, 50, 20, 10}
	candidates := buildSimulationCandidates(weights)

	metrics := newFakeAdaptiveMetricsProvider(600)
	traceProvider := newFakeTraceProvider()

	connectionTracker := NewDefaultConnectionTracker(10)
	for range 10 {
		connectionTracker.IncrementConnection(candidates[0].Channel.ID)
	}

	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(traceProvider),
		NewErrorAwareStrategy(metrics),
		NewWeightRoundRobinStrategy(metrics),
		NewConnectionAwareStrategy(nil, connectionTracker),
	}

	lb := NewLoadBalancer(&mockSystemService{retryPolicy: &biz.RetryPolicy{Enabled: false}}, nil, strategies...)

	metrics.AdvanceMs(50)

	sorted := lb.Sort(ctx, candidates, "gpt-4")
	require.Len(t, sorted, 1)
	require.NotEqual(t, candidates[0].Channel.ID, sorted[0].Channel.ID)
}

func TestAdaptiveLoadBalancer_Simulation_ErrorMigrationAndRecovery(t *testing.T) {
	ctx := context.Background()
	weights := []int{80, 50, 20, 10}
	candidates := buildSimulationCandidates(weights)

	metrics := newFakeAdaptiveMetricsProvider(600)
	traceProvider := newFakeTraceProvider()
	connectionTracker := NewDefaultConnectionTracker(0)

	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(traceProvider),
		NewErrorAwareStrategy(metrics),
		NewWeightRoundRobinStrategy(metrics),
		NewConnectionAwareStrategy(nil, connectionTracker),
	}

	lb := NewLoadBalancer(&mockSystemService{retryPolicy: &biz.RetryPolicy{Enabled: false}}, nil, strategies...)

	const (
		tickMs = int64(50)
		warmup = 2000
	)

	for range warmup {
		metrics.AdvanceMs(tickMs)

		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.Len(t, sorted, 1)
		metrics.RecordSuccess(sorted[0].Channel.ID)
	}

	failingID := candidates[0].Channel.ID

	// Record 3 failures
	// Penalty: 40 (base) + 3 * 30 (consecutive) = 130
	// ErrorAware Score: 200 - 130 = 70
	for range 3 {
		metrics.AdvanceMs(tickMs)
		metrics.RecordFailure(failingID)
	}

	// Verify it's not picked immediately after failure
	for range 200 {
		metrics.AdvanceMs(tickMs)

		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.Len(t, sorted, 1)
		require.NotEqual(t, failingID, sorted[0].Channel.ID)
		metrics.RecordSuccess(sorted[0].Channel.ID)
	}

	// Wait 2.5 minutes (half of 5 min cooldown)
	// cooldownRatio = 0.5
	// Penalty = 130 * 0.5 = 65
	// ErrorAware Score: 200 - 65 = 135
	// Healthy channels ErrorAware Score: 200
	// Even though it's recovering, healthy channels still have higher score.
	metrics.AdvanceMs(2*60*1000 + 30*1000)

	// Still should not be picked if other channels are healthy enough
	for range 50 {
		metrics.AdvanceMs(tickMs)

		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.NotEqual(t, failingID, sorted[0].Channel.ID)
		metrics.RecordSuccess(sorted[0].Channel.ID)
	}

	// Wait until 6 minutes passed (total > 5 min cooldown)
	metrics.AdvanceMs(3*60*1000 + 30*1000)
	metrics.RecordSuccess(failingID)

	found := false
	for range 2000 {
		metrics.AdvanceMs(tickMs)

		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.Len(t, sorted, 1)

		if sorted[0].Channel.ID == failingID {
			found = true
			break
		}

		metrics.RecordSuccess(sorted[0].Channel.ID)
	}

	require.True(t, found)
}

func TestAdaptiveLoadBalancer_Simulation_ErrorAware_DetailedDecay(t *testing.T) {
	ctx := context.Background()
	weights := []int{100, 100} // Equal weights for simplicity
	candidates := buildSimulationCandidates(weights)

	metrics := newFakeAdaptiveMetricsProvider(600)

	strategies := []LoadBalanceStrategy{
		NewErrorAwareStrategy(metrics),
		NewWeightRoundRobinStrategy(metrics),
	}

	lb := NewLoadBalancer(&mockSystemService{retryPolicy: &biz.RetryPolicy{Enabled: false}}, nil, strategies...)

	ch1 := candidates[0].Channel.ID
	ch2 := candidates[1].Channel.ID

	// 1. ch1 fails 2 times
	// Penalty: 40 + 2 * 30 = 100
	// ErrorAware Score: 200 - 100 = 100
	// WRR Score: 150
	// Total: 250
	// ch2 Total: 200 + 150 = 350
	for range 2 {
		metrics.RecordFailure(ch1)
	}

	// ch2 should be picked
	for range 10 {
		sorted := lb.Sort(ctx, candidates, "gpt-4")
		require.Equal(t, ch2, sorted[0].Channel.ID)
		metrics.RecordSuccess(ch2)
	}

	// 2. Advance 4 minutes (80% of 5 min cooldown)
	// cooldownRatio = 1.0 - (4/5) = 0.2
	// Penalty = 100 * 0.2 = 20
	// ErrorAware Score: 200 - 20 = 180
	metrics.AdvanceMs(4 * 60 * 1000)

	// ch2 has 10 requests.
	// WRR Score for ch2: 150 * exp(-10/150) = 150 * 0.935 = 140
	// ch2 Total: 200 + 140 = 340
	// ch1 Total: 180 + 150 = 330
	// ch2 should still be picked (barely)
	sorted := lb.Sort(ctx, candidates, "gpt-4")
	require.Equal(t, ch2, sorted[0].Channel.ID)

	// 3. Advance 1 more minute (total 5 minutes)
	// Penalty = 0
	// ErrorAware Score: 200
	metrics.AdvanceMs(1 * 60 * 1000)

	// ch1 Total: 200 + 150 = 350
	// ch2 Total: 200 + 140 = 340
	// ch1 should be picked now
	sorted = lb.Sort(ctx, candidates, "gpt-4")
	require.Equal(t, ch1, sorted[0].Channel.ID)
}

func TestAdaptiveLoadBalancer_Simulation_InactivityDecayAllowsComeback(t *testing.T) {
	ctx := context.Background()
	weights := []int{80, 50, 20, 10}
	candidates := buildSimulationCandidates(weights)

	metrics := newFakeAdaptiveMetricsProvider(600)
	traceProvider := newFakeTraceProvider()
	connectionTracker := NewDefaultConnectionTracker(0)

	strategies := []LoadBalanceStrategy{
		NewTraceAwareStrategy(traceProvider),
		NewErrorAwareStrategy(metrics),
		NewWeightRoundRobinStrategy(metrics),
		NewConnectionAwareStrategy(nil, connectionTracker),
	}

	lb := NewLoadBalancer(&mockSystemService{retryPolicy: &biz.RetryPolicy{Enabled: false}}, nil, strategies...)

	heavyID := candidates[0].Channel.ID
	for range 2000 {
		metrics.RecordSuccess(heavyID)
	}

	metrics.AdvanceMs(50)

	sorted := lb.Sort(ctx, candidates, "gpt-4")
	require.Len(t, sorted, 1)
	require.NotEqual(t, heavyID, sorted[0].Channel.ID)

	metrics.AdvanceMs(30 * 60 * 1000)

	sorted2 := lb.Sort(ctx, candidates, "gpt-4")
	require.Len(t, sorted2, 1)
	require.Equal(t, heavyID, sorted2[0].Channel.ID)
}
