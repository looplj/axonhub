package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type deliverySSETrace struct {
	mu    sync.Mutex
	steps []string
}

func (tr *deliverySSETrace) add(step string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.steps = append(tr.steps, step)
}

type deliverySSEStream struct {
	streams.Stream[*httpclient.StreamEvent]

	trace        *deliverySSETrace
	acknowledged bool
	choices      int
	onAck        func()
}

func (s *deliverySSEStream) ExpectedStreamChoices() int { return s.choices }
func (s *deliverySSEStream) AcknowledgeDelivery() {
	s.trace.add("ack")
	s.acknowledged = true
	if s.onAck != nil {
		s.onAck()
	}
}
func (s *deliverySSEStream) DeliveryAcknowledged() bool { return s.acknowledged }

func (s *deliverySSEStream) Current() *httpclient.StreamEvent {
	s.trace.add("read_event")
	return s.Stream.Current()
}

type deliverySSEWriter struct {
	gin.ResponseWriter

	trace        *deliverySSETrace
	body         strings.Builder
	writeErr     error
	short        bool
	flushErr     error
	afterFlush   func()
	flushes      int
	writeBarrier func()
	flushBarrier func()
}

func (w *deliverySSEWriter) Write(p []byte) (int, error) {
	w.trace.add("write")
	if w.writeBarrier != nil {
		w.writeBarrier()
	}
	if w.writeErr != nil {
		return 0, w.writeErr
	}
	if w.short {
		return len(p) - 1, nil
	}
	return w.body.Write(p)
}
func (w *deliverySSEWriter) WriteString(p string) (int, error) { return w.Write([]byte(p)) }
func (w *deliverySSEWriter) FlushError() error {
	w.flushes++
	if w.flushes == 1 {
		w.trace.add("headers")
		return nil
	}
	w.trace.add("flush")
	if w.flushBarrier != nil {
		w.flushBarrier()
	}
	if w.afterFlush != nil {
		w.afterFlush()
	}
	return w.flushErr
}

func runDeliverySSE(c *gin.Context, s streams.Stream[*httpclient.StreamEvent], heartbeat bool) {
	if heartbeat {
		writeSSEStreamWithHeartbeat(c, s, FormatStreamError, time.Hour, sseHeartbeatOpenAI)
	} else {
		WriteSSEStream(c, s)
	}
}

func TestStreamDeliverySSE(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		for _, primed := range []bool{false, true} {
			for _, mode := range []string{"success", "short_write", "write_error", "flush_error", "cancel_during_write", "cancel_during_flush", "queued_terminal_unwritten"} {
				name := mode
				if heartbeat {
					name += "/heartbeat"
				} else {
					name += "/ordinary"
				}
				if primed {
					name += "/primed"
				} else {
					name += "/unprimed"
				}
				t.Run(name, func(t *testing.T) {
					// Given: the actual driver receives a terminal, but transport may reject it.
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					trace := &deliverySSETrace{}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodGet, "/responses", nil).WithContext(ctx)
					writer := &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace}
					c.Writer = writer
					terminal := &httpclient.StreamEvent{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"status":"completed"}}`)}
					events := []*httpclient.StreamEvent{terminal}
					if mode == "queued_terminal_unwritten" {
						events = append([]*httpclient.StreamEvent{{Data: []byte(`{"type":"response.output_text.delta","delta":"hi"}`)}}, terminal)
					}
					source := &deliverySSEStream{Stream: streams.SliceStream(events), trace: trace, choices: 1}
					var stream streams.Stream[*httpclient.StreamEvent] = source
					if primed {
						stream = primeCodexTurnStateHeader(c.Writer.Header(), stream)
					}
					switch mode {
					case "short_write":
						writer.short = true
					case "write_error", "queued_terminal_unwritten":
						writer.writeErr = io.ErrClosedPipe
					case "flush_error":
						writer.flushErr = io.ErrClosedPipe
					case "cancel_during_write":
						writer.writeBarrier = cancel
					case "cancel_during_flush":
						writer.flushBarrier = cancel
					}
					// When
					runDeliverySSE(c, stream, heartbeat)
					// Then: a terminal frame that was written and flushed successfully is
					// delivered even when the client cancels during that write or flush.
					// Only a transport failure or an unwritten queued terminal stays
					// unacknowledged.
					wantAcknowledged := mode == "success" || mode == "cancel_during_write" || mode == "cancel_during_flush"
					require.Equal(t, wantAcknowledged, source.DeliveryAcknowledged())
					if wantAcknowledged {
						require.Contains(t, writer.body.String(), `"type":"response.completed"`)
						require.Equal(t, "flush", trace.steps[len(trace.steps)-2])
						require.Equal(t, "ack", trace.steps[len(trace.steps)-1])
					}
					t.Logf("driver order: %v", trace.steps)
				})
			}
		}
	}
}

func TestStreamDeliverySSEShortWrites(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(map[bool]string{false: "event", true: "heartbeat"}[heartbeat], func(t *testing.T) {
			// Given
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			writer := &deliverySSEWriter{ResponseWriter: c.Writer, trace: &deliverySSETrace{}, short: true}
			// When
			var err error
			if heartbeat {
				err = writeSSEHeartbeatEvent(t.Context(), writer, sseHeartbeatOpenAI)
			} else {
				err = writeSSEEvent(t.Context(), writer, "message_stop", []byte(`{"type":"message_stop"}`))
			}
			// Then
			require.ErrorIs(t, err, io.ErrShortWrite)
			require.Zero(t, writer.flushes)
		})
	}
}

func TestStreamDeliverySSEHeldWrite(t *testing.T) {
	for _, heartbeat := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "heartbeat"}[heartbeat], func(t *testing.T) {
			// Given: hold the write after the terminal is read, then cancel before return.
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			trace := &deliverySSETrace{}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			var once sync.Once
			c.Writer = &deliverySSEWriter{ResponseWriter: c.Writer, trace: trace, writeBarrier: func() {
				once.Do(func() { close(entered); <-release })
			}}
			source := &deliverySSEStream{Stream: streams.SliceStream([]*httpclient.StreamEvent{{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)}}), trace: trace}
			// When
			go func() {
				defer close(done)
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("writer panic: %v", r)
					}
				}()
				runDeliverySSE(c, source, heartbeat)
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("write barrier not reached")
			}
			cancel()
			close(release)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("writer did not exit")
			}
			// Then: the held terminal frame still reached the transport, so a
			// cancel that lands before the writer returns cannot erase delivery.
			require.True(t, source.DeliveryAcknowledged())
			require.Contains(t, trace.steps, "ack")
			t.Logf("cancel-before-return driver order: %v", trace.steps)
		})
	}
}
