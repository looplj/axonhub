package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// rateLimitExhaustedScore is the penalty score for channels that have exhausted their rate limits
// or are in cooldown. Must exceed the maximum possible positive score sum from all other strategies
// (currently ~1530: Trace=1000 + Error=200 + WeightRR=150 + Latency=80 + RateLimit=100)
// so that exhausted channels always rank last, while still remaining as fallback candidates.
const rateLimitExhaustedScore = -10000

// RateLimitAwareStrategy adjusts channel scores based on configured RPM/TPM rate limits and concurrency limits.
// Channels that have exhausted their rate limits receive a heavily negative score to be ranked last.
type RateLimitAwareStrategy struct {
	requestTracker    *ChannelRequestTracker
	connectionTracker ConnectionTracker
	maxScore          float64
}

// NewRateLimitAwareStrategy creates a new rate limit aware load balancing strategy.
func NewRateLimitAwareStrategy(tracker *ChannelRequestTracker, connectionTracker ConnectionTracker) *RateLimitAwareStrategy {
	return &RateLimitAwareStrategy{
		requestTracker:    tracker,
		connectionTracker: connectionTracker,
		maxScore:          100.0,
	}
}

// Name returns the strategy name.
func (s *RateLimitAwareStrategy) Name() string {
	return "RateLimitAware"
}

func (s *RateLimitAwareStrategy) resolveConcurrencyLimit(channel *biz.Channel) (limit int64, source string, configured bool) {
	if channel.Settings != nil && channel.Settings.RateLimit != nil {
		if rl := channel.Settings.RateLimit; rl.MaxConcurrent != nil && *rl.MaxConcurrent > 0 {
			return *rl.MaxConcurrent, "rate_limit_config", true
		}
	}

	if s.connectionTracker == nil {
		return 0, "", false
	}

	limit = int64(s.connectionTracker.GetMaxConnections(channel.ID))
	if limit <= 0 {
		return 0, "", false
	}

	return limit, "connection_tracker_default", false
}

// Score calculates the score based on channel rate limit usage.
// This is the production path with minimal overhead.
func (s *RateLimitAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Check if channel is in cooldown (429 Retry-After)
	if s.requestTracker.IsCoolingDown(channel.ID) {
		return rateLimitExhaustedScore
	}

	settings := channel.Settings
	if settings == nil || settings.RateLimit == nil {
		if s.connectionTracker != nil {
			if concurrencyLimit, _, _ := s.resolveConcurrencyLimit(channel); concurrencyLimit > 0 {
				concurrent := s.connectionTracker.GetActiveConnections(channel.ID)
				if int64(concurrent) >= concurrencyLimit {
					return rateLimitExhaustedScore
				}

				ratio := float64(concurrent) / float64(concurrencyLimit)

				score := s.maxScore * (1 - ratio)
				if score < 0 {
					score = 0
				}

				return score
			}
		}

		return s.maxScore
	}

	rl := settings.RateLimit

	var maxRatio float64

	// Check RPM (Requests Per Minute)
	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.requestTracker.GetRequestCount(channel.ID)
		if rpm >= *rl.RPM {
			return rateLimitExhaustedScore
		}

		ratio := float64(rpm) / float64(*rl.RPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	// Check TPM (Tokens Per Minute)
	if rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.requestTracker.GetTokenCount(channel.ID)
		if tpm >= *rl.TPM {
			return rateLimitExhaustedScore
		}

		ratio := float64(tpm) / float64(*rl.TPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	if s.connectionTracker != nil {
		if concurrencyLimit, _, _ := s.resolveConcurrencyLimit(channel); concurrencyLimit > 0 {
			concurrent := s.connectionTracker.GetActiveConnections(channel.ID)
			if int64(concurrent) >= concurrencyLimit {
				return rateLimitExhaustedScore
			}

			ratio := float64(concurrent) / float64(concurrencyLimit)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	score := s.maxScore * (1 - maxRatio)
	if score < 0 {
		score = 0
	}

	return score
}

// ScoreWithDebug calculates the score with detailed debug information.
func (s *RateLimitAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	startTime := time.Now()

	details := map[string]any{
		"channel_id": channel.ID,
	}

	// Check if channel is in cooldown (429 Retry-After)
	if until, ok := s.requestTracker.GetCooldownUntil(channel.ID); ok {
		score := float64(rateLimitExhaustedScore)
		details["reason"] = "channel_in_cooldown"
		details["exhausted"] = true
		details["cooldown_until"] = until.Format(time.RFC3339)

		return score, StrategyScore{
			StrategyName: s.Name(),
			Score:        score,
			Details:      details,
			Duration:     time.Since(startTime),
		}
	}

	settings := channel.Settings

	if settings == nil || settings.RateLimit == nil {
		if concurrencyLimit, source, _ := s.resolveConcurrencyLimit(channel); concurrencyLimit > 0 && s.connectionTracker != nil {
			concurrent := s.connectionTracker.GetActiveConnections(channel.ID)
			details["concurrent_limit"] = concurrencyLimit
			details["concurrent_current"] = concurrent
			details["concurrency_limit_source"] = source

			if int64(concurrent) >= concurrencyLimit {
				score := float64(rateLimitExhaustedScore)
				details["concurrent_exhausted"] = true
				details["exhausted"] = true
				details["score"] = score

				return score, StrategyScore{
					StrategyName: s.Name(),
					Score:        score,
					Details:      details,
					Duration:     time.Since(startTime),
				}
			}

			maxRatio := float64(concurrent) / float64(concurrencyLimit)

			score := s.maxScore * (1 - maxRatio)
			if score < 0 {
				score = 0
			}

			details["max_ratio"] = maxRatio
			details["score"] = score
			details["reason"] = "default_connection_limit_fallback"

			return score, StrategyScore{
				StrategyName: s.Name(),
				Score:        score,
				Details:      details,
				Duration:     time.Since(startTime),
			}
		}

		score := s.maxScore
		details["reason"] = "no_rate_limit_configured"

		return score, StrategyScore{
			StrategyName: s.Name(),
			Score:        score,
			Details:      details,
			Duration:     time.Since(startTime),
		}
	}

	rl := settings.RateLimit

	var maxRatio float64

	exhausted := false

	// Check RPM
	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.requestTracker.GetRequestCount(channel.ID)
		details["rpm_limit"] = *rl.RPM
		details["rpm_current"] = rpm

		if rpm >= *rl.RPM {
			exhausted = true
			details["rpm_exhausted"] = true
		} else {
			ratio := float64(rpm) / float64(*rl.RPM)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	// Check TPM
	if rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.requestTracker.GetTokenCount(channel.ID)
		details["tpm_limit"] = *rl.TPM
		details["tpm_current"] = tpm

		if tpm >= *rl.TPM {
			exhausted = true
			details["tpm_exhausted"] = true
		} else {
			ratio := float64(tpm) / float64(*rl.TPM)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	// Check concurrent requests using explicit MaxConcurrent first, then default tracker fallback.
	if s.connectionTracker != nil {
		if concurrencyLimit, source, configured := s.resolveConcurrencyLimit(channel); concurrencyLimit > 0 {
			concurrent := s.connectionTracker.GetActiveConnections(channel.ID)
			details["concurrent_limit"] = concurrencyLimit
			details["concurrent_current"] = concurrent
			details["concurrency_limit_source"] = source
			details["concurrent_limit_configured"] = configured

			if int64(concurrent) >= concurrencyLimit {
				exhausted = true
				details["concurrent_exhausted"] = true
			} else {
				ratio := float64(concurrent) / float64(concurrencyLimit)
				if ratio > maxRatio {
					maxRatio = ratio
				}
			}
		}
	}

	var score float64
	if exhausted {
		score = rateLimitExhaustedScore
		details["exhausted"] = true
	} else {
		score = s.maxScore * (1 - maxRatio)
		if score < 0 {
			score = 0
		}
	}

	details["max_ratio"] = maxRatio
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
