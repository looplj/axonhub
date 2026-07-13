package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestA8_AnthropicCitationNativeDetails_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"msg_citation_native",
		"type":"message",
		"role":"assistant",
		"model":"claude-future",
		"content":[{"type":"text","text":"answer","citations":[{
			"type":"char_location",
			"encrypted_index":"enc-index",
			"cited_text":"original quote",
			"future_citation_detail":{"keep":true}
		}]}],
		"stop_reason":"end_turn",
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

	var source, got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(replayed.Body, &got))
	require.JSONEq(t, string(source["content"]), string(got["content"]))
}
