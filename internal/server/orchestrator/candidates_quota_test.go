package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm"
)

type mockQuotaStatusProvider struct {
	statuses map[int]*biz.QuotaChannelStatus
}

func (m *mockQuotaStatusProvider) GetQuotaStatus(_ context.Context, channelID int) *biz.QuotaChannelStatus {
	if m.statuses == nil {
		return nil
	}
	return m.statuses[channelID]
}

type quotaRequestCandidateSelector struct {
	candidatesByModel map[string][]*ChannelModelsCandidate
}

func (s *quotaRequestCandidateSelector) Select(_ context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	return s.candidatesByModel[req.Model], nil
}

func quotaRoutingCandidate(id int, mode objects.QuotaRoutingMode) *ChannelModelsCandidate {
	return &ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{
			ID:       id,
			Name:     "channel",
			Settings: &objects.ChannelSettings{QuotaRoutingMode: mode},
		}},
	}
}

func quotaRoutingStatus(status providerquotastatus.Status) *biz.QuotaChannelStatus {
	return &biz.QuotaChannelStatus{Status: status, Ready: status != providerquotastatus.StatusExhausted}
}

func quotaStickyOnlyStatus() *biz.QuotaChannelStatus {
	start := time.Now().Add(-2 * time.Hour)
	reset := time.Now().Add(time.Hour)
	return &biz.QuotaChannelStatus{
		Status: providerquotastatus.StatusAvailable,
		Ready:  true,
		Limits: []provider_quota.QuotaLimitStatus{{
			Type:        provider_quota.QuotaLimitTypeToken,
			Status:      "available",
			UsageRatio:  0.9,
			Window:      provider_quota.QuotaWindow5h,
			PeriodStart: &start,
			NextResetAt: &reset,
		}},
	}
}

func quotaExhaustedStatus() *biz.QuotaChannelStatus {
	return quotaRoutingStatus(providerquotastatus.StatusExhausted)
}

func newQuotaRoutingSelector(
	candidates []*ChannelModelsCandidate,
	statuses map[int]*biz.QuotaChannelStatus,
	settings biz.QuotaRoutingSettings,
) (*LoadBalancedSelector, *QuotaRoutingGate) {
	provider := &mockQuotaStatusProvider{statuses: statuses}
	gate := NewQuotaRoutingGate(provider, settings)
	policy := &biz.RetryPolicy{
		Enabled:           true,
		MaxChannelRetries: 2,
		TraceStickyMode:   biz.TraceStickyPreferPreviousChannel,
	}
	selector := WithRoutingPolicyLoadBalancedSelector(
		&staticChannelSelector{candidates: candidates},
		nil,
		policy,
		&fakePreviousChannelProvider{traceChannelIDs: map[int]int{10: 2}},
		nil,
		nil,
		gate,
	)
	return selector, gate
}

func TestQuotaRoutingGate_excludes_non_pinned_stickyOnly_while_open_exists(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
		2: quotaStickyOnlyStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})
	candidates := []*ChannelModelsCandidate{
		quotaRoutingCandidate(1, ""),
		quotaRoutingCandidate(2, ""),
	}

	// When
	got, _ := gate.Filter(context.Background(), candidates, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
	require.Equal(t, 1, gate.DroppedCount())
}

func TestLoadBalancedSelector_retains_stickyOnly_candidate_and_pins_it_first(t *testing.T) {
	// Given
	candidates := []*ChannelModelsCandidate{quotaRoutingCandidate(1, ""), quotaRoutingCandidate(2, "")}
	selector, gate := newQuotaRoutingSelector(candidates, map[int]*biz.QuotaChannelStatus{
		1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
		2: quotaStickyOnlyStatus(),
	}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})
	ctx := contexts.WithTrace(context.Background(), &ent.Trace{ID: 10, ThreadID: 20})

	// When
	got, err := selector.Select(ctx, &llm.Request{Model: "model"})

	// Then
	require.NoError(t, err)
	require.Equal(t, []int{2, 1}, quotaRoutingIDs(got))
	require.True(t, got[0].TraceSticky)
	require.Zero(t, gate.DroppedCount())
}

func TestLoadBalancedSelector_abandons_exhausted_sticky_candidate(t *testing.T) {
	// Given
	selector, gate := newQuotaRoutingSelector(
		[]*ChannelModelsCandidate{quotaRoutingCandidate(1, ""), quotaRoutingCandidate(2, "")},
		map[int]*biz.QuotaChannelStatus{1: quotaRoutingStatus(providerquotastatus.StatusAvailable), 2: quotaExhaustedStatus()},
		biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure},
	)
	ctx := contexts.WithTrace(context.Background(), &ent.Trace{ID: 10, ThreadID: 20})

	// When
	got, err := selector.Select(ctx, &llm.Request{Model: "model"})

	// Then
	require.NoError(t, err)
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
	require.Equal(t, 1, gate.DroppedCount())
}

func TestQuotaRoutingGate_phaseTwo_returns_single_stickyOnly_candidate(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaStickyOnlyStatus()}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})

	// When
	got, _ := gate.Filter(context.Background(), []*ChannelModelsCandidate{quotaRoutingCandidate(1, "")}, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
	require.Zero(t, gate.DroppedCount())
}

func TestQuotaRoutingGate_does_not_apply_phaseTwo_when_open_exists(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
		2: quotaStickyOnlyStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})

	// When
	got, _ := gate.Filter(context.Background(), []*ChannelModelsCandidate{quotaRoutingCandidate(1, ""), quotaRoutingCandidate(2, "")}, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
}

func TestQuotaRoutingGate_mixed_stickyOnly_keeps_only_resolved_sticky_and_open(t *testing.T) {
	// Given
	selector, _ := newQuotaRoutingSelector(
		[]*ChannelModelsCandidate{quotaRoutingCandidate(1, ""), quotaRoutingCandidate(2, ""), quotaRoutingCandidate(3, "")},
		map[int]*biz.QuotaChannelStatus{1: quotaRoutingStatus(providerquotastatus.StatusAvailable), 2: quotaStickyOnlyStatus(), 3: quotaStickyOnlyStatus()},
		biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure},
	)
	ctx := contexts.WithTrace(context.Background(), &ent.Trace{ID: 10, ThreadID: 20})

	// When
	got, err := selector.Select(ctx, &llm.Request{Model: "model"})

	// Then only the resolved sticky channel and the open channel are preferred;
	// the other pressured channel is appended after them.
	require.NoError(t, err)
	require.Equal(t, []int{2, 1, 3}, quotaRoutingIDs(got))
}

func TestQuotaRoutingGate_ignoreQuota_keeps_exhausted_channel(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaExhaustedStatus()}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})

	// When
	got, _ := gate.Filter(context.Background(), []*ChannelModelsCandidate{quotaRoutingCandidate(1, objects.QuotaRoutingModeIgnoreQuota)}, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
	require.Zero(t, gate.DroppedCount())
}

func TestQuotaRoutingGate_unknown_channel_mode_fails_closed(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaExhaustedStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeIgnoreQuota})

	candidate := quotaRoutingCandidate(1, objects.QuotaRoutingMode("unknown"))

	// When
	decision := gate.evaluate(context.Background(), candidate, provider_quota.QuotaLimitTypeToken, 0)

	// Then
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, decision.mode)
	require.False(t, decision.keepPhaseOne)
}

func TestQuotaRoutingGate_channel_override_beats_global_and_inherit_uses_global(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaExhaustedStatus(),
		2: quotaExhaustedStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})
	candidates := []*ChannelModelsCandidate{
		quotaRoutingCandidate(1, objects.QuotaRoutingModeIgnoreQuota),
		quotaRoutingCandidate(2, ""),
	}

	// When
	got, _ := gate.Filter(context.Background(), candidates, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
	require.Equal(t, 1, gate.DroppedCount())
}

func TestQuotaRoutingGate_uses_request_modality_and_keeps_unknown_neutral(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: {
			Status: providerquotastatus.StatusWarning,
			Limits: []provider_quota.QuotaLimitStatus{
				{Type: provider_quota.QuotaLimitTypeImage, Status: "exhausted", UsageRatio: 1},
				{Type: provider_quota.QuotaLimitTypeToken, Status: "available", UsageRatio: 0.2},
			},
		},
		2: quotaRoutingStatus(providerquotastatus.StatusUnknown),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})
	candidates := []*ChannelModelsCandidate{quotaRoutingCandidate(1, ""), quotaRoutingCandidate(2, "")}

	// When
	got, _ := gate.Filter(context.Background(), candidates, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1, 2}, quotaRoutingIDs(got))
	got, _ = gate.Filter(context.Background(), candidates, &llm.Request{Image: &llm.ImageRequest{}}, 0)
	require.Equal(t, []int{2}, quotaRoutingIDs(got))
}

func TestSelectCandidates_returns_quota_error_only_when_gate_dropped_candidates(t *testing.T) {
	// Given
	provider := &mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaExhaustedStatus()}}
	settings := &RequestSettings{QuotaRouting: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted}}
	inbound := &PersistentInboundTransformer{state: &PersistenceState{
		CandidateSelector: &staticChannelSelector{candidates: []*ChannelModelsCandidate{quotaRoutingCandidate(1, "")}},
		Settings:          settings,
	}}
	middleware := selectCandidates(inbound, provider)

	// When
	_, err := middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: "model"})

	// Then
	var quotaErr *QuotaExhaustedError
	require.True(t, errors.As(err, &quotaErr))

	// Given
	inbound = &PersistentInboundTransformer{state: &PersistenceState{CandidateSelector: &staticChannelSelector{}, Settings: settings}}
	middleware = selectCandidates(inbound, provider)

	// When
	_, err = middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: "model"})

	// Then
	require.Error(t, err)
	require.False(t, errors.As(err, &quotaErr))
}

func TestSelectCandidates_empty_loadBalancers_still_gates_exhausted_candidate(t *testing.T) {
	// Given
	inbound := &PersistentInboundTransformer{state: &PersistenceState{
		CandidateSelector: &staticChannelSelector{candidates: []*ChannelModelsCandidate{quotaRoutingCandidate(7, "")}},
		LoadBalancers:     map[string]*LoadBalancer{},
		Settings:          &RequestSettings{QuotaRouting: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted}},
	}}
	middleware := selectCandidates(inbound, &mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{7: quotaExhaustedStatus()}})

	// When
	_, err := middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: "model"})

	// Then
	var quotaErr *QuotaExhaustedError
	require.True(t, errors.As(err, &quotaErr))
	require.Empty(t, inbound.state.ChannelModelsCandidates)
}

func TestSelectCandidates_concurrent_requests_have_independent_drop_counts(t *testing.T) {
	// Given
	inbound := &PersistentInboundTransformer{state: &PersistenceState{
		CandidateSelector: &quotaRequestCandidateSelector{candidatesByModel: map[string][]*ChannelModelsCandidate{
			"exhausted": {quotaRoutingCandidate(1, "")},
		}},
		LoadBalancers: map[string]*LoadBalancer{},
		Settings:      &RequestSettings{QuotaRouting: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted}},
	}}
	middleware := selectCandidates(
		inbound,
		&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaExhaustedStatus()}},
	)
	results := make(chan struct {
		model string
		err   error
	}, 2)
	var wg sync.WaitGroup
	for _, model := range []string{"exhausted", "empty"} {
		wg.Add(1)
		go func(model string) {
			defer wg.Done()
			_, err := middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: model})
			results <- struct {
				model string
				err   error
			}{model, err}
		}(model)
	}
	wg.Wait()
	close(results)

	// When
	observed := make(map[string]error, 2)
	for result := range results {
		observed[result.model] = result.err
	}

	// Then
	require.Len(t, observed, 2)
	var quotaErr *QuotaExhaustedError
	require.True(t, errors.As(observed["exhausted"], &quotaErr))
	require.False(t, errors.As(observed["empty"], &quotaErr))
	require.ErrorIs(t, observed["empty"], biz.ErrInvalidModel)
}

func TestQuotaRoutingGate_filters_exhausted_fallback_models_and_retains_healthy_entry(t *testing.T) {
	// Given a channel candidate where the first model (Claude) is exhausted, but the second (Gemini) is available.
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: {
			Status: providerquotastatus.StatusAvailable,
			Limits: []provider_quota.QuotaLimitStatus{
				{
					Type:              provider_quota.QuotaLimitTypeToken,
					Status:            "exhausted",
					UsageRatio:        1.0,
					Window:            "claude_gpt_7d",
					AvailabilityGroup: "antigravity_pool:claude_gpt",
				},
				{
					Type:              provider_quota.QuotaLimitTypeToken,
					Status:            "available",
					UsageRatio:        0.1,
					Window:            "gemini_7d",
					AvailabilityGroup: "antigravity_pool:gemini",
				},
			},
		},
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})

	candidate := &ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}},
		Models: []biz.ChannelModelEntry{
			{RequestModel: "claude-sonnet-4-6", ActualModel: "claude-sonnet-4-6"},
			{RequestModel: "gemini-3.8-flash", ActualModel: "gemini-3.8-flash"},
		},
		modelAPIFormats: []string{"anthropic/messages", "gemini/contents"},
	}

	// When
	got, _ := gate.Filter(context.Background(), []*ChannelModelsCandidate{candidate}, &llm.Request{}, 0)

	// Then: the candidate is kept, the exhausted model is pruned, and the healthy fallback is retained.
	require.Len(t, got, 1)
	require.Equal(t, 1, got[0].Channel.ID)
	require.Len(t, got[0].Models, 1)
	require.Equal(t, "gemini-3.8-flash", got[0].Models[0].ActualModel)
	require.Equal(t, []string{"gemini/contents"}, got[0].modelAPIFormats)
	require.Zero(t, gate.DroppedCount())
}

func TestQuotaRoutingGate_backpressure_does_not_admit_pressured_first_model_in_phase_one(t *testing.T) {
	// Channel 1 has Claude (window exhausted + balance fallback -> RoutingStickyOnly)
	// first and Gemini (RoutingOpen) second. Channel 2 is entirely open.
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: {
			Status: providerquotastatus.StatusAvailable,
			Limits: []provider_quota.QuotaLimitStatus{
				{
					Type:              provider_quota.QuotaLimitTypeToken,
					Status:            "exhausted",
					UsageRatio:        1.0,
					Window:            "claude_gpt_5h",
					AvailabilityGroup: "antigravity_pool:claude_gpt",
				},
				{
					Type:              provider_quota.QuotaLimitTypeToken,
					Status:            "available",
					Window:            "pay_as_you_go",
					AvailabilityGroup: "antigravity_pool:claude_gpt",
				},
				{
					Type:              provider_quota.QuotaLimitTypeToken,
					Status:            "available",
					UsageRatio:        0.1,
					Window:            "gemini_7d",
					AvailabilityGroup: "antigravity_pool:gemini",
				},
			},
		},
		2: quotaRoutingStatus(providerquotastatus.StatusAvailable),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})

	candidate1 := &ChannelModelsCandidate{
		Channel: &biz.Channel{Channel: &ent.Channel{ID: 1}},
		Models: []biz.ChannelModelEntry{
			{RequestModel: "claude-sonnet-4-6", ActualModel: "claude-sonnet-4-6"},
			{RequestModel: "gemini-3.8-flash", ActualModel: "gemini-3.8-flash"},
		},
	}
	candidate2 := quotaRoutingCandidate(2, "")

	// When Phase 1 has Candidate 2 available, Candidate 1 must not bypass backpressure in Phase 1
	// because its attempted first model is pressured (RoutingStickyOnly).
	got, _ := gate.Filter(context.Background(), []*ChannelModelsCandidate{candidate1, candidate2}, &llm.Request{}, 0)
	require.Equal(t, []int{2}, quotaRoutingIDs(got))
}

func TestQuotaRoutingGate_reserves_pressured_candidate_as_last_resort(t *testing.T) {
	// Given one open and one pressured channel under backpressure mode.
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
		2: quotaStickyOnlyStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})

	// When
	preferred, lastResort := gate.Filter(context.Background(), []*ChannelModelsCandidate{
		quotaRoutingCandidate(1, ""),
		quotaRoutingCandidate(2, ""),
	}, &llm.Request{}, 0)

	// Then the pressured channel leaves the preferred list but stays reserved.
	require.Equal(t, []int{1}, quotaRoutingIDs(preferred))
	require.Equal(t, []int{2}, quotaRoutingIDs(lastResort))
	require.Equal(t, 1, gate.DroppedCount())
}

func TestQuotaRoutingGate_does_not_reserve_exhausted_candidate(t *testing.T) {
	// Given one open and one exhausted channel under backpressure mode.
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
		2: quotaExhaustedStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})

	// When
	preferred, lastResort := gate.Filter(context.Background(), []*ChannelModelsCandidate{
		quotaRoutingCandidate(1, ""),
		quotaRoutingCandidate(2, ""),
	}, &llm.Request{}, 0)

	// Then an exhausted channel is never reserved: retrying it cannot succeed.
	require.Equal(t, []int{1}, quotaRoutingIDs(preferred))
	require.Empty(t, lastResort)
	require.Equal(t, 1, gate.DroppedCount())
}

func TestQuotaRoutingGate_keeps_pressured_candidate_in_removeOnExhausted_mode(t *testing.T) {
	// Given a pressured channel under remove_on_exhausted mode.
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
		2: quotaStickyOnlyStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})

	// When
	preferred, lastResort := gate.Filter(context.Background(), []*ChannelModelsCandidate{
		quotaRoutingCandidate(1, ""),
		quotaRoutingCandidate(2, ""),
	}, &llm.Request{}, 0)

	// Then the mode keeps pressured channels in the preferred list.
	require.Equal(t, []int{1, 2}, quotaRoutingIDs(preferred))
	require.Empty(t, lastResort)
	require.Zero(t, gate.DroppedCount())
}

func TestQuotaRoutingGate_reserves_nothing_when_phase_two_admits_pressured_candidates(t *testing.T) {
	// Given only pressured candidates: phase two admits all of them.
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
		1: quotaStickyOnlyStatus(),
	}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure})

	// When
	preferred, lastResort := gate.Filter(context.Background(), []*ChannelModelsCandidate{
		quotaRoutingCandidate(1, ""),
	}, &llm.Request{}, 0)

	// Then the candidate is preferred, not reserved.
	require.Equal(t, []int{1}, quotaRoutingIDs(preferred))
	require.Empty(t, lastResort)
	require.Zero(t, gate.DroppedCount())
}

func TestLoadBalancedSelector_appends_reserved_quota_fallback_candidates(t *testing.T) {
	// Given one open and three pressured channels; the retry policy allows two
	// channel retries, so at most two fallbacks are reserved.
	selector, _ := newQuotaRoutingSelector(
		[]*ChannelModelsCandidate{
			quotaRoutingCandidate(1, ""),
			quotaRoutingCandidate(2, ""),
			quotaRoutingCandidate(3, ""),
			quotaRoutingCandidate(4, ""),
		},
		map[int]*biz.QuotaChannelStatus{
			1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
			2: quotaStickyOnlyStatus(),
			3: quotaStickyOnlyStatus(),
			4: quotaStickyOnlyStatus(),
		},
		biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure},
	)

	// When
	got, err := selector.Select(context.Background(), &llm.Request{Model: "model"})

	// Then the preferred channel leads and the fallbacks follow, capped by the
	// retry budget.
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3}, quotaRoutingIDs(got))
}

func TestLoadBalancedSelector_sticky_path_appends_quota_fallback_candidates(t *testing.T) {
	// Given a sticky open channel, another open channel and a pressured channel.
	selector, _ := newQuotaRoutingSelector(
		[]*ChannelModelsCandidate{
			quotaRoutingCandidate(1, ""),
			quotaRoutingCandidate(2, ""),
			quotaRoutingCandidate(3, ""),
		},
		map[int]*biz.QuotaChannelStatus{
			1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
			2: quotaRoutingStatus(providerquotastatus.StatusAvailable),
			3: quotaStickyOnlyStatus(),
		},
		biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure},
	)
	ctx := contexts.WithTrace(context.Background(), &ent.Trace{ID: 10, ThreadID: 20})

	// When
	got, err := selector.Select(ctx, &llm.Request{Model: "model"})

	// Then the sticky channel leads, the remaining preferred channel follows and
	// the pressured channel is appended at the tail.
	require.NoError(t, err)
	require.Equal(t, []int{2, 1, 3}, quotaRoutingIDs(got))
	require.True(t, got[0].TraceSticky)
}

func TestSelectCandidates_appends_pressured_candidate_without_load_balancer(t *testing.T) {
	// Given no load balancer configured.
	inbound := &PersistentInboundTransformer{state: &PersistenceState{
		CandidateSelector: &staticChannelSelector{candidates: []*ChannelModelsCandidate{
			quotaRoutingCandidate(1, ""),
			quotaRoutingCandidate(2, ""),
		}},
		LoadBalancers: map[string]*LoadBalancer{},
		Settings: &RequestSettings{
			QuotaRouting: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeBackpressure},
			RetryPolicy:  biz.RetryPolicy{Enabled: true, MaxChannelRetries: 2},
		},
	}}
	middleware := selectCandidates(
		inbound,
		&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{
			1: quotaRoutingStatus(providerquotastatus.StatusAvailable),
			2: quotaStickyOnlyStatus(),
		}},
	)

	// When
	_, err := middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: "model"})

	// Then the pressured channel is appended instead of being dropped.
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, quotaRoutingIDs(inbound.state.ChannelModelsCandidates))
}

func TestQuotaFallbackLimit(t *testing.T) {
	require.Zero(t, quotaFallbackLimit(nil))
	require.Zero(t, quotaFallbackLimit(&biz.RetryPolicy{Enabled: false, MaxChannelRetries: 3}))
	require.Zero(t, quotaFallbackLimit(&biz.RetryPolicy{Enabled: true, MaxChannelRetries: 0}))
	require.Equal(t, 3, quotaFallbackLimit(&biz.RetryPolicy{Enabled: true, MaxChannelRetries: 3}))
}

func TestQuotaFallbackCandidates_orders_by_priority_and_skips_duplicate_channels(t *testing.T) {
	// Given pressured candidates of the same channel under different priorities.
	highPriority := quotaRoutingCandidate(1, "")
	highPriority.Priority = 5
	first := quotaRoutingCandidate(2, "")
	first.Priority = 1
	lowPriority := quotaRoutingCandidate(1, "")
	lowPriority.Priority = 0
	second := quotaRoutingCandidate(3, "")
	second.Priority = 1

	// When
	got := quotaFallbackCandidates([]*ChannelModelsCandidate{highPriority, first, lowPriority, second}, 3)

	// Then priority order wins and the duplicate channel keeps its best entry.
	require.Equal(t, []int{1, 2, 3}, quotaRoutingIDs(got))
	require.Same(t, lowPriority, got[0])

	// And the retry budget caps the appended candidates.
	require.Equal(t, []int{1}, quotaRoutingIDs(quotaFallbackCandidates([]*ChannelModelsCandidate{first, lowPriority}, 1)))
}

func quotaRoutingIDs(candidates []*ChannelModelsCandidate) []int {
	ids := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.Channel.ID)
	}
	return ids
}
