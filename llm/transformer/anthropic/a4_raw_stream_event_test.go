package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestA4_UnknownAnthropicStreamEvent_SameProtocolRawReplay(t *testing.T) {
	source := &httpclient.StreamEvent{
		Type: "future_provider_event",
		Data: []byte(`{"type":"future_provider_event","index":3,"future":{"x":1}}`),
	}

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformStream(context.Background(), nil, streams.SliceStream([]*httpclient.StreamEvent{source}))
	require.NoError(t, err)
	chunks, err := streams.All(canonical)
	require.NoError(t, err)
	require.Len(t, chunks, 2, "raw event plus done marker")

	inbound := NewInboundTransformer()
	replayed, err := inbound.TransformStream(context.Background(), streams.SliceStream(chunks))
	require.NoError(t, err)
	events, err := streams.All(replayed)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, source.Type, events[0].Type)

	var want, got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source.Data, &want))
	require.NoError(t, json.Unmarshal(events[0].Data, &got))
	require.Equal(t, want, got)
}

func TestA4_UnknownAnthropicContentBlockLifecycle_SameProtocolRawReplay(t *testing.T) {
	source := []*httpclient.StreamEvent{
		{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"msg_future_block","type":"message","role":"assistant","model":"claude-3-5-sonnet","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`)},
		{Type: "content_block_start", Data: []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"future_content","token":"start"}}`)},
		{Type: "content_block_delta", Data: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"future_delta","token":"delta"}}`)},
		{Type: "content_block_stop", Data: []byte(`{"type":"content_block_stop","index":0}`)},
		{Type: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`)},
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

	wantByType := map[string][]byte{}
	for _, event := range source[1:4] {
		wantByType[event.Type] = event.Data
	}
	for eventType, wantData := range wantByType {
		var gotData []byte
		for _, event := range events {
			if event.Type == eventType {
				var parsed StreamEvent
				require.NoError(t, json.Unmarshal(event.Data, &parsed))
				if parsed.Index != nil && *parsed.Index == 0 {
					gotData = event.Data
					break
				}
			}
		}
		require.NotEmpty(t, gotData, "missing replayed %s event", eventType)
		var want, got map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(wantData, &want))
		require.NoError(t, json.Unmarshal(gotData, &got))
		require.Equal(t, want, got, "must preserve unknown content block %s event", eventType)
	}

	stopsAtIndexZero := 0
	for _, event := range events {
		if event.Type != "content_block_stop" {
			continue
		}
		var parsed StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &parsed))
		if parsed.Index != nil && *parsed.Index == 0 {
			stopsAtIndexZero++
		}
	}
	require.Equal(t, 1, stopsAtIndexZero, "raw lifecycle replay must not synthesize a duplicate block stop")
}

func TestA4_UnknownContentBlockThenText_SynchronizesContentIndex(t *testing.T) {
	source := []*httpclient.StreamEvent{
		{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"msg_future_then_text","type":"message","role":"assistant","model":"claude-3-5-sonnet","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`)},
		{Type: "content_block_start", Data: []byte(`{"type":"content_block_start","index":0,"content_block":{"type":"future_content","token":"start"}}`)},
		{Type: "content_block_delta", Data: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"future_delta","token":"delta"}}`)},
		{Type: "content_block_stop", Data: []byte(`{"type":"content_block_stop","index":0}`)},
		{Type: "content_block_start", Data: []byte(`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`)},
		{Type: "content_block_delta", Data: []byte(`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"after raw block"}}`)},
		{Type: "content_block_stop", Data: []byte(`{"type":"content_block_stop","index":1}`)},
		{Type: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`)},
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

	var textStart *StreamEvent
	stopsByIndex := map[int64]int{}
	for _, event := range events {
		var parsed StreamEvent
		require.NoError(t, json.Unmarshal(event.Data, &parsed))
		if event.Type == "content_block_start" && parsed.ContentBlock != nil && parsed.ContentBlock.Type == "text" {
			textStart = &parsed
		}
		if event.Type == "content_block_stop" && parsed.Index != nil {
			stopsByIndex[*parsed.Index]++
		}
	}

	require.NotNil(t, textStart, "the structured text block must be emitted after the raw lifecycle")
	require.NotNil(t, textStart.Index)
	require.Equal(t, int64(1), *textStart.Index, "text must start after the replayed raw block")
	require.Equal(t, 1, stopsByIndex[0], "raw lifecycle must not gain a duplicate stop")
	require.Equal(t, 1, stopsByIndex[1], "structured text block must close at its own index")
}
