package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func webSocketTestContext() context.Context {
	return shared.WithSessionScope(context.Background(), "test-scope")
}

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
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
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
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
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

func TestWebSocketExecutorReusesConnectionForSameSession(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "session-1", r.Header.Get(webSocketSessionHeader))
		upgrades.Add(1)

		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for i := 0; i < 2; i++ {
			var payload map[string]any
			require.NoError(t, conn.ReadJSON(&payload))
			require.Equal(t, "response.create", payload["type"])
			require.Equal(t, fmt.Sprintf("turn-%d", i+1), payload["instructions"])

			require.NoError(t, conn.WriteJSON(map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id":         fmt.Sprintf("resp_%d", i+1),
					"object":     "response",
					"created_at": 1700000000,
					"model":      "gpt-5",
					"status":     "completed",
					"output":     []any{},
				},
			}))
		}
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	for i := 0; i < 2; i++ {
		stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"session-1"},
			},
			Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
			Body: []byte(fmt.Sprintf(`{"model":"gpt-5","instructions":"turn-%d"}`, i+1)),
		})
		require.NoError(t, err)
		require.True(t, stream.Next())
		require.Equal(t, "response.completed", stream.Current().Type)
		require.False(t, stream.Next())
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}

	require.Equal(t, int32(1), upgrades.Load())
}

func TestWebSocketExecutorDoesNotPoolWithoutSession(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

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
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	for i := 0; i < 2; i++ {
		stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
			Body:   []byte(`{"model":"gpt-5"}`),
		})
		require.NoError(t, err)
		require.True(t, stream.Next())
		require.False(t, stream.Next())
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}

	require.Equal(t, int32(2), upgrades.Load())
}

func TestWebSocketExecutorKeepsExplicitPreviousResponseIDOnFreshConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		require.Equal(t, "client_prev", payload["previous_response_id"])
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
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"explicit-previous"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5","previous_response_id":"client_prev","input":[{"id":"first","type":"message"}]}`),
	})
	require.NoError(t, err)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	require.NoError(t, stream.Close())
}

func TestWebSocketExecutorSeparatesPoolByOrganizationHeaders(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

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
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	for _, org := range []string{"org-a", "org-b"} {
		stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"org-session"},
				webSocketOrgHeader:     []string{org},
			},
			Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
			Body: []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message"}]}`),
		})
		require.NoError(t, err)
		require.True(t, stream.Next())
		require.Equal(t, "response.completed", stream.Current().Type)
		require.False(t, stream.Next())
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}

	require.Equal(t, int32(2), upgrades.Load())
}

func TestWebSocketExecutorEvictsPooledConnectionOnFailedOrCancelled(t *testing.T) {
	for _, terminalType := range []string{"response.failed", "response.cancelled", "response.incomplete"} {
		t.Run(terminalType, func(t *testing.T) {
			upgrader := websocket.Upgrader{}
			var upgrades atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrade := upgrades.Add(1)

				conn, err := upgrader.Upgrade(w, r, nil)
				require.NoError(t, err)
				defer conn.Close()

				var payload map[string]any
				require.NoError(t, conn.ReadJSON(&payload))
				require.NotContains(t, payload, "previous_response_id")

				if upgrade == 1 {
					require.NoError(t, conn.WriteJSON(map[string]any{
						"type": terminalType,
						"response": map[string]any{
							"id":     "resp_bad",
							"status": strings.TrimPrefix(terminalType, "response."),
						},
					}))
					return
				}

				require.NoError(t, conn.WriteJSON(map[string]any{
					"type": "response.completed",
					"response": map[string]any{
						"id":         "resp_ok",
						"object":     "response",
						"created_at": 1700000000,
						"model":      "gpt-5",
						"status":     "completed",
						"output":     []any{},
					},
				}))
			}))
			defer server.Close()

			executor := NewWebSocketExecutor(nil)
			body := []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message"}]}`)
			for i := 0; i < 2; i++ {
				stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
					Method: http.MethodPost,
					URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
					Headers: http.Header{
						webSocketSessionHeader: []string{"terminal-evict-" + terminalType},
					},
					Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
					Body: body,
				})
				require.NoError(t, err)
				require.True(t, stream.Next())
				if i == 0 {
					require.Equal(t, terminalType, stream.Current().Type)
				} else {
					require.Equal(t, "response.completed", stream.Current().Type)
				}
				require.False(t, stream.Next())
				require.NoError(t, stream.Err())
				require.NoError(t, stream.Close())
			}

			require.Equal(t, int32(2), upgrades.Load())
		})
	}
}

func TestWebSocketExecutorSendsOnlyNewInputOnReusedSession(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var first map[string]any
		require.NoError(t, conn.ReadJSON(&first))
		firstInput, ok := first["input"].([]any)
		require.True(t, ok)
		require.Len(t, firstInput, 1)
		require.NotContains(t, first, "previous_response_id")
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_1",
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))

		var second map[string]any
		require.NoError(t, conn.ReadJSON(&second))
		require.Equal(t, "resp_1", second["previous_response_id"])
		secondInput, ok := second["input"].([]any)
		require.True(t, ok)
		require.Len(t, secondInput, 1)
		message, ok := secondInput[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "second", message["id"])
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_2",
				"object":     "response",
				"created_at": 1700000001,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	firstBody := []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user","content":[{"type":"input_text","text":"first"}]}]}`)
	secondBody := []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user","content":[{"type":"input_text","text":"first"}]},{"id":"second","type":"message","role":"user","content":[{"type":"input_text","text":"second"}]}]}`)

	for _, body := range [][]byte{firstBody, secondBody} {
		stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"diff-session"},
			},
			Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
			Body: body,
		})
		require.NoError(t, err)
		require.True(t, stream.Next())
		require.Equal(t, "response.completed", stream.Current().Type)
		require.False(t, stream.Next())
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}

	require.Equal(t, int32(1), upgrades.Load())
}

func TestWebSocketExecutorReconnectsWhenInputIsNotAppendOnly(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)

		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		input, ok := payload["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		require.NotContains(t, payload, "previous_response_id")

		responseID := "resp_1"
		if upgrade == 2 {
			message, ok := input[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "rewritten", message["id"])
			responseID = "resp_rewritten"
		}
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         responseID,
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))
		if upgrade == 1 {
			_, _, err = conn.ReadMessage()
			require.Error(t, err)
		}
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	firstBody := []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user","content":[{"type":"input_text","text":"first"}]}]}`)
	rewrittenBody := []byte(`{"model":"gpt-5","input":[{"id":"rewritten","type":"message","role":"user","content":[{"type":"input_text","text":"rewritten"}]}]}`)

	for _, body := range [][]byte{firstBody, rewrittenBody} {
		stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"rewrite-session"},
			},
			Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
			Body: body,
		})
		require.NoError(t, err)
		require.True(t, stream.Next())
		require.Equal(t, "response.completed", stream.Current().Type)
		require.False(t, stream.Next())
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}

	require.Equal(t, int32(2), upgrades.Load())
}

func TestWebSocketExecutorEvictsPooledConnectionOnEarlyClose(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	firstPayloadRead := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		upgrade := upgrades.Add(1)
		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))

		if upgrade == 1 {
			close(firstPayloadRead)
			_, _, err = conn.ReadMessage()
			require.Error(t, err)
			return
		}

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
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"session-early-close"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	<-firstPayloadRead
	require.NoError(t, stream.Close())

	stream, err = executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"session-early-close"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	require.NoError(t, stream.Close())
	require.Equal(t, int32(2), upgrades.Load())
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
