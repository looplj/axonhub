package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm"
)

type ProviderQuotaSelector struct {
	wrapped       CandidateSelector
	provider      ProviderQuotaStatusProvider
	systemService QuotaEnforcementSettingsProvider
	// FilteredCount holds the number of candidates removed by the last Select() call.
	// It is only populated in ExhaustedOnly mode; DePrioritize mode returns early
	// without setting it. Read after Select() to distinguish "no candidates due to
	// quota exhaustion" from "no candidates at all".
	FilteredCount int
}

func WithProviderQuotaSelector(wrapped CandidateSelector, provider ProviderQuotaStatusProvider, systemService QuotaEnforcementSettingsProvider) *ProviderQuotaSelector {
	return &ProviderQuotaSelector{
		wrapped:       wrapped,
		provider:      provider,
		systemService: systemService,
	}
}

func (s *ProviderQuotaSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	if s.provider == nil {
		return candidates, nil
	}

	settings := s.systemService.QuotaEnforcementSettingsOrDefault(ctx)

	if !settings.Enabled || settings.Mode == biz.QuotaEnforcementModeDePrioritize {
		return candidates, nil
	}

	limitType := provider_quota.RequestModality(req.Image != nil)

	filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		quotaStatus := s.provider.GetQuotaStatus(c.Channel.ID)

		if quotaStatus == nil {
			return true
		}

		effectiveStatus, _ := quotaStatus.EffectiveStatus(limitType)

		switch effectiveStatus {
		case providerquotastatus.StatusAvailable,
			providerquotastatus.StatusWarning,
			providerquotastatus.StatusUnknown:
			return true
		case providerquotastatus.StatusExhausted:
			return false
		default:
			return true
		}
	})

	s.FilteredCount = len(candidates) - len(filtered)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "ProviderQuotaSelector: filtered candidates",
			log.String("model", req.Model),
			log.String("mode", string(settings.Mode)),
			log.String("limit_type", string(limitType)),
			log.Int("before", len(candidates)),
			log.Int("after", len(filtered)),
		)
	}

	return filtered, nil
}
