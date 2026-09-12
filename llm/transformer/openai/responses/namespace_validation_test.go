package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestNamespaceValidation_NameConflicts(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		functions []llm.Function
		history   []llm.ToolCall
	}{
		{name: "namespace before direct", functions: []llm.Function{{Name: "search", Namespace: "docs"}, {Name: "docs__search"}}},
		{name: "direct before namespace", functions: []llm.Function{{Name: "docs__search"}, {Name: "search", Namespace: "docs"}}},
		{name: "ambiguous underscores", functions: []llm.Function{{Name: "c", Namespace: "a__b"}, {Name: "b__c", Namespace: "a"}}},
		{name: "duplicate namespace function", functions: []llm.Function{{Name: "search", Namespace: "docs"}, {Name: "search", Namespace: "docs"}}},
		{name: "duplicate direct function", functions: []llm.Function{{Name: "search"}, {Name: "search"}}},
		{name: "history conflicts with declaration", functions: []llm.Function{{Name: "docs__search"}}, history: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "search", Namespace: "docs"}}}},
		{name: "history identities conflict", history: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "c", Namespace: "a__b"}}, {Function: llm.FunctionCall{Name: "b__c", Namespace: "a"}}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			input := Request{Model: "test", Input: Input{Text: lo.ToPtr("hi")}}
			for _, fn := range scenario.functions {
				tool := Tool{Type: "function", Name: fn.Name}
				if fn.Namespace != "" {
					tool = Tool{Type: "namespace", Name: fn.Namespace, Tools: []Tool{tool}}
				}
				input.Tools = append(input.Tools, tool)
			}
			if len(scenario.history) > 0 {
				input.Input = Input{}
				for _, call := range scenario.history {
					input.Input.Items = append(input.Input.Items, Item{Type: "function_call", CallID: "call_1", Name: call.Function.Name, Namespace: call.Function.Namespace, Arguments: "{}"})
				}
			}
			raw, err := json.Marshal(input)
			require.NoError(t, err)
			wire, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
			require.Nil(t, wire)
		})
	}
}
