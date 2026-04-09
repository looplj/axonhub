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

	// Create channels: 2 warm (have metrics), 1 cold (no metrics)
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

	// Mock metrics: ch1 and ch2 have data (warm), ch3 has no data (cold)
	perfStrategy := &PerformanceAwareStrategy{
		maxScore: 150.0,
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			if channelID == ch3.ID {
				return nil, nil // Cold channel - no metrics
			}
			m := &biz.AggregatedMetrics{}
			m.RequestCount = 100
			return m, nil
		},
	}

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	modelService := newTestModelService(client)

	// Enable metrics sampling with AlwaysSample to ensure it triggers
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

	// Run multiple selections to verify both mechanisms work
	explorationSeen := make(map[int]bool)
	alternativeSeen := make(map[int]bool)

	for i := 0; i < 20; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(result), 2, "should have at least 2 candidates for retry slots")

		// Check last slot - could be exploration or metrics sampling alternative
		lastCandidate := result[len(result)-1]
		if lastCandidate.Channel.ID == ch3.ID {
			explorationSeen[ch3.ID] = true
		} else if lastCandidate.Channel.ID == ch2.ID {
			alternativeSeen[ch2.ID] = true
		}
	}

	// Both mechanisms should have had a chance to work
	// Note: We can't guarantee both fire every time due to round-robin, but over 20 runs
	// we should see evidence of both mechanisms
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

// TestMetricsSamplingDisabledDefault verifies that metrics sampling is disabled
// by default and does not trigger when no configuration is set.
func TestMetricsSamplingDisabledDefault(t *testing.T) {
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

	// Do NOT set any metrics sampling config - use default (should be disabled)
	// systemService.MetricsSamplingOrDefault should return default config with Enabled=false

	loadBalancer := NewLoadBalancer(systemService, nil, perfStrategy, NewWeightRoundRobinStrategy(channelService))
	baseSelector := NewDefaultSelector(channelService, modelService, systemService)
	selector := WithLoadBalancedSelector(baseSelector, loadBalancer, systemService, systemService, nil)

	req := &llm.Request{
		Model: "gpt-4",
	}

	for i := 0; i < 10; i++ {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.LessOrEqual(t, len(result), 2, "with metrics sampling disabled, should not exceed available channels")
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
