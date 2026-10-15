package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestStreamDeliverySSEWrappers(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "heartbeat"}[heartbeat], func(t *testing.T) {
			// Given: use the production error-policy, Responses adapter and priming chain.
			trace := &deliverySSETrace{}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/responses", nil)
			c.Writer = &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace}
			source := &deliverySSEStream{trace: trace, choices: 2, Stream: streams.SliceStream([]*httpclient.StreamEvent{
				{Type: "response.completed", Data: []byte(`{"type":"response.completed","sequence_number":0,"response":{"status":"completed"}}`)},
			})}
			policy := newUpstreamErrorStream(t.Context(), source, &biz.SystemService{})
			adapted, _ := newResponsesStreamAdapter(t.Context(), policy, nil)
			primed := primeCodexTurnStateHeader(c.Writer.Header(), adapted)
			// When
			runDeliverySSE(c, primed, heartbeat)
			// Then: every wrapper reports the same owner and preserves expected choices.
			require.True(t, source.DeliveryAcknowledged())
			for _, stream := range []streams.Stream[*httpclient.StreamEvent]{policy, adapted, primed} {
				observer, ok := stream.(streams.DeliveryAcknowledger)
				require.True(t, ok)
				require.True(t, observer.DeliveryAcknowledged())
				require.Equal(t, 2, streamExpectedChoices(stream))
			}
			require.Equal(t, 1, countDeliveryAcks(trace.steps))
			t.Logf("policy -> adapter -> prime -> driver order: %v", trace.steps)
		})
	}
}

func countDeliveryAcks(steps []string) int {
	n := 0
	for _, step := range steps {
		if step == "ack" {
			n++
		}
	}
	return n
}

type deliveryPlainFlusher struct {
	*httptest.ResponseRecorder

	trace *deliverySSETrace
}

func (w *deliveryPlainFlusher) Flush() {
	w.ResponseRecorder.Flush()
	w.trace.add("plain_flush_return")
}

func TestStreamDeliverySSEPlainFlusher(t *testing.T) {
	// Given: a Flusher with no error-reporting capability.
	trace := &deliverySSETrace{}
	w := &deliveryPlainFlusher{ResponseRecorder: httptest.NewRecorder(), trace: trace}
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s := &deliverySSEStream{trace: trace, Stream: streams.SliceStream([]*httpclient.StreamEvent{
		{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
	})}
	// When
	WriteSSEStream(c, s)
	// Then: this proves observable local Flush return only, not network receipt.
	require.True(t, s.DeliveryAcknowledged())
	require.Equal(t, []string{"plain_flush_return", "read_event", "plain_flush_return", "ack"}, trace.steps)
	require.Contains(t, w.Body.String(), `"type":"message_stop"`)
	t.Logf("plain Flusher local order: %v", trace.steps)
}

type deliveryHeartbeatStream struct {
	*deliverySSEStream

	release <-chan struct{}
}

func (s *deliveryHeartbeatStream) Next() bool { <-s.release; return false }

type deliveryHeartbeatWriter struct {
	*deliverySSEWriter

	release  chan struct{}
	released bool
}

func (w *deliveryHeartbeatWriter) FlushError() error {
	err := w.deliverySSEWriter.FlushError()
	if !w.released && w.flushes > 1 {
		w.released = true
		close(w.release)
	}
	return err
}

func TestStreamDeliverySSEHeartbeatNeverAcknowledges(t *testing.T) {
	// Given: Next is blocked until a real heartbeat frame has flushed.
	trace := &deliverySSETrace{}
	release := make(chan struct{})
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	w := &deliveryHeartbeatWriter{deliverySSEWriter: &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace}, release: release}
	c.Writer = w
	s := &deliveryHeartbeatStream{deliverySSEStream: &deliverySSEStream{trace: trace, Stream: streams.SliceStream([]*httpclient.StreamEvent{})}, release: release}
	// When
	writeSSEStreamWithHeartbeat(c, s, FormatStreamError, time.Nanosecond, sseHeartbeatOpenAI)
	// Then
	require.Contains(t, w.body.String(), ": keep-alive\n\n")
	require.False(t, s.DeliveryAcknowledged())
	require.NotContains(t, trace.steps, "ack")
	t.Logf("heartbeat driver order: %v", trace.steps)
}
