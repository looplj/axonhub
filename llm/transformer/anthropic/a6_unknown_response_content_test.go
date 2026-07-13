package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestA6_UnknownResponseContentBlock_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"id":"msg_future_block",
		"type":"message",
		"role":"assistant",
		"model":"claude-future",
		"content":[
			{"type":"text","text":"before"},
			{"type":"future_result","id":"future_1","payload":{"keep":true,"items":[1,2]}},
			{"type":"text","text":"after"}
		],
		"stop_reason":"end_turn",
		"stop_sequence":null,
		"usage":{"input_tokens":2,"output_tokens":3}
	}`)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	canonical, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)

	require.NotNil(t, canonical.ProviderExtensions)
	require.NotNil(t, canonical.ProviderExtensions.Anthropic)
	require.NotNil(t, canonical.ProviderExtensions.Anthropic.Response)
	native := canonical.ProviderExtensions.Anthropic.Response
	require.Len(t, native.RawContent, 3, "full Anthropic response content must live on ProviderExtensions.Response.RawContent")
	require.JSONEq(t, `{"type":"future_result","id":"future_1","payload":{"keep":true,"items":[1,2]}}`, string(native.RawContent[1]))

	if canonical.TransformerMetadata != nil {
		_, hasLegacy := canonical.TransformerMetadata["anthropic_response_content"]
		require.False(t, hasLegacy, "canonical response must not store full response content in TransformerMetadata")
	}

	cloned := llm.CloneProviderExtensions(canonical.ProviderExtensions)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.Anthropic)
	require.NotNil(t, cloned.Anthropic.Response)
	require.Len(t, cloned.Anthropic.Response.RawContent, 3)
	require.JSONEq(t, string(native.RawContent[1]), string(cloned.Anthropic.Response.RawContent[1]))
	cloned.Anthropic.Response.RawContent[1][2] = 'X'
	require.NotEqual(t, string(native.RawContent[1]), string(cloned.Anthropic.Response.RawContent[1]),
		"RawContent clone must be independent of source")
	require.JSONEq(t, `{"type":"future_result","id":"future_1","payload":{"keep":true,"items":[1,2]}}`, string(native.RawContent[1]))

	inbound := NewInboundTransformer()
	replayed, err := inbound.TransformResponse(t.Context(), canonical)
	require.NoError(t, err)

	var source, got Message
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(replayed.Body, &got))
	require.Len(t, got.Content, 3, "unknown response content block must not be dropped")
	require.JSONEq(t, string(source.Content[1].Raw), string(got.Content[1].Raw))
}
