package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesNativeContinuationIsolatedByStreamID(t *testing.T) {
	// Given
	ids := make(chan string, 2)
	server := newResponsesWebSocketTestServer(t, func(ctx context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		id, _ := shared.GetSessionID(ctx)
		ids <- id
		return orchestrator.ChatCompletionResult{ChatCompletion: &httpclient.Response{Body: []byte(`{"id":"done","status":"completed","output":[]}`)}}, nil
	}, nil)
	conn := dialResponsesWebSocket(t, server.URL, http.Header{"Session-Id": {"client"}})
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	// When
	for _, frame := range []string{`{"type":"response.create","stream_id":"a","model":"gpt-5","input":"a"}`, `{"type":"response.create","stream_id":"b","model":"gpt-5","input":"b"}`} {
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(frame)))
		_, _, err := conn.ReadMessage()
		require.NoError(t, err)
	}
	// Then
	first, second := <-ids, <-ids
	require.NotEqual(t, first, second)
	require.NotEqual(t, "client", first)
}

func TestResponsesPersistedFramePreservesExactRawBytes(t *testing.T) {
	// Given
	frame := []byte("{ \"type\":\"response.create\", \"model\":\"gpt-5\", \"input\":\"raw\", \"stream\":false }")
	session := &responsesWebSocketSession{}
	// When
	request, _, err := session.prepareRequest(httptest.NewRequest(http.MethodGet, "/v1/responses", nil), frame)
	// Then
	require.Nil(t, err)
	require.Equal(t, frame, request.JSONBody)
}
