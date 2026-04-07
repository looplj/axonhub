package orchestrator

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
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
		{name: "500ms exponential decay", ttftMs: 500, want: 150.0 * math.Exp(-500.0/1000.0)},
		{name: "zero ttft clamps to zero", ttftMs: 0, want: 0},
		{name: "negative ttft clamps to zero", ttftMs: -10, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.calculateTTFTScore(tt.ttftMs)
			if math.Abs(got-tt.want) > 1e-9 {
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
		{name: "50 tps logarithmic scaling", tps: 50, want: 150.0 * (1.0 - math.Exp(-50.0/30.0))},
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
	staleSelection := time.Now().Add(-10 * time.Minute)
	realtimeTTFT := 400.0
	realtimeTPS := 60.0
	historicalTTFT := 600.0
	historicalTPS := 30.0

	strategy := &PerformanceAwareStrategy{
		maxScore:         150.0,
		historicalWeight: 0.5,
		realtimeWeight:   0.5,
		getMetricsFunc: func(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
			metrics := &biz.AggregatedMetrics{
				LastSelectedAt:         &staleSelection,
				AvgFirstTokenLatencyMs: &realtimeTTFT,
				AvgTokensPerSecond:     &realtimeTPS,
			}
			metrics.RequestCount = 20

			return metrics, nil
		},
		getProbesFunc: func(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error) {
			return []*biz.ChannelProbePoint{{
				AvgTimeToFirstTokenMs: &historicalTTFT,
				AvgTokensPerSecond:    &historicalTPS,
			}}, nil
		},
	}

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	got := strategy.Score(context.Background(), channel)

	combinedTTFT := 0.5*historicalTTFT + 0.5*realtimeTTFT
	combinedTPS := 0.5*historicalTPS + 0.5*realtimeTPS
	wantTTFTScore := 150.0 * math.Exp(-combinedTTFT/1000.0)
	wantTPSScore := 150.0 * (1.0 - math.Exp(-combinedTPS/30.0))
	want := 0.5*wantTTFTScore + 0.5*wantTPSScore

	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("Score() = %v, want %v", got, want)
	}
	if got < 0 || got > 150.0 {
		t.Fatalf("Score() out of range: %v", got)
	}

	debugScore, strategyScore := strategy.ScoreWithDebug(context.Background(), channel)
	if math.Abs(debugScore-want) > 1e-9 {
		t.Fatalf("ScoreWithDebug() score = %v, want %v", debugScore, want)
	}
	if math.Abs(strategyScore.Score-want) > 1e-9 {
		t.Fatalf("StrategyScore.Score = %v, want %v", strategyScore.Score, want)
	}
	if strategyScore.Details["ttft_score_weight"] != 0.5 {
		t.Fatalf("ttft_score_weight = %v, want 0.5", strategyScore.Details["ttft_score_weight"])
	}
	if strategyScore.Details["tps_score_weight"] != 0.5 {
		t.Fatalf("tps_score_weight = %v, want 0.5", strategyScore.Details["tps_score_weight"])
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
				getMetricsFunc: func(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
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
		getMetricsFunc: func(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
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
		getMetricsFunc: func(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
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
				getMetricsFunc: func(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
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
