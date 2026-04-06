package orchestrator

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/looplj/axonhub/internal/server/biz"
)

// Validation constants
const (
	defaultMaxScore         = 150.0
	defaultHistoricalWeight = 0.5
	defaultRealtimeWeight   = 0.5
	weightEpsilon           = 0.001
)

// Cold start constants
const (
	// ColdStartDuration is the time period after channel selection during which
	// the channel is considered in cold start state
	ColdStartDuration = 5 * time.Minute

	// ColdStartBoostScore is the score given to channels in cold start state
	// This is 80% of maxScore (150), giving new channels a competitive advantage
	// while they gather performance metrics
	ColdStartBoostScore = 120.0

	// ColdStartMinRequests is the minimum number of requests before a channel
	// is considered to have exited cold start state
	ColdStartMinRequests = 10
)

// ChannelPerformance represents combined performance metrics for a channel.
// It contains both historical (probe) and real-time metrics.
type ChannelPerformance struct {
	// AvgTTFTMs is the average Time To First Token in milliseconds
	AvgTTFTMs float64
	// AvgTokensPerSecond is the average tokens generated per second
	AvgTokensPerSecond float64
	// HistoricalWeight is the weight given to historical data (0.0 to 1.0)
	HistoricalWeight float64
	// RealtimeWeight is the weight given to real-time data (0.0 to 1.0)
	RealtimeWeight float64
}

// PerformanceAwareStrategy prioritizes channels based on their performance metrics.
// This strategy considers latency, throughput, error rates, and resource utilization
// to route requests to the most performant channels.
type PerformanceAwareStrategy struct {
	channelService *biz.ChannelService
	probeService   *biz.ChannelProbeService
	getMetricsFunc func(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error)
	getProbesFunc  func(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error)
	// maxScore is the maximum score for a perfectly performing channel (default: 150)
	maxScore float64
	// historicalWeight is the weight for historical probe data (default: 0.5)
	historicalWeight float64
	// realtimeWeight is the weight for real-time metrics (default: 0.5)
	realtimeWeight float64
}

// Option is a functional option for configuring PerformanceAwareStrategy
type Option func(*PerformanceAwareStrategy)

// WithMaxScore sets the maximum score for the strategy
func WithMaxScore(maxScore float64) Option {
	return func(s *PerformanceAwareStrategy) {
		s.maxScore = maxScore
	}
}

// WithWeights sets the historical and realtime weights
func WithWeights(historicalWeight, realtimeWeight float64) Option {
	return func(s *PerformanceAwareStrategy) {
		s.historicalWeight = historicalWeight
		s.realtimeWeight = realtimeWeight
	}
}

// Validation errors
var (
	ErrInvalidMaxScore         = errors.New("maxScore must be positive")
	ErrInvalidHistoricalWeight = errors.New("historicalWeight must be in [0,1] range")
	ErrInvalidRealtimeWeight   = errors.New("realtimeWeight must be in [0,1] range")
	ErrWeightsMustSumToOne     = errors.New("historicalWeight and realtimeWeight must sum to 1.0 (within epsilon)")
)

// NewPerformanceAwareStrategy creates a new performance-aware load balancing strategy.
// It validates the provided weights and returns errors for invalid configurations.
// If maxScore is not positive, weights are not in [0,1] range, or weights don't sum to 1.0,
// an error is returned.
func NewPerformanceAwareStrategy(channelService *biz.ChannelService, probeService *biz.ChannelProbeService, opts ...Option) (*PerformanceAwareStrategy, error) {
	s := &PerformanceAwareStrategy{
		channelService:   channelService,
		probeService:     probeService,
		maxScore:         defaultMaxScore,
		historicalWeight: defaultHistoricalWeight,
		realtimeWeight:   defaultRealtimeWeight,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Validate maxScore
	if s.maxScore <= 0 {
		return nil, ErrInvalidMaxScore
	}

	// Validate weight ranges
	if s.historicalWeight < 0 || s.historicalWeight > 1 {
		return nil, ErrInvalidHistoricalWeight
	}
	if s.realtimeWeight < 0 || s.realtimeWeight > 1 {
		return nil, ErrInvalidRealtimeWeight
	}

	// Validate weights sum to 1.0 (within epsilon)
	weightSum := s.historicalWeight + s.realtimeWeight
	if math.Abs(weightSum-1.0) > weightEpsilon {
		return nil, ErrWeightsMustSumToOne
	}

	return s, nil
}

// Name returns the strategy name for debugging and logging.
func (s *PerformanceAwareStrategy) Name() string {
	return "performance-aware"
}

// Score calculates a score for a channel based on performance metrics.
// Higher scores indicate better performance.
// Returns a score between 0 and maxScore.
// Cold start channels receive a boost score to give them opportunity to gather metrics.
func (s *PerformanceAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	if channel == nil {
		return 0.0
	}

	if metrics, err := s.getChannelMetrics(ctx, channel.ID); err == nil && metrics != nil {
		if s.isColdStart(metrics) {
			return ColdStartBoostScore
		}
	}

	performance := s.getChannelPerformance(ctx, channel.ID)
	if performance == nil {
		return 0.0
	}

	// Score only needs the combined score, discard component scores
	combinedScore, _, _ := s.calculateCombinedScore(performance)
	return combinedScore
}

// isColdStart determines if a channel is in cold start state.
// A channel is in cold start if:
// 1. It has fewer than ColdStartMinRequests (10) requests, OR
// 2. LastSelectedAt is within ColdStartDuration (5 minutes) from now
func (s *PerformanceAwareStrategy) isColdStart(metrics *biz.AggregatedMetrics) bool {
	if metrics.RequestCount < ColdStartMinRequests {
		return true
	}

	if metrics.LastSelectedAt != nil {
		sinceLastSelected := time.Since(*metrics.LastSelectedAt)
		if sinceLastSelected < ColdStartDuration {
			return true
		}
	}

	return false
}

// ScoreWithDebug calculates a score with detailed debug information.
// Returns the score and a StrategyScore with debug details.
func (s *PerformanceAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	start := time.Now()
	details := map[string]any{}

	if metrics, err := s.getChannelMetrics(ctx, channel.ID); err == nil && metrics != nil {
		details["request_count"] = metrics.RequestCount
		details["last_selected_at"] = metrics.LastSelectedAt
		if s.isColdStart(metrics) {
			details["reason"] = "cold_start"
			details["cold_start_boost_score"] = ColdStartBoostScore

			return ColdStartBoostScore, StrategyScore{
				StrategyName: s.Name(),
				Score:        ColdStartBoostScore,
				Details:      details,
				Duration:     time.Since(start),
			}
		}
	}

	performance := s.getChannelPerformance(ctx, channel.ID)
	if performance == nil {
		details["reason"] = "no_performance_data"

		return 0.0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0.0,
			Details:      details,
			Duration:     time.Since(start),
		}
	}

	combinedScore, ttftScore, tpsScore := s.calculateCombinedScore(performance)

	details["avg_ttft_ms"] = performance.AvgTTFTMs
	details["avg_tokens_per_second"] = performance.AvgTokensPerSecond
	details["historical_weight"] = performance.HistoricalWeight
	details["realtime_weight"] = performance.RealtimeWeight
	details["ttft_score"] = ttftScore
	details["tps_score"] = tpsScore
	details["ttft_score_weight"] = 0.5
	details["tps_score_weight"] = 0.5
	details["combined_score_unclamped"] = 0.5*ttftScore + 0.5*tpsScore

	return combinedScore, StrategyScore{
		StrategyName: s.Name(),
		Score:        combinedScore,
		Details:      details,
		Duration:     time.Since(start),
	}
}

func (s *PerformanceAwareStrategy) scoreMax() float64 {
	if s.maxScore > 0 {
		return s.maxScore
	}

	return 150.0
}

func (s *PerformanceAwareStrategy) calculateTTFTScore(ttftMs float64) float64 {
	if ttftMs <= 0 {
		return 0
	}

	score := s.scoreMax() * math.Exp(-ttftMs/1000.0)
	if score < 0 {
		return 0
	}
	if score > s.scoreMax() {
		return s.scoreMax()
	}

	return score
}

func (s *PerformanceAwareStrategy) calculateTPSScore(tps float64) float64 {
	if tps <= 0 {
		return 0
	}

	score := s.scoreMax() * (1.0 - math.Exp(-tps/30.0))
	if score < 0 {
		return 0
	}
	if score > s.scoreMax() {
		return s.scoreMax()
	}

	return score
}

func (s *PerformanceAwareStrategy) calculateCombinedScore(performance *ChannelPerformance) (float64, float64, float64) {
	if performance == nil {
		return 0, 0, 0
	}

	ttftScore := s.calculateTTFTScore(performance.AvgTTFTMs)
	tpsScore := s.calculateTPSScore(performance.AvgTokensPerSecond)
	combinedScore := 0.5*ttftScore + 0.5*tpsScore

	if combinedScore < 0 {
		combinedScore = 0
	}
	if combinedScore > s.scoreMax() {
		combinedScore = s.scoreMax()
	}

	return combinedScore, ttftScore, tpsScore
}

func (s *PerformanceAwareStrategy) getChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error) {
	if s.getMetricsFunc != nil {
		return s.getMetricsFunc(ctx, channelID)
	}
	if s.channelService == nil {
		return nil, nil
	}

	return s.channelService.GetChannelMetrics(ctx, channelID)
}

func (s *PerformanceAwareStrategy) getChannelProbes(ctx context.Context, channelID int) ([]*biz.ChannelProbePoint, error) {
	if s.getProbesFunc != nil {
		return s.getProbesFunc(ctx, channelID)
	}
	if s.probeService == nil {
		return nil, nil
	}

	return s.probeService.GetProbesByChannelID(ctx, channelID)
}

// getChannelPerformance retrieves combined performance metrics for a channel.
// It queries both historical probe data and real-time metrics, combining them
// with configurable weights (default: 50% historical, 50% real-time).
// Returns nil if no data is available from either source.
func (s *PerformanceAwareStrategy) getChannelPerformance(ctx context.Context, channelID int) *ChannelPerformance {
	var historicalTTFT, historicalTPS *float64
	var realtimeTTFT, realtimeTPS *float64

	// Query historical data from ChannelProbeService
	if probes, err := s.getChannelProbes(ctx, channelID); err == nil && len(probes) > 0 {
		// Calculate averages from probe points that have data
		var totalTTFT, totalTPS float64
		var ttftCount, tpsCount int

		for _, probe := range probes {
			if probe.AvgTimeToFirstTokenMs != nil && *probe.AvgTimeToFirstTokenMs > 0 {
				totalTTFT += *probe.AvgTimeToFirstTokenMs
				ttftCount++
			}
			if probe.AvgTokensPerSecond != nil && *probe.AvgTokensPerSecond > 0 {
				totalTPS += *probe.AvgTokensPerSecond
				tpsCount++
			}
		}

		if ttftCount > 0 {
			avgTTFT := totalTTFT / float64(ttftCount)
			historicalTTFT = &avgTTFT
		}
		if tpsCount > 0 {
			avgTPS := totalTPS / float64(tpsCount)
			historicalTPS = &avgTPS
		}
	}

	// Query real-time metrics from ChannelService
	if metrics, err := s.getChannelMetrics(ctx, channelID); err == nil && metrics != nil {
		if metrics.AvgFirstTokenLatencyMs != nil && *metrics.AvgFirstTokenLatencyMs > 0 {
			realtimeTTFT = metrics.AvgFirstTokenLatencyMs
		}
		if metrics.AvgTokensPerSecond != nil && *metrics.AvgTokensPerSecond > 0 {
			realtimeTPS = metrics.AvgTokensPerSecond
		}
	}

	// Check if we have any data at all
	if historicalTTFT == nil && realtimeTTFT == nil &&
		historicalTPS == nil && realtimeTPS == nil {
		return nil
	}

	// Combine data with weights
	result := &ChannelPerformance{
		HistoricalWeight: s.historicalWeight,
		RealtimeWeight:   s.realtimeWeight,
	}

	// Calculate weighted average for TTFT
	if historicalTTFT != nil && realtimeTTFT != nil {
		result.AvgTTFTMs = s.historicalWeight*(*historicalTTFT) + s.realtimeWeight*(*realtimeTTFT)
	} else if historicalTTFT != nil {
		result.AvgTTFTMs = *historicalTTFT
	} else if realtimeTTFT != nil {
		result.AvgTTFTMs = *realtimeTTFT
	}

	// Calculate weighted average for TPS
	if historicalTPS != nil && realtimeTPS != nil {
		result.AvgTokensPerSecond = s.historicalWeight*(*historicalTPS) + s.realtimeWeight*(*realtimeTPS)
	} else if historicalTPS != nil {
		result.AvgTokensPerSecond = *historicalTPS
	} else if realtimeTPS != nil {
		result.AvgTokensPerSecond = *realtimeTPS
	}

	return result
}
