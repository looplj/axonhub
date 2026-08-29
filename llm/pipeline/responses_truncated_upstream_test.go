package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	responsestransformer "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

// TestPipeline_ResponsesRealUpstreamTruncatedChunkedBody reproduces the
// production incident end-to-end with a REAL HTTP upstream: the upstream
// streams a full Responses event sequence (response.created ... response.completed)
// and then the chunked response body is truncated (connection closed mid-chunk,
// no terminating 0\r\n\r\n). This is the exact "unexpected EOF" shape observed
// with pi behind axonhub.
//
// Expected client-visible behavior: once response.completed is delivered, the
// stream ends cleanly without a conflicting response.failed or bare error event.
func TestPipeline_ResponsesRealUpstreamTruncatedChunkedBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher := w.(http.Flusher)
		flusher.Flush()

		events := []string{
			`event: response.created
data: {"type":"response.created","response":{"id":"resp_trunc","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}

`,
			`event: response.output_item.added
data: {"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"enc_1"}}

`,
			`event: response.completed
data: {"type":"response.completed","response":{"id":"resp_trunc","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[],"usage":{"input_tokens":100,"output_tokens":50,"total_tokens":150}}}

`,
		}

		for _, e := range events {
			_, _ = fmt.Fprint(w, e)
			flusher.Flush()
		}

		// Hold the connection open like a real provider would (client is
		// consuming the completed response / executing tool calls), then break
		// the chunked body mid-stream: closing the connection without the
		// terminating chunk produces io.ErrUnexpectedEOF on the reader side.
		time.Sleep(200 * time.Millisecond)

		// Write a partial chunk then hijack-close without terminating the
		// chunked encoding.
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, err := hj.Hijack()
			if err == nil {
				_, _ = conn.Write([]byte("event: response.output_text.de")) // partial frame, no terminator
				_ = conn.Close()
				return
			}
		}

		// Fallback: abrupt close.
		panic("cannot hijack connection")
	}))
	defer upstream.Close()

	// Real HTTP executor: exercises the actual DoStream path with the real
	// SSE decoder, so truncation errors match production shapes.
	httpClient := httpclient.NewHttpClient()
	executor := &mockExecutor{
		doStreamFunc: func(ctx context.Context, request *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			request.URL = upstream.URL + "/v1/responses"
			return httpClient.DoStream(ctx, request)
		},
	}

	inbound := responsestransformer.NewInboundTransformer()
	outbound, err := responsestransformer.NewOutboundTransformer(upstream.URL, "test-api-key")
	require.NoError(t, err)

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
	for result.EventStream.Next() {
		cur := result.EventStream.Current()
		var ev map[string]any
		require.NoError(t, json.Unmarshal(cur.Data, &ev))
		if typ, ok := ev["type"].(string); ok {
			eventTypes = append(eventTypes, typ)
		}
	}

	streamErr := result.EventStream.Err()
	t.Logf("events: %v", eventTypes)
	t.Logf("stream err: %v (nil=%v)", streamErr, streamErr == nil)

	// The completed terminal event was delivered to the client.
	require.Contains(t, eventTypes, string(responsestransformer.StreamEventTypeResponseCompleted))

	// After the completed terminal event, the stream must NOT surface a bare
	// error: the SSE writer would append `event: error`, which openai SDK
	// clients treat as fatal even though the response already completed.
	if streamErr != nil {
		t.Fatalf("bare stream error after response.completed breaks SDK clients: %v", streamErr)
	}

	// No second terminal event may arrive after response.completed.
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
}
