package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// RateLimitAwareStrategy adjusts channel scores based on configured RPM/TPM rate limits and concurrency limits.
// Channels that have exhausted their rate limits receive a heavily negative score to be skipped.
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

// Score calculates the score based on channel rate limit usage.
// This is the production path with minimal overhead.
func (s *RateLimitAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	// Check if channel is in cooldown (429 Retry-After)
	if s.requestTracker.IsCoolingDown(channel.ID) {
		return -1000
	}

	settings := channel.Settings
	if settings == nil || settings.RateLimit == nil {
		return s.maxScore
	}

	rl := settings.RateLimit

	var maxRatio float64

	// Check RPM (Requests Per Minute)
	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.requestTracker.GetRequestCount(channel.ID)
		if rpm >= *rl.RPM {
			return -1000
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
			return -1000
		}

		ratio := float64(tpm) / float64(*rl.TPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	// Check concurrent requests
	if rl.MaxConcurrent != nil && *rl.MaxConcurrent > 0 && s.connectionTracker != nil {
		concurrent := s.connectionTracker.GetActiveConnections(channel.ID)
		if int64(concurrent) >= *rl.MaxConcurrent {
			return -1000
		}

		ratio := float64(concurrent) / float64(*rl.MaxConcurrent)
		if ratio > maxRatio {
			maxRatio = ratio
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
		score := -1000.0
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

	// Check concurrent requests
	if rl.MaxConcurrent != nil && *rl.MaxConcurrent > 0 && s.connectionTracker != nil {
		concurrent := s.connectionTracker.GetActiveConnections(channel.ID)
		details["concurrent_limit"] = *rl.MaxConcurrent
		details["concurrent_current"] = concurrent

		if int64(concurrent) >= *rl.MaxConcurrent {
			exhausted = true
			details["concurrent_exhausted"] = true
		} else {
			ratio := float64(concurrent) / float64(*rl.MaxConcurrent)
			if ratio > maxRatio {
				maxRatio = ratio
			}
		}
	}

	var score float64
	if exhausted {
		score = -1000
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
