package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyRawToolChoice(t *testing.T) {
	tests := []struct {
		name             string
		raw              string
		kind             RawToolChoiceKind
		selectorType     string
		fullyRepresented bool
	}{
		{name: "string mode", raw: `"auto"`, kind: RawToolChoiceKindString, fullyRepresented: true},
		{name: "object mode", raw: `{"mode":"auto"}`, kind: RawToolChoiceKindModeObject, fullyRepresented: true},
		{
			name: "named function", raw: `{"type":"function","name":"lookup"}`,
			kind: RawToolChoiceKindNamed, selectorType: "function", fullyRepresented: true,
		},
		{
			name: "allowed tools", raw: `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"lookup"}]}`,
			kind: RawToolChoiceKindAllowedTools, selectorType: ToolChoiceTypeAllowedTools, fullyRepresented: true,
		},
		{
			name: "allowed tools without list", raw: `{"type":"allowed_tools","mode":"auto"}`,
			kind: RawToolChoiceKindAllowedTools, selectorType: ToolChoiceTypeAllowedTools,
		},
		{
			name: "future selector", raw: `{"type":"future_selector","policy":"strict"}`,
			kind: RawToolChoiceKindUnknown, selectorType: "future_selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyRawToolChoice(json.RawMessage(tt.raw))
			require.Equal(t, tt.kind, got.Kind)
			require.Equal(t, tt.selectorType, got.SelectorType)
			require.Equal(t, tt.fullyRepresented, got.FullyRepresented)
		})
	}
}
