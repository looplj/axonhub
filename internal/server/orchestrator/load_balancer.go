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
type LoadBalanceStrategy interface {
	// Score calculates a score for a channel. Higher scores indicate higher priority.
	// Returns a score between 0 and 1000.
	// This is the production path with minimal overhead.
	Score(ctx context.Context, channel *biz.Channel) float64

	// ScoreWithDebug calculates a score with detailed debug information.
	// Returns the score and a StrategyScore with debug details.
	// This should have identical logic to Score() except for debug logging.
	ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore)

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

// RetryPolicyProvider interface defines the methods needed from RetryPolicyProvider.
type RetryPolicyProvider interface {
	RetryPolicyOrDefault(ctx context.Context) *biz.RetryPolicy
}

// Save the requested model ID in the context, to let model aware strategy use it, e.g. circuit breaker.
type modelContextKey struct{}
type streamContextKey struct{}
type quotaLimitTypeContextKey struct{}

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

func contextWithQuotaLimitType(ctx context.Context, limitType string) context.Context {
	return context.WithValue(ctx, quotaLimitTypeContextKey{}, limitType)
}

func quotaLimitTypeFromContext(ctx context.Context) string {
	if lt, ok := ctx.Value(quotaLimitTypeContextKey{}).(string); ok {
		return lt
	}
	return ""
}

// LoadBalancer applies multiple strategies to sort channels by priority.
type LoadBalancer struct {
	strategies             []LoadBalanceStrategy
	systemService          RetryPolicyProvider
	selectionTracker       ChannelSelectionTracker
	weightTieBreaker       bool
	roundRobinHealthFilter *RoundRobinHealthStrategy
	debug                  bool
}

// NewLoadBalancer creates a new load balancer with the given strategies.
// All strategy scores are summed with equal weight; registration order does not affect scoring.
func NewLoadBalancer(systemService RetryPolicyProvider, selectionTracker ChannelSelectionTracker, strategies ...LoadBalanceStrategy) *LoadBalancer {
	debug := strings.EqualFold(os.Getenv("AXONHUB_DEBUG_LOAD_BALANCER_ENABLED"), "true")

	return &LoadBalancer{
		strategies:       strategies,
		systemService:    systemService,
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
}

// Sort sorts candidates according to the configured strategies.
// Returns a new slice with top k candidates sorted by descending priority.
// The top k value is calculated internally based on the retry policy.
func (lb *LoadBalancer) Sort(ctx context.Context, candidates []*ChannelModelsCandidate, model string, stream bool) []*ChannelModelsCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	// Add model information to context for circuit-breaker strategy
	ctx = contextWithRequestedModel(ctx, model)
	ctx = contextWithRequestStream(ctx, stream)

	// Calculate topK based on retry policy
	topK := lb.calculateTopK(ctx, candidates)

	// Use debug path if debug mode is enabled
	debugEnabled := IsDebugEnabled(ctx)
	if lb.debug || debugEnabled {
		return lb.sortWithDebug(ctx, candidates, model, topK)
	}

	// Production path - minimal overhead
	return lb.sortProduction(ctx, candidates, topK)
}

// sortProduction is the fast path without debug overhead.
// Uses partial sorting to efficiently get only the top k candidates.
func (lb *LoadBalancer) sortProduction(ctx context.Context, candidates []*ChannelModelsCandidate, topK int) []*ChannelModelsCandidate {
	scored := make([]candidateScore, len(candidates))
	for i, c := range candidates {
		totalScore := 0.0
		// Apply all strategies
		for _, strategy := range lb.strategies {
			totalScore += strategy.Score(ctx, c.Channel)
		}

		scored[i] = candidateScore{
			candidate: c,
			score:     totalScore,
			index:     i,
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
	if len(result) > 0 && result[0] != nil && result[0].Channel != nil && lb.selectionTracker != nil {
		lb.selectionTracker.IncrementChannelSelection(result[0].Channel.ID)
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

		if score.candidate != nil && score.candidate.Channel != nil && lb.roundRobinHealthFilter.IsUnhealthy(ctx, score.candidate.Channel) {
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
func (lb *LoadBalancer) sortWithDebug(ctx context.Context, candidates []*ChannelModelsCandidate, model string, topK int) []*ChannelModelsCandidate {
	startTime := time.Now()

	// Calculate detailed scores for each candidate
	decisions := make([]ChannelDecision, len(candidates))
	for i, c := range candidates {
		totalScore := 0.0
		strategyScores := make([]StrategyScore, 0, len(lb.strategies))

		// Apply all strategies and collect detailed scores
		for _, strategy := range lb.strategies {
			scoreStart := time.Now()
			score, strategyScore := strategy.ScoreWithDebug(ctx, c.Channel)
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
	if len(result) > 0 && result[0] != nil && result[0].Channel != nil && lb.selectionTracker != nil {
		lb.selectionTracker.IncrementChannelSelection(result[0].Channel.ID)
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

		if decision.Channel != nil && lb.roundRobinHealthFilter.IsUnhealthy(ctx, decision.Channel) {
			unhealthy = append(unhealthy, decision)
			continue
		}

		healthy = append(healthy, decision)
	}

	result := append(healthy, unhealthy...)
	return append(result, hardUnavailable...)
}

// calculateTopK determines how many candidates to select based on retry policy.
func (lb *LoadBalancer) calculateTopK(ctx context.Context, candidates []*ChannelModelsCandidate) int {
	retryPolicy := lb.systemService.RetryPolicyOrDefault(ctx)

	// Calculate topK based on retry policy
	// If retry is enabled, we need 1 + MaxChannelRetries candidates
	// (1 for initial attempt + MaxChannelRetries for retries)
	// If retry is disabled, we only need 1 candidate
	topK := 1
	if retryPolicy.Enabled {
		topK = 1 + retryPolicy.MaxChannelRetries
	}

	// Normalize topK: if topK <= 0 or topK >= len(candidates), sort all
	// This is to ensure we don't sort more candidates than available
	if topK <= 0 || topK >= len(candidates) {
		topK = len(candidates)
	}

	return topK
}

// logDecision logs the complete load balancing decision.
func (lb *LoadBalancer) logDecision(ctx context.Context, candidates []*ChannelModelsCandidate, model string, decisions []ChannelDecision, topK int, totalDuration time.Duration) {
	// Log summary
	if len(decisions) > 0 {
		topChannel := decisions[0]
		retryPolicy := lb.systemService.RetryPolicyOrDefault(ctx)
		log.Info(ctx, "Load balancing decision completed",
			log.Int("total_channels", len(candidates)),
			log.Int("selected_channels", topK),
			log.Bool("retry_enabled", retryPolicy.Enabled),
			log.Int("max_channel_retries", retryPolicy.MaxChannelRetries),
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
