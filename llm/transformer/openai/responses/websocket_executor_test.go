package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func webSocketTestContext() context.Context {
	return shared.WithSessionScope(context.Background(), "test-scope")
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)

	return parsed
}

func TestOutboundCustomizeExecutorUsesCurrentExecutor(t *testing.T) {
	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.openai.com/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		Transport:      TransportWebSocket,
	})
	require.NoError(t, err)

	firstClient := httpclient.NewHttpClientWithProxy(&httpclient.ProxyConfig{Type: httpclient.ProxyTypeDisabled}, httpclient.WithInsecureSkipVerify(true))
	secondClient := httpclient.NewHttpClientWithProxy(&httpclient.ProxyConfig{Type: httpclient.ProxyTypeURL, URL: "http://127.0.0.1:18080"})

	first, ok := outbound.CustomizeExecutor(firstClient).(*WebSocketExecutor)
	require.True(t, ok)
	second, ok := outbound.CustomizeExecutor(secondClient).(*WebSocketExecutor)
	require.True(t, ok)
	again, ok := outbound.CustomizeExecutor(firstClient).(*WebSocketExecutor)
	require.True(t, ok)

	require.NotSame(t, first, second)
	require.Same(t, first, again)
	require.Same(t, firstClient, first.Inner())
	require.Same(t, secondClient, second.Inner())
	require.NotNil(t, first.dialer.TLSClientConfig)
	require.True(t, first.dialer.TLSClientConfig.InsecureSkipVerify)

	req := &http.Request{URL: mustParseURL(t, "https://example.com/v1/responses")}
	firstProxy, err := first.dialer.Proxy(req)
	require.NoError(t, err)
	require.Nil(t, firstProxy)
	secondProxy, err := second.dialer.Proxy(req)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:18080", secondProxy.String())
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

func TestWebSocketStreamReportsContextCancellationWhileReadBlocks(t *testing.T) {
	upgrader := websocket.Upgrader{}
	payloadRead := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		close(payloadRead)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(webSocketTestContext())
	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(ctx, &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body:   []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	defer stream.Close()

	<-payloadRead
	nextResult := make(chan bool, 1)
	go func() {
		nextResult <- stream.Next()
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case ok := <-nextResult:
		require.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("stream.Next did not unblock after context cancellation")
	}
	require.ErrorIs(t, stream.Err(), context.Canceled)
}

func TestWebSocketStreamReportsContextCancellationWhenAlreadyClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(webSocketTestContext())
	stream := &webSocketStream{ctx: ctx, done: make(chan struct{})}

	stream.finish(true)
	cancel()

	require.False(t, stream.Next())
	require.ErrorIs(t, stream.Err(), context.Canceled)
}

func TestWebSocketStreamDoesNotOverwriteTerminalResponseWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(webSocketTestContext())
	stream := &webSocketStream{ctx: ctx, done: make(chan struct{})}
	stream.markTerminal()
	stream.finish(false)
	cancel()

	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
}

func TestWebSocketExecutorDoReturnsErrorForTopLevelErrorEvent(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type":    "error",
			"code":    "bad_request",
			"message": "invalid websocket request",
		}))
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	resp, err := executor.Do(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body:   []byte(`{"model":"gpt-5"}`),
	})

	require.Nil(t, resp)
	require.ErrorContains(t, err, "bad_request: invalid websocket request")
}

func TestWebSocketExecutorDoAggregatesFailedResponseEvent(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		status := "failed"
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.failed",
			"response": map[string]any{
				"id":         "resp_failed",
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     status,
				"error": map[string]any{
					"code":    "server_error",
					"message": "upstream failed",
				},
			},
		}))
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	resp, err := executor.Do(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body:   []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body Response
	require.NoError(t, json.Unmarshal(resp.Body, &body))
	require.Equal(t, "resp_failed", body.ID)
	require.NotNil(t, body.Status)
	require.Equal(t, "failed", *body.Status)
	require.NotNil(t, body.Error)
	require.Equal(t, "server_error", body.Error.Code)
	require.Equal(t, "upstream failed", body.Error.Message)
}

func TestWebSocketExecutorDoAggregatesCancelledAndIncompleteResponseEvents(t *testing.T) {
	tests := []struct {
		name       string
		eventType  string
		status     string
		responseID string
		response   map[string]any
		assertBody func(*testing.T, Response)
	}{
		{
			name:       "cancelled",
			eventType:  "response.cancelled",
			status:     "canceled",
			responseID: "resp_cancelled",
			response:   map[string]any{},
		},
		{
			name:       "incomplete",
			eventType:  "response.incomplete",
			status:     "incomplete",
			responseID: "resp_incomplete",
			response: map[string]any{
				"incomplete_details": map[string]any{"reason": "max_output_tokens"},
			},
			assertBody: func(t *testing.T, body Response) {
				t.Helper()
				require.NotNil(t, body.IncompleteDetails)
				require.Equal(t, "max_output_tokens", body.IncompleteDetails.Reason)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upgrader := websocket.Upgrader{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := upgrader.Upgrade(w, r, nil)
				require.NoError(t, err)
				defer conn.Close()

				var payload map[string]any
				require.NoError(t, conn.ReadJSON(&payload))
				response := map[string]any{
					"id":         tt.responseID,
					"object":     "response",
					"created_at": 1700000000,
					"model":      "gpt-5",
					"status":     tt.status,
					"output":     []any{},
				}
				for key, value := range tt.response {
					response[key] = value
				}
				require.NoError(t, conn.WriteJSON(map[string]any{
					"type":     tt.eventType,
					"response": response,
				}))
			}))
			defer server.Close()

			executor := NewWebSocketExecutor(nil)
			resp, err := executor.Do(webSocketTestContext(), &httpclient.Request{
				Method: http.MethodPost,
				URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
				Auth:   &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
				Body:   []byte(`{"model":"gpt-5"}`),
			})
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var body Response
			require.NoError(t, json.Unmarshal(resp.Body, &body))
			require.Equal(t, tt.responseID, body.ID)
			require.NotNil(t, body.Status)
			require.Equal(t, tt.status, *body.Status)
			if tt.assertBody != nil {
				tt.assertBody(t, body)
			}
		})
	}
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

func TestWebSocketExecutorReconnectsForDifferentExplicitPreviousResponseID(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))

		if upgrade == 1 {
			require.NotContains(t, payload, "previous_response_id")
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
			return
		}

		require.Equal(t, "client_prev", payload["previous_response_id"])
		input, ok := payload["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 2)
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_2",
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
	ctx := webSocketTestContext()
	for _, body := range [][]byte{
		[]byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user"}]}`),
		[]byte(`{"model":"gpt-5","previous_response_id":"client_prev","input":[{"id":"first","type":"message","role":"user"},{"id":"second","type":"message","role":"user"}]}`),
	} {
		stream, err := executor.DoStream(ctx, &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"explicit-reconnect"},
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

func TestRestorePayloadMapRestoresMutatedPayload(t *testing.T) {
	originalInput := []any{
		map[string]any{"id": "user_1", "type": "message", "role": "user"},
		map[string]any{"id": "user_2", "type": "message", "role": "user"},
	}
	payload := map[string]any{
		"model": "gpt-5",
		"input": originalInput,
	}
	original := clonePayloadMap(payload)

	payload["input"] = []json.RawMessage{json.RawMessage(`{"id":"user_2","type":"message","role":"user"}`)}
	payload["previous_response_id"] = "resp_1"
	restorePayloadMap(payload, original)

	require.Equal(t, "gpt-5", payload["model"])
	require.Equal(t, originalInput, payload["input"])
	require.NotContains(t, payload, "previous_response_id")
}

func TestWebSocketExecutorReconnectsWhenSuffixStartsWithAssistantOutput(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))

		if upgrade == 1 {
			require.NotContains(t, payload, "previous_response_id")
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
			return
		}

		require.NotContains(t, payload, "previous_response_id")
		input, ok := payload["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 3)
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_2",
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
	ctx := webSocketTestContext()
	for _, body := range [][]byte{
		[]byte(`{"model":"gpt-5","input":[{"id":"user_1","type":"message","role":"user"}]}`),
		[]byte(`{"model":"gpt-5","input":[{"id":"user_1","type":"message","role":"user"},{"id":"assistant_1","type":"message","role":"assistant"},{"id":"user_2","type":"message","role":"user"}]}`),
	} {
		stream, err := executor.DoStream(ctx, &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"assistant-reconnect"},
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

func TestWebSocketStreamReturnsErrorWhenReusedConnectionClosesBeforeEvent(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		if upgrade == 1 {
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
			require.NoError(t, conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")))
			return
		}
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	ctx := webSocketTestContext()
	stream, err := executor.DoStream(ctx, &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"stale-close"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user"}]}`),
	})
	require.NoError(t, err)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	require.NoError(t, stream.Close())

	stream, err = executor.DoStream(ctx, &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"stale-close"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user"},{"id":"second","type":"message","role":"user"}]}`),
	})
	if err != nil {
		return
	}
	defer stream.Close()
	require.False(t, stream.Next())
	require.ErrorContains(t, stream.Err(), "websocket closed before response event")
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

func TestWebSocketExecutorKeepsConnectionWhenInputExceedsRetainedLimit(t *testing.T) {
	upgrader := websocket.Upgrader{}
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()

		for i := 0; i < 2; i++ {
			var payload map[string]any
			if err := conn.ReadJSON(&payload); err != nil {
				return
			}
			require.NotContains(t, payload, "previous_response_id")
			input, ok := payload["input"].([]any)
			require.True(t, ok)
			require.Len(t, input, 1)

			require.NoError(t, conn.WriteJSON(map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"id":         fmt.Sprintf("resp_%d", i+1),
					"object":     "response",
					"created_at": 1700000000 + i,
					"model":      "gpt-5",
					"status":     "completed",
					"output":     []any{},
				},
			}))
		}
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	executor.maxRetainedInput = 1
	body := []byte(`{"model":"gpt-5","input":[{"id":"large","type":"message","role":"user","content":[{"type":"input_text","text":"larger than retained limit"}]}]}`)

	for i := 0; i < 2; i++ {
		stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"large-input-session"},
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

func TestHeaderPoolIdentityIgnoresPerRequestTraceHeaders(t *testing.T) {
	base := http.Header{
		"X-Trace-Id":           []string{"trace-1"},
		"X-Request-Id":         []string{"request-1"},
		"Traceparent":          []string{"00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01"},
		"X-Custom-Routing":     []string{"stable"},
		"OpenAI-Beta":          []string{WebSocketBetaHeaderValue},
		webSocketSessionHeader: []string{"session-1"},
	}
	changedTrace := base.Clone()
	changedTrace.Set("X-Trace-Id", "trace-2")
	changedTrace.Set("X-Request-Id", "request-2")
	changedTrace.Set("Traceparent", "00-cccccccccccccccccccccccccccccccc-dddddddddddddddd-01")

	reKeyed := base.Clone()
	reKeyed.Set("X-Custom-Routing", "other")

	require.Equal(t, headerPoolIdentity(base), headerPoolIdentity(changedTrace))
	require.NotEqual(t, headerPoolIdentity(base), headerPoolIdentity(reKeyed))
}

func TestWebSocketExecutorBackgroundCleanupClosesIdleConnections(t *testing.T) {
	upgrader := websocket.Upgrader{}
	connectionClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		defer close(connectionClosed)

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_cleanup",
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	executor.idleTTL = 20 * time.Millisecond
	executor.maxLifetime = time.Hour
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"cleanup-session"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())

	select {
	case <-connectionClosed:
	case <-time.After(time.Second):
		t.Fatal("idle websocket connection was not closed by background cleanup")
	}
}

func TestWebSocketExecutorCloseClosesIdleConnections(t *testing.T) {
	upgrader := websocket.Upgrader{}
	connectionClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)
		defer conn.Close()
		defer close(connectionClosed)

		var payload map[string]any
		require.NoError(t, conn.ReadJSON(&payload))
		require.NoError(t, conn.WriteJSON(map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":         "resp_close",
				"object":     "response",
				"created_at": 1700000000,
				"model":      "gpt-5",
				"status":     "completed",
				"output":     []any{},
			},
		}))
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"close-session"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	require.True(t, stream.Next())
	require.Equal(t, "response.completed", stream.Current().Type)
	require.False(t, stream.Next())
	require.NoError(t, stream.Err())
	require.NoError(t, executor.Close())

	select {
	case <-connectionClosed:
	case <-time.After(time.Second):
		t.Fatal("idle websocket connection was not closed by executor Close")
	}
	require.Empty(t, executor.pool)
	require.False(t, executor.cleanupScheduled)
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
