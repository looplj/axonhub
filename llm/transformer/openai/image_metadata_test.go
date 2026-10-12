package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestImageResponseMetadataRoundTrip(t *testing.T) {
	for _, fixture := range []struct{ name, body string }{
		{"native", `{"created":1791771243,"data":[{"b64_json":"c3ludGhldGlj","generation_id":"synthetic-generation","revised_prompt":"synthetic"},{"url":"https://example.invalid/image","generation_id":"synthetic-url"}],"safety_identifier":null,"background":"opaque","output_format":"png","quality":"low","size":"1254x1254","usage":{"input_tokens":26,"output_tokens":515,"total_tokens":541,"input_tokens_details":{"image_tokens":10,"text_tokens":16,"cached_tokens":4,"cached_tokens_details":{"image_tokens":3,"text_tokens":1}},"output_tokens_details":{"image_tokens":515,"text_tokens":0}}}`},
		{"identifier", `{"created":7,"data":[{"b64_json":"c3ludGhldGlj"}],"safety_identifier":"synthetic-safety","usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}`},
		{"omitted", `{"created":7,"data":[{"url":"https://example.invalid/image"}]}`},
		{"zero_cached", `{"created":7,"data":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0,"input_tokens_details":{"image_tokens":0,"text_tokens":0,"cached_tokens":0},"output_tokens_details":{"image_tokens":0,"text_tokens":0}}}`},
		{"zero_reasoning", `{"created":7,"data":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0,"output_tokens_details":{"reasoning_tokens":0}}}`},
		{"omitted_reasoning", `{"created":7,"data":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0,"output_tokens_details":{}}}`},
		{"nonzero_reasoning", `{"created":7,"data":[],"usage":{"input_tokens":0,"output_tokens":1,"total_tokens":1,"output_tokens_details":{"reasoning_tokens":1}}}`},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			// Given a native JSON image response with independently specified metadata.
			outbound, err := NewOutboundTransformer("https://example.invalid/v1", "synthetic-key")
			require.NoError(t, err)
			// When it traverses the native outbound and inbound response transformers.
			response, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
				StatusCode: http.StatusOK, Body: []byte(fixture.body),
				Request: &httpclient.Request{RequestType: "image", APIFormat: string(llm.APIFormatOpenAIImageGeneration)},
			})
			require.NoError(t, err)
			stored, err := json.Marshal(response)
			require.NoError(t, err)
			var restored llm.Response
			require.NoError(t, json.Unmarshal(stored, &restored))
			encoded, err := NewImageGenerationInboundTransformer().TransformResponse(context.Background(), &restored)
			require.NoError(t, err)
			// Then every native field survives, including zero details and null/omission.
			require.JSONEq(t, fixture.body, string(encoded.Body))
			var result map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded.Body, &result))
			require.NotContains(t, result, "generation_id")
		})
	}
}

func TestNativeImageUsageOverridesAccountingProjection(t *testing.T) {
	// Given native image usage and a subsequently changed generic accounting projection.
	outbound, err := NewOutboundTransformer("https://example.invalid/v1", "synthetic-key")
	require.NoError(t, err)
	response, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"created":7,"data":[],"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30,"output_tokens_details":{"image_tokens":20,"text_tokens":0},"cost":0.25}}`),
		Request:    &httpclient.Request{APIFormat: string(llm.APIFormatOpenAIImageGeneration)},
	})
	require.NoError(t, err)
	response.Usage = &llm.Usage{PromptTokens: 900, CompletionTokens: 800, TotalTokens: 1700, Cost: lo.ToPtr(0.5)}
	// When returning the image JSON response after calculated-cost injection.
	encoded, err := NewImageGenerationInboundTransformer().TransformResponse(context.Background(), response)
	require.NoError(t, err)
	// Then native image counts/details win, while calculated cost is overlaid without mutation.
	require.JSONEq(t, `{"created":7,"data":[],"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30,"output_tokens_details":{"image_tokens":20,"text_tokens":0},"cost":0.5}}`, string(encoded.Body))
	require.Equal(t, 0.25, *response.Image.Usage.Cost)
}

func TestNativeImageProviderCharacterization(t *testing.T) {
	// Given the existing native OpenAI provider and legacy image JSON response.
	outbound, err := NewOutboundTransformer("https://example.invalid/v1", "synthetic-key")
	require.NoError(t, err)
	body := `{"created":123,"data":[{"url":"https://example.invalid/image","revised_prompt":"synthetic"}]}`
	// When the response crosses both public transformer surfaces.
	response, err := outbound.TransformResponse(context.Background(), &httpclient.Response{
		StatusCode: http.StatusOK, Body: []byte(body), Request: &httpclient.Request{RequestType: "image", APIFormat: string(llm.APIFormatOpenAIImageGeneration)},
	})
	require.NoError(t, err)
	encoded, err := NewImageGenerationInboundTransformer().TransformResponse(context.Background(), response)
	require.NoError(t, err)
	// Then the established native JSON result is preserved.
	require.JSONEq(t, body, string(encoded.Body))
}
