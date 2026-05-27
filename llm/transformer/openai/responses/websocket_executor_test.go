package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestWebSocketExecutorDoStreamSendsResponseCreate(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, WebSocketBetaHeaderValue, r.Header.Get("OpenAI-Beta"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		require.Equal(t, "response.create", payload["type"])
		require.Equal(t, "gpt-5", payload["model"])
		require.Equal(t, "Be concise", payload["instructions"])
		require.NotContains(t, payload, "stream")
		require.NotContains(t, payload, "background")

		require.NoError(t, conn.WriteJSON(map[string]any{
			"type":            "response.completed",
			"sequence_number": 1,
			"response": map[string]any{
				"id":         "resp_test",
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))
		require.NoError(t, conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(context.Background(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body:   []byte(`{"model":"gpt-5","instructions":"Be concise","stream":true,"background":false}`),
	})
	require.NoError(t, err)
	defer stream.Close()

	require.True(t, stream.Next())
	event := stream.Current()
	require.Equal(t, "response.completed", event.Type)
	require.JSONEq(t, `{"type":"response.completed","sequence_number":1,"response":{"id":"resp_test","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`, string(event.Data))
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
}

func TestWebSocketStreamStopsAfterTerminalEventWithoutCloseFrame(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_test",
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))
		<-r.Context().Done()
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(context.Background(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body:   []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	defer stream.Close()

	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
}

func TestNormalizeWebSocketEventFlattensNestedError(t *testing.T) {
	raw := []byte(`{"type":"error","status":400,"error":{"type":"invalid_request_error","message":"bad request","param":"model","code":"bad_model"}}`)

	normalized := normalizeWebSocketEvent(raw)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(normalized, &payload))
	require.Equal(t, "error", payload["type"])
	require.Equal(t, "bad_model", payload["code"])
	require.Equal(t, "bad request", payload["message"])
	require.Equal(t, "model", payload["param"])
}

func TestToWebSocketURL(t *testing.T) {
	got, err := toWebSocketURL("https://api.openai.com/v1/responses")
	require.NoError(t, err)
	require.Equal(t, "wss://api.openai.com/v1/responses", got)

	got, err = toWebSocketURL("http://localhost:8080/v1/responses")
	require.NoError(t, err)
	require.Equal(t, "ws://localhost:8080/v1/responses", got)
}
