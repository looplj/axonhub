package responses_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestImageEditRoundTripReturnsImagesResponse(t *testing.T) {
	outbound, err := responses.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	providerReq, err := outbound.TransformRequest(t.Context(), &llm.Request{
		Model:       "gpt-image-1",
		RequestType: llm.RequestTypeImage,
		APIFormat:   llm.APIFormatOpenAIImageEdit,
		Image: &llm.ImageRequest{
			Prompt:       "edit this image",
			Images:       [][]byte{[]byte("source-image")},
			OutputFormat: "webp",
		},
	})
	require.NoError(t, err)
	require.Equal(t, llm.RequestTypeImage.String(), providerReq.RequestType)
	require.Equal(t, llm.APIFormatOpenAIResponse.String(), providerReq.APIFormat)

	llmResp, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Request:    providerReq,
		Body: []byte(`{
			"id": "resp_image_123",
			"object": "response",
			"created_at": 1759161016,
			"status": "completed",
			"model": "gpt-image-1",
			"output": [{
				"id": "img_123",
				"type": "image_generation_call",
				"status": "completed",
				"result": "data:image/webp;base64,aW1hZ2UtZGF0YQ=="
			}]
		}`),
	})
	require.NoError(t, err)

	clientResp, err := openai.NewImageEditInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, clientResp.StatusCode)

	var body map[string]any
	require.NoError(t, json.Unmarshal(clientResp.Body, &body))
	require.Contains(t, body, "created")
	require.Contains(t, body, "data")
	require.NotContains(t, body, "id")
	require.NotContains(t, body, "object")
	require.NotContains(t, body, "output")
	require.NotContains(t, body, "tools")

	data, ok := body["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 1)
	image, ok := data[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "aW1hZ2UtZGF0YQ==", image["b64_json"])
}
