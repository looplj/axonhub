package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// quotaExhaustedScore is the penalty applied to channels whose provider quota
// is exhausted. It must dominate the maximum positive sum from all other
// strategies so exhausted channels rank last while still being available as
// fallback candidates.
const quotaExhaustedScore = -10000

// warningUsageRatio approximates channel usage when entering warning state.
const warningUsageRatio = 0.8

// QuotaEnforcementSettingsProvider provides quota enforcement configuration.
type QuotaEnforcementSettingsProvider interface {
	QuotaEnforcementSettingsOrDefault(ctx context.Context) *biz.QuotaEnforcementSettings
}

// QuotaAwareStrategy adjusts channel scores based on provider quota status.
// Channels with exhausted quota receive a large negative penalty so the load
// balancer deprioritises them. Warning-state channels are penalised
// proportionally when the enforcement mode is QuotaEnforcementModeDePrioritize.
type QuotaAwareStrategy struct {
	provider       ProviderQuotaStatusProvider
	systemService  QuotaEnforcementSettingsProvider
	maxScore       float64
}

func NewQuotaAwareStrategy(provider ProviderQuotaStatusProvider, systemService QuotaEnforcementSettingsProvider) *QuotaAwareStrategy {
	return &QuotaAwareStrategy{
		provider:      provider,
		systemService: systemService,
		maxScore:      100.0,
	}
}

func (s *QuotaAwareStrategy) Name() string {
	return "QuotaAware"
}

func (s *QuotaAwareStrategy) Score(ctx context.Context, channel *biz.Channel) float64 {
	score, _ := s.score(ctx, channel, nil)
	return score
}

func (s *QuotaAwareStrategy) ScoreWithDebug(ctx context.Context, channel *biz.Channel) (float64, StrategyScore) {
	startTime := time.Now()

	details := map[string]any{
		"channel_id": channel.ID,
	}

	score, reason := s.score(ctx, channel, details)

	details["score"] = score
	details["score_reason"] = reason

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "QuotaAwareStrategy: scoring",
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

// score is the unified scorer. When details is nil, no diagnostic info is
// recorded. Returns the final score and a human-readable reason string.
func (s *QuotaAwareStrategy) score(ctx context.Context, channel *biz.Channel, details map[string]any) (float64, string) {
	settings := s.systemService.QuotaEnforcementSettingsOrDefault(ctx)

	if !settings.Enabled {
		if details != nil {
			details["enforcement_enabled"] = false
		}
		return 0, "enforcement_disabled"
	}

	if s.provider == nil {
		if details != nil {
			details["quota_status"] = "no_provider"
		}
		return 0, "no_quota_provider"
	}

	if details != nil {
		details["enforcement_enabled"] = true
		details["mode"] = settings.Mode
	}

	quotaStatus := s.provider.GetQuotaStatus(channel.ID)

	if quotaStatus == nil {
		if details != nil {
			details["quota_status"] = "no_data"
		}
		return 0, "no_quota_data"
	}

	if details != nil {
		details["quota_status"] = quotaStatus.Status
	}

	switch quotaStatus.Status {
	case "unknown":
		return 0, "status_unknown"

	case "exhausted":
		return quotaExhaustedScore, "quota_exhausted"

	case "warning":
		if settings.Mode == biz.QuotaEnforcementModeDePrioritize {
			usageRatio := warningUsageRatio
			score := -scaleScore(s.maxScore, 1-usageRatio)
			if details != nil {
				details["usage_ratio"] = usageRatio
				details["scaled_score"] = score
			}
			return score, "warning_de_prioritize"
		}
		// exhausted_only mode: warning is acceptable, no penalty.
		return 0, "warning_exhausted_only"

	case "available":
		return 0, "status_available"

	default:
		if details != nil {
			details["quota_status"] = "unrecognized"
			details["raw_status"] = quotaStatus.Status
		}
		return 0, "status_unrecognized"
	}
}
