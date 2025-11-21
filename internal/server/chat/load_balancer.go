package chat

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

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
	Details map[string]interface{}
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

// LoadBalancer applies multiple strategies to sort channels by priority.
type LoadBalancer struct {
	strategies []LoadBalanceStrategy
	debug      bool
}

// NewLoadBalancer creates a new load balancer with the given strategies.
// Strategies are applied in order, with earlier strategies having higher weight.
func NewLoadBalancer(strategies ...LoadBalanceStrategy) *LoadBalancer {
	debug := strings.EqualFold(os.Getenv("AXONHUB_LOAD_BALANCER_DEBUG"), "true")

	return &LoadBalancer{
		strategies: strategies,
		debug:      debug,
	}
}

// channelScore holds a channel and its calculated score.
type channelScore struct {
	channel *biz.Channel
	score   float64
}

// Sort sorts channels according to the configured strategies.
// Returns a new slice with channels sorted by descending priority.
func (lb *LoadBalancer) Sort(ctx context.Context, channels []*biz.Channel, model string) []*biz.Channel {
	if len(channels) == 0 {
		return channels
	}

	if len(channels) == 1 {
		return channels
	}

	// Use debug path if debug mode is enabled
	debugEnabled := IsDebugEnabled(ctx)
	if lb.debug || debugEnabled {
		return lb.sortWithDebug(ctx, channels, model)
	}

	// Production path - minimal overhead
	return lb.sortProduction(ctx, channels)
}

// sortProduction is the fast path without debug overhead.
func (lb *LoadBalancer) sortProduction(ctx context.Context, channels []*biz.Channel) []*biz.Channel {
	scored := make([]channelScore, len(channels))
	for i, ch := range channels {
		totalScore := 0.0
		// Apply all strategies
		for _, strategy := range lb.strategies {
			totalScore += strategy.Score(ctx, ch)
		}

		scored[i] = channelScore{
			channel: ch,
			score:   totalScore,
		}
	}

	// Sort by total score descending (higher score = higher priority)
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted channels
	return lo.Map(scored, func(ch channelScore, _ int) *biz.Channel { return ch.channel })
}

// sortWithDebug is the debug path with detailed logging.
func (lb *LoadBalancer) sortWithDebug(ctx context.Context, channels []*biz.Channel, model string) []*biz.Channel {
	startTime := time.Now()

	// Calculate detailed scores for each channel
	decisions := make([]ChannelDecision, len(channels))
	for i, ch := range channels {
		totalScore := 0.0
		strategyScores := make([]StrategyScore, 0, len(lb.strategies))

		// Apply all strategies and collect detailed scores
		for _, strategy := range lb.strategies {
			scoreStart := time.Now()
			score, strategyScore := strategy.ScoreWithDebug(ctx, ch)
			strategyScore.Duration = time.Since(scoreStart)
			strategyScores = append(strategyScores, strategyScore)
			totalScore += score
		}

		decisions[i] = ChannelDecision{
			Channel:        ch,
			TotalScore:     totalScore,
			StrategyScores: strategyScores,
			FinalRank:      0, // Will be set after sorting
		}
	}

	// Sort by total score descending (higher score = higher priority)
	sort.SliceStable(decisions, func(i, j int) bool {
		return decisions[i].TotalScore > decisions[j].TotalScore
	})

	// Set final ranks
	for i := range decisions {
		decisions[i].FinalRank = i + 1
	}

	// Log the decision with all details
	lb.logDecision(ctx, channels, model, decisions, time.Since(startTime))

	return lo.Map(decisions, func(decision ChannelDecision, _ int) *biz.Channel { return decision.Channel })
}

// logDecision logs the complete load balancing decision.
func (lb *LoadBalancer) logDecision(ctx context.Context, channels []*biz.Channel, model string, decisions []ChannelDecision, totalDuration time.Duration) {
	// Log summary
	if len(decisions) > 0 {
		topChannel := decisions[0]
		log.Info(ctx, "Load balancing decision completed",
			log.Int("channel_count", len(channels)),
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
		strategySummary := make(map[string]interface{})
		for _, s := range info.StrategyScores {
			strategySummary[s.StrategyName] = map[string]interface{}{
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
