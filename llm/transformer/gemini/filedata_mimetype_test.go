package gemini

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
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
						"fileUri": "gs://example-bucket/image.png"
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
