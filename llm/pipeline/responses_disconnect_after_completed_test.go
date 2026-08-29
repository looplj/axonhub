package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	responsestransformer "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// errAfterSliceStream yields the given events, then surfaces err from Err()
// (like an upstream that sends response.completed and then the chunked body is
// truncated — the decoder returns io.ErrUnexpectedEOF / "unexpected EOF").
type errAfterSliceStream struct {
	events []*httpclient.StreamEvent
	index  int
	err    error
}

// Next reports whether more events remain in the slice.
func (s *errAfterSliceStream) Next() bool { return s.index < len(s.events) }

// Current returns the next buffered event.
func (s *errAfterSliceStream) Current() *httpclient.StreamEvent {
	if s.index < len(s.events) {
		item := s.events[s.index]
		s.index++
		return item
	}
	return nil
}

// Err returns the configured stream error.
func (s *errAfterSliceStream) Err() error { return s.err }

// Close is a no-op for the in-memory slice stream.
func (s *errAfterSliceStream) Close() error { return nil }

// TestPipeline_ResponsesUpstreamDisconnectAfterCompleted reproduces the
// production incident: the upstream sends a full event sequence ending in
// response.completed, then the connection breaks (chunked body truncated,
// decoder returns "unexpected EOF"). The client (pi) must receive the
// already-delivered response.completed event with a clean stream end, without a
// conflicting response.failed or bare error.
func TestPipeline_ResponsesUpstreamDisconnectAfterCompleted(t *testing.T) {
	inbound := responsestransformer.NewInboundTransformer()
	outbound, err := responsestransformer.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	streamEvents := []*httpclient.StreamEvent{
		{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_disconnect","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
		{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAA_1"}}`)},
		{Type: "response.output_item.done", Data: []byte(`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAA_1"}}`)},
		{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_disconnect","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}}}`)},
	}

	executor := &mockExecutor{doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
		return &errAfterSliceStream{
			events: streamEvents,
			err:    errors.New("unexpected EOF"),
		}, nil
	}}

	pipe := pipeline.NewFactory(executor).Pipeline(inbound, outbound)
	body, err := json.Marshal(map[string]any{
		"model":  "gpt-5",
		"stream": true,
		"input":  "hello",
	})
	require.NoError(t, err)

	result, err := pipe.Process(context.Background(), &httpclient.Request{
		Method:      http.MethodPost,
		URL:         "/v1/responses",
		ContentType: "application/json",
		Headers:     http.Header{"Content-Type": []string{"application/json"}},
		Body:        body,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Stream)

	var eventTypes []string
	var lastEvent map[string]any
	for result.EventStream.Next() {
		cur := result.EventStream.Current()
		var ev map[string]any
		require.NoError(t, json.Unmarshal(cur.Data, &ev))
		eventTypes = append(eventTypes, ev["type"].(string))
		lastEvent = ev
	}

	streamErr := result.EventStream.Err()
	t.Logf("events: %v", eventTypes)
	t.Logf("stream err: %v", streamErr)

	// After response.completed was delivered, an upstream disconnect must not
	// degrade the client stream into any second terminal outcome.
	require.Contains(t, eventTypes, string(responsestransformer.StreamEventTypeResponseCompleted))
	completedCount := 0
	for _, et := range eventTypes {
		switch et {
		case string(responsestransformer.StreamEventTypeResponseCompleted):
			completedCount++
		case string(responsestransformer.StreamEventTypeResponseFailed), "error":
			t.Fatalf("unexpected terminal event after response.completed: %s", et)
		}
	}
	require.Equal(t, 1, completedCount)

	if streamErr != nil {
		// A non-nil stream error after a completed terminal event is a bug: the
		// SSE writer turns it into `event: error`, which the openai SDK treats
		// as a fatal error even though the response already completed.
		t.Fatalf("stream error after response.completed must be nil, got: %v", streamErr)
	}

	_ = lastEvent
}
