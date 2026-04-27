package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/server/biz"
)

type mockQuotaStatusProvider struct {
	statuses map[int]*biz.QuotaChannelStatus
}

func (m *mockQuotaStatusProvider) GetQuotaStatus(channelID int) *biz.QuotaChannelStatus {
	if m.statuses == nil {
		return nil
	}
	return m.statuses[channelID]
}

type mockQuotaEnforcementSettingsProvider struct {
	settings *biz.QuotaEnforcementSettings
}

func (m *mockQuotaEnforcementSettingsProvider) QuotaEnforcementSettingsOrDefault(_ context.Context) *biz.QuotaEnforcementSettings {
	if m.settings == nil {
		return &biz.QuotaEnforcementSettings{Enabled: false, Mode: biz.QuotaEnforcementModeExhaustedOnly}
	}
	return m.settings
}

func TestQuotaAwareStrategy_Score_EnforcementDisabled(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Exhausted(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, float64(quotaExhaustedScore), strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Warning_DePrioritize(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusWarning, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeDePrioritize},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score := strategy.Score(ctx, channel)
	assert.Less(t, score, 0.0, "warning in de_prioritize mode should have negative score (penalty)")
	assert.Greater(t, score, float64(quotaExhaustedScore), "warning should rank above exhausted")
	assert.InDelta(t, -20.0, score, 0.0001)
}

func TestQuotaAwareStrategy_Score_Warning_ExhaustedOnly(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusWarning, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Available(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Unknown(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusUnknown, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_NilQuotaData(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_NilProvider(t *testing.T) {
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(nil, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_UnrecognizedStatus(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "something_else", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_ScoreWithDebug_EnforcementDisabled(t *testing.T) {
	provider := &mockQuotaStatusProvider{}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "QuotaAware", debug.StrategyName)
	assert.Equal(t, false, debug.Details["enforcement_enabled"])
	assert.Equal(t, "enforcement_disabled", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_Exhausted(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, float64(quotaExhaustedScore), score)
	assert.Equal(t, "QuotaAware", debug.StrategyName)
	assert.Equal(t, true, debug.Details["enforcement_enabled"])
	assert.Equal(t, string(biz.QuotaEnforcementModeExhaustedOnly), string(debug.Details["mode"].(biz.QuotaEnforcementMode)))
	assert.Equal(t, string(providerquotastatus.StatusExhausted), string(debug.Details["quota_status"].(providerquotastatus.Status)))
	assert.Equal(t, "quota_exhausted", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_Warning_DePrioritize(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusWarning, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeDePrioritize},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.InDelta(t, -20.0, score, 0.0001)
	assert.Equal(t, "QuotaAware", debug.StrategyName)
	assert.Equal(t, string(providerquotastatus.StatusWarning), string(debug.Details["quota_status"].(providerquotastatus.Status)))
	assert.Equal(t, "warning_de_prioritize", debug.Details["score_reason"])
	assert.Equal(t, 0.8, debug.Details["usage_ratio"])
	assert.InDelta(t, -20.0, debug.Details["scaled_score"], 0.0001)
}

func TestQuotaAwareStrategy_ScoreWithDebug_NilQuotaData(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "no_data", debug.Details["quota_status"])
	assert.Equal(t, "no_quota_data", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_NilProvider(t *testing.T) {
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(nil, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "no_provider", debug.Details["quota_status"])
	assert.Equal(t, "no_quota_provider", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_Available(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusAvailable, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, string(providerquotastatus.StatusAvailable), string(debug.Details["quota_status"].(providerquotastatus.Status)))
	assert.Equal(t, "status_available", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_Unknown(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusUnknown, Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, string(providerquotastatus.StatusUnknown), string(debug.Details["quota_status"].(providerquotastatus.Status)))
	assert.Equal(t, "status_unknown", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_UnrecognizedStatus(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "glitched", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeExhaustedOnly},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "unrecognized", debug.Details["quota_status"])
	assert.Equal(t, "glitched", string(debug.Details["raw_status"].(providerquotastatus.Status)))
	assert.Equal(t, "status_unrecognized", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_Name(t *testing.T) {
	strategy := NewQuotaAwareStrategy(nil, nil)
	assert.Equal(t, "QuotaAware", strategy.Name())
}

func TestQuotaAwareStrategy_Score_MultipleChannels(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
			2: {Status: providerquotastatus.StatusAvailable, Ready: true},
			3: {Status: providerquotastatus.StatusWarning, Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: biz.QuotaEnforcementModeDePrioritize},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	c1 := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "c1"}}
	c2 := &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "c2"}}
	c3 := &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "c3"}}

	ctx := context.Background()
	assert.Equal(t, float64(quotaExhaustedScore), strategy.Score(ctx, c1))
	assert.Equal(t, 0.0, strategy.Score(ctx, c2))
	assert.InDelta(t, -20.0, strategy.Score(ctx, c3), 0.0001)
}

func TestQuotaAwareStrategy_Score_ExhaustedBothModes(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: providerquotastatus.StatusExhausted, Ready: false},
		},
	}

	for _, mode := range []biz.QuotaEnforcementMode{biz.QuotaEnforcementModeExhaustedOnly, biz.QuotaEnforcementModeDePrioritize} {
		settings := &mockQuotaEnforcementSettingsProvider{
			settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: mode},
		}
		strategy := NewQuotaAwareStrategy(provider, settings)

		channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
		ctx := context.Background()

		assert.Equal(t, float64(quotaExhaustedScore), strategy.Score(ctx, channel),
			"exhausted should get penalty in mode=%s", mode)
	}
}
