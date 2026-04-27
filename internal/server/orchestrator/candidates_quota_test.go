package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestProviderQuotaSelector_ExhaustedOnlyMode(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
			2: {Status: "warning", Ready: true},
			3: {Status: "available", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "warning"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 2, got[0].Channel.ID)
	require.Equal(t, 3, got[1].Channel.ID)
}

func TestProviderQuotaSelector_DePrioritizeMode(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
			2: {Status: "warning", Ready: true},
			3: {Status: "available", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "de_prioritize"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "warning"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 2, got[0].Channel.ID)
	require.Equal(t, 3, got[1].Channel.ID)
}

func TestProviderQuotaSelector_EnforcementDisabled(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: "exhausted_only"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_AllExhausted(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
			2: {Status: "exhausted", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "c1"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "c2"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestProviderQuotaSelector_NoQuotaData(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "no-data"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_NilProvider(t *testing.T) {
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, nil, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_WrappedError(t *testing.T) {
	provider := &mockQuotaStatusProvider{}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{err: errors.New("inner error")}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	_, err := selector.Select(context.Background(), &llm.Request{})

	require.Error(t, err)
	require.Equal(t, "inner error", err.Error())
}

func TestProviderQuotaSelector_EmptyCandidates(t *testing.T) {
	provider := &mockQuotaStatusProvider{}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{candidates: []*ChannelModelsCandidate{}}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestProviderQuotaSelector_UnknownStatusKept(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "unknown", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "unknown"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 1)
}

func TestProviderQuotaSelector_MixedCandidates(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
			2: {Status: "warning", Ready: true},
			3: {Status: "available", Ready: true},
			4: {Status: "unknown", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "de_prioritize"},
	}

	inner := &mockSelector{
		candidates: []*ChannelModelsCandidate{
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "exhausted"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "warning"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "available"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 4, Name: "unknown"}}},
			{Channel: &biz.Channel{Channel: &ent.Channel{ID: 5, Name: "no-data"}}},
		},
	}

	selector := WithProviderQuotaSelector(inner, provider, settings)
	got, err := selector.Select(context.Background(), &llm.Request{})

	require.NoError(t, err)
	require.Len(t, got, 4)

	ids := make([]int, len(got))
	for i, c := range got {
		ids[i] = c.Channel.ID
	}
	require.ElementsMatch(t, []int{2, 3, 4, 5}, ids)
}
