package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
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
		return &biz.QuotaEnforcementSettings{Enabled: false, Mode: "exhausted_only"}
	}
	return m.settings
}

func TestQuotaAwareStrategy_Score_EnforcementDisabled(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Exhausted(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, float64(quotaExhaustedScore), strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Warning_DePrioritize(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "warning", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "de_prioritize"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score := strategy.Score(ctx, channel)
	// usageRatio=0.8, score = scaleScore(100, 1-0.8) = scaleScore(100, 0.2) = 20
	assert.Greater(t, score, 0.0, "warning in de_prioritize mode should have positive score")
	assert.Less(t, score, 100.0, "warning in de_prioritize mode should have score < 100")
	assert.InDelta(t, 20.0, score, 0.0001)
}

func TestQuotaAwareStrategy_Score_Warning_ExhaustedOnly(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "warning", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Available(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "available", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_Unknown(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "unknown", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
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
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_Score_NilProvider(t *testing.T) {
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
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
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	assert.Equal(t, 0.0, strategy.Score(ctx, channel))
}

func TestQuotaAwareStrategy_ScoreWithDebug_EnforcementDisabled(t *testing.T) {
	provider := &mockQuotaStatusProvider{}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: false, Mode: "exhausted_only"},
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
			1: {Status: "exhausted", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, float64(quotaExhaustedScore), score)
	assert.Equal(t, "QuotaAware", debug.StrategyName)
	assert.Equal(t, true, debug.Details["enforcement_enabled"])
	assert.Equal(t, "exhausted_only", debug.Details["mode"])
	assert.Equal(t, "exhausted", debug.Details["quota_status"])
	assert.Equal(t, "quota_exhausted", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_Warning_DePrioritize(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "warning", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "de_prioritize"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.InDelta(t, 20.0, score, 0.0001)
	assert.Equal(t, "QuotaAware", debug.StrategyName)
	assert.Equal(t, "warning", debug.Details["quota_status"])
	assert.Equal(t, "warning_de_prioritize", debug.Details["score_reason"])
	assert.Equal(t, 0.8, debug.Details["usage_ratio"])
	assert.InDelta(t, 20.0, debug.Details["scaled_score"], 0.0001)
}

func TestQuotaAwareStrategy_ScoreWithDebug_NilQuotaData(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
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
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
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
			1: {Status: "available", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "available", debug.Details["quota_status"])
	assert.Equal(t, "status_available", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_Unknown(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "unknown", Ready: false},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "unknown", debug.Details["quota_status"])
	assert.Equal(t, "status_unknown", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_ScoreWithDebug_UnrecognizedStatus(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "glitched", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "exhausted_only"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	channel := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "test"}}
	ctx := context.Background()

	score, debug := strategy.ScoreWithDebug(ctx, channel)

	assert.Equal(t, 0.0, score)
	assert.Equal(t, "unrecognized", debug.Details["quota_status"])
	assert.Equal(t, "glitched", debug.Details["raw_status"])
	assert.Equal(t, "status_unrecognized", debug.Details["score_reason"])
}

func TestQuotaAwareStrategy_Name(t *testing.T) {
	strategy := NewQuotaAwareStrategy(nil, nil)
	assert.Equal(t, "QuotaAware", strategy.Name())
}

func TestQuotaAwareStrategy_Score_MultipleChannels(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
			2: {Status: "available", Ready: true},
			3: {Status: "warning", Ready: true},
		},
	}
	settings := &mockQuotaEnforcementSettingsProvider{
		settings: &biz.QuotaEnforcementSettings{Enabled: true, Mode: "de_prioritize"},
	}
	strategy := NewQuotaAwareStrategy(provider, settings)

	c1 := &biz.Channel{Channel: &ent.Channel{ID: 1, Name: "c1"}}
	c2 := &biz.Channel{Channel: &ent.Channel{ID: 2, Name: "c2"}}
	c3 := &biz.Channel{Channel: &ent.Channel{ID: 3, Name: "c3"}}

	ctx := context.Background()
	assert.Equal(t, float64(quotaExhaustedScore), strategy.Score(ctx, c1))
	assert.Equal(t, 0.0, strategy.Score(ctx, c2))
	assert.InDelta(t, 20.0, strategy.Score(ctx, c3), 0.0001)
}

func TestQuotaAwareStrategy_Score_ExhaustedBothModes(t *testing.T) {
	provider := &mockQuotaStatusProvider{
		statuses: map[int]*biz.QuotaChannelStatus{
			1: {Status: "exhausted", Ready: false},
		},
	}

	for _, mode := range []string{"exhausted_only", "de_prioritize"} {
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
