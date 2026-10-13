package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestResponsesProtocolWireMatrix(t *testing.T) {
	tests := []struct {
		name         string
		source       llm.APIFormat
		target       string
		endpointPath string
		expectedPath string
	}{
		{name: "http to http", source: llm.APIFormatOpenAIResponse, target: responses.TransportHTTP, expectedPath: "/v1/responses"},
		{name: "websocket to websocket", source: llm.APIFormatOpenAIResponseWebSocket, target: responses.TransportWebSocket, expectedPath: "/v1/responses"},
		{name: "websocket to http", source: llm.APIFormatOpenAIResponseWebSocket, target: responses.TransportHTTP, expectedPath: "/v1/responses"},
		{name: "http to websocket", source: llm.APIFormatOpenAIResponse, target: responses.TransportWebSocket, expectedPath: "/v1/responses"},
		{name: "http custom endpoint", source: llm.APIFormatOpenAIResponse, target: responses.TransportHTTP, endpointPath: "/custom/responses", expectedPath: "/custom/responses"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var method, path, body string
			var upgraded bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				payload, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				mu.Lock()
				method, path, body = r.Method, r.URL.Path, string(payload)
				upgraded = websocket.IsWebSocketUpgrade(r)
				mu.Unlock()

				if upgraded {
					conn, err := (&websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}).Upgrade(w, r, nil)
					require.NoError(t, err)
					defer conn.Close()
					_, message, err := conn.ReadMessage()
					require.NoError(t, err)
					mu.Lock()
					body = string(message)
					mu.Unlock()
					require.NoError(t, conn.WriteJSON(map[string]any{
						"type": "response.completed",
						"response": map[string]any{
							"id":     "resp_wire",
							"status": "completed",
							"output": []any{map[string]any{"id": "item_wire", "type": "message"}},
						},
					}))
					return
				}

				w.Header().Set("Content-Type", "application/json")
				_, err = io.WriteString(w, `{"id":"resp_wire","status":"completed","output":[{"id":"item_wire","type":"message"}]}`)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)

			outbound, err := responses.NewOutboundTransformerWithConfig(&responses.Config{
				BaseURL:        server.URL,
				APIKeyProvider: auth.NewStaticKeyProvider("synthetic"),
				Transport:      tt.target,
				EndpointPath:   tt.endpointPath,
			})
			require.NoError(t, err)
			requestBody, err := responses.AdaptRequestBody(
				[]byte(`{"model":"wire-model","input":[{"type":"function_call_output","call_id":"call_wire","item_id":"item_wire","output":"canary"}],"provider_canary":"keep"}`),
				llm.APIFormatOpenAIResponse,
				outbound.APIFormat(),
			)
			require.NoError(t, err)
			request, err := outbound.TransformRequest(context.Background(), &llm.Request{Model: "wire-model", RequestType: llm.RequestTypeChat, APIFormat: tt.source, RawRequest: &httpclient.Request{Body: requestBody}})
			require.NoError(t, err)
			request.Body = requestBody
			custom := pipeline.ChannelCustomizedExecutor(outbound)
			executor := custom.CustomizeExecutor(httpclient.NewHttpClientWithClient(server.Client()))
			if tt.target == responses.TransportWebSocket {
				stream, err := executor.DoStream(context.Background(), request)
				require.NoError(t, err)
				require.True(t, stream.Next())
				require.Contains(t, string(stream.Current().Data), "item_wire")
				require.NoError(t, stream.Close())
			} else {
				response, err := executor.Do(context.Background(), request)
				require.NoError(t, err)
				require.Contains(t, string(response.Body), "item_wire")
			}
			mu.Lock()
			expectedMethod := http.MethodPost
			if tt.target == responses.TransportWebSocket {
				expectedMethod = http.MethodGet
			}
			require.Equal(t, expectedMethod, method)
			require.Equal(t, tt.expectedPath, path)
			require.Equal(t, tt.target == responses.TransportWebSocket, upgraded)
			require.Contains(t, body, "call_wire")
			require.Contains(t, string(requestBody), "call_wire")
			mu.Unlock()
		})
	}
}
