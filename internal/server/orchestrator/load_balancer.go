package orchestrator

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/viterin/partial"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// metricsUnavailableReason marks debug details of a metrics-based score that
// fell back to a neutral value because the candidate had no metrics snapshot.
const metricsUnavailableReason = "metrics_unavailable"

// ChannelMetricsProvider provides channel performance metrics.
type ChannelMetricsProvider interface {
	GetChannelMetrics(ctx context.Context, channelID int) (*biz.AggregatedMetrics, error)
}

// ChannelSelectionTracker tracks channel selections for load balancing.
// This is used to increment request count at selection time rather than completion time,
// ensuring concurrent/burst requests don't all select the same channel.
type ChannelSelectionTracker interface {
	IncrementChannelSelection(channelID int)
}

// LoadBalanceStrategy defines the interface for load balancing strategies.
// Each strategy can score and sort channels based on different criteria.
//
// metrics is the channel snapshot the load balancer fetched once for this
// candidate and shares across all strategies. It is nil when the load balancer
// has no metrics provider or the fetch failed; strategies that depend on
// metrics must fall back to a neutral score in that case.
type LoadBalanceStrategy interface {
	// Score calculates a score for a channel. Higher scores indicate higher priority.
	// Returns a score between 0 and 1000.
	// This is the production path with minimal overhead.
	Score(ctx context.Context, channel *biz.Channel, metrics *biz.AggregatedMetrics) float64

	// ScoreWithDebug calculates a score with detailed debug information.
	// Returns the score and a StrategyScore with debug details.
	// This should have identical logic to Score() except for debug logging.
	ScoreWithDebug(ctx context.Context, channel *biz.Channel, metrics *biz.AggregatedMetrics) (float64, StrategyScore)

	// Name returns the strategy name for debugging and logging.
	Name() string
}

// StrategyScore holds the detailed scoring information from a single strategy.
type StrategyScore struct {
	// StrategyName is the name of the strategy
	StrategyName string
	// Score is the score calculated by this strategy
	Score float64
	// Details contains strategy-specific information
	Details map[string]any
	// Duration is the time spent on scoring
	Duration time.Duration
}

// ChannelDecision holds detailed scoring information for a single channel.
type ChannelDecision struct {
	// Channel is the channel object
	Channel *biz.Channel
	// OriginalIndex is the candidate's position before sorting.
	OriginalIndex int
	// TotalScore is the sum of all strategy scores
	TotalScore float64
	// StrategyScores contains scores from each strategy
	StrategyScores []StrategyScore
	// FinalRank is the final ranking (1 = highest priority)
	FinalRank int

	metrics *biz.AggregatedMetrics
}

// DecisionLog represents a complete load balancing decision.
type DecisionLog struct {
	// Timestamp when the decision was made
	Timestamp time.Time
	// ChannelCount is the number of channels considered
	ChannelCount int
	// TotalDuration is the time spent on load balancing
	TotalDuration time.Duration
	// Channels contains detailed information for each channel
	Channels []ChannelDecision
}

// Save the requested model ID in the context, to let model aware strategy use it, e.g. circuit breaker.
type modelContextKey struct{}
type streamContextKey struct{}

// contextWithRequestedModel adds the requested model ID to the context.
func contextWithRequestedModel(ctx context.Context, modelID string) context.Context {
	return context.WithValue(ctx, modelContextKey{}, modelID)
}

// requestedModelFromContext extracts the requested model ID from the context.
func requestedModelFromContext(ctx context.Context) string {
	if model, ok := ctx.Value(modelContextKey{}).(string); ok {
		return model
	}

	return ""
}

func contextWithRequestStream(ctx context.Context, stream bool) context.Context {
	return context.WithValue(ctx, streamContextKey{}, stream)
}

func requestStreamFromContext(ctx context.Context) bool {
	stream, _ := ctx.Value(streamContextKey{}).(bool)

	return stream
}

// LoadBalancer applies multiple strategies to sort channels by priority.
type LoadBalancer struct {
	strategies             []LoadBalanceStrategy
	metricsProvider        ChannelMetricsProvider
	selectionTracker       ChannelSelectionTracker
	weightTieBreaker       bool
	roundRobinHealthFilter *RoundRobinHealthStrategy
	debug                  bool
}

// NewLoadBalancer creates a new load balancer with the given strategies.
// All strategy scores are summed with equal weight; registration order does not affect scoring.
// metricsProvider may be nil when no strategy reads channel metrics; the load
// balancer then skips the per-candidate metrics fetch entirely.
func NewLoadBalancer(metricsProvider ChannelMetricsProvider, selectionTracker ChannelSelectionTracker, strategies ...LoadBalanceStrategy) *LoadBalancer {
	debug := strings.EqualFold(os.Getenv("AXONHUB_DEBUG_LOAD_BALANCER_ENABLED"), "true")

	return &LoadBalancer{
		strategies:       strategies,
		metricsProvider:  metricsProvider,
		selectionTracker: selectionTracker,
		weightTieBreaker: true,
		debug:            debug,
	}
}

// WithoutWeightTieBreaker disables OrderingWeight as a tie-breaker and uses
// input order when scores are equal.
func (lb *LoadBalancer) WithoutWeightTieBreaker() *LoadBalancer {
	lb.weightTieBreaker = false

	return lb
}

// WithRoundRobinHealthFilter moves recently failing channels behind healthy
// round-robin candidates after the round-robin order is calculated.
func (lb *LoadBalancer) WithRoundRobinHealthFilter(filter *RoundRobinHealthStrategy) *LoadBalancer {
	lb.roundRobinHealthFilter = filter

	return lb
}

// candidateScore holds a candidate and its calculated score.
type candidateScore struct {
	candidate *ChannelModelsCandidate
	score     float64
	index     int
	metrics   *biz.AggregatedMetrics
}

// Sort sorts candidates according to the configured strategies.
// Returns a new slice with the top topK candidates sorted by descending priority.
// A topK of zero or less, or one at least len(candidates), sorts every candidate.
func (lb *LoadBalancer) Sort(ctx context.Context, candidates []*ChannelModelsCandidate, model string, stream bool, topK int) []*ChannelModelsCandidate {
	return lb.sort(ctx, candidates, model, stream, topK, true)
}

// SortWithoutTracking sorts candidates without recording a channel selection.
// It is used when a sticky candidate is already selected and the remaining
// candidates are only fallbacks.
func (lb *LoadBalancer) SortWithoutTracking(ctx context.Context, candidates []*ChannelModelsCandidate, model string, stream bool, topK int) []*ChannelModelsCandidate {
	return lb.sort(ctx, candidates, model, stream, topK, false)
}

// SortTopKWithoutTracking sorts candidates and keeps at most topK of them,
// instead of deriving topK from the retry policy. Callers use it when they need
// more candidates than the retry budget, for example to keep spare candidates
// for local (preflight) rejections that do not consume retries.
func (lb *LoadBalancer) SortTopKWithoutTracking(
	ctx context.Context,
	candidates []*ChannelModelsCandidate,
	model string,
	stream bool,
	topK int,
) []*ChannelModelsCandidate {
	return lb.sortWithLimit(ctx, candidates, model, stream, false, topK)
}

// TrackSelection records a selected channel.
func (lb *LoadBalancer) TrackSelection(candidate *ChannelModelsCandidate) {
	if candidate != nil && candidate.Channel != nil && lb.selectionTracker != nil {
		lb.selectionTracker.IncrementChannelSelection(candidate.Channel.ID)
	}
}

// sort orders candidates, recording a channel selection when trackSelection is set.
// The caller owns topK: it is a retry-policy decision, not a load-balancer one,
// so callers that need spare candidates can ask for more than the retry budget.
func (lb *LoadBalancer) sort(
	ctx context.Context,
	candidates []*ChannelModelsCandidate,
	model string,
	stream bool,
	topK int,
	trackSelection bool,
) []*ChannelModelsCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	return lb.sortWithLimit(ctx, candidates, model, stream, trackSelection, topK)
}

// sortWithLimit sorts candidates, keeping at most topK of them.
func (lb *LoadBalancer) sortWithLimit(
	ctx context.Context,
	candidates []*ChannelModelsCandidate,
	model string,
	stream bool,
	trackSelection bool,
	topK int,
) []*ChannelModelsCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	if topK <= 0 || topK > len(candidates) {
		topK = len(candidates)
	}

	// Add model information to context for circuit-breaker strategy
	ctx = contextWithRequestedModel(ctx, model)
	ctx = contextWithRequestStream(ctx, stream)

	// Use debug path if debug mode is enabled
	debugEnabled := IsDebugEnabled(ctx)
	if lb.debug || debugEnabled {
		return lb.sortWithDebug(ctx, candidates, model, topK, trackSelection)
	}

	// Production path - minimal overhead
	return lb.sortProduction(ctx, candidates, topK, trackSelection)
}

// sortProduction is the fast path without debug overhead.
// Uses partial sorting to efficiently get only the top k candidates.
func (lb *LoadBalancer) sortProduction(
	ctx context.Context,
	candidates []*ChannelModelsCandidate,
	topK int,
	trackSelection bool,
) []*ChannelModelsCandidate {
	scored := make([]candidateScore, len(candidates))
	for i, c := range candidates {
		metrics, _ := lb.channelMetrics(ctx, c.Channel)

		totalScore := 0.0
		// Apply all strategies
		for _, strategy := range lb.strategies {
			totalScore += strategy.Score(ctx, c.Channel, metrics)
		}

		scored[i] = candidateScore{
			candidate: c,
			score:     totalScore,
			index:     i,
			metrics:   metrics,
		}
	}

	sortK := topK
	if lb.roundRobinHealthFilter != nil {
		sortK = len(scored)
	}

	// Use partial sort to efficiently get top k candidates
	// Sort by total score descending (higher score = higher priority)
	// When scores are equal, use OrderingWeight as tie-breaker (higher weight = higher priority)
	// Do NOT use channel ID as tie-breaker to avoid deterministic ordering that causes uneven distribution
	partial.SortFunc(scored, sortK, func(a, b candidateScore) int {
		if a.score > b.score {
			return -1
		} else if a.score < b.score {
			return 1
		}

		if lb.weightTieBreaker && a.candidate != nil && b.candidate != nil && a.candidate.Channel != nil && b.candidate.Channel != nil {
			if a.candidate.Channel.OrderingWeight > b.candidate.Channel.OrderingWeight {
				return -1
			} else if a.candidate.Channel.OrderingWeight < b.candidate.Channel.OrderingWeight {
				return 1
			}
		}

		if !lb.weightTieBreaker {
			if a.index < b.index {
				return -1
			} else if a.index > b.index {
				return 1
			}
		}

		return 0
	})

	selected := scored[:sortK]
	if lb.roundRobinHealthFilter != nil {
		selected = lb.prioritizeHealthyRoundRobinScores(ctx, selected)
	}
	if len(selected) > topK {
		selected = selected[:topK]
	}

	// Extract top k sorted candidates
	result := lo.Map(selected, func(ch candidateScore, _ int) *ChannelModelsCandidate { return ch.candidate })

	// Increment selection count for the top candidate to ensure subsequent
	// concurrent requests see the updated count and select different channels
	if trackSelection && len(result) > 0 {
		lb.TrackSelection(result[0])
	}

	return result
}

func (lb *LoadBalancer) prioritizeHealthyRoundRobinScores(ctx context.Context, scored []candidateScore) []candidateScore {
	if lb.roundRobinHealthFilter == nil {
		return scored
	}

	healthy := make([]candidateScore, 0, len(scored))
	unhealthy := make([]candidateScore, 0)
	hardUnavailable := make([]candidateScore, 0)

	for _, score := range scored {
		if isHardUnavailableScore(score.score) {
			hardUnavailable = append(hardUnavailable, score)
			continue
		}

		if lb.roundRobinHealthFilter.IsUnhealthy(score.metrics) {
			unhealthy = append(unhealthy, score)
			continue
		}

		healthy = append(healthy, score)
	}

	result := append(healthy, unhealthy...)
	return append(result, hardUnavailable...)
}

func isHardUnavailableScore(score float64) bool {
	return score <= rateLimitExhaustedScore/2
}

// sortWithDebug is the debug path with detailed logging.
// Uses partial sorting to efficiently get only the top k candidates.
func (lb *LoadBalancer) sortWithDebug(
	ctx context.Context,
	candidates []*ChannelModelsCandidate,
	model string,
	topK int,
	trackSelection bool,
) []*ChannelModelsCandidate {
	startTime := time.Now()

	// Calculate detailed scores for each candidate
	decisions := make([]ChannelDecision, len(candidates))
	for i, c := range candidates {
		metrics, err := lb.channelMetrics(ctx, c.Channel)
		if err != nil {
			log.Warn(ctx, "failed to get channel metrics, strategies use neutral scores",
				log.Int("channel_id", c.Channel.ID),
				log.String("channel_name", c.Channel.Name),
				log.Cause(err),
			)
		}

		totalScore := 0.0
		strategyScores := make([]StrategyScore, 0, len(lb.strategies))

		// Apply all strategies and collect detailed scores
		for _, strategy := range lb.strategies {
			scoreStart := time.Now()
			score, strategyScore := strategy.ScoreWithDebug(ctx, c.Channel, metrics)
			strategyScore.Duration = time.Since(scoreStart)
			strategyScores = append(strategyScores, strategyScore)
			totalScore += score
		}

		decisions[i] = ChannelDecision{
			Channel:        c.Channel,
			OriginalIndex:  i,
			TotalScore:     totalScore,
			StrategyScores: strategyScores,
			FinalRank:      0, // Will be set after sorting
			metrics:        metrics,
		}
	}

	// Use partial sort to efficiently get top k candidates
	// Sort by total score descending (higher score = higher priority)
	// When scores are equal, use OrderingWeight as tie-breaker (higher weight = higher priority)
	// Do NOT use channel ID as tie-breaker to avoid deterministic ordering that causes uneven distribution
	sortK := topK
	if lb.roundRobinHealthFilter != nil {
		sortK = len(decisions)
	}

	partial.SortFunc(decisions, sortK, func(a, b ChannelDecision) int {
		if a.TotalScore > b.TotalScore {
			return -1
		} else if a.TotalScore < b.TotalScore {
			return 1
		}

		if lb.weightTieBreaker && a.Channel != nil && b.Channel != nil {
			if a.Channel.OrderingWeight > b.Channel.OrderingWeight {
				return -1
			} else if a.Channel.OrderingWeight < b.Channel.OrderingWeight {
				return 1
			}
		}

		if !lb.weightTieBreaker {
			if a.OriginalIndex < b.OriginalIndex {
				return -1
			} else if a.OriginalIndex > b.OriginalIndex {
				return 1
			}
		}

		return 0
	})

	selected := decisions[:sortK]
	if lb.roundRobinHealthFilter != nil {
		selected = lb.prioritizeHealthyRoundRobinDecisions(ctx, selected)
	}
	if len(selected) > topK {
		selected = selected[:topK]
	}

	// Set final ranks for top k
	for i := range selected {
		selected[i].FinalRank = i + 1
	}

	// Log the decision with all details (only top k)
	lb.logDecision(ctx, candidates, model, selected, topK, time.Since(startTime))

	result := lo.Map(selected, func(decision ChannelDecision, _ int) *ChannelModelsCandidate {
		// Find the corresponding candidate by channel ID
		for _, c := range candidates {
			if c.Channel.ID == decision.Channel.ID {
				return c
			}
		}

		return nil
	})

	// Increment selection count for the top candidate to ensure subsequent
	// concurrent requests see the updated count and select different channels
	if trackSelection && len(result) > 0 {
		lb.TrackSelection(result[0])
	}

	return result
}

func (lb *LoadBalancer) prioritizeHealthyRoundRobinDecisions(ctx context.Context, decisions []ChannelDecision) []ChannelDecision {
	if lb.roundRobinHealthFilter == nil {
		return decisions
	}

	healthy := make([]ChannelDecision, 0, len(decisions))
	unhealthy := make([]ChannelDecision, 0)
	hardUnavailable := make([]ChannelDecision, 0)

	for _, decision := range decisions {
		if isHardUnavailableScore(decision.TotalScore) {
			hardUnavailable = append(hardUnavailable, decision)
			continue
		}

		if lb.roundRobinHealthFilter.IsUnhealthy(decision.metrics) {
			unhealthy = append(unhealthy, decision)
			continue
		}

		healthy = append(healthy, decision)
	}

	result := append(healthy, unhealthy...)
	return append(result, hardUnavailable...)
}

// channelMetrics fetches the channel's metrics snapshot once so every strategy
// scores the candidate from the same data. It returns nil metrics when the load
// balancer has no metrics provider.
func (lb *LoadBalancer) channelMetrics(ctx context.Context, channel *biz.Channel) (*biz.AggregatedMetrics, error) {
	if lb.metricsProvider == nil || channel == nil {
		return nil, nil
	}

	metrics, err := lb.metricsProvider.GetChannelMetrics(ctx, channel.ID)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// logDecision logs the complete load balancing decision.
func (lb *LoadBalancer) logDecision(ctx context.Context, candidates []*ChannelModelsCandidate, model string, decisions []ChannelDecision, topK int, totalDuration time.Duration) {
	// Log summary
	if len(decisions) > 0 {
		topChannel := decisions[0]
		log.Info(ctx, "Load balancing decision completed",
			log.Int("total_channels", len(candidates)),
			log.Int("selected_channels", topK),
			log.Duration("duration", totalDuration),
			log.Int("top_channel_id", topChannel.Channel.ID),
			log.String("top_channel_name", topChannel.Channel.Name),
			log.Float64("top_channel_score", topChannel.TotalScore),
			log.String("model", model),
		)
	}

	// Log individual channel details
	for _, info := range decisions {
		// Create a simplified log entry with strategy breakdown
		strategySummary := make(map[string]any)
		for _, s := range info.StrategyScores {
			strategySummary[s.StrategyName] = map[string]any{
				"score":    s.Score,
				"duration": s.Duration,
			}
		}

		log.Info(ctx, "Channel load balancing details",
			log.Int("channel_id", info.Channel.ID),
			log.String("channel_name", info.Channel.Name),
			log.Float64("total_score", info.TotalScore),
			log.Int("final_rank", info.FinalRank),
			log.Any("strategy_breakdown", strategySummary),
			log.String("model", model),
		)
	}
}
