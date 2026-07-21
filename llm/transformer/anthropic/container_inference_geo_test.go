package anthropic

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	openai "github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestAnthropicContainerAndInferenceGeoSameProtocolRoundTrip(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-container-inference-geo.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	require.NotNil(t, llmReq.ProviderExtensions)
	require.NotNil(t, llmReq.ProviderExtensions.Anthropic)
	require.NotNil(t, llmReq.ProviderExtensions.Anthropic.Request)
	container := llmReq.ProviderExtensions.Anthropic.Request.Container
	require.NotEmpty(t, container)
	require.JSONEq(t, `{"id":"container_123","skills":[{"type":"custom","skill_id":"skill_abc"}],"future_nested":{"enabled":true,"note":"unknown-field"}}`, string(container))

	geo := llmReq.ProviderExtensions.Anthropic.Request.InferenceGeo
	require.NotEmpty(t, geo)
	require.JSONEq(t, `"us"`, string(geo))

	// Must not widen the common request model.
	_, hasContainer := llmReq.TransformerMetadata["container"]
	require.False(t, hasContainer)

	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstream.Body, &outboundBody))
	require.JSONEq(t, string(container), string(outboundBody["container"]))
	require.JSONEq(t, string(geo), string(outboundBody["inference_geo"]))
}

func TestAnthropicContainerAndInferenceGeoNotSynthesizedToOpenAIChat(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-container-inference-geo.request.json")
	require.NoError(t, err)

	inbound := NewInboundTransformer()
	llmReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)

	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	upstream, err := outbound.TransformRequest(t.Context(), llmReq)
	require.NoError(t, err)

	var outboundBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstream.Body, &outboundBody))
	require.NotContains(t, outboundBody, "container")
	require.NotContains(t, outboundBody, "inference_geo")
}

func TestAnthropicContainerAndInferenceGeoOmittedWhenAbsent(t *testing.T) {
	chatReq := &llm.Request{
		Model:     "claude-3-sonnet-20240229",
		MaxTokens: lo.ToPtr(int64(1024)),
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: lo.ToPtr("hello")},
		}},
	}
	anthropicReq, err := convertToAnthropicRequest(chatReq)
	require.NoError(t, err)
	require.Empty(t, anthropicReq.Container)
	require.Empty(t, anthropicReq.InferenceGeo)
}

func TestAnthropicContainerAndInferenceGeoDiagnosesLossyDowngradeToChatAndResponses(t *testing.T) {
	body, err := os.ReadFile("testdata/anthropic-container-inference-geo.request.json")
	require.NoError(t, err)
	inbound := NewInboundTransformer()

	requireHasLossy := func(t *testing.T, req *llm.Request, field string, target llm.APIFormat) {
		t.Helper()
		found := false
		for _, d := range llm.LossyDowngrades(req) {
			if d.SourceProtocol == llm.APIFormatAnthropicMessage &&
				d.SourceField == field &&
				d.TargetProtocol == target &&
				d.Reason == llm.LossyDowngradeReasonNoEquivalentSemantics {
				found = true
				break
			}
		}
		require.Truef(t, found, "missing LossyDowngrade for %s -> %s: %#v", field, target, llm.LossyDowngrades(req))
	}

	respLLMReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	respOutbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	respReq, err := respOutbound.TransformRequest(t.Context(), respLLMReq)
	require.NoError(t, err)
	var respBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(respReq.Body, &respBody))
	require.NotContains(t, respBody, "container")
	require.NotContains(t, respBody, "inference_geo")
	requireHasLossy(t, respLLMReq, "container", llm.APIFormatOpenAIResponse)
	requireHasLossy(t, respLLMReq, "inference_geo", llm.APIFormatOpenAIResponse)

	chatLLMReq, err := inbound.TransformRequest(t.Context(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    body,
	})
	require.NoError(t, err)
	chatOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-key")
	require.NoError(t, err)
	chatReq, err := chatOutbound.TransformRequest(t.Context(), chatLLMReq)
	require.NoError(t, err)
	var chatBody map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(chatReq.Body, &chatBody))
	require.NotContains(t, chatBody, "container")
	require.NotContains(t, chatBody, "inference_geo")
	requireHasLossy(t, chatLLMReq, "container", llm.APIFormatOpenAIChatCompletion)
	requireHasLossy(t, chatLLMReq, "inference_geo", llm.APIFormatOpenAIChatCompletion)
}
