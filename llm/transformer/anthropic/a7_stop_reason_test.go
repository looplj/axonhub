package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestA7_AnthropicNativeStopReasons_SameProtocolRoundTrip(t *testing.T) {
	for _, stopReason := range []string{"pause_turn", "refusal"} {
		t.Run(stopReason, func(t *testing.T) {
			body := []byte(`{
				"id":"msg_stop_reason",
				"type":"message",
				"role":"assistant",
				"model":"claude-future",
				"content":[{"type":"text","text":"done"}],
				"stop_reason":` + string(mustJSON(t, stopReason)) + `,
				"stop_sequence":null,
				"usage":{"input_tokens":1,"output_tokens":1}
			}`)

			outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
			require.NoError(t, err)
			canonical, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			})
			require.NoError(t, err)

			inbound := NewInboundTransformer()
			replayed, err := inbound.TransformResponse(t.Context(), canonical)
			require.NoError(t, err)

			var got Message
			require.NoError(t, json.Unmarshal(replayed.Body, &got))
			require.NotNil(t, got.StopReason)
			require.Equal(t, stopReason, *got.StopReason)
		})
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
