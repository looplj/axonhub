package responses

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestWebSocketRecoveredReadFailureMarkedAfterWriteRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(webSocketTestContext(), 5*time.Second)
	defer cancel()
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()
		if !assert.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second))) {
			return
		}
		var payload map[string]any
		if !assert.NoError(t, conn.ReadJSON(&payload)) {
			return
		}
		if upgrade == 1 {
			if !assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[]}}`))) {
				return
			}
			_, _, err = conn.ReadMessage()
			assert.Error(t, err)
			return
		}
		if !assert.NotContains(t, payload, "previous_response_id") {
			return
		}
		input, ok := payload["input"].([]any)
		if !assert.True(t, ok) || !assert.Len(t, input, 2) {
			return
		}
	}))
	defer server.Close()
	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	request := &httpclient.Request{URL: server.URL, Headers: http.Header{webSocketSessionHeader: []string{"write-marker"}}, Body: []byte(`{"input":[{"role":"user","content":"one"}]}`)}
	first, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, first.Next())
	require.NoError(t, first.(*webSocketStream).lease.conn.Close())
	request.Body = []byte(`{"input":[{"role":"user","content":"one"},{"role":"user","content":"two"}]}`)
	second, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, second.(*webSocketStream).recoveryUsed)
	require.False(t, second.Next())
	require.ErrorIs(t, second.Err(), shared.ErrResponsesWebSocketReconnectFailed)
	require.Equal(t, int32(2), upgrades.Load())
}

func TestWebSocketCloseCancelsProductionReconnectHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(webSocketTestContext(), 5*time.Second)
	defer cancel()
	dialStarted := make(chan struct{})
	allowHandshake := make(chan struct{})
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if upgrades.Add(1) == 2 {
			close(dialStarted)
			select {
			case <-allowHandshake:
			case <-r.Context().Done():
			}
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()
		if !assert.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second))) {
			return
		}
		if _, _, err := conn.ReadMessage(); !assert.NoError(t, err) {
			return
		}
		if !assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[]}}`))) {
			return
		}
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	request := &httpclient.Request{URL: server.URL, Headers: http.Header{webSocketSessionHeader: []string{"close-dial"}}, Body: []byte(`{"input":[{"role":"user","content":"one"}]}`)}
	first, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, first.Next())
	request.Body = []byte(`{"input":[{"role":"user","content":"one"},{"role":"user","content":"two"}]}`)
	second, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.NoError(t, second.(*webSocketStream).lease.conn.Close())
	nextDone := make(chan bool, 1)
	go func() { nextDone <- second.Next() }()
	select {
	case <-dialStarted:
	case <-ctx.Done():
		t.Fatal("replacement handshake did not start")
	}
	require.NoError(t, second.Close())
	close(allowHandshake)
	select {
	case next := <-nextDone:
		require.False(t, next)
	case <-time.After(time.Second):
		t.Fatal("Close did not cancel reconnect dial")
	}
	require.NoError(t, ctx.Err())
	require.NoError(t, second.Err())
	require.Equal(t, int32(2), upgrades.Load())
}

func TestWebSocketInternalReconnectStopsAfterAnyEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(webSocketTestContext(), 5*time.Second)
	defer cancel()
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close()
		if !assert.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second))) {
			return
		}
		if _, _, err := conn.ReadMessage(); !assert.NoError(t, err) {
			return
		}
		if !assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[]}}`))) {
			return
		}
		if _, _, err := conn.ReadMessage(); !assert.NoError(t, err) {
			return
		}
		assert.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created","response":{"id":"resp_2","status":"in_progress"}}`)))
	}))
	defer server.Close()
	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	request := &httpclient.Request{URL: server.URL, Headers: http.Header{webSocketSessionHeader: []string{"metadata"}}, Body: []byte(`{"input":[{"role":"user","content":"one"}]}`)}
	first, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, first.Next())
	request.Body = []byte(`{"input":[{"role":"user","content":"one"},{"role":"user","content":"two"}]}`)
	second, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, second.Next())
	require.Equal(t, "response.created", second.Current().Type)
	require.False(t, second.Next())
	require.Error(t, second.Err())
	require.False(t, shared.IsResponsesWebSocketReconnectError(second.Err()))
	require.Equal(t, int32(1), upgrades.Load())
}
