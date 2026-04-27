package orchestrator

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/llm"
)

// ProviderQuotaSelector filters out channels whose provider quota is exhausted.
// It wraps another CandidateSelector and removes exhausted candidates from the
// result set based on the current QuotaEnforcementSettings.
//
// In "exhausted_only" mode, only exhausted channels are filtered out.
// In "de_prioritize" mode, exhausted channels are also filtered out (the
// QuotaAwareStrategy handles scoring for warning-state channels).
// Channels with nil quota data, "available", "warning", or "unknown" status
// are always kept.
type ProviderQuotaSelector struct {
	wrapped       CandidateSelector
	provider      ProviderQuotaStatusProvider
	systemService QuotaEnforcementSettingsProvider
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

	if !settings.Enabled {
		return candidates, nil
	}

	filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		quotaStatus := s.provider.GetQuotaStatus(c.Channel.ID)

		if quotaStatus == nil {
			return true
		}

		switch quotaStatus.Status {
		case "exhausted":
			return false
		default:
			// "available", "warning", "unknown", and any unrecognized status
			// are kept. The QuotaAwareStrategy handles scoring for warning
			// channels in "de_prioritize" mode.
			return true
		}
	})

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "ProviderQuotaSelector: filtered candidates",
			log.String("model", req.Model),
			log.String("mode", settings.Mode),
			log.Int("before", len(candidates)),
			log.Int("after", len(filtered)),
		)
	}

	return filtered, nil
}
