package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
	"github.com/stretchr/testify/require"
)

func TestResponsesDisconnectReleasesOwnedLeases(t *testing.T) {
	// Given
	var upgrades atomic.Int32
	closed := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		upgrades.Add(1)
		defer conn.Close()
		defer func() { closed <- struct{}{} }()
		for {
			var frame json.RawMessage
			if conn.ReadJSON(&frame) != nil {
				return
			}
			if conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"provider_id","status":"completed","output":[]}}`)) != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	executor := NewWebSocketExecutor(nil)
	t.Cleanup(func() { require.NoError(t, executor.Close()) })
	budget := shared.NewResponsesBudget(64 << 20)
	a, b := shared.NewResponsesLane("server_lane_a", budget), shared.NewResponsesLane("server_lane_b", budget)
	t.Cleanup(a.Close)
	t.Cleanup(b.Close)
	turn := func(lane *shared.ResponsesLane, body string) {
		ctx := shared.WithResponsesAttempt(shared.WithResponsesLane(shared.WithResponsesWebSocket(webSocketTestContext()), lane), &shared.ResponsesAttempt{})
		stream, err := executor.DoStream(ctx, &httpclient.Request{URL: server.URL, Headers: http.Header{"Session_id": {"same_client_id"}}, Body: []byte(body)})
		require.NoError(t, err)
		for stream.Next() {
		}
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}
	turn(a, `{"input":[]}`)
	turn(b, `{"input":[]}`)
	// When
	a.Close()
	<-closed
	turn(b, `{"previous_response_id":"provider_id","input":[{"role":"user","content":"survivor"}]}`)
	// Then
	require.Equal(t, int32(2), upgrades.Load())
	executor.mu.Lock()
	require.Len(t, executor.pool, 1)
	executor.mu.Unlock()
}
