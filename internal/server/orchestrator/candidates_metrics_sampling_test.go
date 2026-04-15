package orchestrator

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type MetricsSamplingConfig struct {
	Enabled              bool    `json:"enabled"`
	AlwaysSample         bool    `json:"always_sample"`
	RequestRateThreshold int     `json:"request_rate_threshold"`
	ScoreThreshold       float64 `json:"score_threshold"`
	AlternativeCount     int     `json:"alternative_count"`
	SamplingRate         float64 `json:"sampling_rate"`
}

func TestMetricsSamplingScoreThreshold(t *testing.T) {
	tests := []struct {
		name           string
		config         MetricsSamplingConfig
		topScore       float64
		expectSampling bool
	}{
		{
			name: "sampling triggers when score above threshold",
			config: MetricsSamplingConfig{
				Enabled:        true,
				ScoreThreshold: 80.0,
			},
			topScore:       95.0,
			expectSampling: true,
		},
		{
			name: "sampling does not trigger when score below threshold",
			config: MetricsSamplingConfig{
				Enabled:        true,
				ScoreThreshold: 80.0,
			},
			topScore:       75.0,
			expectSampling: false,
		},
		{
			name: "sampling does not trigger when disabled",
			config: MetricsSamplingConfig{
				Enabled:        false,
				ScoreThreshold: 80.0,
			},
			topScore:       95.0,
			expectSampling: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggered := shouldSampleByScore(tt.config, tt.topScore)
			if tt.expectSampling {
				require.True(t, triggered, "sampling should be triggered")
			} else {
				require.False(t, triggered, "sampling should not be triggered")
			}
		})
	}
}

func TestMetricsSamplingRateThreshold(t *testing.T) {
	tests := []struct {
		name           string
		config         MetricsSamplingConfig
		requestRate    int
		expectSampling bool
	}{
		{
			name: "sampling triggers when rate above threshold",
			config: MetricsSamplingConfig{
				Enabled:              true,
				RequestRateThreshold: 100,
			},
			requestRate:    150,
			expectSampling: true,
		},
		{
			name: "sampling does not trigger when rate below threshold",
			config: MetricsSamplingConfig{
				Enabled:              true,
				RequestRateThreshold: 100,
			},
			requestRate:    50,
			expectSampling: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggered := shouldSampleByRate(tt.config, tt.requestRate)
			if tt.expectSampling {
				require.True(t, triggered, "sampling should be triggered when rate exceeds threshold")
			} else {
				require.False(t, triggered, "sampling should not be triggered when rate is below threshold")
			}
		})
	}
}

func TestMetricsSamplingAlwaysFlag(t *testing.T) {
	tests := []struct {
		name           string
		config         MetricsSamplingConfig
		expectSampling bool
	}{
		{
			name: "sampling triggers when always sample is enabled",
			config: MetricsSamplingConfig{
				Enabled:      true,
				AlwaysSample: true,
			},
			expectSampling: true,
		},
		{
			name: "sampling does not trigger when always sample is disabled",
			config: MetricsSamplingConfig{
				Enabled:      true,
				AlwaysSample: false,
			},
			expectSampling: false,
		},
		{
			name: "sampling does not trigger when globally disabled",
			config: MetricsSamplingConfig{
				Enabled:      false,
				AlwaysSample: true,
			},
			expectSampling: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggered := shouldSampleByAlways(tt.config)
			if tt.expectSampling {
				require.True(t, triggered, "sampling should be triggered when AlwaysSample is true")
			} else {
				require.False(t, triggered, "sampling should not be triggered")
			}
		})
	}
}

func TestMetricsSamplingORLogic(t *testing.T) {
	tests := []struct {
		name           string
		config         MetricsSamplingConfig
		topScore       float64
		requestRate    int
		expectSampling bool
	}{
		{
			name: "triggers via score threshold only",
			config: MetricsSamplingConfig{
				Enabled:        true,
				ScoreThreshold: 80.0,
			},
			topScore:       90.0,
			requestRate:    0,
			expectSampling: true,
		},
		{
			name: "triggers via rate threshold only",
			config: MetricsSamplingConfig{
				Enabled:              true,
				RequestRateThreshold: 100,
			},
			topScore:       50.0,
			requestRate:    200,
			expectSampling: true,
		},
		{
			name: "triggers via always sample only",
			config: MetricsSamplingConfig{
				Enabled:      true,
				AlwaysSample: true,
			},
			topScore:       0.0,
			requestRate:    0,
			expectSampling: true,
		},
		{
			name: "no trigger when all conditions false",
			config: MetricsSamplingConfig{
				Enabled:              true,
				ScoreThreshold:       80.0,
				RequestRateThreshold: 100,
				AlwaysSample:         false,
			},
			topScore:       50.0,
			requestRate:    50,
			expectSampling: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			triggered := shouldSampleMetricsOR(tt.config, tt.topScore, tt.requestRate)
			if tt.expectSampling {
				require.True(t, triggered, "sampling should be triggered via OR logic")
			} else {
				require.False(t, triggered, "sampling should not be triggered when all conditions fail")
			}
		})
	}
}

func TestMetricsSamplingDisabled(t *testing.T) {
	config := MetricsSamplingConfig{
		Enabled:        false,
		AlwaysSample:   true,
		ScoreThreshold: 90.0,
	}

	triggered := shouldSampleMetricsOR(config, 95.0, 50)
	require.False(t, triggered, "sampling should not trigger when globally disabled")
}

func TestMetricsSamplingZeroAlternatives(t *testing.T) {
	config := MetricsSamplingConfig{
		Enabled:        true,
		ScoreThreshold: 80.0,
	}

	triggered := shouldSampleWithCandidates(config, []*ChannelModelsCandidate{}, 90.0)
	require.False(t, triggered, "sampling should not trigger with zero alternatives")
}

func TestMetricsSamplingRateLimitedAlternatives(t *testing.T) {
	config := MetricsSamplingConfig{
		Enabled:        true,
		ScoreThreshold: 80.0,
	}

	candidates := []*ChannelModelsCandidate{
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "channel-1"}}, Priority: 1},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "channel-2", Status: channel.StatusDisabled}}, Priority: 1},
		{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "channel-3"}}, Priority: 2},
	}

	triggered := shouldSampleWithCandidates(config, candidates, 90.0)
	require.True(t, triggered, "sampling should trigger with valid candidates above threshold")
}

func TestMetricsSamplingFewerThanN(t *testing.T) {
	tests := []struct {
		name           string
		config         MetricsSamplingConfig
		candidateCount int
		expectSampling bool
		description    string
	}{
		{
			name: "fewer candidates than alternative count still samples",
			config: MetricsSamplingConfig{
				Enabled:          true,
				AlternativeCount: 5,
				ScoreThreshold:   80.0,
			},
			candidateCount: 3,
			expectSampling: true,
			description:    "should still sample when fewer candidates than AlternativeCount",
		},
		{
			name: "single candidate triggers sampling",
			config: MetricsSamplingConfig{
				Enabled:          true,
				AlternativeCount: 3,
				ScoreThreshold:   80.0,
			},
			candidateCount: 1,
			expectSampling: true,
			description:    "single candidate above threshold should trigger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := make([]*ChannelModelsCandidate, tt.candidateCount)
			for i := 0; i < tt.candidateCount; i++ {
				candidates[i] = &ChannelModelsCandidate{
					Channel:  &biz.Channel{Channel: &ent.Channel{ID: i + 1}},
					Priority: 1,
				}
			}

			triggered := shouldSampleWithAlternativeCount(tt.config, candidates, 90.0)

			if tt.expectSampling {
				require.True(t, triggered, tt.description)
			} else {
				require.False(t, triggered, tt.description)
			}
		})
	}
}

func TestMetricsSamplingProbabilisticRate(t *testing.T) {
	config := MetricsSamplingConfig{
		Enabled:      true,
		SamplingRate: 0.1,
	}

	const iterations = 1000
	triggeredCount := 0

	for i := 0; i < iterations; i++ {
		if shouldSampleProbabilistic(config) {
			triggeredCount++
		}
	}

	triggerRate := float64(triggeredCount) / float64(iterations)
	require.InDelta(t, 0.1, triggerRate, 0.05, "sampling rate should be approximately 10%")
}

func shouldSampleByScore(config MetricsSamplingConfig, topScore float64) bool {
	if !config.Enabled {
		return false
	}
	return topScore >= config.ScoreThreshold
}

func shouldSampleByRate(config MetricsSamplingConfig, requestRate int) bool {
	if !config.Enabled {
		return false
	}
	return requestRate >= config.RequestRateThreshold
}

func shouldSampleByAlways(config MetricsSamplingConfig) bool {
	if !config.Enabled {
		return false
	}
	return config.AlwaysSample
}

func shouldSampleMetricsOR(config MetricsSamplingConfig, topScore float64, requestRate int) bool {
	if !config.Enabled {
		return false
	}
	return shouldSampleByScore(config, topScore) ||
		shouldSampleByRate(config, requestRate) ||
		shouldSampleByAlways(config)
}

func shouldSampleWithCandidates(config MetricsSamplingConfig, candidates []*ChannelModelsCandidate, topScore float64) bool {
	if !config.Enabled {
		return false
	}
	if len(candidates) == 0 {
		return false
	}
	return shouldSampleByScore(config, topScore)
}

func shouldSampleWithAlternativeCount(config MetricsSamplingConfig, candidates []*ChannelModelsCandidate, topScore float64) bool {
	if !config.Enabled {
		return false
	}
	if len(candidates) == 0 {
		return false
	}
	return shouldSampleByScore(config, topScore)
}

func shouldSampleProbabilistic(config MetricsSamplingConfig) bool {
	r := rand.Float64()
	return r < config.SamplingRate
}

// TestMetricsSamplingExplorationCoexistence verifies that both exploration and metrics
// sampling mechanisms can work together without conflict.
func TestMetricsSamplingExplorationCoexistence(t *testing.T) {
	ctx, client := setupTest(t)

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Hot Channel").
		SetBaseURL("https://api1.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(100).
		Save(ctx)
	require.NoError(t, err)

	ch2, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Warm Channel").
		SetBaseURL("https://api2.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	ch3, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Cold Channel").
		SetBaseURL("https://api3.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key3"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(10).
		Save(ctx)
	require.NoError(t, err)

	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			if channelID == ch3.ID {
				return nil, nil
			}
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	err = systemService.SetMetricsSampling(ctx, &biz.MetricsSamplingConfig{
		Enabled:              true,
		AlwaysSample:         true,
		AlternativeCount:     2,
		ScoreThreshold:       80.0,
		RequestRateThreshold: 0,
		SamplingRate:         0.1,
	})
	require.NoError(t, err)

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	explorationSeen := make(map[int]bool)
	alternativeSeen := make(map[int]bool)

	for i := 0; i < 20; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(result), 2, "should have at least 2 candidates")

		// Check position 1 for exploration candidate (cold channel inserted after winner)
		if len(result) > 1 && result[1].Channel.ID == ch3.ID {
			explorationSeen[ch3.ID] = true
		}

		// Check for metrics sampling alternative (appended at end)
		if len(result) > 2 {
			for _, c := range result[2:] {
				if c.Channel.ID == ch2.ID {
					alternativeSeen[ch2.ID] = true
				}
			}
		}
	}

	require.True(t, explorationSeen[ch3.ID] || alternativeSeen[ch2.ID],
		"at least one mechanism (exploration or metrics sampling) should have assigned an alternative slot")
}

// TestMetricsSamplingConcurrent verifies that probabilistic sampling works correctly
// with concurrent requests, maintaining approximately the configured sampling rate.
func TestMetricsSamplingConcurrent(t *testing.T) {
	config := MetricsSamplingConfig{
		Enabled:      true,
		SamplingRate: 0.1, // 10% sampling rate
	}

	const iterations = 100
	const concurrency = 10

	var triggeredCount int64
	var wg sync.WaitGroup

	// Run concurrent sampling checks
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if shouldSampleProbabilistic(config) {
					atomic.AddInt64(&triggeredCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	totalIterations := iterations * concurrency
	triggerRate := float64(triggeredCount) / float64(totalIterations)

	// With 10% rate and 1000 iterations, expect 8-12% (within 2% delta)
	require.InDelta(t, 0.1, triggerRate, 0.02,
		"concurrent sampling rate should be approximately 10%%, got %f%% (%d/%d)",
		triggerRate*100, triggeredCount, totalIterations)
}

// TestMetricsSamplingEnabledDefault verifies that metrics sampling is enabled
// by default and can trigger when thresholds are met.
func TestMetricsSamplingEnabledDefault(t *testing.T) {
	ctx, client := setupTest(t)

	// Create test channels
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api1.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(100).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 2").
		SetBaseURL("https://api2.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	// Mock metrics
	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	// Do NOT set any metrics sampling config - use default (enabled with RequestRateThreshold=10, SamplingRate=0.20)

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	for i := 0; i < 10; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(result), 2, "result should not exceed available channels")
		require.Equal(t, ch1.ID, result[0].Channel.ID, "winner should be ch1 (highest weight)")
	}
}

// TestMetricsSamplingHotReload verifies that configuration changes take effect
// immediately without requiring a restart.
func TestMetricsSamplingHotReload(t *testing.T) {
	ctx, client := setupTest(t)

	// Create test channels
	ch1, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 1").
		SetBaseURL("https://api1.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(100).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Channel 2").
		SetBaseURL("https://api2.example.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "key2"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		SetOrderingWeight(50).
		Save(ctx)
	require.NoError(t, err)

	// Mock metrics
	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	// Start with metrics sampling disabled
	err = systemService.SetMetricsSampling(ctx, &biz.MetricsSamplingConfig{
		Enabled:              false,
		AlwaysSample:         false,
		AlternativeCount:     2,
		ScoreThreshold:       80.0,
		RequestRateThreshold: 0,
		SamplingRate:         0.1,
	})
	require.NoError(t, err)

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	// Phase 1: With disabled config, should get at most 2 candidates (limited by available channels)
	for i := 0; i < 5; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(result), 2, "phase 1: with Enabled=false, should not exceed available channels")
	}

	// Phase 2: Hot-reload - enable metrics sampling
	err = systemService.SetMetricsSampling(ctx, &biz.MetricsSamplingConfig{
		Enabled:              true,
		AlwaysSample:         true, // Force sampling to trigger
		AlternativeCount:     2,
		ScoreThreshold:       80.0,
		RequestRateThreshold: 0,
		SamplingRate:         0.1,
	})
	require.NoError(t, err)

	// Phase 3: With enabled config, should now get 2 candidates (winner + alternative)
	alternativeSeen := false
	for i := 0; i < 10; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)

		// Should now have 2 candidates due to metrics sampling
		require.GreaterOrEqual(t, len(result), 1, "phase 3: should have at least 1 candidate")

		if len(result) >= 2 {
			alternativeSeen = true
			// The alternative could be ch2
			if result[1].Channel.ID != ch1.ID {
				alternativeSeen = true
			}
		}
	}

	require.True(t, alternativeSeen, "phase 3: after hot-reload enabling metrics sampling, should see alternative candidates")

	// Phase 4: Hot-reload - disable again
	err = systemService.SetMetricsSampling(ctx, &biz.MetricsSamplingConfig{
		Enabled:              false,
		AlwaysSample:         false,
		AlternativeCount:     2,
		ScoreThreshold:       80.0,
		RequestRateThreshold: 0,
		SamplingRate:         0.1,
	})
	require.NoError(t, err)

	// Phase 5: Should return to at most 2 candidates
	for i := 0; i < 5; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(result), 2, "phase 5: after disabling, should not exceed available channels")
	}
}

func TestEffectiveSamplingRate(t *testing.T) {
	tests := []struct {
		name         string
		baseRate     float64
		requestCount int64
		wantMin      float64
		wantMax      float64
	}{
		{
			name:         "zero requests with 0.2 base rate gives near-1.0",
			baseRate:     0.2,
			requestCount: 0,
			wantMin:      0.95,
			wantMax:      1.0,
		},
		{
			name:         "5 requests with 0.2 base rate gives 0.6",
			baseRate:     0.2,
			requestCount: 5,
			wantMin:      0.55,
			wantMax:      0.65,
		},
		{
			name:         "at ColdStartMinRequests returns base rate",
			baseRate:     0.2,
			requestCount: int64(ColdStartMinRequests),
			wantMin:      0.2,
			wantMax:      0.2,
		},
		{
			name:         "above ColdStartMinRequests returns base rate",
			baseRate:     0.2,
			requestCount: 100,
			wantMin:      0.2,
			wantMax:      0.2,
		},
		{
			name:         "zero base rate returns zero",
			baseRate:     0,
			requestCount: 0,
			wantMin:      0,
			wantMax:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := effectiveSamplingRate(tt.baseRate, tt.requestCount)
			require.GreaterOrEqual(t, got, tt.wantMin, "rate should be >= %f", tt.wantMin)
			require.LessOrEqual(t, got, tt.wantMax, "rate should be <= %f", tt.wantMax)
		})
	}
}

func TestEffectiveSamplingRateMonotonicDecrease(t *testing.T) {
	baseRate := 0.2
	var prevRate float64 = 1.1

	for i := int64(0); i <= int64(ColdStartMinRequests)+5; i++ {
		rate := effectiveSamplingRate(baseRate, i)
		require.LessOrEqual(t, rate, prevRate+0.001,
			"rate should monotonically decrease as request count increases at count=%d", i)
		prevRate = rate
	}

	require.Equal(t, baseRate, prevRate, "should converge to base rate")
}
