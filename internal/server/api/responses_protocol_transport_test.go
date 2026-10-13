package api

import (
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestResponsesProtocolIntegrationUpstreamWebSocket(t *testing.T) {
	for _, downstream := range []string{"http", "ws"} {
		t.Run(downstream, func(t *testing.T) {
			// Given: the production handler/orchestrator and a native WS provider.
			frames := make(chan []byte, 4)
			closed := make(chan struct{})
			upstream := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/v1/responses", r.URL.Path)
				require.Equal(t, "Bearer synthetic-key", r.Header.Get("Authorization"))
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.Close()
				defer close(closed)
				require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
				for {
					_, body, err := conn.ReadMessage()
					if err != nil {
						return
					}
					frames <- body
					if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created","sequence_number":0,"response":{"id":"provider-native","object":"response","created_at":1700000000,"model":"synthetic-model","status":"in_progress","output":[]}}`)); err != nil {
						t.Error(err)
						return
					}
					if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.output_item.added","sequence_number":1,"output_index":0,"item":{"type":"message","id":"provider-tool-item","status":"in_progress","role":"assistant","content":[]}}`)); err != nil {
						t.Error(err)
						return
					}
					if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.output_text.delta","sequence_number":2,"output_index":0,"content_index":0,"item_id":"provider-tool-item","delta":"synthetic-answer"}`)); err != nil {
						t.Error(err)
						return
					}
					if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.output_item.done","sequence_number":3,"output_index":0,"item":{"type":"message","id":"provider-tool-item","status":"completed","role":"assistant","content":[{"type":"output_text","text":"synthetic-answer"}]}}`)); err != nil {
						t.Error(err)
						return
					}
					completed := `{"type":"response.completed","sequence_number":4,"response":{"id":"provider-native","object":"response","created_at":1700000000,"model":"synthetic-model","status":"completed","output":[{"type":"message","id":"provider-tool-item","role":"assistant","content":[{"type":"output_text","text":"synthetic-answer"}]}]}}`
					if err := conn.WriteMessage(websocket.TextMessage, []byte(completed)); err != nil {
						t.Error(err)
						return
					}
				}
			}))
			client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL, func(create *ent.ChannelCreate) {
				create.SetEndpoints([]objects.ChannelEndpoint{{APIFormat: "openai/responses-ws"}})
			})
			gateway := responsesIntegrationGateway(t, client, handlers, project)
			// When
			if downstream == "http" {
				status, response := responsesIntegrationPost(t, gateway, "/v1/responses", `{"model":"synthetic-model","input":"root-canary"}`)
				require.Equal(t, http.StatusOK, status, string(response))
				require.Equal(t, "provider-native", gjson.GetBytes(response, "id").String())
				require.Equal(t, "provider-tool-item", gjson.GetBytes(response, "output.0.id").String(), string(response))
			} else {
				conn := responsesIntegrationDial(t, gateway)
				response := responsesIntegrationTurn(t, conn, `{"type":"response.create","stream_id":"lane-canary","model":"synthetic-model","input":"root-canary"}`)
				require.Equal(t, "provider-native", gjson.GetBytes(response, "response.id").String(), string(response))
				require.Equal(t, "provider-tool-item", gjson.GetBytes(response, "response.output.0.id").String(), string(response))
				require.Equal(t, "lane-canary", gjson.GetBytes(response, "stream_id").String())
				require.NoError(t, conn.Close())
				responsesIntegrationAwait(t, closed)
			}
			// Then: envelopes and provider identity remain native, with no cumulative replay.
			first := <-frames
			firstInput := gjson.GetBytes(first, "input")
			require.Contains(t, firstInput.Raw, "root-canary")
			require.Equal(t, "response.create", gjson.GetBytes(first, "type").String())
		})
	}
}
