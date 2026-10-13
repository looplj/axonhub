package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesRecoveryPreservesSurvivorLane(t *testing.T) {
	// Given
	server := newResponsesWebSocketTestServer(t, func(ctx context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		if gjson.GetBytes(request.Body, "previous_response_id").Exists() {
			return orchestrator.ChatCompletionResult{}, shared.ResponsesRecoveryError()
		}
		return orchestrator.ChatCompletionResult{ChatCompletion: &httpclient.Response{Body: []byte(`{"id":"survivor","status":"completed","output":[]}`)}}, nil
	}, nil)
	conn := dialResponsesWebSocket(t, server.URL, http.Header{})
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	// When
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"failed","model":"gpt-5","previous_response_id":"missing","input":"delta"}`)))
	_, recovery, err := conn.ReadMessage()
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","stream_id":"other","model":"gpt-5","input":"root"}`)))
	_, completed, err := conn.ReadMessage()
	// Then
	require.NoError(t, err)
	require.Equal(t, int64(400), gjson.GetBytes(recovery, "status").Int())
	require.Equal(t, "previous_response_not_found", gjson.GetBytes(recovery, "error.code").String())
	require.Equal(t, "failed", gjson.GetBytes(recovery, "stream_id").String())
	require.Equal(t, "other", gjson.GetBytes(completed, "stream_id").String())
	require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String())
}
