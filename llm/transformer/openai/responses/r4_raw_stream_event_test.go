package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// R4: a valid official Responses SSE event without a canonical chunk mapping
// must survive the same-protocol outbound/inbound stream path verbatim.
func TestR4_UnknownResponsesStreamEvent_SameProtocolRawReplay(t *testing.T) {
	source := &httpclient.StreamEvent{
		Type: "response.audio.delta",
		Data: []byte(`{"type":"response.audio.delta","sequence_number":9,"item_id":"audio_1","output_index":0,"content_index":0,"delta":"AQID","future_audio_field":{"x":1}}`),
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(context.Background(), nil, streams.SliceStream([]*httpclient.StreamEvent{source}))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)
	require.Len(t, chunks, 2, "raw event plus done marker")
	require.NotNil(t, chunks[0].ProviderExtensions)

	inbound := NewInboundTransformer()
	replayed, err := inbound.TransformStream(context.Background(), streams.SliceStream(chunks))
	require.NoError(t, err)
	events, err := streams.All(replayed)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "response.audio.delta", events[0].Type)

	var want, got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source.Data, &want))
	require.NoError(t, json.Unmarshal(events[0].Data, &got))
	require.Equal(t, want, got)
}

func TestR4_QueuedResponsesStreamEvent_SameProtocolRawReplay(t *testing.T) {
	source := &httpclient.StreamEvent{
		Type: "response.queued",
		Data: []byte(`{"type":"response.queued","sequence_number":2,"response":{"id":"resp_q","object":"response","created_at":1,"model":"gpt-5","status":"queued","output":[]}}`),
	}

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(context.Background(), nil, streams.SliceStream([]*httpclient.StreamEvent{source}))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	replayed, err := inbound.TransformStream(context.Background(), streams.SliceStream(chunks))
	require.NoError(t, err)
	events, err := streams.All(replayed)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "response.queued", events[0].Type)
	require.JSONEq(t, string(source.Data), string(events[0].Data))
}
