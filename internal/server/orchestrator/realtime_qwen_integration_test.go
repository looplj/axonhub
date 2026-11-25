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

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

func TestRealtimeQwenConfigurationDuplexAndTools(t *testing.T) {
	for _, model := range []string{"qwen3.8-omni-flash-realtime", "qwen3.5-omni-plus-realtime", "qwen3.5-omni-flash-realtime"} {
		t.Run(model, func(t *testing.T) {
			upstream, provider := realtimeSocketPair(t)
			require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"session.created","event_id":"created","session":{"id":"s1","model":"`+model+`","modalities":["text","audio"],"voice":"Tina","input_audio_format":"pcm","output_audio_format":"pcm"}}`)))
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
				_, data, err := realtimeReadWithTimeout(provider, 2*time.Second)
				if err == nil && gjson.GetBytes(data, "type").String() != "session.update" {
					err = fmt.Errorf("configuration event missing")
				}
				if err == nil {
					err = realtimeWriteText(provider, []byte(ack))
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
				done <- rt.Run(ctx, downstream, upstream, rt.Options{IdleTimeout: time.Second}, rt.Hooks{Client: conversation.adapter.Client, Server: conversation.adapter.Server, ServerFollowup: conversation.ServerFollowup})
			}()
			require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"conversation.item.created","event_id":"qwen-item","item":{"id":"i1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}]}}`)))
			for _, kind := range []string{"conversation.item.added", "conversation.item.done"} {
				_, frame, err := realtimeReadWithTimeout(client, time.Second)
				require.NoError(t, err)
				require.Equal(t, kind, gjson.GetBytes(frame, "type").String())
				require.Equal(t, "i1", gjson.GetBytes(frame, "item.id").String())
				require.Equal(t, "output_text", gjson.GetBytes(frame, "item.content.0.type").String())
			}

			send := func(data string) {
				t.Helper()
				require.NoError(t, realtimeWriteText(client, []byte(data)))
			}
			readProvider := func() []byte {
				t.Helper()
				_, data, err := realtimeReadWithTimeout(provider, time.Second)
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
				require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"`+event.from+`","response_id":"r1","item_id":"i1","call_id":"call1","delta":"AAA="}`)))
				_, data, err := realtimeReadWithTimeout(client, time.Second)
				require.NoError(t, err)
				var e map[string]any
				require.NoError(t, json.Unmarshal(data, &e))
				require.Equal(t, event.to, e["type"])
				require.Equal(t, "call1", e["call_id"])
			}
			_ = client.CloseNow()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("Qwen disconnect leaked session")
			}
		})
	}
}

func TestRealtimeProbeAcknowledgesProviderItemLifecycle(t *testing.T) {
	for _, format := range []llm.APIFormat{llm.APIFormatOpenAIRealtimeWebSocket, llm.APIFormatQwenRealtimeWebSocket} {
		t.Run(string(format), func(t *testing.T) {
			conn, provider := realtimeSocketPair(t)
			if format == llm.APIFormatOpenAIRealtimeWebSocket {
				require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"conversation.item.added"}`)))
				require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"conversation.item.done"}`)))
			} else {
				require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"conversation.item.created"}`)))
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, waitRealtimeItemAcknowledgement(ctx, conn, format))
		})
	}
}

func TestRealtimeTranslationProbeConfirmsUpdateAndClose(t *testing.T) {
	upstream, provider := realtimeSocketPair(t)
	adapter := &openairt.Translation{}
	_, err := adapter.Server([]byte(`{"type":"session.created","session":{"type":"translation","model":"gpt-realtime-translate"}}`))
	require.NoError(t, err)

	result := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("translation probe panic: %v", r)
				result <- fmt.Errorf("translation probe panic")
			}
		}()
		result <- probeTranslationSession(context.Background(), upstream, adapter)
	}()
	_, update, err := realtimeReadWithTimeout(provider, time.Second)
	require.NoError(t, err)
	require.Equal(t, "session.update", gjson.GetBytes(update, "type").String())
	require.Equal(t, "es", gjson.GetBytes(update, "session.audio.output.language").String())
	require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"session.updated","session":{"type":"translation","model":"gpt-realtime-translate","audio":{"output":{"language":"es"}}}}`)))
	_, closeEvent, err := realtimeReadWithTimeout(provider, time.Second)
	require.NoError(t, err)
	require.Equal(t, "session.close", gjson.GetBytes(closeEvent, "type").String())
	require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"session.closed"}`)))
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("translation probe did not finish")
	}
}

func TestRealtimeTranslationHandoffClearsSharedConfigurationTimeout(t *testing.T) {
	upstream, provider := realtimeSocketPair(t)
	require.NoError(t, realtimeWriteText(provider, []byte(`{"type":"session.created","session":{"type":"translation","model":"gpt-realtime-translate"}}`)))
	target := &RealtimeTarget{
		Translation: true,
		Model:       "gpt-realtime-translate",
		ClientModel: "gpt-realtime-translate",
	}
	conversation, initial, err := (&RealtimeService{}).configure(context.Background(), upstream, target, rt.Config{HandshakeTimeout: time.Second}.Defaults())
	require.NoError(t, err)
	require.Len(t, initial, 1)
	require.NotNil(t, conversation.translation)
	require.False(t, conversation.state.ConfigurationExpired(-time.Second), "translation adapter owns the configuration acknowledgement timeout")

	update := []byte(`{"type":"session.update","event_id":"language-update","session":{"audio":{"output":{"language":"en"}}}}`)
	_, err = conversation.adapter.Client(update)
	require.NoError(t, err)
	_, err = conversation.Server([]byte(`{"type":"session.updated","session":{"type":"translation","model":"gpt-realtime-translate","audio":{"output":{"language":"en"}}}}`))
	require.NoError(t, err)
	require.False(t, conversation.state.ConfigurationExpired(-time.Second))
}
