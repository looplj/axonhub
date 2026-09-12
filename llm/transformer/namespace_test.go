package transformer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestValidateFlatFunctionNames_RepeatedHistory(t *testing.T) {
	request := &llm.Request{
		Tools: []llm.Tool{
			{Type: "function", Function: llm.Function{Name: "search", Namespace: "docs"}},
			{Type: "custom", Function: llm.Function{Name: "docs__search"}},
		},
		Messages: []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{
			{Type: "function", Function: llm.FunctionCall{Name: "search", Namespace: "docs"}},
			{Function: llm.FunctionCall{Name: "search", Namespace: "docs"}},
			{Type: "custom", Function: llm.FunctionCall{Name: "docs__search"}},
		}}},
	}
	require.NoError(t, ValidateFlatFunctionNames(request))
	request.Tools = nil
	require.NoError(t, ValidateFlatFunctionNames(request))
}

func TestValidateNamespaceToolChoice_Modes(t *testing.T) {
	require.NoError(t, ValidateNamespaceToolChoice(nil, nil))
	for _, mode := range []string{"auto", "none", "required"} {
		choice := &llm.ToolChoice{ToolChoice: &mode}
		require.NoError(t, ValidateNamespaceToolChoice(choice, nil))
		choice.Tools = []llm.ToolOption{{Type: "namespace", Name: "docs"}}
		require.ErrorIs(t, ValidateNamespaceToolChoice(choice, nil), ErrInvalidRequest)
	}
}
