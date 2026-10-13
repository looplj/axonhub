package api

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestResponsesProtocolNativeContinuationRetainsCompletedConnection(t *testing.T) {
	for _, provider := range []channel.Type{channel.TypeOpenaiResponses, channel.TypeCodex} {
		t.Run(string(provider), func(t *testing.T) {
			// Given: a production handler and provider with connection-local context.
			var connections atomic.Int32
			frames := make(chan []byte, 2)
			upstream := responsesIntegrationServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				connections.Add(1)
				defer conn.Close()
				if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
					t.Error(err)
					return
				}
				for turn := 0; ; turn++ {
					_, body, err := conn.ReadMessage()
					if err != nil {
						return
					}
					frames <- body
					response := `{"type":"response.completed","response":{"id":"provider-tool","status":"completed","output":[{"type":"function_call","id":"fc-tool","call_id":"call-tool","name":"payload","arguments":"{}"}]}}`
					if turn > 0 {
						response = `{"type":"response.completed","response":{"id":"provider-final","status":"completed","output":[]}}`
					}
					if err := conn.WriteMessage(websocket.TextMessage, []byte(response)); err != nil {
						t.Error(err)
						return
					}
				}
			}))
			client, handlers, project := newResponsesIntegrationFixture(t, upstream.URL, func(create *ent.ChannelCreate) {
				create.SetType(provider)
				create.SetEndpoints([]objects.ChannelEndpoint{{APIFormat: "openai/responses-ws"}})
				create.SetSettings(&objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{{Model: "synthetic-model", APIFormats: []string{"openai/responses-ws"}}}})
			})
			gateway := responsesIntegrationGateway(t, client, handlers, project)
			conn := responsesIntegrationDial(t, gateway)
			first := responsesIntegrationTurn(t, conn, `{"type":"response.create","model":"synthetic-model","input":"root-canary"}`)
			require.Equal(t, "provider-tool", gjson.GetBytes(first, "response.id").String(), string(first))
			firstFrame := <-frames
			require.False(t, gjson.GetBytes(firstFrame, "previous_response_id").Exists())
			output := strings.Repeat("x", (2<<20)+3)
			body, err := json.Marshal(map[string]any{
				"type": "response.create", "model": "synthetic-model", "previous_response_id": "provider-tool",
				"input": []map[string]string{{"type": "function_call_output", "call_id": "call-tool", "output": output}},
			})
			require.NoError(t, err)
			// When: the paired large tool output continues the completed native turn.
			completed := responsesIntegrationTurn(t, conn, string(body))
			// Then: the exact previous ID and output reach the same upstream connection.
			require.Equal(t, "provider-final", gjson.GetBytes(completed, "response.id").String(), string(completed))
			secondFrame := <-frames
			require.Equal(t, int32(1), connections.Load())
			require.Equal(t, "provider-tool", gjson.GetBytes(secondFrame, "previous_response_id").String())
			require.Len(t, gjson.GetBytes(secondFrame, "input").Array(), 1)
			require.Equal(t, "call-tool", gjson.GetBytes(secondFrame, "input.0.call_id").String())
			require.Equal(t, sha256.Sum256([]byte(output)), sha256.Sum256([]byte(gjson.GetBytes(secondFrame, "input.0.output").String())))
		})
	}
}
