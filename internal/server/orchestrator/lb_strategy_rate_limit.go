package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// RateLimitAwareStrategy adjusts channel scores based on configured RPM/TPM rate limits.
// Channels that have exhausted their rate limits receive a heavily negative score to be skipped.
type RateLimitAwareStrategy struct {
	tracker  *ChannelRequestTracker
	maxScore float64
}

// NewRateLimitAwareStrategy creates a new rate limit aware load balancing strategy.
func NewRateLimitAwareStrategy(tracker *ChannelRequestTracker) *RateLimitAwareStrategy {
	return &RateLimitAwareStrategy{
		tracker:  tracker,
		maxScore: 100.0,
	}
}

// Name returns the strategy name.
func (s *RateLimitAwareStrategy) Name() string {
	return "RateLimitAware"
}

// Score calculates the score based on channel rate limit usage.
// This is the production path with minimal overhead.
func (s *RateLimitAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	settings := channel.Settings
	if settings == nil || settings.RateLimit == nil {
		return s.maxScore
	}

	rl := settings.RateLimit

	var maxRatio float64

	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.tracker.GetRequestCount(channel.ID)
		if rpm >= *rl.RPM {
			return -1000
		}

		ratio := float64(rpm) / float64(*rl.RPM)
		if ratio > maxRatio {
			maxRatio = ratio
		}
	}

	if rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.tracker.GetTokenCount(channel.ID)
		if tpm >= *rl.TPM {
			return -1000
		}

		ratio := float64(tpm) / float64(*rl.TPM)
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

	settings := channel.Settings
	details := map[string]any{
		"channel_id": channel.ID,
	}

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

	if rl.RPM != nil && *rl.RPM > 0 {
		rpm := s.tracker.GetRequestCount(channel.ID)
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

	if rl.TPM != nil && *rl.TPM > 0 {
		tpm := s.tracker.GetTokenCount(channel.ID)
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
