package gemini_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/gemini"
)

func TestOutboundTransformer_ImageGenerationRequest(t *testing.T) {
	tests := []struct {
		name    string
		request *llm.Request
		wantErr bool
	}{
		{
			name: "image generation with prompt",
			request: &llm.Request{
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Generate a beautiful sunset over mountains",
				},
			},
			wantErr: false,
		},
		{
			name: "image generation with size",
			request: &llm.Request{
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Generate a cat",
					Size:   "1024x1024",
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
			name: "missing image field",
			request: &llm.Request{
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			request: &llm.Request{
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
				Image:       &llm.ImageRequest{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := gemini.NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")
			require.NoError(t, err)

			req, err := transformer.TransformRequest(context.Background(), tt.request)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)

			// Verify it's a POST request to generateContent
			require.Equal(t, http.MethodPost, req.Method)
			require.Contains(t, req.URL, ":generateContent")

			// Verify request type is set
			require.Equal(t, llm.RequestTypeImage.String(), req.RequestType)

			// Parse body and verify structure
			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			// Verify generationConfig contains responseModalities
			genConfig, ok := body["generationConfig"].(map[string]any)
			require.True(t, ok, "generationConfig should be present")

			modalities, ok := genConfig["responseModalities"].([]any)
			require.True(t, ok, "responseModalities should be present")
			require.Contains(t, modalities, "TEXT")
			require.Contains(t, modalities, "IMAGE")

			// Verify model is saved in TransformerMetadata
			require.NotNil(t, req.TransformerMetadata)
			require.Equal(t, tt.request.Model, req.TransformerMetadata["model"])
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
				Model:       "gemini-2.5-flash-image",
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
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "Combine these two images",
					Images: [][]byte{pngData, jpegData},
				},
			},
			wantErr:          false,
			expectedImgCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := gemini.NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")
			require.NoError(t, err)

			req, err := transformer.TransformRequest(context.Background(), tt.request)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, req)

			// Parse body and verify structure
			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			// Verify contents structure
			contents, ok := body["contents"].([]any)
			require.True(t, ok, "contents should be present")
			require.Len(t, contents, 1)

			firstContent, ok := contents[0].(map[string]any)
			require.True(t, ok)

			parts, ok := firstContent["parts"].([]any)
			require.True(t, ok, "parts should be present")

			// Count inlineData parts and text parts
			imageCount := 0
			textCount := 0

			for _, part := range parts {
				partMap, ok := part.(map[string]any)
				require.True(t, ok)

				if _, hasInlineData := partMap["inlineData"]; hasInlineData {
					imageCount++

					inlineData, ok := partMap["inlineData"].(map[string]any)
					require.True(t, ok)

					mimeType, ok := inlineData["mimeType"].(string)
					require.True(t, ok)
					require.True(t, strings.HasPrefix(mimeType, "image/"), "mimeType should be an image type")
				}

				if _, hasText := partMap["text"]; hasText {
					textCount++
				}
			}

			require.Equal(t, tt.expectedImgCount, imageCount, "expected %d image parts", tt.expectedImgCount)
			require.Equal(t, 1, textCount, "expected 1 text part")
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
			name: "response with image in inlineData",
			response: `{
				"candidates": [{
					"content": {
						"parts": [
							{"inlineData": {"mimeType": "image/png", "data": "iVBORw0KGgo"}}
						],
						"role": "model"
					},
					"finishReason": "STOP"
				}],
				"usageMetadata": {
					"promptTokenCount": 10,
					"candidatesTokenCount": 100,
					"totalTokenCount": 110
				},
				"responseId": "resp-123"
			}`,
			want: &llm.Response{
				ID:          "resp-123",
				Object:      "chat.completion",
				Model:       "test-model",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageResponse{
					Data: []llm.ImageData{
						{B64JSON: "iVBORw0KGgo"},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 100,
					TotalTokens:      110,
				},
			},
		},
		{
			name: "response with multiple images",
			response: `{
				"candidates": [{
					"content": {
						"parts": [
							{"inlineData": {"mimeType": "image/png", "data": "image1base64"}},
							{"text": "Here are the images"},
							{"inlineData": {"mimeType": "image/jpeg", "data": "image2base64"}}
						],
						"role": "model"
					},
					"finishReason": "STOP"
				}],
				"usageMetadata": {
					"promptTokenCount": 20,
					"candidatesTokenCount": 200,
					"totalTokenCount": 220
				}
			}`,
			want: &llm.Response{
				Object:      "chat.completion",
				Model:       "test-model",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageResponse{
					Data: []llm.ImageData{
						{B64JSON: "image1base64"},
						{B64JSON: "image2base64"},
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     20,
					CompletionTokens: 200,
					TotalTokens:      220,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := gemini.NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")
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
			require.Equal(t, tt.want.Object, got.Object)
			require.Equal(t, tt.want.Model, got.Model)
			require.Equal(t, tt.want.RequestType, got.RequestType)

			// Verify image response
			require.NotNil(t, got.Image)
			require.Equal(t, len(tt.want.Image.Data), len(got.Image.Data))

			for i, expectedData := range tt.want.Image.Data {
				require.Equal(t, expectedData.B64JSON, got.Image.Data[i].B64JSON)
			}

			// Verify usage
			require.NotNil(t, got.Usage)
			require.Equal(t, tt.want.Usage.PromptTokens, got.Usage.PromptTokens)
			require.Equal(t, tt.want.Usage.CompletionTokens, got.Usage.CompletionTokens)
			require.Equal(t, tt.want.Usage.TotalTokens, got.Usage.TotalTokens)
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
			transformer, err := gemini.NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")
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

func TestMapSizeToAspectRatio(t *testing.T) {
	tests := []struct {
		size     string
		expected string
	}{
		{"1024x1024", "1:1"},
		{"512x512", "1:1"},
		{"1792x1024", "16:9"},
		{"1024x1792", "9:16"},
		{"1536x1024", "3:2"},
		{"1024x1536", "2:3"},
		{"1024x768", "4:3"},
		{"768x1024", "3:4"},
		{"16:9", "16:9"},
		{"unknown", "1:1"},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			// Create request with size
			request := &llm.Request{
				Model:       "gemini-2.5-flash-image",
				RequestType: llm.RequestTypeImage,
				Image: &llm.ImageRequest{
					Prompt: "test",
					Size:   tt.size,
				},
			}

			transformer, err := gemini.NewOutboundTransformer("https://generativelanguage.googleapis.com", "test-api-key")
			require.NoError(t, err)

			req, err := transformer.TransformRequest(context.Background(), request)
			require.NoError(t, err)

			var body map[string]any

			err = json.Unmarshal(req.Body, &body)
			require.NoError(t, err)

			genConfig := body["generationConfig"].(map[string]any)
			imageConfig, ok := genConfig["imageConfig"].(map[string]any)
			require.True(t, ok, "imageConfig should be present")

			aspectRatio := imageConfig["aspectRatio"].(string)
			require.Equal(t, tt.expected, aspectRatio)
		})
	}
}
