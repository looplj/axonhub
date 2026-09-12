package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func TestNamespaceMetadataIsolation(t *testing.T) {
	src := &llm.Request{
		Tools:    []llm.Tool{{Type: "function", Function: llm.Function{Name: "docs__search", Namespace: "docs"}}},
		Messages: []llm.Message{{ToolCalls: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "retired__read", Namespace: "retired"}}}}},
	}
	existing := map[string]any{"other": "preserved"}
	src.TransformerMetadata = existing
	src.TransformerMetadata = namespaceMetadata(src)
	require.NotContains(t, existing, namespaceMappingMetadataKey)
	// Namespace lookup accepts the JSON-decoded metadata representation.
	data, err := json.Marshal(src)
	require.NoError(t, err)
	src = &llm.Request{}
	require.NoError(t, json.Unmarshal(data, src))
	first := *src
	first.TransformerMetadata = src.TransformerMetadata
	require.NotContains(t, existing, namespaceMappingMetadataKey)
	require.Equal(t, "preserved", first.TransformerMetadata["other"])
	src.Tools[0].Function.Namespace = "changed"
	second := *src
	second.TransformerMetadata = namespaceMetadata(src)
	response := &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{ToolCalls: []llm.ToolCall{
		{Type: "function", Function: llm.FunctionCall{Name: "docs__search"}},
		{Type: "function", Function: llm.FunctionCall{Name: "retired__read"}},
		{Type: "function", Function: llm.FunctionCall{Name: "unknown__tool"}},
		{Type: "function", Function: llm.FunctionCall{Name: "docs__search", Namespace: "explicit"}},
	}}}}}
	one := mapResponseFunctionNames(responseWithTestMetadata(&first, response), false)
	two := mapResponseFunctionNames(responseWithTestMetadata(&second, response), false)
	require.Equal(t, "docs", one.Choices[0].Message.ToolCalls[0].Function.Namespace)
	require.Equal(t, "changed", two.Choices[0].Message.ToolCalls[0].Function.Namespace)
	require.Equal(t, "search", one.Choices[0].Message.ToolCalls[0].Function.Name)
	require.Equal(t, "retired", one.Choices[0].Message.ToolCalls[1].Function.Namespace)
	require.Empty(t, one.Choices[0].Message.ToolCalls[2].Function.Namespace)
	require.Equal(t, "explicit", one.Choices[0].Message.ToolCalls[3].Function.Namespace)
	require.Empty(t, response.Choices[0].Message.ToolCalls[0].Function.Namespace)
	require.Same(t, response, mapResponseFunctionNames(response, false))
	require.Same(t, llm.DoneResponse, mapResponseFunctionNames(llm.DoneResponse, false))
	require.NotContains(t, namespaceMetadata(&llm.Request{TransformerMetadata: first.TransformerMetadata}), namespaceMappingMetadataKey)
}

func TestNamespaceStreamRepeatedCurrent(t *testing.T) {
	req := &llm.Request{
		Tools: []llm.Tool{{Type: "function", Function: llm.Function{Name: "docs__search", Namespace: "docs"}}},
	}
	req.TransformerMetadata = namespaceMetadata(req)
	original := &llm.Response{Choices: []llm.Choice{{Delta: &llm.Message{ToolCalls: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "docs__search"}}}}}}}
	stream := streams.MapErr(streamWithTestMetadata(req, streams.SliceStream([]*llm.Response{original, original, llm.DoneResponse})), func(resp *llm.Response) (*llm.Response, error) {
		return mapResponseFunctionNames(resp, false), nil
	})
	defer stream.Close()
	for range 2 {
		require.True(t, stream.Next())
		first := stream.Current()
		require.Same(t, first, stream.Current())
		require.Equal(t, "docs", first.Choices[0].Delta.ToolCalls[0].Function.Namespace)
		require.Equal(t, "search", first.Choices[0].Delta.ToolCalls[0].Function.Name)
	}
	require.Empty(t, original.Choices[0].Delta.ToolCalls[0].Function.Namespace)
	require.True(t, stream.Next())
	require.Same(t, llm.DoneResponse, stream.Current())
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
}
