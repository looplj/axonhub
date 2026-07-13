package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestA5_StreamMatchedStopSequence_SameProtocolRoundTrip(t *testing.T) {
	source := []*httpclient.StreamEvent{
		{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"msg_stream_stop_1","type":"message","role":"assistant","model":"claude-3-5-sonnet","content":[],"usage":{"input_tokens":2,"output_tokens":0}}}`)},
		{Type: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"stop_sequence","stop_sequence":"###END###"},"usage":{"output_tokens":3}}`)},
		{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
	}

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(context.Background(), nil, streams.SliceStream(source))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	replayed, err := inbound.TransformStream(context.Background(), streams.SliceStream(chunks))
	require.NoError(t, err)
	events, err := streams.All(replayed)
	require.NoError(t, err)

	var got *StreamEvent
	for _, event := range events {
		if event.Type != "message_delta" {
			continue
		}
		var candidate StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &candidate))
		got = &candidate
		break
	}
	require.NotNil(t, got, "same-protocol replay must emit message_delta")
	require.NotNil(t, got.Delta)
	require.Equal(t, "stop_sequence", *got.Delta.StopReason)
	require.Equal(t, "###END###", *got.Delta.StopSequence)
}
