package zai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestBuildImageGenerationAPIRequest(t *testing.T) {
	config := &Config{
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
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
				Model: "cogview-4-250304",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &[]string{"a cute cat"}[0],
						},
					},
				},
				Modalities: []string{"image"},
			},
			expectError: false,
			expectURL:   "https://api.example.com/images/generations",
		},
		{
			name: "image generation with quality and size",
			chatReq: &llm.Request{
				Model: "cogview-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &[]string{"a beautiful landscape"}[0],
						},
					},
				},
				Modalities: []string{"image"},
				Tools: []llm.Tool{
					{
						Type: "image_generation",
						ImageGeneration: &llm.ImageGeneration{
							Quality: "hd",
							Size:    "1024x1024",
						},
					},
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/images/generations",
		},
		{
			name: "image generation with user_id from metadata",
			chatReq: &llm.Request{
				Model: "cogview-3-flash",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &[]string{"a futuristic city"}[0],
						},
					},
				},
				Modalities: []string{"image"},
				Metadata: map[string]string{
					"user_id": "test-user-123",
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/images/generations",
		},
		{
			name: "image generation with watermark disabled",
			chatReq: &llm.Request{
				Model: "cogview-4-250304",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &[]string{"a digital artwork"}[0],
						},
					},
				},
				Modalities: []string{"image"},
				Tools: []llm.Tool{
					{
						Type: "image_generation",
						ImageGeneration: &llm.ImageGeneration{
							Watermark: true,
						},
					},
				},
			},
			expectError: false,
			expectURL:   "https://api.example.com/images/generations",
		},
		{
			name: "invalid user_id length",
			chatReq: &llm.Request{
				Model: "cogview-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: &[]string{"test"}[0],
						},
					},
				},
				Modalities: []string{"image"},
				Metadata: map[string]string{
					"user_id": "123", // Too short
				},
			},
			expectError: true,
		},
		{
			name: "no prompt in messages",
			chatReq: &llm.Request{
				Model: "cogview-4",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/png;base64,test",
									},
								},
							},
						},
					},
				},
				Modalities: []string{"image"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := transformer.(*OutboundTransformer).buildImageGenerationAPIRequest(tt.chatReq)

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
			if len(tt.chatReq.Tools) > 0 {
				tool := tt.chatReq.Tools[0]
				if tool.ImageGeneration != nil {
					if tool.ImageGeneration.Quality != "" {
						switch tool.ImageGeneration.Quality {
						case "high":
							assert.Equal(t, "hd", reqBody["quality"])
						case "low":
							assert.Equal(t, "standard", reqBody["quality"])
						}
					}

					if tool.ImageGeneration.Size != "" {
						assert.Equal(t, tool.ImageGeneration.Size, reqBody["size"])
					}

					if tool.ImageGeneration.Watermark {
						assert.Equal(t, true, reqBody["watermark_enabled"])
					} else {
						assert.Equal(t, false, reqBody["watermark_enabled"])
					}
				}
			}

			// Check user_id from metadata
			if tt.chatReq.Metadata != nil && tt.chatReq.Metadata["user_id"] != "" {
				assert.Equal(t, tt.chatReq.Metadata["user_id"], reqBody["user_id"])
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
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
										},
									},
								},
							},
						},
						FinishReason: &[]string{"stop"}[0],
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
			assert.Len(t, resp.Choices, len(tt.expect.Choices))

			for i, choice := range resp.Choices {
				expectedChoice := tt.expect.Choices[i]
				assert.Equal(t, expectedChoice.Index, choice.Index)
				assert.Equal(t, expectedChoice.Message.Role, choice.Message.Role)
				assert.Equal(t, expectedChoice.FinishReason, choice.FinishReason)
				assert.Len(t, choice.Message.Content.MultipleContent, 1)
				assert.Equal(t, "image_url", choice.Message.Content.MultipleContent[0].Type)
				assert.NotNil(t, choice.Message.Content.MultipleContent[0].ImageURL)
				// The URL should be a data URL starting with "data:image/"
				assert.Contains(t, choice.Message.Content.MultipleContent[0].ImageURL.URL, "data:image/")
				assert.Contains(t, choice.Message.Content.MultipleContent[0].ImageURL.URL, "base64,")
			}
		})
	}
}

func TestHasImagesInMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		expect   bool
	}{
		{
			name: "no images",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: &[]string{"hello"}[0],
					},
				},
			},
			expect: false,
		},
		{
			name: "has image in multiple content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "text",
								Text: &[]string{"describe this image"}[0],
							},
							{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: "data:image/png;base64,test",
								},
							},
						},
					},
				},
			},
			expect: true,
		},
		{
			name: "no image in multiple content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "text",
								Text: &[]string{"hello"}[0],
							},
						},
					},
				},
			},
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasImagesInMessages(tt.messages)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestExtractPromptFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		expect   string
		error    bool
	}{
		{
			name: "text content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: &[]string{"a cute cat"}[0],
					},
				},
			},
			expect: "a cute cat",
			error:  false,
		},
		{
			name: "text in multiple content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "text",
								Text: &[]string{"a beautiful landscape"}[0],
							},
						},
					},
				},
			},
			expect: "a beautiful landscape",
			error:  false,
		},
		{
			name: "no text content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: "data:image/png;base64,test",
								},
							},
						},
					},
				},
			},
			expect: "",
			error:  true,
		},
		{
			name:     "empty messages",
			messages: []llm.Message{},
			expect:   "",
			error:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractPromptFromMessages(tt.messages)

			if tt.error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expect, result)
			}
		})
	}
}
