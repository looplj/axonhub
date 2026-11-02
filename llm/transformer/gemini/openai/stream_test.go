package geminioai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm/streams"
)

// Gemini's OpenAI-compatible endpoint streams each parallel tool call whole in its own
// chunk, without tool_calls[].index, and finishes with "stop".
const parallelToolStream = "gemini_openai-parallel_tool.stream.jsonl"

func streamedToolCalls(t *testing.T, chunks []*httpclient.StreamEvent) []llm.ToolCall {
	t.Helper()

	outbound, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
	require.NoError(t, err)

	stream, err := outbound.TransformStream(t.Context(), &httpclient.Request{}, streams.SliceStream(chunks))
	require.NoError(t, err)

	responses, err := streams.All(stream)
	require.NoError(t, err)

	var toolCalls []llm.ToolCall

	for _, resp := range responses {
		for _, choice := range resp.Choices {
			if choice.Delta != nil {
				toolCalls = append(toolCalls, choice.Delta.ToolCalls...)
			}
		}
	}

	return toolCalls
}

func TestOutboundTransformer_TransformStream_IndexesParallelToolCallsWithoutIndex(t *testing.T) {
	chunks, err := xtest.LoadStreamChunks(t, parallelToolStream)
	require.NoError(t, err)

	toolCalls := streamedToolCalls(t, chunks)

	require.Len(t, toolCalls, 2)
	assert.Equal(t, 0, toolCalls[0].Index)
	assert.Equal(t, "call_4234484", toolCalls[0].ID)
	assert.Equal(t, 1, toolCalls[1].Index)
	assert.Equal(t, "call_4234487", toolCalls[1].ID)
}

func TestOutboundTransformer_TransformStream_AppendsUnindexedArgumentFragmentToLastToolCall(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"function":{"arguments":"\"Paris\"}"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_b","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Tokyo\"}"}}]}}]}`)},
		{Data: []byte(`[DONE]`)},
	}

	toolCalls := streamedToolCalls(t, chunks)

	require.Len(t, toolCalls, 3)
	assert.Equal(t, []int{0, 0, 1}, []int{toolCalls[0].Index, toolCalls[1].Index, toolCalls[2].Index})
}

func TestOutboundTransformer_TransformStream_ContinuesUnindexedToolCallWithSameID(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_a","function":{"arguments":"\"Paris\"}"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_b","type":"function","function":{"name":"get_weather","arguments":"{}"}}]}}]}`)},
		{Data: []byte(`[DONE]`)},
	}

	toolCalls := streamedToolCalls(t, chunks)

	require.Len(t, toolCalls, 3)
	assert.Equal(t, []int{0, 0, 1}, []int{toolCalls[0].Index, toolCalls[1].Index, toolCalls[2].Index})
}

func TestOutboundTransformer_TransformStream_IndexesUnindexedToolCallsPerChoice(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[` +
			`{"index":0,"delta":{"tool_calls":[{"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"{}"}}]}},` +
			`{"index":1,"delta":{"tool_calls":[{"id":"call_b","type":"function","function":{"name":"get_weather","arguments":"{}"}},` +
			`{"id":"call_c","type":"function","function":{"name":"get_time","arguments":"{}"}}]}}]}`)},
		{Data: []byte(`[DONE]`)},
	}

	toolCalls := streamedToolCalls(t, chunks)

	require.Len(t, toolCalls, 3)
	assert.Equal(t, []string{"call_a", "call_b", "call_c"}, []string{toolCalls[0].ID, toolCalls[1].ID, toolCalls[2].ID})
	assert.Equal(t, []int{0, 0, 1}, []int{toolCalls[0].Index, toolCalls[1].Index, toolCalls[2].Index})
}

func TestOutboundTransformer_AggregateStreamChunks_KeepsParallelToolCallsWithoutIndex(t *testing.T) {
	chunks, err := xtest.LoadStreamChunks(t, parallelToolStream)
	require.NoError(t, err)

	outbound, err := NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-key")
	require.NoError(t, err)

	data, _, err := outbound.AggregateStreamChunks(t.Context(), &httpclient.Request{}, chunks)
	require.NoError(t, err)

	var resp llm.Response
	require.NoError(t, json.Unmarshal(data, &resp))
	require.Len(t, resp.Choices, 1)
	require.NotNil(t, resp.Choices[0].Message)

	toolCalls := resp.Choices[0].Message.ToolCalls
	require.Len(t, toolCalls, 2)
	assert.Equal(t, "call_4234484", toolCalls[0].ID)
	assert.JSONEq(t, `{"city":"Paris"}`, toolCalls[0].Function.Arguments)
	assert.Equal(t, "call_4234487", toolCalls[1].ID)
	assert.JSONEq(t, `{"city":"Tokyo"}`, toolCalls[1].Function.Arguments)
}

func TestOutboundTransformer_TransformStream_ContinuesUnindexedFragmentOfLastIndexedToolCall(t *testing.T) {
	chunks := []*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"get_weather","arguments":"{\"city\":"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"get_time","arguments":"{}"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Paris"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"function":{"arguments":"\"}"}}]}}]}`)},
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_a","function":{"arguments":" "}}]}}]}`)},
		{Data: []byte(`[DONE]`)},
	}

	toolCalls := streamedToolCalls(t, chunks)

	require.Len(t, toolCalls, 5)
	assert.Equal(t, []int{0, 1, 0, 0, 0}, []int{toolCalls[0].Index, toolCalls[1].Index, toolCalls[2].Index, toolCalls[3].Index, toolCalls[4].Index})
}
