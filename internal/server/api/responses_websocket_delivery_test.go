package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type responsesWebSocketDeliveryStream struct {
	streams.Stream[*httpclient.StreamEvent]

	mu            sync.Mutex
	acknowledged  bool
	acknowledgedC chan struct{}
	closedC       chan struct{}
	closeOnce     sync.Once
}

func (s *responsesWebSocketDeliveryStream) AcknowledgeDelivery() {
	s.mu.Lock()
	if !s.acknowledged {
		s.acknowledged = true
		close(s.acknowledgedC)
	}
	s.mu.Unlock()
}

func (s *responsesWebSocketDeliveryStream) DeliveryAcknowledged() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.acknowledged
}

func (s *responsesWebSocketDeliveryStream) Close() error {
	err := s.Stream.Close()
	s.closeOnce.Do(func() { close(s.closedC) })
	return err
}

func newResponsesWebSocketDeliveryStream(eventType string) *responsesWebSocketDeliveryStream {
	return &responsesWebSocketDeliveryStream{
		Stream: streams.SliceStream([]*httpclient.StreamEvent{{
			Type: eventType,
			Data: []byte(`{"type":"` + eventType + `","response":{"status":"` + responseWebSocketStatus(eventType) + `"}}`),
		}}),
		acknowledgedC: make(chan struct{}),
		closedC:       make(chan struct{}),
	}
}

func responseWebSocketStatus(eventType string) string {
	switch eventType {
	case "response.failed":
		return "failed"
	case "response.incomplete":
		return "incomplete"
	case "response.cancelled":
		return "cancelled"
	default:
		return "completed"
	}
}

func TestStreamDeliveryWebSocketAcknowledgesSuccessfulTerminalWrite(t *testing.T) {
	// Given: a real loopback WebSocket and a stream with an optional observer.
	stream := newResponsesWebSocketDeliveryStream("response.completed")
	process := func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil
	}
	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)

	// When: the client receives the semantic terminal event, then disconnects.
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
	_, _, err := conn.ReadMessage()
	require.NoError(t, err)
	defer conn.Close()

	// Then: acknowledgement follows the successful local WebSocket write.
	select {
	case <-stream.acknowledgedC:
	case <-time.After(time.Second):
		t.Fatal("successful terminal write was not acknowledged")
	}
	require.NoError(t, conn.Close())
	require.True(t, stream.DeliveryAcknowledged())
}

func TestStreamDeliveryWebSocketDoesNotAcknowledgeUnsuccessfulTerminal(t *testing.T) {
	// Given: a real loopback WebSocket and an unsuccessful terminal event.
	for _, terminal := range []string{"response.failed", "response.incomplete", "response.cancelled", "error"} {
		t.Run(terminal, func(t *testing.T) {
			stream := newResponsesWebSocketDeliveryStream(terminal)
			process := func(context.Context, *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
				return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil
			}
			server := newResponsesWebSocketTestServer(t, process, nil)
			conn := dialResponsesWebSocket(t, server.URL, nil)
			defer conn.Close()

			// When: the failed terminal event is written to the socket.
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
			require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
			_, _, err := conn.ReadMessage()
			require.NoError(t, err)

			// Then: provider failure never becomes successful delivery acknowledgement.
			select {
			case <-stream.closedC:
			case <-time.After(5 * time.Second):
				t.Fatal("unsuccessful turn stream did not close")
			}
			require.False(t, stream.DeliveryAcknowledged())
		})
	}
}

func TestStreamDeliveryWebSocketCancellationBeforeTerminalReleasesTurn(t *testing.T) {
	// Given: a turn that has started but has not produced its terminal event.
	started := make(chan struct{})
	returned := make(chan struct{})
	stream := newResponsesWebSocketDeliveryStream("response.completed")
	process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		close(started)
		<-ctx.Done()
		close(returned)
		return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, nil
	}
	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()

	// When: the WebSocket closes before the semantic terminal can be written.
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"hello"}`)))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("turn did not start")
	}
	conn.Close()

	// Then: the canceled turn is released without acknowledgement or a leak.
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("canceled turn did not return")
	}
	select {
	case <-stream.acknowledgedC:
		t.Fatal("canceled turn was acknowledged")
	default:
	}
	require.False(t, stream.DeliveryAcknowledged())
	select {
	case <-stream.closedC:
	case <-time.After(time.Second):
		t.Fatal("canceled turn stream was not closed")
	}
}

func TestStreamDeliveryWebSocketAcknowledgementIsolatedPerTurn(t *testing.T) {
	// Given: two turns on one lane with separate delivery observers.
	first := newResponsesWebSocketDeliveryStream("response.completed")
	second := newResponsesWebSocketDeliveryStream("response.completed")
	secondStarted := make(chan struct{})
	turn := 0
	var mu sync.Mutex
	process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		mu.Lock()
		turn++
		current := turn
		mu.Unlock()
		if current == 2 {
			close(secondStarted)
			<-ctx.Done()
			return orchestrator.ChatCompletionResult{ChatCompletionStream: second}, nil
		}
		return orchestrator.ChatCompletionResult{ChatCompletionStream: first}, nil
	}
	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))

	// When: the first turn completes, then a second same-lane turn is canceled.
	request := []byte(`{"type":"response.create","stream_id":"lane-a","model":"gpt-test","input":"hello"}`)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, request))
	_, _, err := conn.ReadMessage()
	require.NoError(t, err)
	select {
	case <-first.acknowledgedC:
	case <-time.After(time.Second):
		t.Fatal("first turn was not acknowledged")
	}
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, request))
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second turn did not start")
	}
	conn.Close()
	select {
	case <-second.closedC:
	case <-time.After(5 * time.Second):
		t.Fatal("second canceled turn did not close")
	}

	// Then: the first acknowledgement survives session cancellation and the second does not.
	require.True(t, first.DeliveryAcknowledged())
	select {
	case <-second.acknowledgedC:
		t.Fatal("second canceled turn was acknowledged")
	default:
	}
	require.False(t, second.DeliveryAcknowledged())
}
