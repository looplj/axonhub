package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestStreamDeliveryWebSocketDisconnectReleasesEveryLane(t *testing.T) {
	// Given: real loopback sockets with active, same-lane queued, and slot-waiting turns.
	started := make(chan struct{}, responsesWebSocketMaxActive)
	finished := make(chan *responsesWebSocketDispatcher, 1)
	reserved := make(chan struct{}, responsesWebSocketMaxActive+2)
	process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		started <- struct{}{}
		<-ctx.Done()
		return orchestrator.ChatCompletionResult{}, ctx.Err()
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := new(websocket.Upgrader).Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()
		d := newResponsesWebSocketDispatcher(ctx, cancel, conn, &responsesWebSocketWriter{conn: conn}, r, 0, process, nil)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			streamID, envelopeErr := responsesWebSocketEnvelopeStreamID(message)
			if envelopeErr != nil {
				t.Error(envelopeErr)
				break
			}
			lane, laneErr := d.lane(streamID)
			if laneErr != nil || !d.reserve(len(message)) {
				t.Error("failed to reserve lane")
				break
			}
			d.dispatch(lane, streamID, message)
			reserved <- struct{}{}
		}
		d.wait()
		finished <- d
	}))
	t.Cleanup(server.Close)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()
	for i := range responsesWebSocketMaxActive {
		message := []byte(`{"type":"response.create","stream_id":"lane-` + string(rune('a'+i)) + `","model":"gpt-test"}`)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, message))
	}
	for range responsesWebSocketMaxActive {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("active turn did not start")
		}
	}
	for _, lane := range []string{"lane-a", "waiting-for-slot"} {
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"`+lane+`","model":"gpt-test"}`)))
	}
	for range responsesWebSocketMaxActive + 2 {
		select {
		case <-reserved:
		case <-time.After(5 * time.Second):
			t.Fatal("queued turn was not reserved")
		}
	}
	// When: the client closes while every active slot is occupied.
	require.NoError(t, conn.Close())
	// Then: all goroutines, active slots, pending slots, and lane tails are released.
	select {
	case d := <-finished:
		require.Empty(t, d.active)
		require.Empty(t, d.pending)
		for id, lane := range d.lanes {
			select {
			case <-lane.tail:
			default:
				t.Errorf("lane %s was not released", id)
			}
		}
		t.Logf("released %d lane tails and %d queued/active turns", len(d.lanes), responsesWebSocketMaxActive+2)
	case <-time.After(5 * time.Second):
		t.Fatal("dispatcher did not quiesce")
	}
}

func TestStreamDeliveryWebSocketTurnContextsAreIndependent(t *testing.T) {
	// Given: two same-lane turns without a configured timeout.
	contexts := make(chan context.Context, 2)
	process := func(ctx context.Context, _ *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		contexts <- ctx
		return orchestrator.ChatCompletionResult{ChatCompletionStream: newResponsesWebSocketDeliveryStream("response.completed")}, nil
	}
	server := newResponsesWebSocketTestServer(t, process, nil)
	conn := dialResponsesWebSocket(t, server.URL, nil)
	defer conn.Close()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	// When: consecutive turns complete while the connection remains open.
	for range 2 {
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"lane-a","model":"gpt-test"}`)))
		_, data, err := conn.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, "response.completed", gjson.GetBytes(data, "type").String())
	}
	first, second := <-contexts, <-contexts
	// Then: each turn has its own cancellation lifetime, independent of the open session.
	require.False(t, first == second, "turns reused the same context")
	for _, ctx := range []context.Context{first, second} {
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("completed turn context was not released")
		}
	}
}
