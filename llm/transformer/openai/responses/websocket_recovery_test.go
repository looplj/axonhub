package responses

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesProviderRecoveryUsesNonRetrySentinel(t *testing.T) {
	// Given
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, _, err = conn.ReadMessage()
		if err != nil {
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","status":400,"error":{"code":"previous_response_not_found","type":"invalid_request_error"}}`)); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	executor := NewWebSocketExecutor(nil)
	t.Cleanup(func() { require.NoError(t, executor.Close()) })
	stream, err := executor.DoStream(t.Context(), &httpclient.Request{URL: server.URL, Body: []byte(`{"input":[]}`)})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stream.Close()) })
	// When
	for stream.Next() {
	}
	// Then
	require.ErrorIs(t, stream.Err(), shared.ErrResponsesContinuationUnavailable)
}
