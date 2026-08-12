package gemini

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestGeminiToVertexFileData_preservesMIMEType(t *testing.T) {
	// Given
	inbound := NewInboundTransformer()
	outbound, err := NewOutboundTransformerWithConfig(Config{
		BaseURL:      "https://us-central1-aiplatform.googleapis.com",
		PlatformType: PlatformVertex,
	})
	require.NoError(t, err)

	inboundRequest := &httpclient.Request{
		Path: "/v1beta/models/gemini-2.5-flash:generateContent",
		Body: []byte(`{
			"contents": [{
				"role": "user",
				"parts": [{
					"fileData": {
						"mimeType": "image/png",
						"fileUri": "gs://example-bucket/image.jpg"
					}
				}]
			}]
		}`),
	}

	// When
	llmRequest, err := inbound.TransformRequest(t.Context(), inboundRequest)
	require.NoError(t, err)
	vertexRequest, err := outbound.TransformRequest(t.Context(), llmRequest)
	require.NoError(t, err)

	var got GenerateContentRequest
	require.NoError(t, json.Unmarshal(vertexRequest.Body, &got))

	// Then
	require.Len(t, got.Contents, 1)
	require.Len(t, got.Contents[0].Parts, 1)
	require.NotNil(t, got.Contents[0].Parts[0].FileData)
	require.Equal(t, "image/png", got.Contents[0].Parts[0].FileData.MIMEType)
}

func TestImageMIMEType_leavesUnknownURLTypesEmpty(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "extensionless", url: "https://assets.example.com/images/input"},
		{name: "unknown extension", url: "https://assets.example.com/images/input.axonhubtest"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := &llm.ImageURL{URL: tt.url}

			require.Empty(t, imageMIMEType(image))
		})
	}
}

func TestOpenAIImageURLToVertexFileData_infersMIMETypeFromURLPath(t *testing.T) {
	// Given
	inbound := openai.NewInboundTransformer()
	outbound, err := NewOutboundTransformerWithConfig(Config{
		BaseURL:      "https://aiplatform.googleapis.com/v1",
		PlatformType: PlatformVertex,
	})
	require.NoError(t, err)

	inboundRequest := &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gemini-test-model",
			"messages": [{
				"role": "user",
				"content": [{
					"type": "image_url",
					"image_url": {
						"url": "https://assets.example.com/images/input.jpg?token=test"
					}
				}]
			}]
		}`),
	}

	// When
	llmRequest, err := inbound.TransformRequest(t.Context(), inboundRequest)
	require.NoError(t, err)
	vertexRequest, err := outbound.TransformRequest(t.Context(), llmRequest)
	require.NoError(t, err)

	var got GenerateContentRequest
	require.NoError(t, json.Unmarshal(vertexRequest.Body, &got))

	// Then
	require.Len(t, got.Contents, 1)
	require.Len(t, got.Contents[0].Parts, 1)
	require.NotNil(t, got.Contents[0].Parts[0].FileData)
	require.Equal(t, "image/jpeg", got.Contents[0].Parts[0].FileData.MIMEType)
}
