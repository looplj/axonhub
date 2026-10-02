package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesStream_PreservesTerminal_when_TrailingTransportFails(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, terminal := range []string{"response.completed", "response.failed", "response.incomplete", "error"} {
			t.Run(terminal+fmtResponsesCase(heartbeat, nil), func(t *testing.T) {
				// Given an existing terminal event and a trailing transport error.
				data := []byte(`{"type":"` + terminal + `","sequence_number":7,"response":{"id":"resp_original","status":"failed","error":{"code":"provider_error","message":"original detail"}}}`)
				event := &httpclient.StreamEvent{Type: terminal, Data: data}
				source := &errorAfterStream{items: []*httpclient.StreamEvent{event}, err: io.ErrUnexpectedEOF}
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = httptest.NewRequest(http.MethodPost, "/custom/responses-alias", nil)
				h := &ChatCompletionHandlers{streamAdapterFactory: newResponsesStreamAdapter, sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
				// When the explicitly registered Responses adapter drains the source.
				h.writeSSEStream(c, source)
				// Then it forwards the existing terminal once and preserves the original error.
				require.Equal(t, 1, strings.Count(w.Body.String(), "data:"))
				require.Contains(t, w.Body.String(), string(data))
				require.Equal(t, data, event.Data)
				require.ErrorIs(t, source.Err(), io.ErrUnexpectedEOF)
			})
		}
	}
}

func TestSSEEncoder_DefaultWire_when_ResponsesPathHasNoAdapter(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(fmtResponsesCase(heartbeat, nil), func(t *testing.T) {
			// Given the default protocol on a path ending in responses.
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			h := &ChatCompletionHandlers{sseKeepAlive: SSEKeepAliveConfig{Enabled: heartbeat, Interval: time.Hour}, sseHeartbeatFormat: sseHeartbeatOpenAI}
			// When the stream ends with a transport failure.
			h.writeSSEStream(c, &errorAfterStream{err: io.ErrUnexpectedEOF})
			// Then the existing default wire shape is retained regardless of URL.
			event := parseSSEErrorEvent(t, w.Body.String())
			require.Contains(t, event, "error")
			require.NotContains(t, event, "sequence_number")
			require.NotContains(t, event, "type")
		})
	}
}

func TestResponsesStream_SourceError_when_AdapterDrains(t *testing.T) {
	// Given a stream adapter and an original context error.
	source := &errorAfterStream{err: context.Canceled}
	stream, _ := newResponsesStreamAdapter(context.Background(), source, nil)
	// When the adapter reaches the source error.
	require.False(t, stream.Next())
	// Then the adapter preserves the source cancellation.
	require.ErrorIs(t, stream.Err(), context.Canceled)
}
