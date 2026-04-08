package orchestrator

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/stretchr/testify/require"
)

func TestPerformanceAwareStrategyInterface(t *testing.T) {
	var _ LoadBalanceStrategy = (*PerformanceAwareStrategy)(nil)

	strategy := &PerformanceAwareStrategy{maxScore: 150.0}
	if strategy.Name() != "performance-aware" {
		t.Errorf("Name() = %q, want %q", strategy.Name(), "performance-aware")
	}

	channel := &biz.Channel{
		Channel: &ent.Channel{ID: 1, Name: "test"},
	}
	score := strategy.Score(context.Background(), channel)
	if score != 0.0 {
		t.Errorf("Score() = %f, want %f", score, 0.0)
	}

	score, strategyScore := strategy.ScoreWithDebug(context.Background(), channel)
	if score != 0.0 {
		t.Errorf("ScoreWithDebug score = %f, want %f", score, 0.0)
	}
	if strategyScore.Score != 0.0 {
		t.Errorf("ScoreWithDebug StrategyScore.Score = %f, want %f", strategyScore.Score, 0.0)
	}
	if strategyScore.StrategyName != "performance-aware" {
		t.Errorf("ScoreWithDebug StrategyScore.StrategyName = %q, want %q", strategyScore.StrategyName, "performance-aware")
	}
	if strategyScore.Details == nil {
		t.Error("ScoreWithDebug StrategyScore.Details should not be nil")
	}
}

func TestTTFTScoring(t *testing.T) {
	strategy := &PerformanceAwareStrategy{maxScore: 150.0}

	tests := []struct {
		name   string
		ttftMs float64
		want   float64
	}{
		// Under 2s threshold: full score
		{name: "500ms under threshold - full score", ttftMs: 500, want: 150.0},
		{name: "1500ms under threshold - full score", ttftMs: 1500, want: 150.0},
		{name: "2000ms at threshold - full score", ttftMs: 2000, want: 150.0},
		// 2-5 seconds: linear decay
		{name: "2500ms just over threshold", ttftMs: 2500, want: 150.0 * (1.0 - 0.1667*0.7)}, // 500/3000 ratio
		{name: "3500ms in penalty zone", ttftMs: 3500, want: 150.0 * (1.0 - 0.5*0.7)},     // 1500/3000 ratio
		{name: "5000ms at ok threshold", ttftMs: 5000, want: 150.0 * 0.3},              // 30% remaining
		// Above 5s: exponential penalty
		{name: "6000ms above ok threshold", ttftMs: 6000, want: 150.0 * 0.3 * math.Exp(-1000.0/3000.0)},
		{name: "8000ms deep penalty", ttftMs: 8000, want: 150.0 * 0.3 * math.Exp(-3000.0/3000.0)},
		// Edge cases
		{name: "zero ttft clamps to zero", ttftMs: 0, want: 0},
		{name: "negative ttft clamps to zero", ttftMs: -10, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.calculateTTFTScore(tt.ttftMs)
			if math.Abs(got-tt.want) > 0.1 { // Allow small floating point tolerance
				t.Fatalf("calculateTTFTScore(%v) = %v, want %v", tt.ttftMs, got, tt.want)
			}
			if got < 0 || got > 150.0 {
				t.Fatalf("calculateTTFTScore(%v) out of range: %v", tt.ttftMs, got)
			}
		})
	}
}
func TestTPSScoring(t *testing.T) {
	strategy := &PerformanceAwareStrategy{maxScore: 150.0}

	tests := []struct {
		name string
		tps  float64
		want float64
	}{
		// k=100: score = 150 * (1 - e^(-tps/100))
		{name: "50 tps with k=100", tps: 50, want: 150.0 * (1.0 - math.Exp(-50.0/100.0))},
		{name: "100 tps with k=100", tps: 100, want: 150.0 * (1.0 - math.Exp(-100.0/100.0))},
		{name: "150 tps with k=100", tps: 150, want: 150.0 * (1.0 - math.Exp(-150.0/100.0))},
		{name: "zero tps clamps to zero", tps: 0, want: 0},
		{name: "negative tps clamps to zero", tps: -5, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.calculateTPSScore(tt.tps)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("calculateTPSScore(%v) = %v, want %v", tt.tps, got, tt.want)
			}
			if got < 0 || got > 150.0 {
				t.Fatalf("calculateTPSScore(%v) out of range: %v", tt.tps, got)
			}
		})
	}
}

func TestCombinedScoring(t *testing.T) {
	strategy := &PerformanceAwareStrategy{maxScore: 150.0}

	// Test that scoring produces valid results for various inputs
	// Using new weights: 35% TTFT, 65% TPS
	tests := []struct {
		name     string
		ttftMs   float64
		tps      float64
		minScore float64 // Expected minimum score
		maxScore float64 // Expected maximum score
	}{
		// TTFT under threshold + moderate TPS
		{name: "low_ttft_moderate_tps", ttftMs: 500, tps: 50, minScore: 80, maxScore: 150},
		// TTFT at threshold + high TPS
		{name: "at_ttft_threshold_high_tps", ttftMs: 2000, tps: 100, minScore: 100, maxScore: 150},
		// TTFT over threshold + high TPS
		{name: "over_ttft_threshold_high_tps", ttftMs: 2500, tps: 150, minScore: 100, maxScore: 150},
		// TTFT well over threshold
		{name: "high_ttft_high_tps", ttftMs: 3000, tps: 100, minScore: 80, maxScore: 140},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttftScore := strategy.calculateTTFTScore(tt.ttftMs)
			tpsScore := strategy.calculateTPSScore(tt.tps)
			combined := 0.35*ttftScore + 0.65*tpsScore

			if combined < tt.minScore || combined > tt.maxScore {
				t.Fatalf("combined score %v not in expected range [%v, %v] (ttft=%v, tps=%v)",
					combined, tt.minScore, tt.maxScore, ttftScore, tpsScore)
			}
			if combined < 0 || combined > 150.0 {
				t.Fatalf("combined score %v out of valid range [0, 150]", combined)
			}
		})
	}
}

func TestHybridDataSource(t *testing.T) {
	tests := []struct {
		name             string
		probes           []*biz.ChannelProbePoint
		metrics          *biz.AggregatedMetrics
		historicalWeight float64
		realtimeWeight   float64
		wantTTFT         float64
		wantTPS          float64
		wantNil          bool
	}{
		{
			name: "both sources available - 70/30 weight",
			probes: []*biz.ChannelProbePoint{
				{AvgTimeToFirstTokenMs: floatPtr(100), AvgTokensPerSecond: floatPtr(50)},
			},
			metrics: &biz.AggregatedMetrics{
				AvgFirstTokenLatencyMs: floatPtr(200),
				AvgTokensPerSecond:     floatPtr(80),
			},
			historicalWeight: 0.7,
			realtimeWeight:   0.3,
			wantTTFT:         130, // 100 * 0.7 + 200 * 0.3 = 130
			wantTPS:          59,  // 50 * 0.7 + 80 * 0.3 = 59
		},
		{
			name: "only historical data available",
			probes: []*biz.ChannelProbePoint{
				{AvgTimeToFirstTokenMs: floatPtr(100), AvgTokensPerSecond: floatPtr(50)},
			},
			metrics:          &biz.AggregatedMetrics{},
			historicalWeight: 0.5,
			realtimeWeight:   0.5,
			wantTTFT:         100,
			wantTPS:          50,
		},
		{
			name:             "only real-time data available",
			probes:           []*biz.ChannelProbePoint{},
			metrics:          &biz.AggregatedMetrics{AvgFirstTokenLatencyMs: floatPtr(150), AvgTokensPerSecond: floatPtr(55)},
			historicalWeight: 0.5,
			realtimeWeight:   0.5,
			wantTTFT:         150,
			wantTPS:          55,
		},
		{
			name:             "no data available",
			probes:           []*biz.ChannelProbePoint{},
			metrics:          &biz.AggregatedMetrics{},
			historicalWeight: 0.5,
			realtimeWeight:   0.5,
			wantNil:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &PerformanceAwareStrategy{
				historicalWeight: tt.historicalWeight,
				realtimeWeight:   tt.realtimeWeight,
				getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
					return tt.metrics, nil
				},
				getProbesFunc: func(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error) {
					return tt.probes, nil
				},
			}

			result := strategy.getChannelPerformance(context.Background(), 1, "")

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.AvgTTFTMs != tt.wantTTFT {
				t.Errorf("AvgTTFTMs = %f, want %f", result.AvgTTFTMs, tt.wantTTFT)
			}
			if result.AvgTokensPerSecond != tt.wantTPS {
				t.Errorf("AvgTokensPerSecond = %f, want %f", result.AvgTokensPerSecond, tt.wantTPS)
			}
			if result.HistoricalWeight != tt.historicalWeight {
				t.Errorf("HistoricalWeight = %f, want %f", result.HistoricalWeight, tt.historicalWeight)
			}
			if result.RealtimeWeight != tt.realtimeWeight {
				t.Errorf("RealtimeWeight = %f, want %f", result.RealtimeWeight, tt.realtimeWeight)
			}
		})
	}
}

func TestChannelProbeQuery(t *testing.T) {
	probes := []*biz.ChannelProbePoint{
		{AvgTimeToFirstTokenMs: floatPtr(100), AvgTokensPerSecond: floatPtr(50)},
		{AvgTimeToFirstTokenMs: floatPtr(200), AvgTokensPerSecond: floatPtr(60)},
		{AvgTimeToFirstTokenMs: floatPtr(300), AvgTokensPerSecond: floatPtr(70)},
	}

	strategy := &PerformanceAwareStrategy{
		historicalWeight: 0.5,
		realtimeWeight:   0.5,
		getProbesFunc: func(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error) {
			return probes, nil
		},
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			return &biz.AggregatedMetrics{}, nil
		},
	}

	result := strategy.getChannelPerformance(context.Background(), 1, "")

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Average of 100, 200, 300 = 200
	if result.AvgTTFTMs != 200 {
		t.Errorf("AvgTTFTMs = %f, want 200", result.AvgTTFTMs)
	}

	// Average of 50, 60, 70 = 60
	if result.AvgTokensPerSecond != 60 {
		t.Errorf("AvgTokensPerSecond = %f, want 60", result.AvgTokensPerSecond)
	}
}

func TestRealtimeMetricsQuery(t *testing.T) {
	metrics := &biz.AggregatedMetrics{
		AvgFirstTokenLatencyMs: floatPtr(150),
		AvgTokensPerSecond:     floatPtr(75),
	}

	strategy := &PerformanceAwareStrategy{
		historicalWeight: 0.5,
		realtimeWeight:   0.5,
		getProbesFunc: func(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error) {
			return []*biz.ChannelProbePoint{}, nil
		},
		getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
			return metrics, nil
		},
	}

	result := strategy.getChannelPerformance(context.Background(), 1, "")

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.AvgTTFTMs != 150 {
		t.Errorf("AvgTTFTMs = %f, want 150", result.AvgTTFTMs)
	}

	if result.AvgTokensPerSecond != 75 {
		t.Errorf("AvgTokensPerSecond = %f, want 75", result.AvgTokensPerSecond)
	}
}

func TestHybridDataSourceMissingData(t *testing.T) {
	tests := []struct {
		name    string
		probes  []*biz.ChannelProbePoint
		metrics *biz.AggregatedMetrics
		wantNil bool
	}{
		{
			name:    "nil probes and empty metrics",
			probes:  nil,
			metrics: &biz.AggregatedMetrics{},
			wantNil: true,
		},
		{
			name:    "empty probes and nil metrics fields",
			probes:  []*biz.ChannelProbePoint{},
			metrics: &biz.AggregatedMetrics{},
			wantNil: true,
		},
		{
			name: "probes with zero values",
			probes: []*biz.ChannelProbePoint{
				{AvgTimeToFirstTokenMs: floatPtr(0), AvgTokensPerSecond: floatPtr(0)},
			},
			metrics: &biz.AggregatedMetrics{},
			wantNil: true,
		},
		{
			name:    "only TTFT from historical",
			probes:  []*biz.ChannelProbePoint{{AvgTimeToFirstTokenMs: floatPtr(100)}},
			metrics: &biz.AggregatedMetrics{},
			wantNil: false,
		},
		{
			name:    "only TPS from real-time",
			probes:  []*biz.ChannelProbePoint{},
			metrics: &biz.AggregatedMetrics{AvgTokensPerSecond: floatPtr(50)},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &PerformanceAwareStrategy{
				historicalWeight: 0.5,
				realtimeWeight:   0.5,
				getMetricsFunc: func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
					return tt.metrics, nil
				},
				getProbesFunc: func(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error) {
					return tt.probes, nil
				},
			}

			result := strategy.getChannelPerformance(context.Background(), 1, "")

			if tt.wantNil && result != nil {
				t.Errorf("expected nil, got %+v", result)
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

// floatPtr is a helper function to create a *float64
func floatPtr(f float64) *float64 {
	return &f
}

func TestColdStartDetection(t *testing.T) {
	tests := []struct {
		name          string
		metrics       *biz.AggregatedMetrics
		wantColdStart bool
	}{
		{
			name: "zero requests - cold start",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 0
				return m
			}(),
			wantColdStart: true,
		},
		{
			name: "9 requests - cold start (below threshold)",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 9
				return m
			}(),
			wantColdStart: true,
		},
		{
			name: "10 requests - not cold start (at threshold)",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 10
				return m
			}(),
			wantColdStart: false,
		},
		{
			name: "100 requests - not cold start",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 100
				return m
			}(),
			wantColdStart: false,
		},
		{
			name: "recently selected within cold start duration (not idle, warming up)",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 15
				t := time.Now().Add(-2 * time.Minute)
				m.LastSelectedAt = &t
				return m
			}(),
			wantColdStart: false,
		},
		{
			name: "selected just outside cold start duration (idle, needs warm up)",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 15
				t := time.Now().Add(-6 * time.Minute)
				m.LastSelectedAt = &t
				return m
			}(),
			wantColdStart: true,
		},
		{
			name: "nil LastSelectedAt with sufficient requests - not cold start",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 50
				m.LastSelectedAt = nil
				return m
			}(),
			wantColdStart: false,
		},
		{
			name: "nil LastSelectedAt with insufficient requests - cold start",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 5
				m.LastSelectedAt = nil
				return m
			}(),
			wantColdStart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &PerformanceAwareStrategy{}
			got := strategy.isColdStart(tt.metrics)
			if got != tt.wantColdStart {
				t.Errorf("isColdStart() = %v, want %v", got, tt.wantColdStart)
			}
		})
	}
}

func TestColdStartBoost(t *testing.T) {
	strategy := &PerformanceAwareStrategy{}

	zeroMetrics := func() *biz.AggregatedMetrics {
		m := &biz.AggregatedMetrics{}
		m.RequestCount = 0
		return m
	}()
	if !strategy.isColdStart(zeroMetrics) {
		t.Error("expected cold start for zero requests")
	}

	nineMetrics := func() *biz.AggregatedMetrics {
		m := &biz.AggregatedMetrics{}
		m.RequestCount = 9
		return m
	}()
	if !strategy.isColdStart(nineMetrics) {
		t.Error("expected cold start for 9 requests")
	}

	if ColdStartBoostScore != 120.0 {
		t.Errorf("ColdStartBoostScore = %v, want 120.0", ColdStartBoostScore)
	}

	maxScore := 150.0
	expectedBoost := maxScore * 0.8
	if ColdStartBoostScore != expectedBoost {
		t.Errorf("ColdStartBoostScore = %v, want %v (80%% of maxScore)", ColdStartBoostScore, expectedBoost)
	}
}

func TestColdStartEnds(t *testing.T) {
	tests := []struct {
		name          string
		metrics       *biz.AggregatedMetrics
		wantColdStart bool
	}{
		{
			name: "exactly 10 requests exits cold start",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 10
				return m
			}(),
			wantColdStart: false,
		},
		{
			name: "100 requests exits cold start",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 100
				return m
			}(),
			wantColdStart: false,
		},
		{
			name: "selected 6 minutes ago (idle, needs warm up)",
			metrics: func() *biz.AggregatedMetrics {
				m := &biz.AggregatedMetrics{}
				m.RequestCount = 15
				t := time.Now().Add(-6 * time.Minute)
				m.LastSelectedAt = &t
				return m
			}(),
			wantColdStart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := &PerformanceAwareStrategy{}
			got := strategy.isColdStart(tt.metrics)
			if got != tt.wantColdStart {
				t.Errorf("isColdStart() = %v, want %v", got, tt.wantColdStart)
			}
		})
	}
}

func TestPerformanceAwareStrategy_WeightValidation(t *testing.T) {
	tests := []struct {
		name             string
		historicalWeight float64
		realtimeWeight   float64
		expectError      error
	}{
		{
			name:             "valid weights 0.4/0.6 sum to 1.0",
			historicalWeight: 0.4,
			realtimeWeight:   0.6,
			expectError:      nil,
		},
		{
			name:             "valid weights 0.5/0.5 sum to 1.0",
			historicalWeight: 0.5,
			realtimeWeight:   0.5,
			expectError:      nil,
		},
		{
			name:             "valid weights 0.0/1.0 sum to 1.0",
			historicalWeight: 0.0,
			realtimeWeight:   1.0,
			expectError:      nil,
		},
		{
			name:             "invalid weights 0.3/0.6 sum to 0.9",
			historicalWeight: 0.3,
			realtimeWeight:   0.6,
			expectError:      ErrWeightsMustSumToOne,
		},
		{
			name:             "invalid weights 0.5/0.4 sum to 0.9",
			historicalWeight: 0.5,
			realtimeWeight:   0.4,
			expectError:      ErrWeightsMustSumToOne,
		},
		{
			name:             "invalid negative historical weight",
			historicalWeight: -0.1,
			realtimeWeight:   1.1,
			expectError:      ErrInvalidHistoricalWeight,
		},
		{
			name:             "invalid negative realtime weight",
			historicalWeight: 0.5,
			realtimeWeight:   -0.1,
			expectError:      ErrInvalidRealtimeWeight,
		},
		{
			name:             "invalid historical weight > 1.0",
			historicalWeight: 1.5,
			realtimeWeight:   0.5,
			expectError:      ErrInvalidHistoricalWeight,
		},
		{
			name:             "invalid realtime weight > 1.0",
			historicalWeight: 0.5,
			realtimeWeight:   1.5,
			expectError:      ErrInvalidRealtimeWeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := NewPerformanceAwareStrategy(
				nil,
				nil,
				WithWeights(tt.historicalWeight, tt.realtimeWeight),
			)

			if tt.expectError != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tt.expectError)
			} else {
				require.NoError(t, err)
				require.NotNil(t, strategy)
				require.Equal(t, tt.historicalWeight, strategy.historicalWeight)
				require.Equal(t, tt.realtimeWeight, strategy.realtimeWeight)
			}
		})
	}
}

func TestPerformanceAwareStrategy_WithWeightsOption(t *testing.T) {
	// Test that WithWeights option correctly passes weights
	strategy, err := NewPerformanceAwareStrategy(
		nil,
		nil,
		WithWeights(0.3, 0.7),
	)

	require.NoError(t, err)
	require.NotNil(t, strategy)
	require.Equal(t, 0.3, strategy.historicalWeight)
	require.Equal(t, 0.7, strategy.realtimeWeight)
}
