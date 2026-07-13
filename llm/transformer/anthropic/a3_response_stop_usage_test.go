package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// A3: same-protocol public seam must preserve matched stop_sequence, structured
// stop_details, and baseline-evidenced future usage detail on non-stream response.
func TestA3_StopSequenceAndStopDetails_SameProtocolRoundTrip(t *testing.T) {
	body := []byte(`{
		"id": "msg_stop_1",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-5-sonnet-20241022",
		"content": [{"type": "text", "text": "stopped here"}],
		"stop_reason": "stop_sequence",
		"stop_sequence": "###END###",
		"stop_details": {"type": "stop_sequence", "stop_sequence": "###END###", "future_detail": {"k": 1}},
		"usage": {
			"input_tokens": 11,
			"output_tokens": 3,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens": 0,
			"server_tool_use": {"web_search_requests": 2},
			"future_usage_counter": 9
		}
	}`)
	sourceStopDetails := json.RawMessage(`{"type":"stop_sequence","stop_sequence":"###END###","future_detail":{"k":1}}`)
	sourceUsageRaw := json.RawMessage(`{
		"input_tokens": 11,
		"output_tokens": 3,
		"cache_creation_input_tokens": 0,
		"cache_read_input_tokens": 0,
		"server_tool_use": {"web_search_requests": 2},
		"future_usage_counter": 9
	}`)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)

	require.NotNil(t, llmResp.ProviderExtensions)
	require.NotNil(t, llmResp.ProviderExtensions.Anthropic)
	require.NotNil(t, llmResp.ProviderExtensions.Anthropic.Response)
	native := llmResp.ProviderExtensions.Anthropic.Response
	require.NotNil(t, native.StopSequence)
	require.Equal(t, "###END###", *native.StopSequence)
	require.JSONEq(t, string(sourceStopDetails), string(native.StopDetails))
	require.JSONEq(t, string(sourceUsageRaw), string(native.RawUsage))

	if len(llmResp.Choices) > 0 && llmResp.Choices[0].TransformerMetadata != nil {
		_, hasStopSequenceMetadata := llmResp.Choices[0].TransformerMetadata[TransformerMetadataKeyAnthropicStopSequence]
		_, hasStopDetailsMetadata := llmResp.Choices[0].TransformerMetadata["anthropic_stop_details"]
		require.False(t, hasStopSequenceMetadata, "non-stream stop_sequence must be owned by ProviderExtensions")
		require.False(t, hasStopDetailsMetadata, "non-stream stop_details must be owned by ProviderExtensions")
	}
	if llmResp.TransformerMetadata != nil {
		_, hasUsageMetadata := llmResp.TransformerMetadata["anthropic_usage_raw"]
		require.False(t, hasUsageMetadata, "non-stream raw usage must be owned by ProviderExtensions")
	}

	cloned := llm.CloneProviderExtensions(llmResp.ProviderExtensions)
	require.NotNil(t, cloned)
	require.NotNil(t, cloned.Anthropic)
	require.NotNil(t, cloned.Anthropic.Response)
	require.NotSame(t, native.StopSequence, cloned.Anthropic.Response.StopSequence)
	*cloned.Anthropic.Response.StopSequence = "changed"
	cloned.Anthropic.Response.StopDetails[2] = 'X'
	cloned.Anthropic.Response.RawUsage[2] = 'X'
	require.Equal(t, "###END###", *native.StopSequence)
	require.JSONEq(t, string(sourceStopDetails), string(native.StopDetails))
	require.JSONEq(t, string(sourceUsageRaw), string(native.RawUsage))

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var source, out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &source))
	require.NoError(t, json.Unmarshal(httpResp.Body, &out))

	require.JSONEq(t, string(source["stop_reason"]), string(out["stop_reason"]))
	require.JSONEq(t, string(source["stop_sequence"]), string(out["stop_sequence"]),
		"matched stop_sequence string must round-trip")
	require.JSONEq(t, string(source["stop_details"]), string(out["stop_details"]),
		"structured stop_details must round-trip including unknown nested keys")

	var sourceUsage, outUsage map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(source["usage"], &sourceUsage))
	require.NoError(t, json.Unmarshal(out["usage"], &outUsage))
	require.JSONEq(t, string(sourceUsage["input_tokens"]), string(outUsage["input_tokens"]))
	require.JSONEq(t, string(sourceUsage["output_tokens"]), string(outUsage["output_tokens"]))
	require.JSONEq(t, string(sourceUsage["server_tool_use"]), string(outUsage["server_tool_use"]),
		"baseline-evidenced usage detail must be preserved")
	require.JSONEq(t, string(sourceUsage["future_usage_counter"]), string(outUsage["future_usage_counter"]),
		"unknown future usage children must be preserved on same-protocol replay")
}

func TestA3_OmittedStopSequenceAndDetailsRemainOmitted(t *testing.T) {
	body := []byte(`{
		"id": "msg_stop_2",
		"type": "message",
		"role": "assistant",
		"model": "claude-3-5-sonnet-20241022",
		"content": [{"type": "text", "text": "done"}],
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 2, "output_tokens": 1}
	}`)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	httpResp, err := inbound.TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(httpResp.Body, &out))
	_, hasStopSequence := out["stop_sequence"]
	_, hasStopDetails := out["stop_details"]
	require.False(t, hasStopSequence, "omitted stop_sequence must remain omitted")
	require.False(t, hasStopDetails, "omitted stop_details must remain omitted")
}
