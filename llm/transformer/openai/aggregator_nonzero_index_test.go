package openai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestAggregateStreamChunksNonZeroChoiceIndex ensures that a stream whose only
// choice carries a non-zero index aggregates without panicking. choicesAggs is
// keyed by the choice index, so a sparse/non-zero-based index must not be
// looked up positionally.
func TestAggregateStreamChunksNonZeroChoiceIndex(t *testing.T) {
	chunk := `{"id":"chatcmpl-1","model":"gpt-4o-mini","object":"chat.completion.chunk","created":1,` +
		`"choices":[{"index":1,"delta":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`

	chunks := []*httpclient.StreamEvent{{Data: []byte(chunk)}}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	require.NoError(t, json.Unmarshal(gotBytes, &got))
	require.Len(t, got.Choices, 1)
	require.Equal(t, 1, got.Choices[0].Index)
	require.NotNil(t, got.Choices[0].Message.Content.Content)
	require.Equal(t, "hi", *got.Choices[0].Message.Content.Content)
}

func TestAggregateStreamChunksNoUsage(t *testing.T) {
	chunk := `{"id":"chatcmpl-1","model":"gpt-4o-mini","object":"chat.completion.chunk","created":1,` +
		`"choices":[{"index":0,"delta":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`

	gotBytes, meta, err := AggregateStreamChunks(context.Background(), []*httpclient.StreamEvent{{Data: []byte(chunk)}}, DefaultTransformChunk)
	require.NoError(t, err)
	require.Nil(t, meta.Usage)

	var got llm.Response
	require.NoError(t, json.Unmarshal(gotBytes, &got))
	require.Nil(t, got.Usage)
	require.Len(t, got.Choices, 1)
	require.Equal(t, "hi", *got.Choices[0].Message.Content.Content)
}

func TestAggregateStreamChunksNonZeroToolCallIndex(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{
			Data: []byte(`{"id":"chatcmpl-1","model":"gpt-4o-mini","object":"chat.completion.chunk","created":1,` +
				`"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":1,"id":"call_1","type":"function","function":{"name":"search","arguments":"{\"q\":"}}]}}]}`),
		},
		{
			Data: []byte(`{"id":"chatcmpl-1","model":"gpt-4o-mini","object":"chat.completion.chunk","created":1,` +
				`"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"\"axonhub\"}"}}]},"finish_reason":"tool_calls"}]}`),
		},
	}

	gotBytes, _, err := AggregateStreamChunks(context.Background(), chunks, DefaultTransformChunk)
	require.NoError(t, err)

	var got llm.Response
	require.NoError(t, json.Unmarshal(gotBytes, &got))
	require.Len(t, got.Choices, 1)
	require.Len(t, got.Choices[0].Message.ToolCalls, 1)
	require.Equal(t, 1, got.Choices[0].Message.ToolCalls[0].Index)
	require.Equal(t, "call_1", got.Choices[0].Message.ToolCalls[0].ID)
	require.Equal(t, "search", got.Choices[0].Message.ToolCalls[0].Function.Name)
	require.Equal(t, `{"q":"axonhub"}`, got.Choices[0].Message.ToolCalls[0].Function.Arguments)
}
