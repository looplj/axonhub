package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestSplitAutoReasoningEffortModel(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		wantBaseModel  string
		wantEffort     string
		wantNormalized bool
	}{
		{
			name:           "supports xhigh",
			model:          "gpt-5.4-xhigh",
			wantBaseModel:  "gpt-5.4",
			wantEffort:     "xhigh",
			wantNormalized: true,
		},
		{
			name:           "supports max",
			model:          "gpt-5.4-max",
			wantBaseModel:  "gpt-5.4",
			wantEffort:     "max",
			wantNormalized: true,
		},
		{
			name:           "supports uppercase suffix",
			model:          "gpt-5.4-HIGH",
			wantBaseModel:  "gpt-5.4",
			wantEffort:     "high",
			wantNormalized: true,
		},
		{
			name:           "ignores unsupported suffix",
			model:          "gpt-5.4-ultra",
			wantNormalized: false,
		},
		{
			name:           "ignores missing base model",
			model:          "-high",
			wantNormalized: false,
		},
		{
			name:           "ignores model without suffix",
			model:          "gpt-5.4",
			wantNormalized: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseModel, effort, ok := splitAutoReasoningEffortModel(tt.model)
			assert.Equal(t, tt.wantNormalized, ok)
			assert.Equal(t, tt.wantBaseModel, baseModel)
			assert.Equal(t, tt.wantEffort, effort)
		})
	}
}

func TestAutoReasoningEffortMiddleware_OnInboundLlmRequest(t *testing.T) {
	tests := []struct {
		name       string
		settings   biz.SystemModelSettings
		request    *llm.Request
		wantModel  string
		wantEffort string
	}{
		{
			name: "disabled setting leaves request unchanged",
			settings: biz.SystemModelSettings{
				AutoReasoningEffort: false,
			},
			request: &llm.Request{
				Model: "gpt-5.4-xhigh",
			},
			wantModel:  "gpt-5.4-xhigh",
			wantEffort: "",
		},
		{
			name: "enabled setting applies xhigh suffix",
			settings: biz.SystemModelSettings{
				AutoReasoningEffort: true,
			},
			request: &llm.Request{
				Model: "gpt-5.4-xhigh",
			},
			wantModel:  "gpt-5.4",
			wantEffort: "xhigh",
		},
		{
			name: "enabled setting applies max suffix",
			settings: biz.SystemModelSettings{
				AutoReasoningEffort: true,
			},
			request: &llm.Request{
				Model: "gpt-5.4-max",
			},
			wantModel:  "gpt-5.4",
			wantEffort: "max",
		},
		{
			name: "suffix reasoning effort overrides explicit request value",
			settings: biz.SystemModelSettings{
				AutoReasoningEffort: true,
			},
			request: &llm.Request{
				Model:           "gpt-5.4-xhigh",
				ReasoningEffort: "high",
			},
			wantModel:  "gpt-5.4",
			wantEffort: "xhigh",
		},
		{
			name: "unsupported suffix is ignored",
			settings: biz.SystemModelSettings{
				AutoReasoningEffort: true,
			},
			request: &llm.Request{
				Model: "gpt-5.4-ultra",
			},
			wantModel:  "gpt-5.4-ultra",
			wantEffort: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := authz.WithTestBypass(context.Background())

			client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
			defer client.Close()

			ctx = ent.NewContext(ctx, client)
			_, _, systemService, _ := setupTestServices(t, client)

			err := systemService.SetModelSettings(ctx, tt.settings)
			require.NoError(t, err)

			middleware := applyAutoReasoningEffort(systemService)
			result, err := middleware.OnInboundLlmRequest(ctx, tt.request)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tt.wantModel, result.Model)
			assert.Equal(t, tt.wantEffort, result.ReasoningEffort)
		})
	}
}
