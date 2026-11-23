// 职责: 验证 OpenAI 实时翻译的模型隔离、配置确认和音频关闭流程。
// 关联文件:
//   llm/transformer/openai/realtime/translation.go

package realtime

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTranslationEndpointAndEvents(t *testing.T) {
	target, err := TranslationEndpointURL("wss://api.openai.com", "/v1/realtime", TranslationModel)
	require.NoError(t, err)
	require.Equal(t, "wss://api.openai.com/v1/realtime/translations?model=gpt-realtime-translate", target)
	_, err = TranslationEndpointURL("wss://api.openai.com", "", "gpt-realtime-2.1")
	require.Error(t, err)
	_, err = EndpointURL("", "", TranslationModel)
	require.Error(t, err)
	proxyTarget, err := TranslationEndpointURL("wss://proxy.example.test", "", TranslationModel)
	require.NoError(t, err)
	require.Equal(t, "wss://proxy.example.test/v1/realtime/translations?model=gpt-realtime-translate", proxyTarget)
	_, err = TranslationEndpointURL("ws://proxy.example.test", "", TranslationModel)
	require.Error(t, err)
	adapter := &Translation{}
	created := []byte(`{"type":"session.created","session":{"type":"translation","model":"gpt-realtime-translate"}}`)
	wire, err := adapter.Server(created)
	require.NoError(t, err)
	require.Equal(t, created, wire)

	audio := []byte(`{"type":"session.input_audio_buffer.append","audio":"AAA="}`)
	_, err = adapter.Client(audio)
	require.Error(t, err)
	_, err = adapter.Client([]byte(`{"type":"session.update","session":{"model":"other"}}`))
	require.Error(t, err)
	settings := []byte(`{"type":"session.update","session":{"audio":{"output":{"language":"es"}}}}`)
	wire, err = adapter.Client(settings)
	require.NoError(t, err)
	id := gjson.GetBytes(wire, "event_id").String()
	require.NotEmpty(t, id)
	_, err = adapter.Server([]byte(`{"type":"error","error":{"event_id":"unrelated"}}`))
	require.NoError(t, err)
	_, err = adapter.Client(settings)
	require.Error(t, err)
	_, err = adapter.Server([]byte(`{"type":"error","error":{"event_id":"` + id + `"}}`))
	require.NoError(t, err)
	_, err = adapter.Client(settings)
	require.NoError(t, err)
	_, err = adapter.Server([]byte(`{"type":"session.updated","session":{"type":"translation","model":"gpt-realtime-translate","audio":{"output":{"language":"es"}}}}`))
	require.NoError(t, err)
	_, err = adapter.Client([]byte(`{"type":"session.input_audio_buffer.append","audio":"AA=="}`))
	require.Error(t, err)
	wire, err = adapter.Client(audio)
	require.NoError(t, err)
	require.JSONEq(t, string(audio), string(wire))
	wire, err = adapter.Client([]byte(`{"type":"session.close","type":"session.input_audio_buffer.append","audio":"AAA="}`))
	require.NoError(t, err)
	require.JSONEq(t, string(audio), string(wire))
	_, err = adapter.Client([]byte(`{"type":"session.close"}`))
	require.NoError(t, err)
	_, err = adapter.Client(audio)
	require.Error(t, err)
	_, err = adapter.Server([]byte(`{"type":"session.output_audio.delta","delta":"AAA="}`))
	require.NoError(t, err)
	_, err = adapter.Server([]byte(`{"type":"session.closed"}`))
	require.NoError(t, err)
	_, err = adapter.Server([]byte(`{"type":"session.closed"}`))
	require.Error(t, err)
}

func TestTranslationSessionUpdateStripsKnownUnsupportedFields(t *testing.T) {
	adapter := &Translation{}
	_, err := adapter.Server([]byte(`{"type":"session.created","session":{"type":"translation","model":"gpt-realtime-translate"}}`))
	require.NoError(t, err)

	previousLogger := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	input := []byte(`{"type":"session.update","event_id":"update-1","session":{"output_modalities":["audio"],"modalities":["audio"],"instructions":"private instruction value","audio":{"input":{"format":{"type":"audio/pcm","rate":24000}},"output":{"language":"en"}}}}`)
	wire, err := adapter.Client(input)
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"session.update","event_id":"update-1","session":{"audio":{"output":{"language":"en"}}}}`, string(wire))
	require.Contains(t, logs.String(), "WARN")
	for _, field := range []string{"output_modalities", "modalities", "instructions", "audio.input.format"} {
		require.Contains(t, logs.String(), field)
	}
	require.NotContains(t, logs.String(), "private instruction value")
	require.NotContains(t, logs.String(), "audio/pcm")
	require.NotContains(t, logs.String(), "24000")

	_, err = adapter.Server([]byte(`{"type":"session.updated","session":{"type":"translation","model":"gpt-realtime-translate","audio":{"output":{"language":"en"}}}}`))
	require.NoError(t, err)
	audio := []byte(`{"type":"session.input_audio_buffer.append","audio":"AAAAAA=="}`)
	forwarded, err := adapter.Client(audio)
	require.NoError(t, err)
	require.JSONEq(t, string(audio), string(forwarded))
	samples, err := TranslationAudioSamples(forwarded)
	require.NoError(t, err)
	require.EqualValues(t, 2, samples)
}

func TestTranslationSessionUpdateCanBecomeEmptyAfterStripping(t *testing.T) {
	adapter := &Translation{}
	_, err := adapter.Server([]byte(`{"type":"session.created","session":{"type":"translation","model":"gpt-realtime-translate"}}`))
	require.NoError(t, err)
	wire, err := adapter.Client([]byte(`{"type":"session.update","session":{"modalities":["audio"],"instructions":"remove me"}}`))
	require.NoError(t, err)
	require.Equal(t, `{}`, gjson.GetBytes(wire, "session").Raw)
}

func TestTranslationSessionTypeAndModelRemainRejected(t *testing.T) {
	adapter := &Translation{}
	_, err := adapter.Server([]byte(`{"type":"session.created","session":{"type":"translation","model":"gpt-realtime-translate"}}`))
	require.NoError(t, err)
	for _, session := range []string{
		`{"type":"translation"}`,
		`{"model":"gpt-realtime-translate"}`,
	} {
		_, err := adapter.Client([]byte(`{"type":"session.update","session":` + session + `}`))
		require.ErrorIs(t, err, ErrTranslationSessionFields)
	}
}
