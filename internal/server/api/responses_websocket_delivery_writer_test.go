package api

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type responsesWebSocketDeliveryWriter struct {
	write       func([]byte) error
	deadlineErr error
}

func (w *responsesWebSocketDeliveryWriter) SetWriteDeadline(time.Time) error { return w.deadlineErr }

func (w *responsesWebSocketDeliveryWriter) WriteMessage(_ int, data []byte) error {
	return w.write(data)
}

func TestStreamDeliveryWebSocketWriter(t *testing.T) {
	for _, primed := range []bool{false, true} {
		for _, mode := range []string{"success", "write_error", "deadline_error", "cancel_during_write", "queued_terminal_unwritten", "completed_with_failed_status", "completed_with_incomplete_status", "completed_with_cancelled_status"} {
			name := mode + "/unprimed"
			if primed {
				name = mode + "/primed"
			}
			t.Run(name, func(t *testing.T) {
				// Given: the actual writer loop and a deterministic local I/O fault.
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				source := newResponsesWebSocketDeliveryStream("response.completed")
				terminal := source.Stream.Current()
				events := []*httpclient.StreamEvent{terminal}
				switch mode {
				case "queued_terminal_unwritten":
					events = append([]*httpclient.StreamEvent{{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","delta":"hi"}`)}}, terminal)
				case "completed_with_failed_status":
					terminal.Data = []byte(`{"type":"response.completed","response":{"status":"failed"}}`)
				case "completed_with_incomplete_status":
					terminal.Data = []byte(`{"type":"response.completed","response":{"status":"incomplete"}}`)
				case "completed_with_cancelled_status":
					terminal.Data = []byte(`{"type":"response.completed","response":{"status":"cancelled"}}`)
				}
				source.Stream = streams.SliceStream(events)
				var stream streams.Stream[*httpclient.StreamEvent] = source
				if primed {
					stream = primeCodexTurnStateHeader(http.Header{}, stream)
				}
				writes := 0
				transport := &responsesWebSocketDeliveryWriter{write: func(data []byte) error {
					writes++
					require.False(t, source.DeliveryAcknowledged(), "acknowledgement preceded message write return")
					switch mode {
					case "write_error", "queued_terminal_unwritten":
						return io.ErrClosedPipe
					case "cancel_during_write":
						cancel()
					}
					return nil
				}}
				if mode == "deadline_error" {
					transport.deadlineErr = io.ErrClosedPipe
				}
				// When
				err := writeResponsesWebSocketResult(ctx, &responsesWebSocketWriter{conn: transport}, orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil, "lane-a")
				// Then: a terminal frame that was written successfully is delivered,
				// including when the client cancels during the write. Failed or
				// short writes and unwritten queued terminals stay unacknowledged.
				require.Equal(t, mode == "success" || mode == "cancel_during_write", source.DeliveryAcknowledged())
				switch mode {
				case "write_error", "queued_terminal_unwritten", "deadline_error":
					require.ErrorIs(t, err, io.ErrClosedPipe)
				default:
					require.NoError(t, err)
				}
				if mode == "queued_terminal_unwritten" {
					require.Equal(t, 1, writes)
				}
				t.Logf("primed=%v mode=%s writes=%d acknowledged=%v", primed, mode, writes, source.DeliveryAcknowledged())
			})
		}
	}
}

func TestStreamDeliveryWebSocketHeldWrite(t *testing.T) {
	// Given: a message write held after the terminal is consumed from the stream.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	done := make(chan error, 1)
	source := newResponsesWebSocketDeliveryStream("response.completed")
	transport := &responsesWebSocketDeliveryWriter{write: func([]byte) error {
		close(entered)
		<-release
		return nil
	}}
	// When: downstream cancellation occurs before the held message write returns.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("writer panic: %v", r)
			}
		}()
		done <- writeResponsesWebSocketResult(ctx, &responsesWebSocketWriter{conn: transport}, orchestrator.ChatCompletionResult{ChatCompletionStream: source}, nil, "")
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("write barrier not reached")
	}
	cancel()
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not exit")
	}
	// Then: the released write returned success, so the frame is delivered even
	// though the downstream context was canceled while it was held.
	require.True(t, source.DeliveryAcknowledged())
}
