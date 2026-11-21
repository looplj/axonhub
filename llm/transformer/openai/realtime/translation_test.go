// 职责: 验证 OpenAI 实时翻译的模型隔离、配置确认和音频关闭流程。
// 关联文件:
//   llm/transformer/openai/realtime/translation.go

package realtime

import (
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
	_, err = TranslationEndpointURL("wss://untrusted.example", "", TranslationModel)
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
