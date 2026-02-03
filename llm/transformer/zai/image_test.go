package zai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestBuildImageGenerationAPIRequest(t *testing.T) {
	config := &Config{
		BaseURL:        "https://api.example.com/v4",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		name        string
		chatReq     *llm.Request
		expectError bool
		expectURL   string
	}{
		{
			name: "basic image generation request",
			chatReq: &llm.Request{
				Model:       "cogview-4-250304",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image: &llm.ImageRequest{
					Prompt: "a cute cat",
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/v4/images/generations",
		},
		{
			name: "image generation with quality and size",
			chatReq: &llm.Request{
				Model:       "cogview-4",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image: &llm.ImageRequest{
					Prompt:  "a beautiful landscape",
					Quality: "hd",
					Size:    "1024x1024",
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/v4/images/generations",
		},
		{
			name: "image generation with user from Image.User",
			chatReq: &llm.Request{
				Model:       "cogview-3-flash",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image: &llm.ImageRequest{
					Prompt: "a futuristic city",
					User:   "test-user-123",
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/v4/images/generations",
		},
		{
			name: "image generation watermark disabled by default",
			chatReq: &llm.Request{
				Model:       "cogview-4-250304",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image: &llm.ImageRequest{
					Prompt: "a digital artwork",
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/v4/images/generations",
		},
		{
			name: "short user_id is allowed",
			chatReq: &llm.Request{
				Model:       "cogview-4",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image: &llm.ImageRequest{
					Prompt: "test",
					User:   "123",
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/v4/images/generations",
		},
		{
			name: "no prompt in image request",
			chatReq: &llm.Request{
				Model:       "cogview-4",
				RequestType: llm.RequestTypeImage,
				APIFormat:   llm.APIFormatOpenAIImageGeneration,
				Image:       &llm.ImageRequest{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := transformer.(*OutboundTransformer).buildImageGenerationAPIRequest(t.Context(), tt.chatReq)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectURL, req.URL)
			assert.Equal(t, http.MethodPost, req.Method)
			assert.Equal(t, "application/json", req.Headers.Get("Content-Type"))
			assert.Equal(t, "application/json", req.Headers.Get("Accept"))

			// Verify request body
			var reqBody map[string]any

			err = json.Unmarshal(req.Body, &reqBody)
			require.NoError(t, err)

			assert.Equal(t, tt.chatReq.Model, reqBody["model"])
			assert.NotEmpty(t, reqBody["prompt"])

			// Check for specific parameters
			if tt.chatReq.Image != nil {
				if tt.chatReq.Image.Quality != "" {
					assert.Equal(t, tt.chatReq.Image.Quality, reqBody["quality"])
				} else {
					assert.Equal(t, "standard", reqBody["quality"])
				}

				if tt.chatReq.Image.Size != "" {
					assert.Equal(t, tt.chatReq.Image.Size, reqBody["size"])
				} else {
					assert.Equal(t, "1024x1024", reqBody["size"])
				}
			}

			assert.Equal(t, false, reqBody["watermark_enabled"])

			// Check user_id from Image.User
			if tt.chatReq.Image != nil && tt.chatReq.Image.User != "" {
				assert.Equal(t, tt.chatReq.Image.User, reqBody["user_id"])
			}
		})
	}
}

func TestTransformImageGenerationResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *httpclient.Response
		expect   *llm.Response
	}{
		{
			name: "basic image generation response",
			response: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"created": 1234567890,
					"data": [
						{
							"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
						}
					]
				}`),
				Request: &httpclient.Request{
					Metadata: map[string]string{
						"model": "cogview-4-250304",
					},
				},
			},
			expect: &llm.Response{
				ID:      "zai-img-1234567890",
				Object:  "chat.completion",
				Created: 1234567890,
				Model:   "cogview-4-250304",
				Image: &llm.ImageResponse{
					Created: 1234567890,
					Data: []llm.ImageData{
						{
							B64JSON: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
							URL:     "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := transformImageGenerationResponse(context.Background(), tt.response)
			require.NoError(t, err)

			assert.Equal(t, tt.expect.Object, resp.Object)
			assert.Equal(t, tt.expect.Created, resp.Created)
			assert.Equal(t, tt.expect.Model, resp.Model)
			require.NotNil(t, resp.Image)
			assert.Equal(t, tt.expect.Image.Created, resp.Image.Created)
			assert.Len(t, resp.Image.Data, len(tt.expect.Image.Data))
			assert.NotEmpty(t, resp.Image.Data[0].B64JSON)
			// Image response should not have Choices
			assert.Empty(t, resp.Choices)
		})
	}
}
