package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestStreamDeliveryWebSocketHTTPUpstream(t *testing.T) {
	for _, completed := range []bool{false, true} {
		name := "disconnect_before_terminal"
		if completed {
			name = "completed_write_then_disconnect"
		}
		t.Run(name, func(t *testing.T) {
			// Given: synthetic SSE over real HTTP feeding the real WebSocket surface.
			upstreamCanceled := make(chan struct{})
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if _, err := io.WriteString(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_http\"}}\n\n"); err != nil {
					t.Errorf("write created: %v", err)
					return
				}
				w.(http.Flusher).Flush()
				if completed {
					if _, err := io.WriteString(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"); err != nil {
						t.Errorf("write terminal: %v", err)
					}
					return
				}
				<-r.Context().Done()
				close(upstreamCanceled)
			}))
			t.Cleanup(upstream.Close)
			client := httpclient.NewHttpClientWithClient(upstream.Client())
			t.Cleanup(client.CloseIdleConnections)
			sources := make(chan *responsesWebSocketDeliveryStream, 1)
			process := func(ctx context.Context, request *httpclient.Request) (orchestrator.ChatCompletionResult, error) {
				upstreamRequest := *request
				upstreamRequest.URL = upstream.URL + "/v1/responses"
				upstreamRequest.Auth = nil
				stream, err := client.DoStream(ctx, &upstreamRequest)
				if err != nil {
					return orchestrator.ChatCompletionResult{}, err
				}
				source := newResponsesWebSocketDeliveryStream("response.completed")
				source.Stream = stream
				sources <- source
				return orchestrator.ChatCompletionResult{ChatCompletionStream: source}, nil
			}
			server := newResponsesWebSocketTestServer(t, process, nil)
			conn := dialResponsesWebSocket(t, server.URL, nil)
			defer conn.Close()
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
			// When: the socket closes at the chosen observable semantic boundary.
			require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-test","input":"synthetic"}`)))
			_, data, err := conn.ReadMessage()
			require.NoError(t, err)
			require.Equal(t, "response.created", gjson.GetBytes(data, "type").String())
			source := <-sources
			if completed {
				_, data, err = conn.ReadMessage()
				require.NoError(t, err)
				require.Equal(t, "response.completed", gjson.GetBytes(data, "type").String())
				select {
				case <-source.acknowledgedC:
				case <-time.After(5 * time.Second):
					t.Fatal("terminal write was not acknowledged")
				}
			}
			require.NoError(t, conn.Close())
			// Then: only the completed turn is acknowledged; pre-terminal disconnect cancels upstream.
			select {
			case <-source.closedC:
			case <-time.After(5 * time.Second):
				t.Fatal("turn stream did not close")
			}
			require.Equal(t, completed, source.DeliveryAcknowledged())
			if !completed {
				select {
				case <-upstreamCanceled:
				case <-time.After(5 * time.Second):
					t.Fatal("upstream was not canceled")
				}
			}
			t.Logf("real HTTP SSE -> real WebSocket: terminal_written=%v acknowledged=%v", completed, source.DeliveryAcknowledged())
		})
	}
}
