package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

// quotaExhaustedScore is the penalty applied to exhausted channels.
// Chosen to be significantly below any non-exhausted score so that
// exhausted channels always sort last regardless of other strategy scores.
const quotaExhaustedScore = -10000

// warningUsageRatio is the fallback usage ratio used when a channel has
// Warning status but no per-limit data is available. This conservative
// estimate (80%) is used in DePrioritize mode to compute a penalty score.
const warningUsageRatio = 0.8

type QuotaEnforcementSettingsProvider interface {
	QuotaEnforcementSettingsOrDefault(ctx context.Context) *biz.QuotaEnforcementSettings
}

type QuotaAwareStrategy struct {
	provider      ProviderQuotaStatusProvider
	systemService QuotaEnforcementSettingsProvider
	maxScore      float64
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

	limitType := provider_quota.QuotaLimitType(quotaLimitTypeFromContext(ctx))
	effectiveStatus, _ := quotaStatus.EffectiveStatus(limitType)

	if details != nil {
		details["quota_status"] = effectiveStatus
		if limitType != "" {
			details["limit_type"] = string(limitType)
		}
	}

	switch effectiveStatus {
	case providerquotastatus.StatusUnknown:
		return 0, "status_unknown"

	case providerquotastatus.StatusExhausted:
		return quotaExhaustedScore, "quota_exhausted"

	case providerquotastatus.StatusWarning:
		if settings.Mode == biz.QuotaEnforcementModeDePrioritize {
			usageRatio := s.usageRatioForLimit(quotaStatus, limitType)
			score := -scaleScore(s.maxScore, usageRatio)
			if details != nil {
				details["usage_ratio"] = usageRatio
				details["scaled_score"] = score
			}
			return score, "warning_de_prioritize"
		}
		return 0, "warning_exhausted_only"

	case providerquotastatus.StatusAvailable:
		return 0, "status_available"

	default:
		return 0, fmt.Sprintf("status_unrecognized_%s", effectiveStatus)
	}
}

func (s *QuotaAwareStrategy) usageRatioForLimit(quotaStatus *biz.QuotaChannelStatus, limitType provider_quota.QuotaLimitType) float64 {
	if len(quotaStatus.Limits) == 0 {
		return warningUsageRatio
	}

	worstRatio := 0.0
	found := false

	for _, l := range quotaStatus.Limits {
		if l.Type != limitType {
			continue
		}

		found = true
		if l.UsageRatio > worstRatio {
			worstRatio = l.UsageRatio
		}
	}

	if !found {
		return warningUsageRatio
	}

	return worstRatio
}
