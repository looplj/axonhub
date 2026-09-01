package openai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// A provider that ends the stream with [DONE] but no finish_reason must not
// lose tool calls the restorer still buffers (e.g. names held back because
// they are prefixes of other catalog names).
func TestOutboundTransformer_TransformStream_FlushesBufferedToolCallsBeforeDone(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("https://api.openai.com/v1", "test-key")
	require.NoError(t, err)
	transformer := transformerInterface.(*OutboundTransformer)

	req := &httpclient.Request{
		TransformerMetadata: map[string]any{
			ResponsesChatToolCatalogMetadataKey: []string{"Task", "TaskOutput"},
		},
	}

	source := streams.SliceStream([]*httpclient.StreamEvent{
		{Data: []byte(`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_task","type":"function","function":{"name":"Task","arguments":"{\"prompt\":\"fix the bug\"}"}}]}}]}`)},
		{Data: []byte("[DONE]")},
	})

	stream, err := transformer.TransformStream(context.Background(), req, source)
	require.NoError(t, err)

	var calls []llm.ToolCall
	sawDone := false
	for stream.Next() {
		resp := stream.Current()
		if resp == llm.DoneResponse || resp.Object == "[DONE]" {
			sawDone = true
			continue
		}
		for _, choice := range resp.Choices {
			if choice.Delta != nil && !sawDone {
				calls = append(calls, choice.Delta.ToolCalls...)
			}
		}
	}
	require.NoError(t, stream.Err())
	require.True(t, sawDone)
	require.Len(t, calls, 1)
	require.Equal(t, "Task", calls[0].Function.Name)
	require.Equal(t, "call_task", calls[0].ID)
	require.Equal(t, `{"prompt":"fix the bug"}`, calls[0].Function.Arguments)
}
