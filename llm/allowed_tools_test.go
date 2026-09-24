package llm

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestAllowedToolsChoiceJSONRoundTrip(t *testing.T) {
	for _, mode := range []string{"auto", "required"} {
		original := ToolChoice{ToolChoice: lo.ToPtr(mode), NamedToolChoice: &NamedToolChoice{Type: "allowed_tools"}, Tools: []ToolOption{{Type: "function", Name: "a"}, {Type: "function", Name: "docs__b"}}}
		raw, err := json.Marshal(original)
		require.NoError(t, err)
		var decoded ToolChoice
		require.NoError(t, json.Unmarshal(raw, &decoded))
		require.Equal(t, original, decoded)
	}
}

func TestToolChoiceExistingJSONForms(t *testing.T) {
	for _, raw := range []string{`"auto"`, `"required"`, `{"type":"function","function":{"name":"a"}}`, `[{"type":"function","name":"a"}]`} {
		var choice ToolChoice
		require.NoError(t, json.Unmarshal([]byte(raw), &choice))
		encoded, err := json.Marshal(choice)
		require.NoError(t, err)
		require.JSONEq(t, raw, string(encoded))
	}
}
