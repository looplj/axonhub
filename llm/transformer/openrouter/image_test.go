package openrouter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
)

func TestOutboundTransformer_ImageGenerationRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *llm.Request
		wantErr bool
	}{
		{
			name: "image generation with Image field",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Generate a beautiful sunset over mountains",
				},
			},
			wantErr: false,
		},
		{
			name: "image generation with Image field and model override",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Generate a cat",
				},
			},
			wantErr: false,
		},
		{
			name: "missing model",
			request: &llm.Request{
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Generate something",
				},
			},
			wantErr: true,
		},
		{
			name: "missing Image field",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
				Image:       &llm.ImageRequest{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := openrouter.NewOutboundTransformer("https://openrouter.ai/api/v1", "test-api-key")
			require.NoError(t, err)

			req, err := transformer.TransformRequest(context.Background(), tt.request)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)

			// Verify it's a POST request to the OpenRouter image router.
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL, "/images")

			// Verify request type is set
			require.Equal(t, llm.RequestTypeImage.String(), req.RequestType)
			require.Equal(t, llm.APIFormatOpenAIImageGeneration.String(), req.APIFormat)

			// Parse body and verify the image-generation request shape.
			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			require.Equal(t, tt.request.Model, body["model"])
			require.Equal(t, tt.request.Image.Prompt, body["prompt"])

			// Verify stream is not set (must be false for image generation)
			_, hasStream := body["stream"]
			require.False(t, hasStream, "stream should not be set for image generation")

			// Verify model is saved in TransformerMetadata
			require.NotNil(t, req.TransformerMetadata)
			require.Equal(t, tt.request.Model, req.TransformerMetadata["model"])
		})
	}
}

func TestOutboundTransformer_ImageGenerationResponse(t *testing.T) {
	tests := []struct {
		name     string
		response string
		want     *llm.Response
	}{
		{
			name:     "response with data array",
			response: `{"created":1759393520,"data":[{"b64_json":"iVBORw0KGgo","media_type":"image/png"}],"usage":{"prompt_tokens":7,"completion_tokens":1290,"total_tokens":1297}}`,
			want: &llm.Response{
				ID:          "img-1759393520",
				Object:      "chat.completion",
				Created:     1759393520,
				Model:       "test-model",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageResponse{
					Created: 1759393520,
					Data: []llm.ImageData{
						{
							B64JSON: "iVBORw0KGgo",
							URL:     "data:image/png;base64,iVBORw0KGgo",
						},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     7,
					CompletionTokens: 1290,
					TotalTokens:      1297,
				},
			},
		},
		{
			name:     "response with multiple data images",
			response: `{"created":1759393520,"data":[{"b64_json":"/9j/4AAQ","media_type":"image/jpeg"},{"b64_json":"R0lGODlh","media_type":"image/gif"}],"usage":{"prompt_tokens":10,"completion_tokens":100,"total_tokens":110}}`,
			want: &llm.Response{
				ID:          "img-1759393520",
				Object:      "chat.completion",
				Created:     1759393520,
				Model:       "test-model",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageResponse{
					Created: 1759393520,
					Data: []llm.ImageData{
						{
							B64JSON: "/9j/4AAQ",
							URL:     "data:image/jpeg;base64,/9j/4AAQ",
						},
						{
							B64JSON: "R0lGODlh",
							URL:     "data:image/gif;base64,R0lGODlh",
						},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 100,
					TotalTokens:      110,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := openrouter.NewOutboundTransformer("https://openrouter.ai/api/v1", "test-api-key")
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(tt.response),
				Request: &httpclient.Request{
					RequestType: llm.RequestTypeImage.String(),
					TransformerMetadata: map[string]any{
						"model": "test-model",
					},
				},
			}

			got, err := transformer.TransformResponse(context.Background(), httpResp)
			require.NoError(t, err)
			require.NotNil(t, got)

			// Verify basic fields
			require.Equal(t, tt.want.ID, got.ID)
			require.Equal(t, tt.want.Object, got.Object)
			require.Equal(t, tt.want.Created, got.Created)
			require.Equal(t, tt.want.Model, got.Model)
			require.Equal(t, tt.want.RequestType, got.RequestType)

			// Verify image response
			require.NotNil(t, got.Image)
			require.Equal(t, len(tt.want.Image.Data), len(got.Image.Data))

			for i, expectedData := range tt.want.Image.Data {
				require.Equal(t, expectedData.B64JSON, got.Image.Data[i].B64JSON)
				require.Equal(t, expectedData.URL, got.Image.Data[i].URL)
			}

			// Verify usage
			require.NotNil(t, got.Usage)
			require.Equal(t, tt.want.Usage.PromptTokens, got.Usage.PromptTokens)
			require.Equal(t, tt.want.Usage.CompletionTokens, got.Usage.CompletionTokens)
			require.Equal(t, tt.want.Usage.TotalTokens, got.Usage.TotalTokens)
		})
	}
}

func TestOutboundTransformer_ImageEditRequest(t *testing.T) {
	// PNG magic bytes
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	// JPEG magic bytes
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}

	tests := []struct {
		name             string
		request          *llm.Request
		wantErr          bool
		expectedImgCount int
	}{
		{
			name: "image edit with single image",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Add a sunset background to this image",
					Images: [][]byte{pngData},
				},
			},
			wantErr:          false,
			expectedImgCount: 1,
		},
		{
			name: "image edit with multiple images",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Combine these two images",
					Images: [][]byte{pngData, jpegData},
				},
			},
			wantErr:          false,
			expectedImgCount: 2,
		},
		{
			name: "image edit with JPEG image",
			request: &llm.Request{
				Model:       "google/gemini-2.5-flash-image-preview",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Make this image brighter",
					Images: [][]byte{jpegData},
				},
			},
			wantErr:          false,
			expectedImgCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := openrouter.NewOutboundTransformer("https://openrouter.ai/api/v1", "test-api-key")
			require.NoError(t, err)

			req, err := transformer.TransformRequest(context.Background(), tt.request)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)

			// Verify it's a POST request to the OpenRouter image router.
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL, "/images")

			// Parse body and verify structure
			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			require.Equal(t, tt.request.Model, body["model"])
			require.Equal(t, tt.request.Image.Prompt, body["prompt"])

			inputReferences, ok := body["input_references"].([]any)
			require.True(t, ok, "input_references should be present")
			require.Len(t, inputReferences, tt.expectedImgCount)
			for _, reference := range inputReferences {
				referenceMap, ok := reference.(map[string]any)
				require.True(t, ok)
				require.Equal(t, "image_url", referenceMap["type"])
				imageURL, ok := referenceMap["image_url"].(map[string]any)
				require.True(t, ok)
				url, ok := imageURL["url"].(string)
				require.True(t, ok)
				require.True(t, strings.HasPrefix(url, "data:image/"), "image URL should be a data URL")
			}
		})
	}
}

func TestOutboundTransformer_ImageGenerationResponseErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    bool
	}{
		{
			name:       "HTTP error",
			statusCode: http.StatusBadRequest,
			body:       `{"error": {"message": "Invalid request"}}`,
			wantErr:    true,
		},
		{
			name:       "empty body",
			statusCode: http.StatusOK,
			body:       "",
			wantErr:    true,
		},
		{
			name:       "invalid JSON",
			statusCode: http.StatusOK,
			body:       "invalid json",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := openrouter.NewOutboundTransformer("https://openrouter.ai/api/v1", "test-api-key")
			require.NoError(t, err)

			httpResp := &httpclient.Response{
				StatusCode: tt.statusCode,
				Body:       []byte(tt.body),
				Request: &httpclient.Request{
					RequestType: llm.RequestTypeImage.String(),
				},
			}

			_, err = transformer.TransformResponse(context.Background(), httpResp)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
