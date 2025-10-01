package doubao

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestNewOutboundTransformer(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiKey    string
		wantErr   bool
		errString string
	}{
		{
			name:    "valid config",
			baseURL: "https://ark.cn-beijing.volces.com/api/v3",
			apiKey:  "test-api-key",
			wantErr: false,
		},
		{
			name:      "empty base URL",
			baseURL:   "",
			apiKey:    "test-api-key",
			wantErr:   true,
			errString: "base URL is required",
		},
		{
			name:      "empty API key",
			baseURL:   "https://ark.cn-beijing.volces.com/api/v3",
			apiKey:    "",
			wantErr:   true,
			errString: "API key is required",
		},
		{
			name:    "base URL with trailing slash",
			baseURL: "https://ark.cn-beijing.volces.com/api/v3/",
			apiKey:  "test-api-key",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOutboundTransformer(tt.baseURL, tt.apiKey)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestNewOutboundTransformerWithConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errString string
		validate  func(*OutboundTransformer) bool
	}{
		{
			name: "valid config",
			config: &Config{
				BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
				APIKey:  "test-api-key",
			},
			wantErr: false,
			validate: func(t *OutboundTransformer) bool {
				return t.BaseURL == "https://ark.cn-beijing.volces.com/api/v3" && t.APIKey == "test-api-key"
			},
		},
		{
			name: "valid config with trailing slash",
			config: &Config{
				BaseURL: "https://ark.cn-beijing.volces.com/api/v3/",
				APIKey:  "test-api-key",
			},
			wantErr: false,
			validate: func(t *OutboundTransformer) bool {
				return t.BaseURL == "https://ark.cn-beijing.volces.com/api/v3" && t.APIKey == "test-api-key"
			},
		},
		{
			name: "nil config",
			config: &Config{
				BaseURL: "",
				APIKey:  "",
			},
			wantErr:   true,
			errString: "base URL is required",
		},
		{
			name: "empty base URL",
			config: &Config{
				BaseURL: "",
				APIKey:  "test-api-key",
			},
			wantErr:   true,
			errString: "base URL is required",
		},
		{
			name: "empty API key",
			config: &Config{
				BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
				APIKey:  "",
			},
			wantErr:   true,
			errString: "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformer, err := NewOutboundTransformerWithConfig(tt.config)

			if tt.wantErr {
				assert.Error(t, err)

				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, transformer)

			if tt.validate != nil {
				doubaoTransformer := transformer.(*OutboundTransformer)
				assert.True(t, tt.validate(doubaoTransformer))
			}
		})
	}
}

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	// Helper function to create transformer
	createTransformer := func(baseURL, apiKey string) *OutboundTransformer {
		transformerInterface, err := NewOutboundTransformer(baseURL, apiKey)
		if err != nil {
			t.Fatalf("Failed to create transformer: %v", err)
		}

		return transformerInterface.(*OutboundTransformer)
	}

	tests := []struct {
		name        string
		transformer *OutboundTransformer
		request     *llm.Request
		wantErr     bool
		errContains string
		validate    func(*httpclient.Request) bool
	}{
		{
			name:        "valid chat completion request",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request: &llm.Request{
				Model: "ep-20241203072800-8f7f",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				// Validate request structure
				if req.Method != http.MethodPost {
					return false
				}

				if req.URL != "https://ark.cn-beijing.volces.com/api/v3/chat/completions" {
					return false
				}

				if req.Headers.Get("Content-Type") != "application/json" {
					return false
				}

				if req.Auth == nil || req.Auth.Type != "bearer" || req.Auth.APIKey != "test-api-key" {
					return false
				}

				// Validate body structure
				var doubaoReq Request

				err := json.Unmarshal(req.Body, &doubaoReq)
				if err != nil {
					return false
				}

				return doubaoReq.Model == "ep-20241203072800-8f7f" &&
					len(doubaoReq.Messages) == 1 &&
					doubaoReq.Messages[0].Role == "user" &&
					doubaoReq.Messages[0].Content.Content != nil &&
					*doubaoReq.Messages[0].Content.Content == "Hello, world!" &&
					doubaoReq.Metadata == nil // Metadata should be removed
			},
		},
		{
			name:        "request with metadata",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request: &llm.Request{
				Model: "ep-20241203072800-8f7f",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
				Metadata: map[string]string{
					"user_id":    "user123",
					"request_id": "req456",
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var doubaoReq Request

				err := json.Unmarshal(req.Body, &doubaoReq)
				if err != nil {
					return false
				}

				return doubaoReq.UserID == "user123" &&
					doubaoReq.RequestID == "req456" &&
					doubaoReq.Metadata == nil
			},
		},
		{
			name:        "request with metadata auto-generates request_id",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request: &llm.Request{
				Model: "ep-20241203072800-8f7f",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
				Metadata: map[string]string{
					"user_id": "user123",
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var doubaoReq Request

				err := json.Unmarshal(req.Body, &doubaoReq)
				if err != nil {
					return false
				}

				return doubaoReq.UserID == "user123" &&
					doubaoReq.RequestID != "" &&
					strings.HasPrefix(doubaoReq.RequestID, "req_")
			},
		},
		{
			name:        "image generation request with modalities",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request: &llm.Request{
				Model: "doubao-image-pro",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Generate an image of a cat"),
						},
					},
				},
				Modalities: []string{"image"},
				Tools: []llm.Tool{
					{
						Type: "image_generation",
						ImageGeneration: &llm.ImageGeneration{
							Size:      "1024x1024",
							Quality:   "hd",
							Watermark: true,
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				return req.Method == http.MethodPost &&
					req.URL == "https://ark.cn-beijing.volces.com/api/v3/images/generations" &&
					req.Headers.Get("Content-Type") == "application/json" &&
					req.Auth != nil &&
					req.Auth.Type == "bearer" &&
					req.Metadata["outbound_format_type"] == llm.APIFormatOpenAIImageGeneration.String()
			},
		},
		{
			name:        "nil request",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request:     nil,
			wantErr:     true,
			errContains: "chat completion request is nil",
		},
		{
			name:        "missing model",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr:     true,
			errContains: "model is required",
		},
		{
			name:        "empty messages",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key"),
			request: &llm.Request{
				Model:    "ep-20241203072800-8f7f",
				Messages: []llm.Message{},
			},
			wantErr:     true,
			errContains: "messages are required",
		},
		{
			name:        "base URL with trailing slash",
			transformer: createTransformer("https://ark.cn-beijing.volces.com/api/v3/", "test-api-key"),
			request: &llm.Request{
				Model: "ep-20241203072800-8f7f",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello, world!"),
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				return req.URL == "https://ark.cn-beijing.volces.com/api/v3/chat/completions"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.transformer.TransformRequest(t.Context(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("TransformRequest() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("TransformRequest() error = %v, want error containing %v", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Errorf("TransformRequest() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("TransformRequest() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("TransformRequest() validation failed for result: %+v", result)
			}
		})
	}
}

func TestOutboundTransformer_buildImageGenerationAPIRequest(t *testing.T) {
	transformerInterface, err := NewOutboundTransformer("https://ark.cn-beijing.volces.com/api/v3", "test-api-key")
	if err != nil {
		t.Fatalf("Failed to create transformer: %v", err)
	}

	transformer := transformerInterface.(*OutboundTransformer)

	tests := []struct {
		name        string
		request     *llm.Request
		wantErr     bool
		errContains string
		validate    func(*httpclient.Request) bool
	}{
		{
			name: "basic image generation",
			request: &llm.Request{
				Model: "doubao-image-pro",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("A beautiful sunset"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "image_generation",
						ImageGeneration: &llm.ImageGeneration{
							Size:    "1024x1024",
							Quality: "hd",
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var body map[string]interface{}

				err := json.Unmarshal(req.Body, &body)
				if err != nil {
					return false
				}

				return body["model"] == "doubao-image-pro" &&
					body["prompt"] == "A beautiful sunset" &&
					body["response_format"] == "b64_json" &&
					body["size"] == "1024x1024" &&
					body["guidance_scale"] == 7.5
			},
		},
		{
			name: "image editing with image URLs",
			request: &llm.Request{
				Model: "doubao-image-pro",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Modify this image"),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "https://example.com/image.jpg",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var body map[string]interface{}

				err := json.Unmarshal(req.Body, &body)
				if err != nil {
					return false
				}

				return body["prompt"] == "Modify this image" &&
					body["image"] == "https://example.com/image.jpg"
			},
		},
		{
			name: "image with watermark",
			request: &llm.Request{
				Model: "doubao-image-pro",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("A logo"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "image_generation",
						ImageGeneration: &llm.ImageGeneration{
							Watermark: true,
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var body map[string]interface{}

				err := json.Unmarshal(req.Body, &body)
				if err != nil {
					return false
				}

				return body["watermark"] == true
			},
		},
		{
			name: "standard quality mapping",
			request: &llm.Request{
				Model: "doubao-image-pro",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Standard image"),
						},
					},
				},
				Tools: []llm.Tool{
					{
						Type: "image_generation",
						ImageGeneration: &llm.ImageGeneration{
							Quality: "standard",
						},
					},
				},
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var body map[string]interface{}

				err := json.Unmarshal(req.Body, &body)
				if err != nil {
					return false
				}

				return body["guidance_scale"] == 2.5
			},
		},
		{
			name: "with user field",
			request: &llm.Request{
				Model: "doubao-image-pro",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("User image"),
						},
					},
				},
				User: lo.ToPtr("user123"),
			},
			wantErr: false,
			validate: func(req *httpclient.Request) bool {
				var body map[string]interface{}

				err := json.Unmarshal(req.Body, &body)
				if err != nil {
					return false
				}

				return body["user"] == "user123"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.buildImageGenerationAPIRequest(tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("buildImageGenerationAPIRequest() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("buildImageGenerationAPIRequest() error = %v, want error containing %v", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Errorf("buildImageGenerationAPIRequest() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("buildImageGenerationAPIRequest() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("buildImageGenerationAPIRequest() validation failed")
			}
		})
	}
}

func TestHasImagesInMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		expected bool
	}{
		{
			name: "no images",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Just text"),
					},
				},
			},
			expected: false,
		},
		{
			name: "with image URL",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "text",
								Text: lo.ToPtr("Text and image"),
							},
							{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: "https://example.com/image.jpg",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "multiple messages with image",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("First message"),
					},
				},
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: "https://example.com/image2.jpg",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasImagesInMessages(tt.messages)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractPromptFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []llm.Message
		expected string
		wantErr  bool
	}{
		{
			name: "extract from content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("A beautiful sunset"),
					},
				},
			},
			expected: "A beautiful sunset",
			wantErr:  false,
		},
		{
			name: "extract from multiple content",
			messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "image_url",
								ImageURL: &llm.ImageURL{
									URL: "https://example.com/image.jpg",
								},
							},
							{
								Type: "text",
								Text: lo.ToPtr("A cat playing"),
							},
						},
					},
				},
			},
			expected: "A cat playing",
			wantErr:  false,
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
									URL: "https://example.com/image.jpg",
								},
							},
						},
					},
				},
			},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty messages",
			messages: []llm.Message{},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractPromptFromMessages(tt.messages)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractImages(t *testing.T) {
	tests := []struct {
		name     string
		chatReq  *llm.Request
		expected []string
		wantErr  bool
	}{
		{
			name: "extract single image URL",
			chatReq: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Modify this"),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "https://example.com/image.jpg",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"https://example.com/image.jpg"},
			wantErr:  false,
		},
		{
			name: "extract base64 image",
			chatReq: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUoKSj/2wBDAQcHBwoIChMKChMoGhYaKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCj/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCdABmX/9k=",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{
				"data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAYEBQYFBAYGBQYHBwYIChAKCgkJChQODwwQFxQYGBcUFhYaHSUfGhsjHBYWICwgIyYnKSopGR8tMC0oMCUoKSj/2wBDAQcHBwoIChMKChMoGhYaKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCgoKCj/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAv/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCdABmX/9k=",
			},
			wantErr: false,
		},
		{
			name: "extract multiple images",
			chatReq: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "https://example.com/image1.jpg",
									},
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "https://example.com/image2.jpg",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"https://example.com/image1.jpg", "https://example.com/image2.jpg"},
			wantErr:  false,
		},
		{
			name: "no images",
			chatReq: &llm.Request{
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Just text"),
						},
					},
				},
			},
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractImages(tt.chatReq)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to marshal data for tests.
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}
