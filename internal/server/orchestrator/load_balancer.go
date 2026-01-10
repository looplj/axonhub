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

// LoadBalancer applies multiple strategies to sort channels by priority.
type LoadBalancer struct {
	strategies       []LoadBalanceStrategy
	systemService    RetryPolicyProvider
	selectionTracker ChannelSelectionTracker
	debug            bool
}

// NewLoadBalancer creates a new load balancer with the given strategies.
// Strategies are applied in order, with earlier strategies having higher weight.
func NewLoadBalancer(systemService RetryPolicyProvider, selectionTracker ChannelSelectionTracker, strategies ...LoadBalanceStrategy) *LoadBalancer {
	debug := strings.EqualFold(os.Getenv("AXONHUB_DEBUG_LOAD_BALANCER_ENABLED"), "true")

	return &LoadBalancer{
		strategies:       strategies,
		systemService:    systemService,
		selectionTracker: selectionTracker,
		debug:            debug,
	}
}

// candidateScore holds a candidate and its calculated score.
type candidateScore struct {
	candidate *ChannelModelCandidate
	score     float64
}

// Sort sorts candidates according to the configured strategies.
// Returns a new slice with top k candidates sorted by descending priority.
// The top k value is calculated internally based on the retry policy.
func (lb *LoadBalancer) Sort(ctx context.Context, candidates []*ChannelModelCandidate, model string) []*ChannelModelCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	if len(candidates) == 1 {
		return candidates
	}

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
func (lb *LoadBalancer) sortProduction(ctx context.Context, candidates []*ChannelModelCandidate, topK int) []*ChannelModelCandidate {
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
		}
	}

	// Use partial sort to efficiently get top k candidates
	// Sort by total score descending (higher score = higher priority)
	// When scores are equal, use OrderingWeight as tie-breaker (higher weight = higher priority)
	// Do NOT use channel ID as tie-breaker to avoid deterministic ordering that causes uneven distribution
	partial.SortFunc(scored, topK, func(a, b candidateScore) int {
		if a.score > b.score {
			return -1
		} else if a.score < b.score {
			return 1
		}

		if a.candidate != nil && b.candidate != nil && a.candidate.Channel != nil && b.candidate.Channel != nil {
			if a.candidate.Channel.OrderingWeight > b.candidate.Channel.OrderingWeight {
				return -1
			} else if a.candidate.Channel.OrderingWeight < b.candidate.Channel.OrderingWeight {
				return 1
			}
		}

		// When score and weight are equal, return 0 to preserve original order (non-deterministic)
		return 0
	})

	// Extract top k sorted candidates
	result := lo.Map(scored[:topK], func(ch candidateScore, _ int) *ChannelModelCandidate { return ch.candidate })

	// Increment selection count for the top candidate to ensure subsequent
	// concurrent requests see the updated count and select different channels
	if len(result) > 0 && result[0] != nil && result[0].Channel != nil && lb.selectionTracker != nil {
		lb.selectionTracker.IncrementChannelSelection(result[0].Channel.ID)
	}

	return result
}

// sortWithDebug is the debug path with detailed logging.
// Uses partial sorting to efficiently get only the top k candidates.
func (lb *LoadBalancer) sortWithDebug(ctx context.Context, candidates []*ChannelModelCandidate, model string, topK int) []*ChannelModelCandidate {
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
			TotalScore:     totalScore,
			StrategyScores: strategyScores,
			FinalRank:      0, // Will be set after sorting
		}
	}

	// Use partial sort to efficiently get top k candidates
	// Sort by total score descending (higher score = higher priority)
	// When scores are equal, use OrderingWeight as tie-breaker (higher weight = higher priority)
	// Do NOT use channel ID as tie-breaker to avoid deterministic ordering that causes uneven distribution
	partial.SortFunc(decisions, topK, func(a, b ChannelDecision) int {
		if a.TotalScore > b.TotalScore {
			return -1
		} else if a.TotalScore < b.TotalScore {
			return 1
		}

		if a.Channel != nil && b.Channel != nil {
			if a.Channel.OrderingWeight > b.Channel.OrderingWeight {
				return -1
			} else if a.Channel.OrderingWeight < b.Channel.OrderingWeight {
				return 1
			}
		}

		// When score and weight are equal, return 0 to preserve original order (non-deterministic)
		return 0
	})

	// Set final ranks for top k
	for i := range topK {
		decisions[i].FinalRank = i + 1
	}

	// Log the decision with all details (only top k)
	lb.logDecision(ctx, candidates, model, decisions[:topK], topK, time.Since(startTime))

	result := lo.Map(decisions[:topK], func(decision ChannelDecision, _ int) *ChannelModelCandidate {
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

// calculateTopK determines how many candidates to select based on retry policy.
func (lb *LoadBalancer) calculateTopK(ctx context.Context, candidates []*ChannelModelCandidate) int {
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
func (lb *LoadBalancer) logDecision(ctx context.Context, candidates []*ChannelModelCandidate, model string, decisions []ChannelDecision, topK int, totalDuration time.Duration) {
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
