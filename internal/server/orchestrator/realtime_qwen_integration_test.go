// 职责: 验证三种千问模型的真实本地 WebSocket 配置确认、双向音频与工具回传。
// 关联文件:
//
//	internal/server/orchestrator/realtime.go
//	llm/realtime/adapter.go
//	internal/server/realtime/session.go

package orchestrator

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
)

func TestRealtimeQwenConfigurationDuplexAndTools(t *testing.T) {
	for _, model := range []string{"qwen3.8-omni-flash-realtime", "qwen3.5-omni-plus-realtime", "qwen3.5-omni-flash-realtime"} {
		t.Run(model, func(t *testing.T) {
			upstream, provider := realtimeSocketPair(t)
			require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"session.created","event_id":"created","session":{"id":"s1","model":"`+model+`","modalities":["text","audio"],"voice":"Tina","input_audio_format":"pcm","output_audio_format":"pcm"}}`)))
			ack := `{"type":"session.updated","session":{"id":"s1","model":"` + model + `","modalities":["text","audio"],"voice":"Tina","turn_detection":null,"input_audio_format":"pcm","output_audio_format":"pcm"`
			if model != "qwen3.8-omni-flash-realtime" {
				ack += `,"audio":{"input":{"format":{"type":"pcm","sample_rate":24000}},"output":{"format":{"type":"pcm","sample_rate":24000}}}`
			}
			ack += `}}`
			ackResult := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Qwen configuration mock panic: %T", r)
						ackResult <- fmt.Errorf("mock panic: %T", r)
					}
				}()
				_ = provider.SetReadDeadline(time.Now().Add(2 * time.Second))
				_, data, err := provider.ReadMessage()
				if err == nil && gjson.GetBytes(data, "type").String() != "session.update" {
					err = fmt.Errorf("configuration event missing")
				}
				if err == nil {
					err = provider.WriteMessage(websocket.TextMessage, []byte(ack))
				}
				ackResult <- err
			}()
			service := &RealtimeService{}
			conversation, initial, err := service.configure(context.Background(), upstream, &RealtimeTarget{Model: model, ClientModel: "voice", APIFormat: llm.APIFormatQwenRealtimeWebSocket, Channel: &biz.Channel{}}, rt.Config{}.Defaults())
			require.NoError(t, err)
			require.NoError(t, <-ackResult)
			require.True(t, conversation.state.Ready())
			require.Len(t, initial, 1)
			require.Equal(t, "session.created", gjson.GetBytes(initial[0], "type").String())
			require.EqualValues(t, 24000, gjson.GetBytes(initial[0], "session.audio.input.format.rate").Int())
			client, downstream := realtimeSocketPair(t)
			ctx := t.Context()
			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Qwen bridge panic: %T", r)
						done <- fmt.Errorf("bridge panic: %T", r)
					}
				}()
				done <- rt.Run(ctx, downstream, upstream, rt.Options{IdleTimeout: time.Second}, rt.Hooks{Client: conversation.adapter.Client, Server: conversation.adapter.Server})
			}()
			send := func(data string) {
				t.Helper()
				require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(data)))
			}
			readProvider := func() []byte {
				t.Helper()
				_ = provider.SetReadDeadline(time.Now().Add(time.Second))
				_, data, err := provider.ReadMessage()
				require.NoError(t, err)
				return data
			}
			send(`{"type":"input_audio_buffer.append","audio":"` + base64.StdEncoding.EncodeToString(make([]byte, 480)) + `"}`)
			wire := readProvider()
			pcm, err := base64.StdEncoding.DecodeString(gjson.GetBytes(wire, "audio").String())
			require.NoError(t, err)
			expected := 480
			if model == "qwen3.8-omni-flash-realtime" {
				expected = 320
			}
			require.Len(t, pcm, expected)
			for _, data := range []string{`{"type":"input_audio_buffer.commit"}`, `{"type":"response.create"}`, `{"type":"response.cancel","response_id":"r1"}`, `{"type":"conversation.item.create","item":{"type":"function_call_output","call_id":"call1","output":"{}"}}`} {
				send(data)
				got := readProvider()
				require.JSONEq(t, data, string(got))
			}
			for _, event := range []struct{ from, to string }{{"response.audio.delta", "response.output_audio.delta"}, {"response.audio_transcript.delta", "response.output_audio_transcript.delta"}, {"response.text.delta", "response.output_text.delta"}, {"response.function_call_arguments.done", "response.function_call_arguments.done"}} {
				require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"`+event.from+`","response_id":"r1","item_id":"i1","call_id":"call1","delta":"AAA="}`)))
				_ = client.SetReadDeadline(time.Now().Add(time.Second))
				_, data, err := client.ReadMessage()
				require.NoError(t, err)
				var e map[string]any
				require.NoError(t, json.Unmarshal(data, &e))
				require.Equal(t, event.to, e["type"])
				require.Equal(t, "call1", e["call_id"])
			}
			client.Close()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Qwen disconnect leaked session")
			}
		})
	}
}
