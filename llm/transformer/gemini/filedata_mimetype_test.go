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

func TestGeminiToVertexFileData_preservesMediaMIMETypes(t *testing.T) {
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
				"parts": [
					{"fileData": {"mimeType": "image/png", "fileUri": "gs://example-bucket/image.jpg"}},
					{"fileData": {"mimeType": "video/quicktime", "fileUri": "gs://example-bucket/video.mp4"}},
					{"fileData": {"mimeType": "application/pdf", "fileUri": "gs://example-bucket/document.pdf"}},
					{"fileData": {"mimeType": "audio/mpeg", "fileUri": "gs://example-bucket/audio.mp3"}}
				]
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
	require.Len(t, got.Contents[0].Parts, 4)
	require.Equal(t, "image/png", got.Contents[0].Parts[0].FileData.MIMEType)
	require.Equal(t, "video/quicktime", got.Contents[0].Parts[1].FileData.MIMEType)
	require.Equal(t, "application/pdf", got.Contents[0].Parts[2].FileData.MIMEType)
	require.Equal(t, "audio/mpeg", got.Contents[0].Parts[3].FileData.MIMEType)
}

func TestGeminiToVertexFileData_infersMIMEWhenDeclaredTypeIsUnsupported(t *testing.T) {
	inbound := NewInboundTransformer()
	outbound, err := NewOutboundTransformerWithConfig(Config{
		BaseURL:      "https://us-central1-aiplatform.googleapis.com",
		PlatformType: PlatformVertex,
	})
	require.NoError(t, err)

	inboundRequest := &httpclient.Request{
		Path: "/v1beta/models/gemini-test-model:generateContent",
		Body: []byte(`{
			"contents": [{
				"role": "user",
				"parts": [{
					"fileData": {
						"mimeType": "application/octet-stream",
						"fileUri": "https://assets.example.com/audio/input.mp3"
					}
				}]
			}]
		}`),
	}

	llmRequest, err := inbound.TransformRequest(t.Context(), inboundRequest)
	require.NoError(t, err)
	vertexRequest, err := outbound.TransformRequest(t.Context(), llmRequest)
	require.NoError(t, err)

	var got GenerateContentRequest
	require.NoError(t, json.Unmarshal(vertexRequest.Body, &got))
	require.Len(t, got.Contents, 1)
	require.Len(t, got.Contents[0].Parts, 1)
	require.NotNil(t, got.Contents[0].Parts[0].FileData)
	require.Equal(t, "audio/mpeg", got.Contents[0].Parts[0].FileData.MIMEType)
}

func TestImageMIMEType_leavesUnknownURLTypesEmpty(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "extensionless", url: "https://assets.example.com/images/input"},
		{name: "unknown extension", url: "https://assets.example.com/images/input.axonhubtest"},
		{name: "document extension", url: "https://assets.example.com/images/input.pdf"},
		{name: "video extension", url: "https://assets.example.com/images/input.mp4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := &llm.ImageURL{URL: tt.url}

			require.Empty(t, imageMIMEType(image))
		})
	}
}

func TestMediaMIMEType(t *testing.T) {
	tests := []struct {
		name         string
		explicitMIME string
		url          string
		accepts      func(string) bool
		expected     string
	}{
		{
			name:         "explicit MIME takes precedence",
			explicitMIME: "video/quicktime",
			url:          "https://assets.example.com/video/input.mp4",
			accepts:      isVideoMIMEType,
			expected:     "video/quicktime",
		},
		{
			name:     "video from URL",
			url:      "https://assets.example.com/video/input.mp4?token=test",
			accepts:  isVideoMIMEType,
			expected: "video/mp4",
		},
		{
			name:     "Word document from URL",
			url:      "https://assets.example.com/documents/input.docx?token=test",
			accepts:  isDocumentMIMEType,
			expected: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		{
			name:     "Excel document from URL",
			url:      "https://assets.example.com/documents/input.xlsx?token=test",
			accepts:  isDocumentMIMEType,
			expected: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		},
		{
			name:     "PowerPoint document from URL",
			url:      "https://assets.example.com/documents/input.pptx?token=test",
			accepts:  isDocumentMIMEType,
			expected: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		},
		{
			name:     "audio from URL",
			url:      "https://assets.example.com/audio/input.mp3?token=test",
			accepts:  isAudioMIMEType,
			expected: "audio/mpeg",
		},
		{
			name:     "modality mismatch",
			url:      "https://assets.example.com/documents/input.pdf",
			accepts:  isVideoMIMEType,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, mediaMIMEType(tt.explicitMIME, tt.url, tt.accepts))
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
