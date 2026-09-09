package orchestrator

import (
	"context"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm"
)

// QuotaRoutingGate applies the quota routing policy for one request.
type QuotaRoutingGate struct {
	provider     ProviderQuotaStatusProvider
	defaultMode  objects.QuotaRoutingMode
	droppedCount int
}

func NewQuotaRoutingGate(provider ProviderQuotaStatusProvider, settings biz.QuotaRoutingSettings) *QuotaRoutingGate {
	return &QuotaRoutingGate{
		provider:    provider,
		defaultMode: settings.DefaultMode,
	}
}

// DroppedCount reports how many candidates were removed by the last Filter call.
func (g *QuotaRoutingGate) DroppedCount() int {
	if g == nil {
		return 0
	}

	return g.droppedCount
}

// Filter evaluates every candidate before applying the two-phase routing rule.
func (g *QuotaRoutingGate) Filter(
	ctx context.Context,
	candidates []*ChannelModelsCandidate,
	req *llm.Request,
	stickyID int,
) []*ChannelModelsCandidate {
	if g != nil {
		g.droppedCount = 0
	}
	if g == nil || g.provider == nil || len(candidates) == 0 {
		return candidates
	}

	limitType := provider_quota.RequestModality(req.Image != nil)
	decisions := make([]quotaRoutingDecision, 0, len(candidates))
	phaseOne := make([]*ChannelModelsCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		decision := g.evaluate(ctx, candidate, limitType, stickyID)
		decisions = append(decisions, decision)
		if decision.keepPhaseOne {
			phaseOne = append(phaseOne, candidate)
		}
		g.logDecision(ctx, candidate, decision, false)
	}

	filtered := phaseOne
	if len(phaseOne) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "quota routing phase two fallback",
				log.Int("candidate_count", len(decisions)),
			)
		}
		for _, decision := range decisions {
			if decision.keepPhaseTwo {
				filtered = append(filtered, decision.candidate)
				g.logDecision(ctx, decision.candidate, decision, true)
			}
		}
	}

	g.droppedCount = len(candidates) - len(filtered)
	return filtered
}

type quotaRoutingDecision struct {
	candidate    *ChannelModelsCandidate
	mode         objects.QuotaRoutingMode
	state        provider_quota.RoutingState
	reason       string
	keepPhaseOne bool
	keepPhaseTwo bool
}

func (g *QuotaRoutingGate) evaluate(
	ctx context.Context,
	candidate *ChannelModelsCandidate,
	limitType provider_quota.QuotaLimitType,
	stickyID int,
) quotaRoutingDecision {
	mode := g.defaultMode
	if candidate.Channel.Settings != nil && candidate.Channel.Settings.QuotaRoutingMode != "" {
		mode = candidate.Channel.Settings.QuotaRoutingMode
	}
	if mode == "" {
		mode = objects.QuotaRoutingModeRemoveOnExhausted
	}
	if mode != objects.QuotaRoutingModeRemoveOnExhausted &&
		mode != objects.QuotaRoutingModeBackpressure &&
		mode != objects.QuotaRoutingModeIgnoreQuota {
		mode = objects.QuotaRoutingModeRemoveOnExhausted
	}

	decision := quotaRoutingDecision{
		candidate: candidate,
		mode:      mode,
		state:     provider_quota.RoutingUnknown,
	}
	if mode == objects.QuotaRoutingModeIgnoreQuota {
		decision.keepPhaseOne = true
		decision.keepPhaseTwo = true
		decision.reason = "ignore_quota"
		return decision
	}

	status := g.provider.GetQuotaStatus(ctx, candidate.Channel.ID)
	if status == nil {
		decision.keepPhaseOne = true
		decision.keepPhaseTwo = true
		decision.reason = "quota_status_unknown"
		return decision
	}

	decision.state, decision.reason = provider_quota.EvaluateQuotaRouting(
		status.Limits,
		string(status.Status),
		limitType,
		time.Now(),
	)
	if decision.state == provider_quota.RoutingExhausted {
		decision.reason = "exhausted"
		return decision
	}

	switch mode {
	case objects.QuotaRoutingModeRemoveOnExhausted:
		decision.keepPhaseOne = true
		decision.keepPhaseTwo = true
	case objects.QuotaRoutingModeBackpressure:
		decision.keepPhaseOne = decision.state == provider_quota.RoutingOpen || decision.state == provider_quota.RoutingUnknown
		decision.keepPhaseOne = decision.keepPhaseOne ||
			(decision.state == provider_quota.RoutingStickyOnly && candidate.Channel.ID == stickyID && stickyID != 0)
		decision.keepPhaseTwo = decision.state == provider_quota.RoutingStickyOnly
	default:
		decision.keepPhaseOne = true
		decision.keepPhaseTwo = true
	}

	return decision
}

func (g *QuotaRoutingGate) logDecision(
	ctx context.Context,
	candidate *ChannelModelsCandidate,
	decision quotaRoutingDecision,
	phaseTwo bool,
) {
	if !log.DebugEnabled(ctx) {
		return
	}

	reason := decision.reason
	if phaseTwo {
		reason = "no_alternative_fallback"
	}
	log.Debug(ctx, "quota routing candidate decision",
		log.Int("channel_id", candidate.Channel.ID),
		log.String("mode", string(decision.mode)),
		log.String("state", string(decision.state)),
		log.String("reason", reason),
		log.Bool("phase_two", phaseTwo),
	)
}

// QuotaRoutingSelector keeps quota routing enabled when no load balancer is configured.
type QuotaRoutingSelector struct {
	wrapped CandidateSelector
	gate    *QuotaRoutingGate
}

func WithQuotaRoutingSelector(wrapped CandidateSelector, gate *QuotaRoutingGate) *QuotaRoutingSelector {
	return &QuotaRoutingSelector{wrapped: wrapped, gate: gate}
}

func (s *QuotaRoutingSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	return s.gate.Filter(ctx, candidates, req, 0), nil
}
