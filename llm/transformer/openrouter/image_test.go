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

			// Verify it's a POST request to /chat/completions
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL, "/chat/completions")

			// Verify request type is set
			require.Equal(t, llm.RequestTypeImage.String(), req.RequestType)

			// Parse body and verify modalities
			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			modalities, ok := body["modalities"].([]any)
			require.True(t, ok, "modalities should be present")
			require.Contains(t, modalities, "image")
			require.Contains(t, modalities, "text")

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
			name:     "response with images array",
			response: `{"id":"gen-1759393520-lxwGJP80UyDdG9VmVTQj","model":"google/gemini-2.5-flash-image-preview","object":"chat.completion","created":1759393520,"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"","images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo"},"index":0}]}}],"usage":{"prompt_tokens":7,"completion_tokens":1290,"total_tokens":1297}}`,
			want: &llm.Response{
				ID:          "gen-1759393520-lxwGJP80UyDdG9VmVTQj",
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
					Usage: &llm.ImageUsage{
						InputTokens:  7,
						OutputTokens: 1290,
						TotalTokens:  1297,
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
			name:     "response with content array",
			response: `{"id":"gen-test","model":"test-model","object":"chat.completion","created":1759393520,"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":[{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,/9j/4AAQ"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":100,"total_tokens":110}}`,
			want: &llm.Response{
				ID:          "gen-test",
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
					},
					Usage: &llm.ImageUsage{
						InputTokens:  10,
						OutputTokens: 100,
						TotalTokens:  110,
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

			// Verify it's a POST request to /chat/completions
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL, "/chat/completions")

			// Parse body and verify structure
			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			// Verify messages contain content parts
			messages, ok := body["messages"].([]any)
			require.True(t, ok, "messages should be present")
			require.Len(t, messages, 1)

			firstMsg, ok := messages[0].(map[string]any)
			require.True(t, ok)

			content, ok := firstMsg["content"].([]any)
			require.True(t, ok, "content should be an array of parts")

			// Count image_url parts and text parts
			imageCount := 0
			textCount := 0

			for _, part := range content {
				partMap, ok := part.(map[string]any)
				require.True(t, ok)

				partType, ok := partMap["type"].(string)
				require.True(t, ok)

				switch partType {
				case "image_url":
					imageCount++
					// Verify image_url structure
					imageURL, ok := partMap["image_url"].(map[string]any)
					require.True(t, ok)
					url, ok := imageURL["url"].(string)
					require.True(t, ok)
					require.True(t, strings.HasPrefix(url, "data:image/"), "image URL should be a data URL")
				case "text":
					textCount++
					text, ok := partMap["text"].(string)
					require.True(t, ok)
					require.Equal(t, tt.request.Image.Prompt, text)
				}
			}

			require.Equal(t, tt.expectedImgCount, imageCount, "expected %d image parts", tt.expectedImgCount)
			require.Equal(t, 1, textCount, "expected 1 text part")

			// Verify modalities
			modalities, ok := body["modalities"].([]any)
			require.True(t, ok, "modalities should be present")
			require.Contains(t, modalities, "image")
			require.Contains(t, modalities, "text")
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
