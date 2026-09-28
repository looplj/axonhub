package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm/httpclient"
)

// TestWriteSSEStream_ResponsesTerminalEventThenError reproduces the production
// incident (pi client "unexpected EOF"): the upstream delivers a full Responses
// event sequence including response.completed, then the upstream connection
// breaks. The inbound transformer enqueues a trailing response.failed event and
// ends the stream with Err() == nil (the error was already converted to a
// terminal event), but if the stream ALSO surfaces a non-nil error the SSE
// writer appends `event: error` after the terminal event — the openai SDK
// treats that error frame as fatal and throws mid-iteration, even though the
// response already reached a terminal state.
func TestWriteSSEStream_ResponsesTerminalEventThenError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	// Upstream: deltas → response.completed → (disconnect). The inbound
	// transformer converts the disconnect into response.failed, then the
	// stream ends. Error is consumed by the terminal event.
	stream := &errorAfterStream{
		items: []*httpclient.StreamEvent{
			{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"r1"}}`)},
			{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"r1","status":"completed"}}`)},
			{Type: "response.failed", Data: []byte(`{"type":"response.failed","response":{"id":"r1","status":"failed","error":{"type":"server_error","code":"stream_error","message":"unexpected EOF"}}}`)},
		},
		err: nil,
	}

	WriteSSEStream(c, stream)

	body := w.Body.String()
	t.Logf("body:\n%s", body)

	// The terminal response.failed must reach the client intact.
	assert.Contains(t, body, "event:response.failed")
	assert.Contains(t, body, `"code":"stream_error"`)

	// A trailing bare `error` event after a terminal event breaks openai SDK
	// clients (the SDK throws APIError mid-iteration on data.error frames).
	// When the stream already delivered a terminal event, the writer must not
	// append another error frame.
	assert.NotContains(t, body, "event:error")
}

// TestWriteSSEStream_ResponsesErrorAfterTerminalWithStreamErr asserts the
// incident shape: response.completed delivered, stream then errors. The writer
// must not append `event: error` after a terminal event was already written.
func TestWriteSSEStream_ResponsesErrorAfterTerminalWithStreamErr(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	stream := &errorAfterStream{
		items: []*httpclient.StreamEvent{
			{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"r1"}}`)},
			{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"r1","status":"completed"}}`)},
		},
		err: errors.New("unexpected EOF"),
	}

	WriteSSEStream(c, stream)

	body := w.Body.String()
	t.Logf("body:\n%s", body)

	assert.Contains(t, body, "event:response.completed")

	// terminalSeen=true here: the writer already wrote response.completed.
	// Appending `event: error` makes openai SDK clients throw even though the
	// response completed — the exact production incident.
	assert.NotContains(t, body, "event:error")
}
