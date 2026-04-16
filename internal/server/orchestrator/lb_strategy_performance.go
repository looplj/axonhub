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
	defaultMaxScore = 150.0
	weightEpsilon   = 0.001
)

// TTFT scoring thresholds
const (
	TTFTGoodThreshold   = 2000.0
	TTFTOkThreshold     = 5000.0
	TTFTSlowThreshold   = 10000.0
	TTFTOkPenalty       = 0.7
	TTFTSlowFloor       = 0.20
	TTFTExponentialTau  = 5000.0
)

// TPS scoring constant
const (
	// TPSCharacteristicK is the characteristic value for TPS exponential scoring
	// Higher values make TPS scoring more linear across typical ranges
	TPSCharacteristicK = 100.0
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

// Error rate penalty constants
const (
	// ErrorRateLowThreshold is the error rate below which no penalty is applied (5%)
	// Channels with <5% error rate are considered healthy
	ErrorRateLowThreshold = 0.05

	// ErrorRateMediumThreshold is the error rate at which moderate penalty is applied (20%)
	// Channels with 5-20% error rate get gradually increasing penalties
	ErrorRateMediumThreshold = 0.20

	// ErrorRateHighThreshold is the error rate at which severe penalty is applied (50%)
	// Channels with >50% error rate are considered failing and heavily penalized
	ErrorRateHighThreshold = 0.50

	// ErrorRatePenaltyMultiplier controls how aggressively errors are penalized
	// At medium threshold, score is multiplied by (1 - ErrorRatePenaltyMultiplier)
	ErrorRatePenaltyMultiplier = 0.5

	// ErrorRateMinRequests is the minimum requests before error rate is considered
	// This prevents penalizing channels with very few requests
	ErrorRateMinRequests = 5
)

// ChannelPerformance represents combined performance metrics for a channel.
// It contains both historical (probe) and real-time metrics.
type ChannelPerformance struct {
	// AvgTTFTMs is the average Time To First Token in milliseconds
	AvgTTFTMs float64
	// AvgTokensPerSecond is the average tokens generated per second
	AvgTokensPerSecond float64
	// ErrorRate is the ratio of failures to total requests in the sliding window
	ErrorRate float64
	// RequestCount is the total number of requests in the sliding window
	RequestCount int64
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
	getMetricsFunc func(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error)
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
		historicalWeight: 0.0,
		realtimeWeight:   0.0,
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
// Cold start channels (no data or insufficient data) receive a boost score
// to give them opportunity to gather metrics and be discovered.
func (s *PerformanceAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	if channel == nil {
		return 0.0
	}

	model := requestedModelFromContext(ctx)

	// If we have metrics, check for cold start
	if metrics, err := s.getChannelMetrics(ctx, channel.ID, model); err == nil && metrics != nil {
		if s.isColdStart(metrics) {
			return ColdStartBoostScore
		}
	} else {
		// No metrics available — this covers both genuinely new channels
		// (no data yet) and transient infrastructure errors (e.g., Redis timeout).
		// A brand-new channel must not score 0 or it can never be discovered,
		// so we err on the side of giving the boost. Transient boost for a
		// warm channel is self-correcting: once metrics return, it resumes its
		// real score on the next request.
		// TODO: Consider distinguishing nil metrics (no data) from error metrics
		// (infrastructure failure) to avoid boosting warm channels during outages.
		return ColdStartBoostScore
	}

	performance := s.getChannelPerformance(ctx, channel.ID, model)
	if performance == nil {
		return 0.0
	}

	// Score only needs the combined score, discard component scores
	combinedScore, _, _, _ := s.calculateCombinedScore(performance)
	return combinedScore
}

// isColdStart determines if a channel is in cold start state.
// A channel is in cold start if:
// 1. It has fewer than ColdStartMinRequests (10) requests, OR
// 2. LastSelectedAt is MORE than ColdStartDuration (5 minutes) ago (idle channel warming up)
func (s *PerformanceAwareStrategy) isColdStart(metrics *biz.AggregatedMetrics) bool {
	if metrics.RequestCount < ColdStartMinRequests {
		return true
	}

	// If the channel hasn't been used for more than ColdStartDuration,
	// treat it as cold start (warming up after being idle)
	if metrics.LastSelectedAt != nil {
		sinceLastSelected := time.Since(*metrics.LastSelectedAt)
		if sinceLastSelected > ColdStartDuration {
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

	model := requestedModelFromContext(ctx)

	if metrics, err := s.getChannelMetrics(ctx, channel.ID, model); err == nil && metrics != nil {
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
	} else {
		details["reason"] = "cold_start_no_metrics"
		details["cold_start_boost_score"] = ColdStartBoostScore

		return ColdStartBoostScore, StrategyScore{
			StrategyName: s.Name(),
			Score:        ColdStartBoostScore,
			Details:      details,
			Duration:     time.Since(start),
		}
	}

	performance := s.getChannelPerformance(ctx, channel.ID, model)
	if performance == nil {
		details["reason"] = "no_performance_data"

		return 0.0, StrategyScore{
			StrategyName: s.Name(),
			Score:        0.0,
			Details:      details,
			Duration:     time.Since(start),
		}
	}

	combinedScore, ttftScore, tpsScore, errorPenalty := s.calculateCombinedScore(performance)

	details["avg_ttft_ms"] = performance.AvgTTFTMs
	details["avg_tokens_per_second"] = performance.AvgTokensPerSecond
	details["error_rate"] = performance.ErrorRate
	details["error_penalty"] = errorPenalty
	details["historical_weight"] = performance.HistoricalWeight
	details["realtime_weight"] = performance.RealtimeWeight
	details["ttft_score"] = ttftScore
	details["tps_score"] = tpsScore
	details["ttft_score_weight"] = 0.35
	details["tps_score_weight"] = 0.65
	details["combined_score_before_penalty"] = 0.35*ttftScore + 0.65*tpsScore

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

	maxScore := s.scoreMax()

	if ttftMs <= TTFTGoodThreshold {
		return maxScore
	} else if ttftMs <= TTFTOkThreshold {
		ratio := (ttftMs - TTFTGoodThreshold) / (TTFTOkThreshold - TTFTGoodThreshold)
		return maxScore * (1.0 - ratio*TTFTOkPenalty)
	} else if ttftMs <= TTFTSlowThreshold {
		okScore := maxScore * (1.0 - TTFTOkPenalty)
		floorScore := maxScore * TTFTSlowFloor
		ratio := (ttftMs - TTFTOkThreshold) / (TTFTSlowThreshold - TTFTOkThreshold)
		return okScore + (floorScore-okScore)*ratio
	} else {
		floorScore := maxScore * TTFTSlowFloor
		excess := ttftMs - TTFTSlowThreshold
		return floorScore * math.Exp(-excess/TTFTExponentialTau)
	}
}

func (s *PerformanceAwareStrategy) calculateTPSScore(tps float64) float64 {
	if tps <= 0 {
		return 0
	}

	// Use k=100 for better TPS differentiation across typical ranges
	// This prevents early saturation and makes TPS differences meaningful
	score := s.scoreMax() * (1.0 - math.Exp(-tps/TPSCharacteristicK))
	if score < 0 {
		return 0
	}
	if score > s.scoreMax() {
		return s.scoreMax()
	}

	return score
}

func (s *PerformanceAwareStrategy) calculateCombinedScore(performance *ChannelPerformance) (float64, float64, float64, float64) {
	if performance == nil {
		return 0, 0, 0, 0
	}

	ttftScore := s.calculateTTFTScore(performance.AvgTTFTMs)
	tpsScore := s.calculateTPSScore(performance.AvgTokensPerSecond)
	// Weight TPS higher than TTFT: 65% TPS, 35% TTFT
	combinedScore := 0.35*ttftScore + 0.65*tpsScore

	// Apply error rate penalty
	errorPenalty := s.calculateErrorRatePenalty(performance.ErrorRate, performance.RequestCount)
	combinedScore = combinedScore * errorPenalty

	if combinedScore < 0 {
		combinedScore = 0
	}
	if combinedScore > s.scoreMax() {
		combinedScore = s.scoreMax()
	}

	return combinedScore, ttftScore, tpsScore, errorPenalty
}

// calculateErrorRatePenalty returns a multiplier (0.0 to 1.0) to apply to the score
// based on the channel's error rate. Higher error rates result in lower multipliers.
// Returns 1.0 if error rate should not affect scoring (too few requests, cold start).
//
// Penalty tiers:
// - <5% error rate: no penalty (return 1.0)
// - 5-20% error rate: linear penalty from 1.0 to 0.5
// - 20-50% error rate: linear penalty from 0.5 to 0.1
// - >50% error rate: severe penalty (return 0.1 or lower)
func (s *PerformanceAwareStrategy) calculateErrorRatePenalty(errorRate float64, requestCount int64) float64 {
	// Don't penalize channels with too few requests
	if requestCount < ErrorRateMinRequests {
		return 1.0
	}

	// Handle invalid error rates (NaN or negative)
	// NaN comparisons always return false, so this prevents NaN propagation
	if math.IsNaN(errorRate) || errorRate < 0 {
		return 1.0
	}

	// No penalty for healthy error rates
	if errorRate <= ErrorRateLowThreshold {
		return 1.0
	}

	// Linear interpolation for medium error rate zone
	if errorRate <= ErrorRateMediumThreshold {
		// Scale from 1.0 at 5% to 0.5 at 20%
		ratio := (errorRate - ErrorRateLowThreshold) / (ErrorRateMediumThreshold - ErrorRateLowThreshold)
		return 1.0 - ratio*ErrorRatePenaltyMultiplier
	}

	// Linear interpolation for high error rate zone
	if errorRate <= ErrorRateHighThreshold {
		// Scale from 0.5 at 20% to 0.1 at 50%
		ratio := (errorRate - ErrorRateMediumThreshold) / (ErrorRateHighThreshold - ErrorRateMediumThreshold)
		return (1.0 - ErrorRatePenaltyMultiplier) - ratio*0.4
	}

	// Severe penalty for critical error rates (>50%)
	// Exponential decay: at 75% error rate, penalty is ~0.025
	// At 100% error rate, penalty is effectively 0
	excess := errorRate - ErrorRateHighThreshold
	return 0.1 * math.Exp(-excess*3.0)
}

func (s *PerformanceAwareStrategy) getChannelMetrics(ctx context.Context, channelID int, model string) (*biz.AggregatedMetrics, error) {
	if s.getMetricsFunc != nil {
		return s.getMetricsFunc(ctx, channelID, model)
	}
	if s.channelService == nil {
		return nil, nil
	}

	// Only use model-specific metrics for scoring
	// If no model-specific data exists, return nil to trigger cold start
	metrics, err := s.channelService.GetChannelMetrics(ctx, channelID, model)
	if err == nil && metrics != nil && metrics.RequestCount > 0 {
		return metrics, nil
	}

	// No model-specific metrics available - triggers cold start boost
	return nil, errors.New("no model-specific metrics available")
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
func (s *PerformanceAwareStrategy) getChannelPerformance(ctx context.Context, channelID int, model string) *ChannelPerformance {
	var historicalTTFT, historicalTPS *float64
	var realtimeTTFT, realtimeTPS *float64
	var requestCount int64
	var failureCount int64

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
	if metrics, err := s.getChannelMetrics(ctx, channelID, model); err == nil && metrics != nil {
		// Use EWMA values for real-time metrics
		if metrics.StreamingFirstTokenLatencyEWMA > 0 {
			realtimeTTFT = &metrics.StreamingFirstTokenLatencyEWMA
		}
		if metrics.StreamingTokensPerSecondEWMA > 0 {
			realtimeTPS = &metrics.StreamingTokensPerSecondEWMA
		}
		// Get request/failure counts for error rate calculation
		requestCount = metrics.RequestCount
		failureCount = metrics.FailureCount
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
		RequestCount:     requestCount,
	}

	// Calculate error rate
	if requestCount > 0 {
		result.ErrorRate = float64(failureCount) / float64(requestCount)
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

// NeedsModelData checks if a channel has no model-specific performance data.
// This is used by the exploration mechanism to identify channels that need to gather metrics.
// Returns true if the channel has no model-specific metrics for the given model.
func (s *PerformanceAwareStrategy) NeedsModelData(ctx context.Context, channelID int, model string) bool {
	metrics, err := s.getChannelMetrics(ctx, channelID, model)
	if err != nil || metrics == nil {
		return true // No data available
	}
	// Has model-specific data if request count > 0
	return metrics.RequestCount == 0
}
