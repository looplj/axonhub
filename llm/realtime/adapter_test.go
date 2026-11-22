// 职责: 验证 Qwen 与 GA 结构、工具、音频及用量转换。
// 关联文件:
//
//	llm/realtime/adapter.go
package realtime

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGAQwenSessionAndEvents(t *testing.T) {
	for _, model := range modelNames() {
		t.Run(model, func(t *testing.T) {
			a, err := NewAdapter(model, false)
			require.NoError(t, err)
			raw, err := a.Client([]byte(`{"event_id":"e1","type":"session.update","session":{"type":"realtime","output_modalities":["audio"],"audio":{"input":{"format":{"type":"audio/pcm","rate":24000},"turn_detection":null},"output":{"format":{"type":"audio/pcm","rate":24000},"voice":"Tina"}},"tools":[{"type":"function","name":"weather","parameters":{"type":"object"}}]}}`))
			require.NoError(t, err)
			var e map[string]any
			require.NoError(t, json.Unmarshal(raw, &e))
			s := e["session"].(map[string]any)
			require.Equal(t, []any{"text", "audio"}, s["modalities"])
			require.Equal(t, "e1", e["event_id"])
			tools := s["tools"].([]any)
			require.Equal(t, "weather", tools[0].(map[string]any)["function"].(map[string]any)["name"])
			for from, to := range map[string]string{"response.audio.delta": "response.output_audio.delta", "response.audio.done": "response.output_audio.done", "response.audio_transcript.delta": "response.output_audio_transcript.delta", "response.audio_transcript.done": "response.output_audio_transcript.done", "response.text.delta": "response.output_text.delta", "response.text.done": "response.output_text.done", "response.function_call_arguments.delta": "response.function_call_arguments.delta", "response.function_call_arguments.done": "response.function_call_arguments.done"} {
				got, err := a.Server([]byte(`{"type":"` + from + `","event_id":"e2","response_id":"r1","item_id":"i1","call_id":"c1","delta":"AAA="}`))
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(got, &e))
				require.Equal(t, to, e["type"])
				require.Equal(t, "c1", e["call_id"])
			}
			_, err = a.Client([]byte(`{"type":"conversation.item.create","item":{"type":"function_call_output","call_id":"c1","output":"{}"}}`))
			require.NoError(t, err)
		})
	}
}

func TestQwenRejectsUnsupportedParameters(t *testing.T) {
	a, _ := NewAdapter("qwen3.8-omni-flash-realtime", false)
	for _, raw := range []string{`{"type":"session.update"}`, `{"type":"session.update","session":{"type":"realtime","model":"other"}}`, `{"type":"session.update","session":{"type":"realtime","unknown":true}}`, `{"type":"session.update","session":{"type":"realtime","output_modalities":["audio","text"]}}`, `{"type":"session.update","session":{"type":"realtime","audio":{"input":{"format":{"type":"audio/pcmu"}}}}}`, `{"type":"session.update","session":{"type":"realtime","tools":[{"type":"mcp"}]}}`, `{"type":"input_image_buffer.append","image":"AAA="}`, `{"type":"input_audio_buffer.append","audio":"!!"}`} {
		_, err := a.Client([]byte(raw))
		require.Error(t, err, raw)
	}
}

func TestQwen24kPCMAndUsage(t *testing.T) {
	a, _ := NewAdapter("qwen3.5-omni-flash-realtime", false)
	encoded := base64.StdEncoding.EncodeToString(make([]byte, 480))
	wire, err := a.Client([]byte(`{"type":"input_audio_buffer.append","audio":"` + encoded + `"}`))
	require.NoError(t, err)
	var e map[string]any
	require.NoError(t, json.Unmarshal(wire, &e))
	require.Equal(t, encoded, e["audio"])
	id, u, err := ParseUsage([]byte(`{"type":"response.done","response":{"id":"r1","usage":{"input_tokens":100,"output_tokens":30,"total_tokens":130,"input_tokens_details":{"text_tokens":40,"audio_tokens":60},"output_tokens_details":{"text_tokens":10,"audio_tokens":20}}}}`))
	require.NoError(t, err)
	require.Equal(t, "r1", id)
	require.EqualValues(t, 60, u.PromptTokensDetails.AudioTokens)
	_, _, err = ParseUsage([]byte(`{"type":"response.done","response":{"id":"r1"}}`))
	require.Error(t, err)
}

func TestQwenDeclaredServerEventSurface(t *testing.T) {
	a, _ := NewAdapter("qwen3.8-omni-flash-realtime", false)
	for _, kind := range []string{"input_audio_buffer.speech_started", "input_audio_buffer.speech_stopped", "input_audio_buffer.committed", "conversation.item.created", "response.created", "response.output_item.added", "response.output_item.done", "response.content_part.added", "response.content_part.done", "response.function_call_arguments.delta", "response.function_call_arguments.done"} {
		data := []byte(`{"type":"` + kind + `","event_id":"event1","response_id":"response1","item_id":"item1","call_id":"call1"}`)
		converted, err := a.Server(data)
		require.NoError(t, err)
		var event map[string]any
		require.NoError(t, json.Unmarshal(converted, &event))
		expected := kind
		if kind == "conversation.item.created" {
			expected = "conversation.item.added"
		}
		require.Equal(t, expected, event["type"])
		require.Equal(t, "event1", event["event_id"])
		require.Equal(t, "call1", event["call_id"])
	}
	wire, err := a.Client([]byte(`{"type":"session.update","session":{"type":"realtime","audio":{"input":{"turn_detection":{"type":"server_vad","threshold":0.5,"silence_duration_ms":800,"create_response":true,"interrupt_response":true}}}}}`))
	require.NoError(t, err)
	var event map[string]any
	require.NoError(t, json.Unmarshal(wire, &event))
	session := event["session"].(map[string]any)
	require.Equal(t, "server_vad", session["turn_detection"].(map[string]any)["type"])
}

func TestQwenConversationContentUsesGATypes(t *testing.T) {
	a, _ := NewAdapter("qwen3.8-omni-flash-realtime", false)
	for _, tc := range []struct{ role, from, to string }{{"assistant", "input_audio", "output_audio"}, {"assistant", "text", "output_text"}, {"user", "audio", "input_audio"}} {
		wire, err := a.Server([]byte(`{"type":"conversation.item.created","item":{"id":"i1","type":"message","role":"` + tc.role + `","content":[{"type":"` + tc.from + `"}]}}`))
		require.NoError(t, err)
		var e map[string]any
		require.NoError(t, json.Unmarshal(wire, &e))
		part := e["item"].(map[string]any)["content"].([]any)[0].(map[string]any)
		require.Equal(t, tc.to, part["type"])
	}
}

func TestQwenSemanticVADIsModelCapabilityBound(t *testing.T) {
	models := []string{"qwen3.8-omni-flash-realtime", "qwen3.5-omni-plus-realtime", "qwen3.5-omni-flash-realtime"}
	for _, model := range models {
		for _, native := range []bool{false, true} {
			t.Run(model+map[bool]string{false: "/ga", true: "/native"}[native], func(t *testing.T) {
				adapter, err := NewAdapter(model, native)
				require.NoError(t, err)
				session := `{"turn_detection":{"type":"semantic_vad","threshold":-0.5,"silence_duration_ms":800}}`
				if !native {
					session = `{"type":"realtime","output_modalities":["text"],"audio":{"input":{"turn_detection":{"type":"semantic_vad","threshold":-0.5,"silence_duration_ms":800}}}}`
				}
				_, err = adapter.Client([]byte(`{"type":"session.update","session":` + session + `}`))
				require.NoError(t, err)
			})
		}
	}
	adapter, err := NewAdapter("qwen3.8-omni-flash-realtime", false)
	require.NoError(t, err)
	_, err = adapter.Client([]byte(`{"type":"session.update","session":{"type":"realtime","output_modalities":["text"],"audio":{"input":{"turn_detection":{"type":"semantic_vad","threshold":1.1}}}}}`))
	require.Error(t, err, "semantic VAD thresholds outside [-1, 1] must be rejected")
	_, err = adapter.Client([]byte(`{"type":"session.update","session":{"type":"realtime","output_modalities":["text"],"audio":{"input":{"turn_detection":{"type":"server_vad","silence_duration_ms":100}}}}}`))
	require.Error(t, err, "VAD silence durations outside [200, 6000] must be rejected")
}

func TestNativeQwenNegotiatesInputAndOutputRatesIndependently(t *testing.T) {
	for _, model := range modelNames() {
		a, err := NewAdapter(model, true)
		require.NoError(t, err)
		raw := fmt.Sprintf(`{"type":"session.update","session":{"modalities":["text","audio"],"audio":{"input":{"format":{"type":"pcm","sample_rate":%d}},"output":{"format":{"type":"pcm","sample_rate":24000}}},"turn_detection":null}}`, NativeInputRate(model))
		_, err = a.Client([]byte(raw))
		require.NoError(t, err, model)
	}
}

func TestQwenErrorRedactionRetainsRejectedEventID(t *testing.T) {
	for _, native := range []bool{false, true} {
		adapter, err := NewAdapter("qwen3.8-omni-flash-realtime", native)
		require.NoError(t, err)
		raw, err := adapter.Server([]byte(`{"type":"error","event_id":"server-event","error":{"event_id":"client-event","message":"private","param":"private"}}`))
		require.NoError(t, err)
		require.JSONEq(t, `{"type":"error","event_id":"server-event","error":{"event_id":"client-event","type":"invalid_request_error","code":"upstream_error","message":"The Qwen Realtime service rejected an event."}}`, string(raw))
	}
}

func TestQwenUnrelatedErrorPreservesPendingAudioRates(t *testing.T) {
	a, err := NewAdapter("qwen3.5-omni-flash-realtime", true)
	require.NoError(t, err)
	const session = `{"modalities":["text","audio"],"audio":{"input":{"format":{"type":"pcm","sample_rate":16000}},"output":{"format":{"type":"pcm","sample_rate":24000}}},"turn_detection":null}`
	_, err = a.Client([]byte(`{"type":"session.update","event_id":"new-rate","session":` + session + `}`))
	require.NoError(t, err)
	_, err = a.Server([]byte(`{"type":"error","error":{"event_id":"earlier-commit"}}`))
	require.NoError(t, err)
	_, err = a.Server([]byte(`{"type":"session.updated","session":` + session + `}`))
	require.NoError(t, err)

	previousRate := strings.Replace(session, "16000", "24000", 1)
	_, err = a.Client([]byte(`{"type":"session.update","event_id":"rejected-rate","session":` + previousRate + `}`))
	require.NoError(t, err)
	_, err = a.Server([]byte(`{"type":"error","error":{"event_id":"rejected-rate"}}`))
	require.NoError(t, err)
	require.Equal(t, 16000, a.inputRate)
}

func TestQwenReplaysGAAssistantText(t *testing.T) {
	const frame = `{"type":"conversation.item.create","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Earlier answer"}]}}`
	a, err := NewAdapter("qwen3.8-omni-flash-realtime", false)
	require.NoError(t, err)
	wire, err := a.Client([]byte(frame))
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"conversation.item.create","item":{"type":"message","role":"assistant","content":[{"type":"text","text":"Earlier answer"}]}}`, string(wire))

	native, err := NewAdapter("qwen3.8-omni-flash-realtime", true)
	require.NoError(t, err)
	_, err = native.Client([]byte(frame))
	require.Error(t, err)
}

func TestQwenFailedResponseRedaction(t *testing.T) {
	const failed = `{"type":"response.done","event_id":"done-1","response":{"id":"r1","status":"failed","status_details":{"error":{"message":"private provider detail","param":"private parameter"}},"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2,"input_tokens_details":{"text_tokens":1},"output_tokens_details":{"text_tokens":1}}}}`
	for _, native := range []bool{true, false} {
		a, err := NewAdapter("qwen3.8-omni-flash-realtime", native)
		require.NoError(t, err)
		wire, err := a.Server([]byte(failed))
		require.NoError(t, err)
		require.NotContains(t, string(wire), "private")
		require.Contains(t, string(wire), "upstream_error")
		require.Contains(t, string(wire), "done-1")
		require.Contains(t, string(wire), "total_tokens")
	}
	a, err := NewAdapter("qwen3.8-omni-flash-realtime", true)
	require.NoError(t, err)
	normal := []byte(`{ "type": "response.done", "response": {"id":"r2","status":"completed"} }`)
	wire, err := a.Server(normal)
	require.NoError(t, err)
	require.Equal(t, normal, wire)
}

func TestQwenTranscriptionFailureRedactsProviderDetails(t *testing.T) {
	for _, native := range []bool{true, false} {
		adapter, err := NewAdapter("qwen3.8-omni-flash-realtime", native)
		require.NoError(t, err)
		raw := []byte(`{"type":"conversation.item.input_audio_transcription.failed","event_id":"event-1","item_id":"item-1","error":{"event_id":"client-event","message":"private provider detail","param":"secret"}}`)
		redacted, err := adapter.Server(raw)
		require.NoError(t, err)
		require.Contains(t, string(redacted), "event-1")
		require.Contains(t, string(redacted), "item-1")
		require.Contains(t, string(redacted), "client-event")
		require.Contains(t, string(redacted), "upstream_error")
		require.NotContains(t, string(redacted), "private")
		require.NotContains(t, string(redacted), "secret")
	}
}

func TestNativeNestedTurnDetection(t *testing.T) {
	for _, tc := range []struct {
		name, session string
		wantErr       error
	}{
		{"nested", `{"audio":{"input":{"turn_detection":{"type":"semantic_vad","threshold":0.5}}}}`, nil},
		{"manual", `{"audio":{"input":{"turn_detection":null}}}`, nil},
		{"same", `{"turn_detection":null,"audio":{"input":{"turn_detection":null}}}`, nil},
		{"conflict", `{"turn_detection":null,"audio":{"input":{"turn_detection":{"type":"semantic_vad"}}}}`, ErrConflictingTurnDetection},
		{"invalid", `{"audio":{"input":{"turn_detection":{"type":"semantic_vad","threshold":2}}}}`, ErrInvalidNestedTurnDetection},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := NewAdapter("qwen3.8-omni-flash-realtime", true)
			require.NoError(t, err)
			wire, err := a.Client([]byte(`{"type":"session.update","session":` + tc.session + `}`))
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, `{"type":"session.update","session":`+tc.session+`}`, string(wire))
			// Qwen acknowledges the effective PCM formats alongside the update.
			input := `"format":{"type":"pcm","sample_rate":16000}`
			if tc.name == "nested" {
				input += `,"turn_detection":{"type":"semantic_vad","threshold":0.5}`
			} else {
				input += `,"turn_detection":null`
			}
			ack := `{"type":"session.updated","session":{"audio":{"input":{` + input + `},"output":{"format":{"type":"pcm","sample_rate":24000}}}}}`
			got, err := a.Server([]byte(ack))
			require.NoError(t, err)
			require.Equal(t, ack, string(got))
			mixed := `{"type":"session.updated","session":{"input_audio_format":"pcm","output_audio_format":"pcm","audio":{"input":{"turn_detection":null}}}}`
			got, err = a.Server([]byte(mixed))
			require.NoError(t, err)
			require.Equal(t, mixed, string(got))

			bad := `{"type":"session.updated","session":{"input_audio_format":"pcm","output_audio_format":"pcm","audio":{"input":{"format":{"type":"pcm","sample_rate":8000}}}}}`
			_, err = a.Server([]byte(bad))
			require.Error(t, err, "explicit nested PCM format must still be validated")
		})
	}
}
