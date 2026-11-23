// 职责: 验证 OpenAI 原生实时事件保真、安全边界与分模态缓存用量。
// 关联文件:
//
//	llm/transformer/openai/realtime/native.go
package realtime

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNativePreservesGAFieldsAndBytes(t *testing.T) {
	for _, model := range []string{"gpt-realtime-2.1", "gpt-realtime-2.1-mini"} {
		t.Run(model, func(t *testing.T) {
			a, err := NewAdapter(model)
			require.NoError(t, err)
			for _, data := range []string{`{ "type":"session.update", "session":{"type":"realtime","new_official_field":{"x":1},"audio":{"output":{"voice":"marin"}}}}`, `{"type":"future.valid.event","event_id":"e1","extension":true}`} {
				got, err := a.Client([]byte(data))
				require.NoError(t, err)
				require.Equal(t, data, string(got))
			}
			data := []byte(`{ "type":"response.output_text.delta", "response_id":"r1", "delta":"hello", "future_field":true }`)
			got, err := a.Server(data)
			require.NoError(t, err)
			require.Equal(t, data, got)
		})
	}
}
func TestNativeRejectsSecurityControlFields(t *testing.T) {
	a, _ := NewAdapter("gpt-realtime-2.1-mini")
	for _, data := range []string{`{"type":"session.update","session":{"type":"realtime","model":"other"}}`, `{"type":"response.create","response":{"model":"other"}}`, `{"type":"session.update","session":{"type":"transcription"}}`, `{"type":"input_audio_buffer.append","audio":"not!base64"}`, `{"type":"session.update","session":{"type":"realtime","tools":[{"type":"mcp","server_url":"http://127.0.0.1"}]}}`} {
		_, err := a.Client([]byte(data))
		require.Error(t, err, data)
	}
}
func TestNativeUsageModalitiesAndCachedSubsets(t *testing.T) {
	data := []byte(`{"type":"response.done","response":{"id":"r1","usage":{"input_tokens":100,"output_tokens":30,"total_tokens":130,"input_token_details":{"text_tokens":40,"audio_tokens":50,"image_tokens":10,"cached_tokens":30,"cached_tokens_details":{"text_tokens":10,"audio_tokens":15,"image_tokens":5}},"output_token_details":{"text_tokens":10,"audio_tokens":20}}}}`)
	id, u, err := ParseUsage(data)
	require.NoError(t, err)
	require.Equal(t, "r1", id)
	require.EqualValues(t, 100, u.PromptTokens)
	require.EqualValues(t, 50, u.PromptTokensDetails.AudioTokens)
	require.EqualValues(t, 30, u.PromptTokensDetails.CachedTokens)
	require.EqualValues(t, 15, u.Realtime.CachedInputAudioTokens)
	require.EqualValues(t, 10, u.Realtime.InputImageTokens)
	for _, bad := range []string{`{"type":"response.done","response":{"id":"r1"}}`, `{"type":"response.done","response":{"id":"r1","usage":{"input_tokens":-1,"output_tokens":0,"total_tokens":-1}}}`, `{"type":"response.done","response":{"id":"r1","usage":{"input_tokens":1,"output_tokens":0,"total_tokens":1,"input_token_details":{"text_tokens":1,"audio_tokens":1},"output_token_details":{"text_tokens":0,"audio_tokens":0}}}}`} {
		_, _, err := ParseUsage([]byte(bad))
		require.Error(t, err)
	}
}
func TestNativeEndpointAllowsWebSocketHosts(t *testing.T) {
	target, err := EndpointURL("wss://api.openai.com", "/v1/realtime", "gpt-realtime-2.1")
	require.NoError(t, err)
	require.Equal(t, "wss://api.openai.com/v1/realtime?model=gpt-realtime-2.1", target)
	target, err = EndpointURL("wss://proxy.example.test", "/v1/realtime", "gpt-realtime-2.1")
	require.NoError(t, err)
	require.Equal(t, "wss://proxy.example.test/v1/realtime?model=gpt-realtime-2.1", target)
	for _, bad := range []string{"http://api.openai.com", "ws://api.openai.com", "wss://", "wss://127.0.0.1", "wss://user@api.openai.com", "wss://api.openai.com?x=1", "wss://api.openai.com/custom"} {
		_, err := EndpointURL(bad, "", "gpt-realtime-2.1")
		require.Error(t, err)
	}
}

func TestNativeAgentsSessionUpdate(t *testing.T) {
	a, err := NewAdapter("gpt-realtime-2.1-mini")
	require.NoError(t, err)
	a.ClientModel = "voice"
	for _, tc := range []struct {
		name, session, want string
		wantErr             error
	}{
		{"SDK defaults", `{"type":"realtime","model":"voice","audio":{"input":{"transcription":{"model":"gpt-4o-mini-transcribe"},"turn_detection":{"type":"semantic_vad"}}},"extension":true}`, `{"type":"session.update","session":{"type":"realtime","audio":{"input":{"turn_detection":{"type":"semantic_vad"}}},"extension":true}}`, nil},
		{"transcription only", `{"type":"realtime","audio":{"input":{"transcription":{"model":"gpt-4o-mini-transcribe"}}}}`, `{"type":"session.update","session":{"type":"realtime"}}`, nil},
		{"model switch", `{"type":"realtime","model":"gpt-realtime-2.1"}`, "", ErrModelSwitch},
		{"missing type", `{"model":"voice"}`, "", ErrSessionType},
		{"legacy format", `{"type":"realtime","input_audio_format":"pcm16"}`, "", ErrLegacySessionFields},
		{"legacy transcription", `{"type":"realtime","input_audio_transcription":{"model":"gpt-4o-transcribe"}}`, `{"type":"session.update","session":{"type":"realtime"}}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := a.Client([]byte(`{"type":"session.update","session":` + tc.session + `}`))
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tc.want, string(wire))
		})
	}
	for _, field := range []string{"authorization", "api_key", "safety_identifier"} {
		_, err := a.Client([]byte(`{"type":"session.update","session":{"type":"realtime","` + field + `":"synthetic"}}`))
		require.Error(t, err)
	}
}

func TestNativeAdapterDoesNotSilentlyRewriteErrorBytes(t *testing.T) {
	a, err := NewAdapter("gpt-realtime-2.1-mini")
	require.NoError(t, err)
	for _, raw := range []string{`{ "type":"error", "event_id":"e1", "error":{"type":"invalid_request_error","message":"details"}, "extension":true }`, `{ "type":"response.done", "response":{"id":"r1","status":"failed","status_details":{"error":{"message":"details"}}}}`} {
		actual, err := a.Server([]byte(raw))
		require.NoError(t, err)
		require.Equal(t, raw, string(actual))
	}
}
