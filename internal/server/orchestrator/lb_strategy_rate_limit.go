package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// rateLimitExhaustedScore is the penalty applied to channels that have hit a
// hard limit (RPM, TPM, queue) or are in cooldown. It must dominate the maximum
// positive sum from all other strategies so exhausted channels rank last while
// still being available as fallback candidates.
const rateLimitExhaustedScore = -10000

// queueingScoreCeiling caps the score of a hard-mode channel that has spilled
// into its FIFO queue. With a 30% ceiling, a fully-idle peer keeps a clear
// advantage so the load balancer prefers it over the queueing candidate.
const queueingScoreCeiling = 0.3

// RateLimitAwareStrategy adjusts channel scores based on configured RPM/TPM rate
// limits and per-channel concurrency limits. It composes two sub-scores
// (RPM/TPM and concurrency), returning the stricter one. Channels at any hard
// limit are returned as rateLimitExhaustedScore so the load balancer drops them
// to last.
type RateLimitAwareStrategy struct {
	requestTracker *ChannelRequestTracker
	limiterManager *ChannelLimiterManager
	maxScore       float64
}

// NewRateLimitAwareStrategy creates a new rate limit aware load balancing strategy.
// limiterManager may be nil for tests that do not exercise the concurrency
// dimension; in that case the concurrency sub-score collapses to maxScore.
func NewRateLimitAwareStrategy(tracker *ChannelRequestTracker, limiterManager *ChannelLimiterManager) *RateLimitAwareStrategy {
	return &RateLimitAwareStrategy{
		requestTracker: tracker,
		limiterManager: limiterManager,
		maxScore:       100.0,
	}
}

// Name returns the strategy name.
func (s *RateLimitAwareStrategy) Name() string {
	return "RateLimitAware"
}

// Score is the production-path scorer with minimal overhead.
func (s *RateLimitAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	score, _ := s.score(channel, nil)
	return score
}

// ScoreWithDebug returns the same score as Score along with detailed diagnostic info.
func (s *RateLimitAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	startTime := time.Now()

	details := map[string]any{
		"channel_id": channel.ID,
	}

	score, exhaustedBy := s.score(channel, details)

	if exhaustedBy != "" {
		details["exhausted"] = true
		details["exhausted_by"] = exhaustedBy
	}

	details["score"] = score

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "RateLimitAwareStrategy: scoring",
			log.Int("channel_id", channel.ID),
			log.String("channel_name", channel.Name),
			log.Float64("score", score),
			log.Any("details", details),
		)
	}

	return score, StrategyScore{
		StrategyName: s.Name(),
		Score:        score,
		Details:      details,
		Duration:     time.Since(startTime),
	}
}

// score is the unified scorer. When details is nil, no diagnostic info is recorded.
// Returns the final score and, if exhausted, the dimension that exhausted ("cooldown",
// "rpm_or_tpm", or "concurrency"); empty string means not exhausted.
func (s *RateLimitAwareStrategy) score(channel *biz.Channel, details map[string]any) (float64, string) {
	if until, cooling := s.requestTracker.GetCooldownUntil(channel.ID); cooling {
		if details != nil {
			details["reason"] = "channel_in_cooldown"
			details["cooldown_until"] = until.Format(time.RFC3339)
		}

		return rateLimitExhaustedScore, "cooldown"
	}

	rpmTpmScore, exhausted := s.scoreRPMTPM(channel, details)
	if exhausted {
		return rateLimitExhaustedScore, "rpm_or_tpm"
	}

	concurrencyScore, exhausted := s.scoreConcurrency(channel, details)
	if exhausted {
		return rateLimitExhaustedScore, "concurrency"
	}

	if details != nil {
		details["score_rpm_tpm"] = rpmTpmScore
		details["score_concurrency"] = concurrencyScore
	}

	return min(rpmTpmScore, concurrencyScore), ""
}

// scoreRPMTPM applies the requests-per-minute / tokens-per-minute sub-score.
// Returns (score, exhausted). When exhausted is true, the caller short-circuits
// to rateLimitExhaustedScore.
func (s *RateLimitAwareStrategy) scoreRPMTPM(channel *biz.Channel, details map[string]any) (float64, bool) {
	if channel.Settings == nil || channel.Settings.RateLimit == nil {
		if details != nil {
			details["rpm_tpm_reason"] = "no_rpm_tpm_configured"
		}

		return s.maxScore, false
	}

	rl := channel.Settings.RateLimit

	var maxRatio float64

	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.requestTracker.GetRequestCount(channel.ID)

		if details != nil {
			details["rpm_limit"] = *rl.RPM
			details["rpm_current"] = rpm
		}

		if rpm >= *rl.RPM {
			if details != nil {
				details["rpm_exhausted"] = true
			}

			return 0, true
		}

		ratio := float64(rpm) / float64(*rl.RPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	if rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.requestTracker.GetTokenCount(channel.ID)

		if details != nil {
			details["tpm_limit"] = *rl.TPM
			details["tpm_current"] = tpm
		}

		if tpm >= *rl.TPM {
			if details != nil {
				details["tpm_exhausted"] = true
			}

			return 0, true
		}

		ratio := float64(tpm) / float64(*rl.TPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	if details != nil {
		details["rpm_tpm_max_ratio"] = maxRatio
	}

	return scaleScore(s.maxScore, 1-maxRatio), false
}

// scoreConcurrency applies the per-channel concurrency sub-score using the
// limiter manager. Returns (score, exhausted).
//
// Behaviour:
//   - No limiter (channel has no MaxConcurrent): full score, never exhausted.
//   - Soft mode (queueSize == 0): full marks until inFlight >= capacity, then
//     exhausted (LB ranks last but middleware still admits the request).
//   - Hard mode (queueSize > 0): full score below capacity; once spilled into
//     the queue, score is capped at queueingScoreCeiling × maxScore and decays
//     with queue depth; exhausted only when the queue is also full.
func (s *RateLimitAwareStrategy) scoreConcurrency(channel *biz.Channel, details map[string]any) (float64, bool) {
	if s.limiterManager == nil {
		if details != nil {
			details["concurrency_reason"] = "no_limiter_manager"
		}

		return s.maxScore, false
	}

	inFlight, waiting, ok := s.limiterManager.Stats(channel.ID)
	if !ok {
		if details != nil {
			details["concurrency_reason"] = "channel_has_no_limiter"
		}

		return s.maxScore, false
	}

	// Defensive: a stale entry can briefly outlive a "limit disabled" config
	// change before the next GetOrCreate call drops it. Treat it as unlimited
	// rather than dereferencing nil rate-limit pointers.
	rl := channel.Settings.RateLimit
	if rl == nil || rl.MaxConcurrent == nil || *rl.MaxConcurrent <= 0 {
		if details != nil {
			details["concurrency_reason"] = "channel_limiter_disabled"
		}

		return s.maxScore, false
	}

	capacity := int(*rl.MaxConcurrent)

	queueSize := 0
	if rl.QueueSize != nil && *rl.QueueSize > 0 {
		queueSize = int(*rl.QueueSize)
	}

	if details != nil {
		details["concurrent_capacity"] = capacity
		details["concurrent_in_flight"] = inFlight
		details["concurrent_queue_size"] = queueSize
		details["concurrent_waiting"] = waiting
	}

	if queueSize > 0 {
		if inFlight+waiting >= capacity+queueSize {
			if details != nil {
				details["concurrent_exhausted"] = "queue_full"
			}

			return 0, true
		}

		// Hard mode is split into two ranges that must stay monotonic so the
		// LB never prefers a saturated channel over one with free capacity:
		//   - queueing:        [0, queueingScoreCeiling*maxScore]
		//   - below capacity:  [queueingScoreCeiling*maxScore, maxScore]
		queueCeiling := s.maxScore * queueingScoreCeiling

		if inFlight >= capacity {
			waitingRatio := float64(waiting) / float64(queueSize)

			if details != nil {
				details["concurrency_mode"] = "hard_queueing"
				details["concurrency_waiting_ratio"] = waitingRatio
			}

			return scaleScore(queueCeiling, 1-waitingRatio), false
		}

		ratio := float64(inFlight) / float64(capacity)

		if details != nil {
			details["concurrency_mode"] = "hard_below_capacity"
			details["concurrency_inflight_ratio"] = ratio
		}

		return queueCeiling + scaleScore(s.maxScore-queueCeiling, 1-ratio), false
	}

	if inFlight >= capacity {
		if details != nil {
			details["concurrent_exhausted"] = "soft_capacity"
		}

		return 0, true
	}

	ratio := float64(inFlight) / float64(capacity)

	if details != nil {
		details["concurrency_mode"] = "soft"
		details["concurrency_inflight_ratio"] = ratio
	}

	return scaleScore(s.maxScore, 1-ratio), false
}

// scaleScore clamps base * factor to [0, +inf). factor < 0 yields 0.
func scaleScore(base, factor float64) float64 {
	score := base * factor
	if score < 0 {
		score = 0
	}

	return score
}
