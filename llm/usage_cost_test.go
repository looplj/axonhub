package llm

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestUsageUnmarshalJSON_CostShapes(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCost *float64
	}{
		{
			name:     "object cost with string amount",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":{"currency":"CNY","amount":"0.0011","breakdown":[]}}`,
			wantCost: lo.ToPtr(0.0011),
		},
		{
			name:     "object cost with numeric amount",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":{"currency":"CNY","amount":0.25}}`,
			wantCost: lo.ToPtr(0.25),
		},
		{
			name:     "numeric cost",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":0.5}`,
			wantCost: lo.ToPtr(0.5),
		},
		{
			name:     "string cost",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":"0.125"}`,
			wantCost: lo.ToPtr(0.125),
		},
		{
			name:     "NaN string cost",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":"NaN"}`,
			wantCost: nil,
		},
		{
			name:     "Infinity string cost",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":"Infinity"}`,
			wantCost: nil,
		},
		{
			name:     "object cost with NaN string amount",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":{"currency":"CNY","amount":"NaN"}}`,
			wantCost: nil,
		},
		{
			name:     "object cost with Infinity string amount",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":{"currency":"CNY","amount":"Infinity"}}`,
			wantCost: nil,
		},
		{
			name:     "null cost",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"cost":null}`,
			wantCost: nil,
		},
		{
			name:     "missing cost",
			body:     `{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}`,
			wantCost: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var usage Usage
			require.NoError(t, json.Unmarshal([]byte(tt.body), &usage))
			require.Equal(t, int64(10), usage.PromptTokens)
			require.Equal(t, int64(5), usage.CompletionTokens)
			require.Equal(t, int64(15), usage.TotalTokens)

			_, err := json.Marshal(usage)
			require.NoError(t, err)

			if tt.wantCost == nil {
				require.Nil(t, usage.Cost)

				return
			}

			require.NotNil(t, usage.Cost)
			require.InDelta(t, *tt.wantCost, *usage.Cost, 1e-12)
		})
	}
}
