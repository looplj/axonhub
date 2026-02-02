package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestRoundRobinStrategy_Name(t *testing.T) {
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
	strategy := NewRoundRobinStrategy(mockProvider)
	assert.Equal(t, "RoundRobin", strategy.Name())
}

func TestRoundRobinStrategy_Score_ZeroRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with zero requests
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 0
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "new-channel"},
	}

	score := strategy.Score(ctx, channel)
	assert.Equal(t, 150.0, score, "New channels with zero requests should get max score")
}

func TestRoundRobinStrategy_Score_LowRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with low request count (10 requests)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 10
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "low-usage"},
	}

	score := strategy.Score(ctx, channel)
	// Should be high but less than maxScore
	assert.Greater(t, score, 100.0, "Low request channels should get high scores")
	assert.Less(t, score, 150.0, "Score should be less than maxScore")
}

func TestRoundRobinStrategy_Score_ModerateRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with moderate request count (100 requests)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 100
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "moderate-usage"},
	}

	score := strategy.Score(ctx, channel)
	// With exponential decay (scaling factor 150), 100 requests scores ~77.0
	// This provides good differentiation while keeping 500 requests from hitting minimum too early
	assert.Greater(t, score, 70.0, "Moderate usage channels should get moderate-high scores")
	assert.Less(t, score, 80.0, "Score should reflect moderate usage")
}

func TestRoundRobinStrategy_Score_HighRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with high request count (500 requests)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 500
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "high-usage"},
	}

	score := strategy.Score(ctx, channel)
	// With 500 requests, calculated score is ~5.35 which clamps to minScore (10.0)
	// This is expected behavior - heavily used channels get minimum priority
	assert.Equal(t, 10.0, score, "High usage channels should get minimum score when they exceed the decay curve")
}

func TestRoundRobinStrategy_Score_InactivityDecay(t *testing.T) {
	ctx := context.Background()

	activeTime := time.Now()
	// With 5 minute decay, use 10 minutes idle to see significant decay effect
	idleTime := time.Now().Add(-10 * time.Minute)

	activeMetrics := &biz.AggregatedMetrics{}
	activeMetrics.RequestCount = 500
	activeMetrics.LastSelectedAt = &activeTime

	idleMetrics := &biz.AggregatedMetrics{}
	idleMetrics.RequestCount = 500
	idleMetrics.LastSelectedAt = &idleTime

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: activeMetrics,
			2: idleMetrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	activeChannel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "recently-active"},
	}
	idleChannel := &biz.Channel{
		Channel: &ent.Channel{ID: 2, Name: "idle"},
	}

	activeScore := strategy.Score(ctx, activeChannel)
	idleScore := strategy.Score(ctx, idleChannel)

	// With 500 requests and no decay, score is ~5.35 (clamped to 10)
	// With 10 min idle (decay factor ~0.135), effective count ~67.5, score ~96
	assert.Less(t, activeScore, 20.0, "Recently active channel should stay near the lower score bound")
	assert.Greater(t, idleScore, 80.0, "Idle channel should regain score despite historical load")
	assert.Greater(t, idleScore, activeScore, "Idle channel should outrank recently active channel")
}

func TestRoundRobinStrategy_Score_CappedRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with requests exceeding the cap (2000 requests, cap is 1000)
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 2000
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "very-high-usage"},
	}

	score := strategy.Score(ctx, channel)
	// Should be at or near minimum score
	assert.GreaterOrEqual(t, score, 10.0, "Score should not go below minScore")
	assert.LessOrEqual(t, score, 20.0, "Very high usage should result in very low score")
}

func TestRoundRobinStrategy_Score_MetricsError(t *testing.T) {
	ctx := context.Background()

	// Mock provider that returns error
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
		err:     assert.AnError,
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 999, Name: "error-channel"},
	}

	score := strategy.Score(ctx, channel)
	// Should return moderate score (max + min) / 2 = (150 + 10) / 2 = 80
	assert.Equal(t, 80.0, score, "Should return moderate score when metrics unavailable")
}

func TestRoundRobinStrategy_MultipleChannels(t *testing.T) {
	ctx := context.Background()

	// Create metrics for multiple channels
	metrics1 := &biz.AggregatedMetrics{}
	metrics1.RequestCount = 0
	metrics2 := &biz.AggregatedMetrics{}
	metrics2.RequestCount = 50
	metrics3 := &biz.AggregatedMetrics{}
	metrics3.RequestCount = 200
	metrics4 := &biz.AggregatedMetrics{}
	metrics4.RequestCount = 800

	// Multiple channels with different request counts
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics1, // New channel
			2: metrics2, // Low usage
			3: metrics3, // Moderate usage
			4: metrics4, // High usage
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channels := []struct {
		id   int
		name string
	}{
		{1, "channel-new"},
		{2, "channel-low"},
		{3, "channel-moderate"},
		{4, "channel-high"},
	}

	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{
			Channel: &ent.Channel{ID: ch.id, Name: ch.name},
		}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Verify ordering: new > low > moderate > high
	assert.Greater(t, scores[0], scores[1], "New channel should outrank low usage channel")
	assert.Greater(t, scores[1], scores[2], "Low usage channel should outrank moderate usage channel")
	assert.Greater(t, scores[2], scores[3], "Moderate usage channel should outrank high usage channel")

	// Verify specific values
	assert.Equal(t, 150.0, scores[0], "New channel should get max score")
	assert.Equal(t, 10.0, scores[3], "Very high usage channels should get minimum score")
}

func TestRoundRobinStrategy_ScoreWithDebug(t *testing.T) {
	ctx := context.Background()

	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 100
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}

	score, strategyScore := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, "RoundRobin", strategyScore.StrategyName)
	assert.Greater(t, score, 0.0)
	assert.NotNil(t, strategyScore.Details)
	assert.Contains(t, strategyScore.Details, "request_count")
	assert.Contains(t, strategyScore.Details, "max_score")
	assert.Contains(t, strategyScore.Details, "calculated_score")
}

func TestRoundRobinStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		requestCount int64
	}{
		{"zero requests", 0},
		{"low requests", 10},
		{"moderate requests", 100},
		{"high requests", 500},
		{"capped requests", 1500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tc.requestCount

			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{ID: 1, Name: "test"},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for request_count=%d", tc.requestCount)
		})
	}
}

func TestRoundRobinStrategy_WithRealDatabase(t *testing.T) {
	ctx := context.Background()
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	// Create multiple channels
	channels := make([]*ent.Channel, 4)

	for i := range 4 {
		ch, err := client.Channel.Create().
			SetName(fmt.Sprintf("channel-%d", i)).
			SetType("openai").
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			Save(ctx)
		require.NoError(t, err)

		channels[i] = ch
	}

	channelService := newTestChannelService(client)

	// Record different numbers of requests for each channel
	// IncrementChannelSelection is called at selection time in production,
	// which increments aggregatedMetrics.RequestCount. RecordPerformance
	// only increments slot.RequestCount (for sliding window) to avoid double counting.
	requestCounts := []int64{0, 50, 200, 800}
	for i, ch := range channels {
		for j := int64(0); j < requestCounts[i]; j++ {
			// Simulate selection time increment (done by load balancer)
			channelService.IncrementChannelSelection(ch.ID)

			perf := &biz.PerformanceRecord{
				ChannelID:        ch.ID,
				StartTime:        time.Now().Add(-time.Minute),
				EndTime:          time.Now(),
				Success:          true,
				RequestCompleted: true,
			}
			channelService.RecordPerformance(ctx, perf)
		}
	}

	strategy := NewRoundRobinStrategy(channelService)

	// Score all channels
	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{Channel: ch}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Verify ordering based on request counts
	assert.Equal(t, 150.0, scores[0], "Channel with 0 requests should get max score")
	assert.Greater(t, scores[0], scores[1], "Lower request count should have higher score")
	assert.Greater(t, scores[1], scores[2], "Score should decrease with request count")
	assert.Greater(t, scores[2], scores[3], "Highest request count should have lowest score")
}

func TestWeightRoundRobinStrategy_Name(t *testing.T) {
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)
	assert.Equal(t, "WeightRoundRobin", strategy.Name())
}

func TestWeightRoundRobinStrategy_Score_ZeroRequests(t *testing.T) {
	ctx := context.Background()

	// With zero requests, all channels get max score (150) regardless of weight
	// because normalized request count is 0 for all weights
	testCases := []struct {
		name   string
		weight int
	}{
		{"zero weight (treated as very low)", 0},
		{"low weight", 25},
		{"medium weight", 50},
		{"high weight", 100},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = 0
			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewWeightRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "new-channel",
					OrderingWeight: tt.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			assert.Equal(t, 150.0, score, "Zero requests should always give max score")
		})
	}
}

func TestWeightRoundRobinStrategy_Score_ModerateRequests(t *testing.T) {
	ctx := context.Background()

	// With weighted round-robin, higher weight channels need more requests to get the same penalty
	// normalizedCount = requestCount / (weight / 100)
	// score = 150 * exp(-normalizedCount / 150)
	// When weight=0, weightFactor=1.0 (standard round-robin behavior)
	testCases := []struct {
		name             string
		requestCount     int64
		weight           int
		expectedMinScore float64
		expectedMaxScore float64
	}{
		// weight=0 -> weightFactor=1.0, 100 requests -> normalized=100, score ~= 77.0
		{"100 requests, no weight", 100, 0, 70.0, 85.0},
		// weight=25, 100 requests -> normalized=400, score ~= 10.4 -> clamped to 10
		{"100 requests, low weight", 100, 25, 10.0, 11.0},
		// weight=50, 100 requests -> normalized=200, score ~= 39.6
		{"100 requests, medium weight", 100, 50, 35.0, 45.0},
		// weight=100, 100 requests -> normalized=100, score ~= 77.0
		{"100 requests, high weight", 100, 100, 70.0, 85.0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tt.requestCount
			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewWeightRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tt.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			assert.GreaterOrEqual(t, score, tt.expectedMinScore, "Score should be at least expected minimum")
			assert.LessOrEqual(t, score, tt.expectedMaxScore, "Score should not exceed expected maximum")
		})
	}
}

func TestWeightRoundRobinStrategy_Score_HighRequests(t *testing.T) {
	ctx := context.Background()

	// Channel with high request count (500 requests), medium weight (50)
	// normalizedCount = 500 / 0.5 = 1000
	// score = 150 * exp(-1000/150) = ~0.18 -> clamped to 10
	metrics := &biz.AggregatedMetrics{}
	metrics.RequestCount = 500
	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics,
		},
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             1,
			Name:           "high-usage",
			OrderingWeight: 50,
		},
	}

	score := strategy.Score(ctx, channel)
	// With weight=50 and 500 requests, normalized_count=1000
	// score should be at minimum (~10, with small offset from minScore clamping)
	assert.InDelta(t, 10.0, score, 0.1, "High usage with medium weight should hit minimum score")
}

func TestWeightRoundRobinStrategy_Score_MetricsError(t *testing.T) {
	ctx := context.Background()

	// Mock provider that returns error
	mockProvider := &mockMetricsProvider{
		metrics: make(map[int]*biz.AggregatedMetrics),
		err:     assert.AnError,
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	channel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             999,
			Name:           "error-channel",
			OrderingWeight: 25,
		},
	}

	score := strategy.Score(ctx, channel)
	// Should return moderate score (maxScore+minScore)/2 = (150+10)/2 = 80
	assert.Equal(t, 80.0, score, "Should return moderate score when metrics unavailable")
}

func TestWeightRoundRobinStrategy_MultipleChannels(t *testing.T) {
	ctx := context.Background()

	// Test weighted round-robin distribution
	// Channels should be selected proportionally to their weights
	metrics1 := &biz.AggregatedMetrics{}
	metrics1.RequestCount = 0 // New channel
	metrics2 := &biz.AggregatedMetrics{}
	metrics2.RequestCount = 80 // 80 requests with weight 80 -> normalized=100
	metrics3 := &biz.AggregatedMetrics{}
	metrics3.RequestCount = 50 // 50 requests with weight 50 -> normalized=100
	metrics4 := &biz.AggregatedMetrics{}
	metrics4.RequestCount = 10 // 10 requests with weight 10 -> normalized=100

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: metrics1,
			2: metrics2,
			3: metrics3,
			4: metrics4,
		},
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	channels := []struct {
		id     int
		name   string
		weight int
	}{
		{1, "channel-new", 50}, // New, 0 requests
		{2, "channel-w80", 80}, // 80 requests, weight 80
		{3, "channel-w50", 50}, // 50 requests, weight 50
		{4, "channel-w10", 10}, // 10 requests, weight 10
	}

	scores := make([]float64, len(channels))
	for i, ch := range channels {
		channel := &biz.Channel{
			Channel: &ent.Channel{
				ID:             ch.id,
				Name:           ch.name,
				OrderingWeight: ch.weight,
			},
		}
		scores[i] = strategy.Score(ctx, channel)
	}

	// Channel 1 (new) should have highest score
	assert.Equal(t, 150.0, scores[0], "New channel should get max score")

	// Channels 2, 3, 4 all have normalized_count=100, so they should have approximately equal scores
	// score = 150 * exp(-100/150) = ~77.0
	assert.InDelta(t, scores[1], scores[2], 1.0, "Channels with proportional requests should have similar scores")
	assert.InDelta(t, scores[2], scores[3], 1.0, "Channels with proportional requests should have similar scores")
}

func TestWeightRoundRobinStrategy_ScoreConsistency(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name         string
		requestCount int64
		weight       int
	}{
		{"zero requests, zero weight", 0, 0},
		{"zero requests, low weight", 0, 25},
		{"zero requests, high weight", 0, 100},
		{"low requests, low weight", 10, 25},
		{"moderate requests, medium weight", 100, 50},
		{"high requests, high weight", 500, 100},
		{"capped requests, medium weight", 1500, 50},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := &biz.AggregatedMetrics{}
			metrics.RequestCount = tc.requestCount

			mockProvider := &mockMetricsProvider{
				metrics: map[int]*biz.AggregatedMetrics{
					1: metrics,
				},
			}
			strategy := NewWeightRoundRobinStrategy(mockProvider)

			channel := &biz.Channel{
				Channel: &ent.Channel{
					ID:             1,
					Name:           "test",
					OrderingWeight: tc.weight,
				},
			}

			score := strategy.Score(ctx, channel)
			debugScore, _ := strategy.ScoreWithDebug(ctx, channel)

			assert.Equal(t, score, debugScore,
				"Score and ScoreWithDebug must return identical scores for %s", tc.name)
		})
	}
}

func TestWeightRoundRobinStrategy_Score_InactivityDecay(t *testing.T) {
	ctx := context.Background()

	activeTime := time.Now()
	// With 5 minute decay, use 10 minutes idle to see significant decay effect
	idleTime := time.Now().Add(-10 * time.Minute)

	activeMetrics := &biz.AggregatedMetrics{}
	activeMetrics.RequestCount = 400
	activeMetrics.LastSelectedAt = &activeTime

	idleMetrics := &biz.AggregatedMetrics{}
	idleMetrics.RequestCount = 400
	idleMetrics.LastSelectedAt = &idleTime

	mockProvider := &mockMetricsProvider{
		metrics: map[int]*biz.AggregatedMetrics{
			1: activeMetrics,
			2: idleMetrics,
		},
	}
	strategy := NewWeightRoundRobinStrategy(mockProvider)

	// Using weight=100 so normalized_count = effective_count
	activeChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             1,
			Name:           "recent",
			OrderingWeight: 100,
		},
	}
	idleChannel := &biz.Channel{
		Channel: &ent.Channel{
			ID:             2,
			Name:           "idle",
			OrderingWeight: 100,
		},
	}

	activeScore := strategy.Score(ctx, activeChannel)
	idleScore := strategy.Score(ctx, idleChannel)

	// With weight=100, normalized_count = effective_count
	// Active: 400 requests, no decay -> normalized=400, score ~= 10.2
	// Idle: 400 requests, 10min decay (factor ~0.135) -> effective ~54, normalized=54, score ~= 105
	assert.Less(t, activeScore, 20.0, "Recently active channel should have low score")
	assert.Greater(t, idleScore, 80.0, "Idle channel should recover score with decay")
	assert.Greater(t, idleScore, activeScore, "Idle channel should outrank recently active channel")
}
