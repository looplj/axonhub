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

type mockQuotaRoutingSettingsProvider struct {
	settings biz.QuotaRoutingSettings
}

func (m *mockQuotaRoutingSettingsProvider) QuotaRoutingSettingsOrDefault(context.Context) biz.QuotaRoutingSettings {
	return m.settings
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
	policy := &mockRetryPolicyProvider{policy: &biz.RetryPolicy{
		Enabled:           true,
		MaxChannelRetries: 2,
		TraceStickyMode:   biz.TraceStickyPreferPreviousChannel,
	}}
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
	got := gate.Filter(context.Background(), candidates, &llm.Request{}, 0)

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
	got := gate.Filter(context.Background(), []*ChannelModelsCandidate{quotaRoutingCandidate(1, "")}, &llm.Request{}, 0)

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
	got := gate.Filter(context.Background(), []*ChannelModelsCandidate{quotaRoutingCandidate(1, ""), quotaRoutingCandidate(2, "")}, &llm.Request{}, 0)

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

	// Then
	require.NoError(t, err)
	require.Equal(t, []int{2, 1}, quotaRoutingIDs(got))
	require.NotContains(t, quotaRoutingIDs(got), 3)
}

func TestQuotaRoutingGate_ignoreQuota_keeps_exhausted_channel(t *testing.T) {
	// Given
	gate := NewQuotaRoutingGate(&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaExhaustedStatus()}}, biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted})

	// When
	got := gate.Filter(context.Background(), []*ChannelModelsCandidate{quotaRoutingCandidate(1, objects.QuotaRoutingModeIgnoreQuota)}, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1}, quotaRoutingIDs(got))
	require.Zero(t, gate.DroppedCount())
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
	got := gate.Filter(context.Background(), candidates, &llm.Request{}, 0)

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
	got := gate.Filter(context.Background(), candidates, &llm.Request{}, 0)

	// Then
	require.Equal(t, []int{1, 2}, quotaRoutingIDs(got))
	got = gate.Filter(context.Background(), candidates, &llm.Request{Image: &llm.ImageRequest{}}, 0)
	require.Equal(t, []int{2}, quotaRoutingIDs(got))
}

func TestSelectCandidates_returns_quota_error_only_when_gate_dropped_candidates(t *testing.T) {
	// Given
	provider := &mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaExhaustedStatus()}}
	settings := &mockQuotaRoutingSettingsProvider{settings: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted}}
	inbound := &PersistentInboundTransformer{state: &PersistenceState{
		CandidateSelector: &staticChannelSelector{candidates: []*ChannelModelsCandidate{quotaRoutingCandidate(1, "")}},
	}}
	middleware := selectCandidates(inbound, provider, settings)

	// When
	_, err := middleware.OnInboundLlmRequest(context.Background(), &llm.Request{Model: "model"})

	// Then
	var quotaErr *QuotaExhaustedError
	require.True(t, errors.As(err, &quotaErr))

	// Given
	inbound = &PersistentInboundTransformer{state: &PersistenceState{CandidateSelector: &staticChannelSelector{}}}
	middleware = selectCandidates(inbound, provider, settings)

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
	}}
	middleware := selectCandidates(inbound, &mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{7: quotaExhaustedStatus()}}, &mockQuotaRoutingSettingsProvider{settings: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted}})

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
	}}
	middleware := selectCandidates(
		inbound,
		&mockQuotaStatusProvider{statuses: map[int]*biz.QuotaChannelStatus{1: quotaExhaustedStatus()}},
		&mockQuotaRoutingSettingsProvider{settings: biz.QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeRemoveOnExhausted}},
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

func quotaRoutingIDs(candidates []*ChannelModelsCandidate) []int {
	ids := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.Channel.ID)
	}
	return ids
}
