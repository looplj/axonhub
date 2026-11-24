package responses

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
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
	require.NotNil(t, first.transport.TLSClientConfig)
	require.True(t, first.transport.TLSClientConfig.InsecureSkipVerify)

	req := &http.Request{URL: mustParseURL(t, "https://example.com/v1/responses")}
	firstProxy, err := first.transport.Proxy(req)
	require.NoError(t, err)
	require.Nil(t, firstProxy)
	secondProxy, err := second.transport.Proxy(req)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:18080", secondProxy.String())
}

func TestNewWebSocketExecutorOffersHTTP11Only(t *testing.T) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{NextProtos: []string{"h2", "http/1.1"}},
	}
	client := httpclient.NewHttpClientWithClient(&http.Client{Transport: transport})

	executor := NewWebSocketExecutor(client)

	require.Equal(t, []string{"http/1.1"}, executor.transport.TLSClientConfig.NextProtos)
}

func TestWebSocketExecutorDoStreamSendsResponseCreate(t *testing.T) {
	type connectionKey struct{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, WebSocketBetaHeaderValue, r.Header.Get("OpenAI-Beta"))
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.Equal(t, "response.create", payload["type"])
		require.Equal(t, "gpt-5", payload["model"])
		require.Equal(t, "Be concise", payload["instructions"])
		require.NotContains(t, payload, "stream")
		require.NotContains(t, payload, "background")

		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
		// Write an unmasked normal-close frame without waiting for the close handshake.
		rawConn := r.Context().Value(connectionKey{}).(net.Conn)
		require.NoError(t, rawConn.SetWriteDeadline(time.Now().Add(time.Second)))
		_, err = rawConn.Write([]byte{0x88, 0x02, 0x03, 0xe8})
		require.NoError(t, err)
	}))
	server.Config.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, connectionKey{}, conn)
	}
	server.Start()
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	payloadRead := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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

	var httpErr *httpclient.Error
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusBadGateway, httpErr.StatusCode)
	require.True(t, httpclient.IsHTTPStatusCodeRetryable(httpErr.StatusCode))
	require.JSONEq(t, `{
		"error":{
			"type":"bad_request",
			"code":"bad_request",
			"message":"invalid websocket request"
		}
	}`, string(httpErr.Body))
}

func TestTopLevelWebSocketErrorPreservesOfficialErrorStatusAndBody(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "official nested error",
			data: `{
				"type":"error",
				"status":400,
				"error":{
					"type":"invalid_request_error",
					"code":"invalid_value",
					"message":"invalid websocket request",
					"param":"input"
				}
			}`,
			want: `{
				"error":{
					"type":"invalid_request_error",
					"code":"invalid_value",
					"message":"invalid websocket request",
					"param":"input"
				}
			}`,
		},
		{
			name: "legacy flattened error",
			data: `{
				"type":"error",
				"status":400,
				"code":"invalid_value",
				"message":"invalid websocket request",
				"param":"input"
			}`,
			want: `{
				"error":{
					"type":"invalid_value",
					"code":"invalid_value",
					"message":"invalid websocket request",
					"param":"input"
				}
			}`,
		},
		{
			name: "partial nested error retains flattened fields",
			data: `{
				"type":"error",
				"status":400,
				"code":"invalid_value",
				"message":"invalid websocket request",
				"param":"input",
				"error":{"type":"invalid_request_error","request_id":"req_123"}
			}`,
			want: `{
				"error":{
					"type":"invalid_request_error",
					"code":"invalid_value",
					"message":"invalid websocket request",
					"param":"input",
					"request_id":"req_123"
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TopLevelWebSocketError([]*httpclient.StreamEvent{{
				Type: "error",
				Data: []byte(tt.data),
			}})

			var httpErr *httpclient.Error
			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, http.StatusBadRequest, httpErr.StatusCode)
			require.False(t, httpclient.IsHTTPStatusCodeRetryable(httpErr.StatusCode))
			require.JSONEq(t, tt.want, string(httpErr.Body))
		})
	}
}

func TestTopLevelWebSocketErrorReportsProviderFailure(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		wantStatus  int
		wantBody    string
		wantContain []string
	}{
		{
			name:       "string status and numeric code decode leniently",
			data:       `{"type":"error","status":"503","code":503,"message":"upstream overloaded"}`,
			wantStatus: http.StatusServiceUnavailable,
			wantBody: `{
				"error":{
					"type":"upstream_error",
					"code":"503",
					"message":"upstream overloaded"
				}
			}`,
		},
		{
			name:       "fractional status is not a usable status",
			data:       `{"type":"error","status":400.9,"message":"fractional status"}`,
			wantStatus: http.StatusBadGateway,
			wantBody: `{
				"error":{
					"type":"upstream_error",
					"message":"fractional status"
				}
			}`,
		},
		{
			name:       "status-less frame keeps code and message",
			data:       `{"type":"error","code":"server_error","message":"upstream failed"}`,
			wantStatus: http.StatusBadGateway,
			wantBody: `{
				"error":{
					"type":"server_error",
					"code":"server_error",
					"message":"upstream failed"
				}
			}`,
		},
		{
			name:       "nested-only frame keeps nested detail",
			data:       `{"type":"error","error":{"type":"server_error","code":"overloaded","message":"try later"}}`,
			wantStatus: http.StatusBadGateway,
			wantBody: `{
				"error":{
					"type":"server_error",
					"code":"overloaded",
					"message":"try later"
				}
			}`,
		},
		{
			name:       "nested symbolic code supplies the type",
			data:       `{"type":"error","error":{"code":"server_error","message":"try later"}}`,
			wantStatus: http.StatusBadGateway,
			wantBody: `{
				"error":{
					"type":"server_error",
					"code":"server_error",
					"message":"try later"
				}
			}`,
		},
		{
			name:        "empty nested detail falls back to the raw frame",
			data:        `{"type":"error","error":{"type":"server_error","code":"","message":""}}`,
			wantStatus:  http.StatusBadGateway,
			wantContain: []string{`"type":"server_error"`, `"message":"websocket error event: {\"type\":\"error\"`},
		},
		{
			name:        "frame without detail falls back to the raw frame",
			data:        `{"type":"error"}`,
			wantStatus:  http.StatusBadGateway,
			wantContain: []string{`"type":"upstream_error"`, `websocket error event: {\"type\":\"error\"}`},
		},
		{
			name:        "malformed frame keeps the decode error and raw frame",
			data:        `{"type":"error",`,
			wantStatus:  http.StatusBadGateway,
			wantContain: []string{`"type":"upstream_error"`, `failed to decode error frame`, `{\"type\":\"error\",`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TopLevelWebSocketError([]*httpclient.StreamEvent{{
				Type: "error",
				Data: []byte(tt.data),
			}})

			var httpErr *httpclient.Error
			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, tt.wantStatus, httpErr.StatusCode)
			require.True(t, httpclient.IsHTTPStatusCodeRetryable(httpErr.StatusCode),
				"an unclassified upstream error frame must be retryable")

			if tt.wantBody != "" {
				require.JSONEq(t, tt.wantBody, string(httpErr.Body))
			}

			for _, want := range tt.wantContain {
				require.Contains(t, string(httpErr.Body), want)
			}
		})
	}
}

func TestWebSocketExecutorDoAggregatesFailedResponseEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		status := "failed"
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				require.NoError(t, err)
				defer conn.CloseNow()

				var payload map[string]any
				require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
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
				require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "session-1", r.Header.Get(webSocketSessionHeader))
		upgrades.Add(1)

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		for i := 0; i < 2; i++ {
			var payload map[string]any
			require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
			require.Equal(t, "response.create", payload["type"])
			require.Equal(t, fmt.Sprintf("turn-%d", i+1), payload["instructions"])

			require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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

func TestWebSocketExecutorRejectsExplicitPreviousResponseIDOnFreshConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		if err := wsjson.Read(r.Context(), conn, &payload); err != nil {
			return
		}
		t.Error("fresh lease received an unresolved delta")
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
	require.Nil(t, stream)
}

func TestWebSocketExecutorRejectsDifferentExplicitPreviousResponseID(t *testing.T) {
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))

		if upgrade == 1 {
			require.NotContains(t, payload, "previous_response_id")
			require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	for index, body := range [][]byte{
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
		if index == 1 {
			require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
			require.Nil(t, stream)
			continue
		}
		require.NoError(t, err)
		require.True(t, stream.Next())
		require.Equal(t, "response.completed", stream.Current().Type)
		require.False(t, stream.Next())
		require.NoError(t, stream.Err())
		require.NoError(t, stream.Close())
	}

	require.Equal(t, int32(1), upgrades.Load())
}

func TestWebSocketExecutorKeepsConnectionForMatchingExplicitPreviousResponseID(t *testing.T) {
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var first map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &first))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_1",
				"object": "response",
				"status": "completed",
				"output": []any{},
			},
		}))

		var second map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &second))
		require.Equal(t, "resp_1", second["previous_response_id"])
		input, ok := second["input"].([]any)
		require.True(t, ok)
		require.Len(t, input, 1)
		require.Equal(t, "tool_1", input[0].(map[string]any)["id"])
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp_2",
				"object": "response",
				"status": "completed",
				"output": []any{},
			},
		}))
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	ctx := webSocketTestContext()
	for _, body := range [][]byte{
		[]byte(`{"model":"gpt-5","input":[{"id":"user_1","type":"message","role":"user"}]}`),
		[]byte(`{"model":"gpt-5","previous_response_id":"resp_1","input":[{"id":"tool_1","type":"function_call_output","call_id":"call_1","output":"ok"}]}`),
	} {
		stream, err := executor.DoStream(ctx, &httpclient.Request{
			Method: http.MethodPost,
			URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
			Headers: http.Header{
				webSocketSessionHeader: []string{"matching-explicit-previous"},
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

func TestPrepareWebSocketPayloadChecksExplicitPreviousIDWithEmptyCachedInput(t *testing.T) {
	pooled := &pooledWebSocketConn{lastResponseID: "resp_cached"}
	lease := &webSocketLease{pooled: pooled, reused: true}
	payload := map[string]any{
		"model":                "gpt-5",
		"previous_response_id": "resp_other",
		"input":                []any{map[string]any{"type": "message", "role": "user", "content": "next"}},
	}

	err := prepareWebSocketPayloadForLease(payload, lease)
	require.ErrorIs(t, err, shared.ErrResponsesContinuationUnavailable)
}

// TestPrepareWebSocketPayloadKeepsJSONObjectReminderStable pins the review
// finding on #3004: the json_object reminder must keep the same position on
// every turn, or the input prefix check rejects reuse and the next turn opens
// a new WebSocket with the full history.
func TestPrepareWebSocketPayloadKeepsJSONObjectReminderStable(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	systemMessage := llm.Message{
		Role:    "system",
		Content: llm.MessageContent{Content: lo.ToPtr("Return a json object.")},
	}
	userMessage := func(text string) llm.Message {
		return llm.Message{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr(text)},
		}
	}
	assistantMessage := llm.Message{
		Role:    "assistant",
		Content: llm.MessageContent{Content: lo.ToPtr(`{"name":"Ada"}`)},
	}

	payloadFor := func(msgs []llm.Message) map[string]any {
		hreq, err := transformer.TransformRequest(context.Background(), &llm.Request{
			Model:          "gpt-5",
			ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
			Messages:       msgs,
		})
		require.NoError(t, err)

		payload, err := buildWebSocketCreatePayload(hreq.Body)
		require.NoError(t, err)

		return payload
	}

	firstPayload := payloadFor([]llm.Message{systemMessage, userMessage("Ada Lovelace, 36")})

	pooled := &pooledWebSocketConn{}
	firstLease := &webSocketLease{pooled: pooled}
	require.NoError(t, prepareWebSocketPayloadForLease(firstPayload, firstLease))

	output := make([]json.RawMessage, 0)
	for _, item := range convertAssistantMessage(assistantMessage) {
		raw, err := json.Marshal(item)
		require.NoError(t, err)

		output = append(output, raw)
	}

	require.True(t, firstLease.rememberTurn("resp_1", output))

	secondPayload := payloadFor([]llm.Message{
		systemMessage, userMessage("Ada Lovelace, 36"), assistantMessage, userMessage("Lovelace, 36"),
	})
	secondLease := &webSocketLease{pooled: pooled, reused: true}
	require.NoError(t, prepareWebSocketPayloadForLease(secondPayload, secondLease))

	input, ok, err := payloadInputItems(secondPayload)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, input, 1)
	require.Contains(t, string(input[0]), "Lovelace, 36")
	require.Equal(t, "resp_1", secondPayload["previous_response_id"])
}

// regression for #3005: an assistant response that contains "json" (for
// example explanatory prose alongside JSON output) must not suppress the
// json_object reminder on the next turn, or the saved input stops being a
// prefix of the new one and the pooled WebSocket executor rejects reuse.
func TestPrepareWebSocketPayloadKeepsJSONObjectReminderStableAfterAssistantMentionsJSON(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	systemMessage := llm.Message{
		Role:    "system",
		Content: llm.MessageContent{Content: lo.ToPtr("Return a json object.")},
	}
	userMessage := func(text string) llm.Message {
		return llm.Message{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr(text)},
		}
	}
	assistantMessage := llm.Message{
		Role:    "assistant",
		Content: llm.MessageContent{Content: lo.ToPtr(`Here is the JSON: {"name":"Ada"}`)},
	}

	payloadFor := func(msgs []llm.Message) map[string]any {
		hreq, err := transformer.TransformRequest(context.Background(), &llm.Request{
			Model:          "gpt-5",
			ResponseFormat: &llm.ResponseFormat{Type: "json_object"},
			Messages:       msgs,
		})
		require.NoError(t, err)

		payload, err := buildWebSocketCreatePayload(hreq.Body)
		require.NoError(t, err)

		return payload
	}

	firstPayload := payloadFor([]llm.Message{systemMessage, userMessage("Ada Lovelace, 36")})

	pooled := &pooledWebSocketConn{}
	firstLease := &webSocketLease{pooled: pooled}
	require.NoError(t, prepareWebSocketPayloadForLease(firstPayload, firstLease))

	output := make([]json.RawMessage, 0)
	for _, item := range convertAssistantMessage(assistantMessage) {
		raw, err := json.Marshal(item)
		require.NoError(t, err)

		output = append(output, raw)
	}

	require.True(t, firstLease.rememberTurn("resp_1", output))

	secondPayload := payloadFor([]llm.Message{
		systemMessage, userMessage("Ada Lovelace, 36"), assistantMessage, userMessage("Lovelace, 36"),
	})
	secondLease := &webSocketLease{pooled: pooled, reused: true}
	require.NoError(t, prepareWebSocketPayloadForLease(secondPayload, secondLease))

	input, ok, err := payloadInputItems(secondPayload)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, input, 1)
	require.Contains(t, string(input[0]), "Lovelace, 36")
	require.Equal(t, "resp_1", secondPayload["previous_response_id"])
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
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))

		if upgrade == 1 {
			require.NotContains(t, payload, "previous_response_id")
			require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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

func TestWebSocketStreamCloseWhileNextBlockedReportsClosedBeforeResponse(t *testing.T) {
	type connectionKey struct{}
	pongRead := make(chan struct{})
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OnPongReceived: func(context.Context, []byte) { close(pongRead) },
		})
		if !assert.NoError(t, err) {
			return
		}
		defer conn.CloseNow()
		if _, _, err := conn.Read(r.Context()); !assert.NoError(t, err) {
			return
		}
		rawConn := r.Context().Value(connectionKey{}).(net.Conn)
		if !assert.NoError(t, rawConn.SetWriteDeadline(time.Now().Add(time.Second))) {
			return
		}
		// Send an empty ping; the following read consumes the client's pong.
		if _, err := rawConn.Write([]byte{0x89, 0x00}); !assert.NoError(t, err) {
			return
		}
		_, _, err = conn.Read(r.Context())
		assert.Error(t, err)
	}))
	server.Config.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, connectionKey{}, conn)
	}
	server.Start()
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Body:   []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	nextDone := make(chan struct{})
	go func() {
		defer close(nextDone)
		assert.False(t, stream.Next())
	}()
	select {
	case <-pongRead:
	case <-time.After(time.Second):
		t.Fatal("Next did not enter the real socket read")
	}
	require.NoError(t, stream.Close())

	select {
	case <-nextDone:
	case <-time.After(time.Second):
		t.Fatal("Next did not finish after stream.Close")
	}
	// Explicit Close terminates an in-flight read cleanly, unlike parent cancellation.
	require.NoError(t, stream.Err())
}

func TestWebSocketStreamDoesNotRetryFreshConnectionFailure(t *testing.T) {
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.CloseNow()
		if _, _, err := conn.Read(r.Context()); !assert.NoError(t, err) {
			return
		}
		if !assert.NoError(t, conn.CloseNow()) {
			return
		}
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Body:   []byte(`{"model":"gpt-5"}`),
	})
	require.NoError(t, err)
	require.False(t, stream.Next())
	require.Error(t, stream.Err())
	require.Equal(t, int32(1), upgrades.Load())
}

func TestWebSocketExecutorSeparatesPoolByOrganizationHeaders(t *testing.T) {
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
			var upgrades atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrade := upgrades.Add(1)

				conn, err := websocket.Accept(w, r, nil)
				require.NoError(t, err)
				defer conn.CloseNow()

				var payload map[string]any
				require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
				require.NotContains(t, payload, "previous_response_id")

				if upgrade == 1 {
					require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
						"type": terminalType,
						"response": map[string]any{
							"id":     "resp_bad",
							"status": strings.TrimPrefix(terminalType, "response."),
						},
					}))
					return
				}

				require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var first map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &first))
		firstInput, ok := first["input"].([]any)
		require.True(t, ok)
		require.Len(t, firstInput, 1)
		require.NotContains(t, first, "previous_response_id")
		require.NoError(t, conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"response.completed","response":{"id":"resp_1","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"run","arguments":"{}"}]}}`)))

		var second map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &second))
		require.Equal(t, "resp_1", second["previous_response_id"])
		secondInput, ok := second["input"].([]any)
		require.True(t, ok)
		require.Len(t, secondInput, 1)
		message, ok := secondInput[0].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "second", message["id"])
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	secondBody := []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message","role":"user","content":[{"type":"input_text","text":"first"}]},{"id":"fc_1","type":"function_call","call_id":"call_1","name":"run","arguments":"{}"},{"id":"second","type":"message","role":"user","content":[{"type":"input_text","text":"second"}]}]}`)

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
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		for i := 0; i < 2; i++ {
			var payload map[string]any
			if err := wsjson.Read(r.Context(), conn, &payload); err != nil {
				return
			}
			require.NotContains(t, payload, "previous_response_id")
			input, ok := payload["input"].([]any)
			require.True(t, ok)
			require.Len(t, input, 1)

			require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)

		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
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
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
			_, _, err = conn.Read(r.Context())
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
	var upgrades atomic.Int32
	firstPayloadRead := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()

		upgrade := upgrades.Add(1)
		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))

		if upgrade == 1 {
			close(firstPayloadRead)
			_, _, err = conn.Read(r.Context())
			require.Error(t, err)
			return
		}

		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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

// A failed write on a reused session is ambiguous: the upstream may already have
// accepted and executed response.create. The executor must evict the dead session and
// surface the write error instead of resending the request on a new connection, which
// would execute it twice.
func TestWebSocketExecutorDoesNotReplayAfterReusedSessionWriteFailure(t *testing.T) {
	var upgrades atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrades.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.CloseNow()

		var payload map[string]any
		if !assert.NoError(t, wsjson.Read(r.Context(), conn, &payload)) {
			return
		}
		if !assert.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_1", "object": "response", "created_at": 1700000000,
				"model": "gpt-5", "status": "completed", "output": []any{},
			},
		})) {
			return
		}
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	ctx := webSocketTestContext()
	request := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"no-replay-write"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message"}]}`),
	}

	first, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, first.Next())
	require.Equal(t, "response.completed", first.Current().Type)
	require.NoError(t, first.Close())
	require.Equal(t, int32(1), upgrades.Load())

	// Kill the pooled socket while the pool still believes it is healthy, so the
	// next write fails before anything reaches the upstream.
	executor.mu.Lock()
	var pooled *pooledWebSocketConn
	for _, candidate := range executor.pool {
		pooled = candidate

		break
	}
	executor.mu.Unlock()
	require.NotNil(t, pooled)
	require.False(t, pooled.isClosed())
	require.NoError(t, pooled.conn.CloseNow())

	secondRequest := *request
	secondRequest.Body = []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message"},{"id":"second","type":"message"}]}`)
	second, err := executor.DoStream(ctx, &secondRequest)
	require.Error(t, err, "a failed write on a reused session must be surfaced")
	require.Contains(t, err.Error(), "failed to write response.create websocket event")
	require.Nil(t, second)
	require.Equal(t, int32(1), upgrades.Load(),
		"the request must not be resent on a new upstream connection")
}

// A reused session that dies without a close frame must also surface the failure
// rather than silently re-establishing a new upstream connection.
func TestWebSocketExecutorDoesNotReplayAfterReusedSessionReadFailure(t *testing.T) {
	type connectionKey struct{}
	var upgrades atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrade := upgrades.Add(1)
		conn, err := websocket.Accept(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.CloseNow()

		var payload map[string]any
		if !assert.NoError(t, wsjson.Read(r.Context(), conn, &payload)) {
			return
		}
		if !assert.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_1", "object": "response", "created_at": 1700000000,
				"model": "gpt-5", "status": "completed", "output": []any{},
			},
		})) {
			return
		}

		if upgrade > 1 {
			for {
				if _, _, err := conn.Read(r.Context()); err != nil {
					return
				}
			}
		}

		// Drop the peer without a close frame so the client only notices on read.
		tcpConn := r.Context().Value(connectionKey{}).(*net.TCPConn)
		assert.NoError(t, tcpConn.SetLinger(0))
	}))
	server.Config.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
		return context.WithValue(ctx, connectionKey{}, conn)
	}
	server.Start()
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	ctx := webSocketTestContext()
	request := &httpclient.Request{
		Method: http.MethodPost,
		URL:    "http" + strings.TrimPrefix(server.URL, "http") + "/v1/responses",
		Headers: http.Header{
			webSocketSessionHeader: []string{"no-replay-read"},
		},
		Auth: &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: "test-key"},
		Body: []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message"}]}`),
	}

	first, err := executor.DoStream(ctx, request)
	require.NoError(t, err)
	require.True(t, first.Next())
	require.Equal(t, "response.completed", first.Current().Type)
	require.False(t, first.Next())
	require.NoError(t, first.Err())
	require.NoError(t, first.Close())

	time.Sleep(100 * time.Millisecond)

	secondRequest := *request
	secondRequest.Body = []byte(`{"model":"gpt-5","input":[{"id":"first","type":"message"},{"id":"second","type":"message"}]}`)
	second, err := executor.DoStream(ctx, &secondRequest)
	if err == nil {
		for second.Next() {
		}
		require.Error(t, second.Err(), "a dead upstream session must surface an error")
		require.NoError(t, second.Close())
	} else {
		require.Error(t, err)
	}

	require.Equal(t, int32(1), upgrades.Load(), "no new upstream session may be opened")
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
	connectionClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		defer close(connectionClosed)

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
		_, _, _ = conn.Read(r.Context())
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
	connectionClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		defer close(connectionClosed)

		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
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
		_, _, _ = conn.Read(r.Context())
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

func TestNormalizeWebSocketEventPreservesFlattenedFields(t *testing.T) {
	raw := []byte(`{"type":"error","status":400,"code":"invalid_value","message":"flat message","error":{"type":"invalid_request_error","message":"nested message"}}`)

	normalized := normalizeWebSocketEvent(raw)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(normalized, &payload))
	require.Equal(t, "invalid_value", payload["code"])
	require.Equal(t, "flat message", payload["message"])
}

func TestToWebSocketURL(t *testing.T) {
	got, err := toWebSocketURL("https://api.openai.com/v1/responses")
	require.NoError(t, err)
	require.Equal(t, "wss://api.openai.com/v1/responses", got)

	got, err = toWebSocketURL("http://localhost:8080/v1/responses")
	require.NoError(t, err)
	require.Equal(t, "ws://localhost:8080/v1/responses", got)
}

func TestWebSocketExecutorObservesHandshakeHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Handshake-Id", "review")
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		var payload map[string]any
		require.NoError(t, wsjson.Read(r.Context(), conn, &payload))
		require.NoError(t, wsjson.Write(r.Context(), conn, map[string]any{
			"type":     "response.completed",
			"response": map[string]any{"id": "resp_review", "object": "response", "status": "completed", "output": []any{}},
		}))
	}))
	defer server.Close()

	executor := NewWebSocketExecutor(nil)
	defer executor.Close()
	observed := 0
	stream, err := executor.DoStream(webSocketTestContext(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    server.URL + "/v1/responses",
		Body:   []byte(`{"model":"gpt-5","stream":true}`),
		OnResponseHeaders: func(_ context.Context, headers http.Header) {
			observed++
			require.Equal(t, "review", headers.Get("X-Handshake-Id"))
		},
	})
	require.NoError(t, err)
	defer stream.Close()
	for stream.Next() {
		_ = stream.Current()
	}
	require.NoError(t, stream.Err())
	require.Equal(t, 1, observed)
}

func TestWebSocketExecutorStreamIdleTimeout(t *testing.T) {
	type connectionKey struct{}
	for _, heartbeat := range []bool{false, true} {
		t.Run(fmt.Sprintf("heartbeat=%t", heartbeat), func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				require.NoError(t, err)
				defer conn.CloseNow()
				_, _, err = conn.Read(r.Context())
				if err != nil {
					return
				}
				if heartbeat {
					rawConn := r.Context().Value(connectionKey{}).(net.Conn)
					for range 6 {
						time.Sleep(25 * time.Millisecond)
						if err := rawConn.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
							return
						}
						// Unmasked ping with the same payload, without an application message.
						if _, err := rawConn.Write(append([]byte{0x89, 18}, []byte("upstream heartbeat")...)); err != nil {
							return
						}
					}
					if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"response.output_text.delta","delta":"alive"}`)); err != nil {
						return
					}
				}
				// Read until the client's watchdog closes this connection, also
				// consuming pong frames generated by the client's ping handler.
				_, _, _ = conn.Read(r.Context())
			}))
			server.Config.ConnContext = func(ctx context.Context, conn net.Conn) context.Context {
				return context.WithValue(ctx, connectionKey{}, conn)
			}
			server.StartTLS()
			defer server.Close()
			executor := NewWebSocketExecutor(httpclient.NewHttpClientWithClient(server.Client()))
			defer executor.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			stream, err := executor.DoStream(httpclient.WithStreamIdleTimeout(ctx, 100*time.Millisecond), &httpclient.Request{
				Method: http.MethodPost, URL: server.URL, Body: []byte(`{"model":"test"}`),
			})
			require.NoError(t, err)
			defer stream.Close()
			if heartbeat {
				require.True(t, stream.Next(), "upstream control frames must reset the watchdog")
			}
			require.False(t, stream.Next())
			require.ErrorIs(t, stream.Err(), httpclient.ErrStreamIdleTimeout)
			require.NotErrorIs(t, stream.Err(), context.Canceled)
		})
	}
}

func TestWebSocketExecutorStreamIdleTimeoutDuringHandshake(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	executor := NewWebSocketExecutor(httpclient.NewHttpClient())
	defer executor.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	_, err := executor.DoStream(httpclient.WithStreamIdleTimeout(ctx, 50*time.Millisecond), &httpclient.Request{
		Method: http.MethodPost, URL: server.URL, Body: []byte(`{"model":"test"}`),
	})
	require.ErrorIs(t, err, httpclient.ErrStreamIdleTimeout)
}

func TestWebSocketExecutorIdleTimeoutExcludesPoolQueue(t *testing.T) {
	releaseFirst := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		for i := range 2 {
			_, _, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if i == 0 {
				<-releaseFirst
			}
			if err := wsjson.Write(r.Context(), conn, map[string]any{
				"type":     "response.completed",
				"response": map[string]any{"id": fmt.Sprintf("resp_%d", i), "status": "completed", "output": []any{}},
			}); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	defer func() {
		select {
		case <-releaseFirst:
		default:
			close(releaseFirst)
		}
	}()
	executor := NewWebSocketExecutor(httpclient.NewHttpClientWithClient(server.Client()))
	defer executor.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	ctx = shared.WithSessionScope(ctx, "test-scope")
	request := func() *httpclient.Request {
		return &httpclient.Request{
			Method: http.MethodPost, URL: server.URL, Body: []byte(`{"model":"test"}`),
			Headers: http.Header{webSocketSessionHeader: []string{"idle-queue-session"}},
		}
	}
	first, err := executor.DoStream(ctx, request())
	require.NoError(t, err)
	defer first.Close()
	type result struct {
		stream httpclient.StreamDecoder
		err    error
	}
	secondResult := make(chan result, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("queued upstream attempt panicked: %v", recovered)
			}
		}()
		stream, err := executor.DoStream(httpclient.WithStreamIdleTimeout(ctx, 100*time.Millisecond), request())
		secondResult <- result{stream: stream, err: err}
	}()
	select {
	case queued := <-secondResult:
		t.Fatalf("queued request completed before lease release: %v", queued.err)
	case <-time.After(150 * time.Millisecond):
	}
	close(releaseFirst)
	require.True(t, first.Next())
	require.False(t, first.Next())
	require.NoError(t, first.Err())
	select {
	case queued := <-secondResult:
		require.NoError(t, queued.err)
		defer queued.stream.Close()
		require.True(t, queued.stream.Next())
		require.False(t, queued.stream.Next())
		require.NoError(t, queued.stream.Err())
	case <-ctx.Done():
		t.Fatal("queued attempt did not acquire released lease")
	}
}

func TestWebSocketExecutorIdleTimeoutExcludesConsumerDelay(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		require.NoError(t, err)
		defer conn.CloseNow()
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		if err := conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"response.output_text.delta","delta":"first"}`)); err != nil {
			return
		}
		<-release
		_ = conn.Write(r.Context(), websocket.MessageText, []byte(`{"type":"response.output_text.delta","delta":"second"}`))
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	executor := NewWebSocketExecutor(httpclient.NewHttpClientWithClient(server.Client()))
	defer executor.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	stream, err := executor.DoStream(httpclient.WithStreamIdleTimeout(ctx, 50*time.Millisecond), &httpclient.Request{
		Method: http.MethodPost, URL: server.URL, Body: []byte(`{"model":"test"}`),
	})
	require.NoError(t, err)
	defer stream.Close()
	time.Sleep(120 * time.Millisecond)
	require.True(t, stream.Next())
	require.Contains(t, string(stream.Current().Data), "first")
	time.Sleep(120 * time.Millisecond)
	close(release)
	require.True(t, stream.Next())
	require.Contains(t, string(stream.Current().Data), "second")
	require.False(t, stream.Next(), "resuming reads must restore idle timeout enforcement")
	require.ErrorIs(t, stream.Err(), httpclient.ErrStreamIdleTimeout)
}

func TestWebSocketExecutorParentCustomCause(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, nil)
				require.NoError(t, err)
				defer conn.CloseNow()
				_, _, _ = conn.Read(r.Context())
				_, _, _ = conn.Read(r.Context())
			}))
			defer server.Close()
			executor := NewWebSocketExecutor(httpclient.NewHttpClientWithClient(server.Client()))
			defer executor.Close()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			stream, err := executor.DoStream(httpclient.WithStreamIdleTimeout(ctx, timeout), &httpclient.Request{
				Method: http.MethodPost, URL: server.URL, Body: []byte(`{"model":"test"}`),
			})
			require.NoError(t, err)
			defer stream.Close()
			cancel(errors.New("caller-specific cause"))
			require.False(t, stream.Next())
			require.ErrorIs(t, stream.Err(), context.Canceled)
		})
	}
}
