package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestNamespaceRequest_Conflicts(t *testing.T) {
	for _, tt := range []struct {
		name      string
		functions []llm.Function
	}{
		{"namespace before direct", []llm.Function{{Name: "search", Namespace: "docs"}, {Name: "docs__search"}}},
		{"direct before namespace", []llm.Function{{Name: "docs__search"}, {Name: "search", Namespace: "docs"}}},
		{"ambiguous underscore encoding", []llm.Function{{Name: "c", Namespace: "a__b"}, {Name: "b__c", Namespace: "a"}}},
		{"duplicate namespace function", []llm.Function{{Name: "search", Namespace: "docs"}, {Name: "search", Namespace: "docs"}}},
		{"duplicate direct function", []llm.Function{{Name: "search"}, {Name: "search"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{}
			for _, fn := range tt.functions {
				req.Tools = append(req.Tools, llm.Tool{Type: "function", Function: fn})
			}
			_, _, err := prepareNamespaceRequest(req)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		})
	}
	t.Run("history conflicts with declaration", func(t *testing.T) {
		_, _, err := prepareNamespaceRequest(&llm.Request{
			Tools:    []llm.Tool{{Type: "function", Function: llm.Function{Name: "docs__search"}}},
			Messages: []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "search", Namespace: "docs"}}}}},
		})
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	})
}

func TestNamespaceRequest_Choice(t *testing.T) {
	tools := []llm.Tool{{Type: "function", Function: llm.Function{Name: "search", Namespace: "docs"}}}
	for _, tt := range []struct {
		name    string
		choice  *llm.ToolChoice
		tools   []llm.Tool
		want    string
		invalid bool
	}{
		{name: "unique namespace", tools: tools, choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "namespace", Function: llm.ToolFunction{Name: "docs"}}}, want: "docs__search"},
		{name: "missing namespace", choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "namespace", Function: llm.ToolFunction{Name: "docs"}}}, invalid: true},
		{name: "namespace list", tools: tools, choice: &llm.ToolChoice{Tools: []llm.ToolOption{{Type: "namespace", Name: "docs"}}}, want: "docs__search"},
		{name: "function list", tools: tools, choice: &llm.ToolChoice{Tools: []llm.ToolOption{{Type: "function", Name: "search", Namespace: "docs"}}}, want: "docs__search"},
		{name: "named function", tools: tools, choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "function", Function: llm.ToolFunction{Name: "search", Namespace: "docs"}}}, want: "docs__search"},
		{name: "unknown function", tools: tools, choice: &llm.ToolChoice{Tools: []llm.ToolOption{{Type: "function", Name: "missing", Namespace: "docs"}}}, invalid: true},
		{name: "unknown named function", tools: tools, choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "function", Function: llm.ToolFunction{Name: "missing", Namespace: "docs"}}}, invalid: true},
		{name: "direct function with underscores", choice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "function", Function: llm.ToolFunction{Name: "native__function"}}}, want: "native__function"},
		{name: "multiple allowed tools", tools: tools, choice: &llm.ToolChoice{Tools: []llm.ToolOption{{Type: "function", Name: "a"}, {Type: "function", Name: "b"}}}, invalid: true},
		{name: "unsupported tool", tools: tools, choice: &llm.ToolChoice{Tools: []llm.ToolOption{{Type: "tool_search", Name: "a"}}}, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chatNamespaceToolChoice(tt.choice, tt.tools)
			if tt.invalid {
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "function", got.NamedToolChoice.Type)
			require.Equal(t, tt.want, got.NamedToolChoice.Function.Name)
			require.Empty(t, got.NamedToolChoice.Function.Namespace)
			require.Empty(t, got.Tools)
		})
	}
	for _, mode := range []string{"auto", "none", "required"} {
		got, err := chatNamespaceToolChoice(&llm.ToolChoice{ToolChoice: lo.ToPtr(mode)}, tools)
		require.NoError(t, err)
		require.Equal(t, mode, *got.ToolChoice)
	}
}

func TestNamespaceRequest_AttemptIsolation(t *testing.T) {
	req := &llm.Request{
		Model:      "test",
		Tools:      []llm.Tool{{Type: "function", Function: llm.Function{Name: "search__v2", Namespace: "mcp__docs"}}},
		Messages:   []llm.Message{{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_1", Type: "function", Function: llm.FunctionCall{Name: "search__v2", Namespace: "mcp__docs", Arguments: "{}"}}}}},
		ToolChoice: &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "namespace", Function: llm.ToolFunction{Name: "mcp__docs"}}},
	}
	before, err := json.Marshal(req)
	require.NoError(t, err)
	out, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	first, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	second, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	require.JSONEq(t, string(first.Body), string(second.Body))
	after, err := json.Marshal(req)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	var body Request
	require.NoError(t, json.Unmarshal(first.Body, &body))
	require.Equal(t, "mcp__docs__search__v2", body.Tools[0].Function.Name)
	require.Equal(t, "mcp__docs__search__v2", body.Messages[0].ToolCalls[0].Function.Name)
	require.Equal(t, "mcp__docs__search__v2", body.ToolChoice.NamedToolChoice.Function.Name)
	require.NotContains(t, string(first.Body), `"namespace"`)
	mapping := extractNamespaceToolMapping(first)
	mapping["mcp__docs__search__v2"] = llm.ToolFunction{Name: "changed"}
	require.Equal(t, llm.ToolFunction{Name: "search__v2", Namespace: "mcp__docs"}, extractNamespaceToolMapping(second)["mcp__docs__search__v2"])
	// A response belonging to the second attempt must not observe the first's mutation.
	response, err := out.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Request: second, Body: []byte(`{"choices":[{"index":0,"message":{"tool_calls":[{"type":"function","function":{"name":"mcp__docs__search__v2","arguments":"{}"}},{"type":"function","function":{"name":"plain__function","arguments":"{}"}}]}}]}`)})
	require.NoError(t, err)
	calls := response.Choices[0].Message.ToolCalls
	require.Equal(t, "search__v2", calls[0].Function.Name)
	require.Equal(t, "mcp__docs", calls[0].Function.Namespace)
	require.Equal(t, "plain__function", calls[1].Function.Name)
	require.Empty(t, calls[1].Function.Namespace)
}
