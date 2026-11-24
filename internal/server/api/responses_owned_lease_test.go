package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestResponsesDownstreamDisconnectReleasesOwnedIdleLease(t *testing.T) {
	// Given
	closed := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(-1)
		defer close(closed)
		_, _, err = conn.Read(t.Context())
		if err != nil {
			return
		}
		if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"response.completed","response":{"id":"provider","status":"completed","output":[]}}`)); err != nil {
			return
		}
		_, _, _ = conn.Read(t.Context())
	}))
	t.Cleanup(upstream.Close)
	executor := responses.NewWebSocketExecutor(nil)
	t.Cleanup(func() { require.NoError(t, executor.Close()) })
	downstream := newResponsesWebSocketTestServer(t, func(ctx context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
		ctx = shared.WithResponsesAttempt(shared.WithSessionScope(ctx, "api_key:1:project:1"), &shared.ResponsesAttempt{})
		request.URL = upstream.URL
		request.Auth = &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "synthetic"}
		stream, err := executor.DoStream(ctx, request)
		return orchestrator.ChatCompletionResult{ChatCompletionStream: stream}, err
	}, nil)
	conn := dialResponsesWebSocket(t, downstream.URL, nil)
	t.Cleanup(func() { conn.CloseNow() })
	require.NoError(t, conn.Write(t.Context(), websocket.MessageText, []byte(`{"type":"response.create","model":"gpt-5","input":"root"}`)))
	_, completed, err := conn.Read(t.Context())
	require.NoError(t, err)
	require.Equal(t, "response.completed", gjson.GetBytes(completed, "type").String(), string(completed))
	// When
	require.NoError(t, conn.Close(websocket.StatusNormalClosure, ""))
	// Then
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("owned upstream idle lease remained open")
	}
}
