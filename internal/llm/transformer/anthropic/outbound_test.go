package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	tests := []struct {
		name        string
		chatReq     *llm.Request
		expectError bool
	}{
		{
			name: "valid simple request",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, Claude!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with system message",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: func() *string { s := "You are a helpful assistant."; return &s }(),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with multimodal content",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: func() *string { s := "What's in this image?"; return &s }(),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/jpeg;base64,/9j/4AAQSkZJRg...",
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request with temperature and stop sequences",
			chatReq: &llm.Request{
				Model:       "claude-3-sonnet-20240229",
				MaxTokens:   func() *int64 { v := int64(1024); return &v }(),
				Temperature: func() *float64 { v := 0.7; return &v }(),
				Stop: &llm.Stop{
					MultipleStop: []string{"Human:", "Assistant:"},
				},
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "request without max_tokens (should use default)",
			chatReq: &llm.Request{
				Model: "claude-3-sonnet-20240229",
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil request",
			chatReq:     nil,
			expectError: true,
		},
		{
			name: "missing model",
			chatReq: &llm.Request{
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty messages",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages:  []llm.Message{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(context.Background(), tt.chatReq)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, http.MethodPost, result.Method)
				assert.Equal(t, "https://api.anthropic.com/v1/messages", result.URL)
				assert.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				assert.Equal(t, "2023-06-01", result.Headers.Get("anthropic-version"))
				assert.NotEmpty(t, result.Body)

				// Verify the request can be unmarshaled to AnthropicRequest
				var anthropicReq MessageRequest
				err := json.Unmarshal(result.Body, &anthropicReq)
				require.NoError(t, err)
				assert.Equal(t, tt.chatReq.Model, anthropicReq.Model)
				assert.Greater(t, anthropicReq.MaxTokens, int64(0))

				// Verify auth
				if result.Auth != nil {
					assert.Equal(t, "api_key", result.Auth.Type)
					assert.Equal(t, "test-api-key", result.Auth.APIKey)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewOutboundTransformer("", "")

	tests := []struct {
		name        string
		httpResp    *httpclient.Response
		expectError bool
	}{
		{
			name: "valid response",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "msg_123",
					"type": "message",
					"role": "assistant",
					"content": [
						{
							"type": "text",
							"text": "Hello! How can I help you?"
						}
					],
					"model": "claude-3-sonnet-20240229",
					"stop_reason": "end_turn",
					"usage": {
						"input_tokens": 10,
						"output_tokens": 20
					}
				}`),
			},
			expectError: false,
		},
		{
			name: "response with multiple content blocks",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"id": "msg_456",
					"type": "message",
					"role": "assistant",
					"content": [
						{
							"type": "text",
							"text": "I can see"
						},
						{
							"type": "text",
							"text": " an image."
						}
					],
					"model": "claude-3-sonnet-20240229",
					"stop_reason": "end_turn"
				}`),
			},
			expectError: false,
		},
		{
			name:        "nil response",
			httpResp:    nil,
			expectError: true,
		},
		{
			name: "HTTP error response",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"error": {"message": "Invalid request"}}`),
				Error: &httpclient.ResponseError{
					Message: "Invalid request",
				},
			},
			expectError: true,
		},
		{
			name: "empty body",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte{},
			},
			expectError: true,
		},
		{
			name: "invalid JSON",
			httpResp: &httpclient.Response{
				StatusCode: http.StatusOK,
				Body:       []byte(`invalid json`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(context.Background(), tt.httpResp)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, "chat.completion", result.Object)
				assert.NotEmpty(t, result.ID)
				assert.NotEmpty(t, result.Model)
				assert.NotEmpty(t, result.Choices)
				assert.Equal(t, "assistant", result.Choices[0].Message.Role)
			}
		})
	}
}

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	transformer := NewOutboundTransformer("", "")

	tests := []struct {
		name     string
		chunks   [][]byte
		expected string
	}{
		{
			name:     "empty chunks",
			chunks:   [][]byte{},
			expected: "",
		},
		{
			name: "single chunk",
			chunks: [][]byte{
				[]byte(`{
					"type": "message_start",
					"message": {
						"id": "msg_123",
						"type": "message",
						"role": "assistant",
						"content": [],
						"model": "claude-3-sonnet-20240229"
					}
				}`),
				[]byte(`{
					"type": "content_block_delta",
					"index": 0,
					"delta": {
						"type": "text_delta",
						"text": "Hello!"
					}
				}`),
				[]byte(`{
					"type": "message_delta",
					"delta": {
						"stop_reason": "end_turn"
					},
					"usage": {
						"input_tokens": 10,
						"output_tokens": 5
					}
				}`),
			},
			expected: "Hello!",
		},
		{
			name: "multiple content chunks",
			chunks: [][]byte{
				[]byte(`{
					"type": "message_start",
					"message": {
						"id": "msg_456",
						"type": "message",
						"role": "assistant",
						"content": [],
						"model": "claude-3-sonnet-20240229"
					}
				}`),
				[]byte(`{
					"type": "content_block_delta",
					"index": 0,
					"delta": {
						"type": "text_delta",
						"text": "Hello"
					}
				}`),
				[]byte(`{
					"type": "content_block_delta",
					"index": 0,
					"delta": {
						"type": "text_delta",
						"text": ", world!"
					}
				}`),
				[]byte(`{
					"type": "message_delta",
					"delta": {
						"stop_reason": "end_turn"
					}
				}`),
			},
			expected: "Hello, world!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.AggregateStreamChunks(context.Background(), tt.chunks)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.expected == "" {
				assert.Empty(t, result.Choices)
			} else {
				require.NotEmpty(t, result.Choices)
				assert.Equal(t, tt.expected, *result.Choices[0].Message.Content.Content)
				assert.Equal(t, "assistant", result.Choices[0].Message.Role)
			}
		})
	}
}

func TestOutboundTransformer_SetAPIKey(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	newAPIKey := "new-api-key"
	transformer.SetAPIKey(newAPIKey)
	assert.Equal(t, newAPIKey, transformer.apiKey)
}

func TestOutboundTransformer_SetBaseURL(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	newBaseURL := "https://custom.api.com"
	transformer.SetBaseURL(newBaseURL)
	assert.Equal(t, newBaseURL, transformer.baseURL)
}

func TestConvertToAnthropicRequest(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	tests := []struct {
		name     string
		chatReq  *llm.Request
		expected *MessageRequest
	}{
		{
			name: "simple request",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expected: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 1024,
				Messages: []Message{
					{
						Role: "user",
						Content: MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
		},
		{
			name: "request with system message",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: func() *string { s := "You are helpful."; return &s }(),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
			expected: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 1024,
				System:    func() *string { s := "You are helpful."; return &s }(),
				Messages: []Message{
					{
						Role: "user",
						Content: MessageContent{
							Content: func() *string { s := "Hello!"; return &s }(),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.convertToAnthropicRequest(tt.chatReq)
			assert.Equal(t, tt.expected.Model, result.Model)
			assert.Equal(t, tt.expected.MaxTokens, result.MaxTokens)
			assert.Equal(t, tt.expected.System, result.System)
			assert.Equal(t, len(tt.expected.Messages), len(result.Messages))
		})
	}
}

func TestConvertToChatCompletionResponse(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	anthropicResp := &MessageResponse{
		ID:   "msg_123",
		Type: "message",
		Role: "assistant",
		Content: []ContentBlock{
			{
				Type: "text",
				Text: func() *string { s := "Hello! How can I help?"; return &s }(),
			},
		},
		Model:      "claude-3-sonnet-20240229",
		StopReason: func() *string { s := "end_turn"; return &s }(),
		Usage: &Usage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	result := transformer.convertToChatCompletionResponse(anthropicResp)

	assert.Equal(t, "msg_123", result.ID)
	assert.Equal(t, "chat.completion", result.Object)
	assert.Equal(t, "claude-3-sonnet-20240229", result.Model)
	assert.Equal(t, 1, len(result.Choices))
	assert.Equal(t, "assistant", result.Choices[0].Message.Role)
	assert.Equal(t, "Hello! How can I help?", *result.Choices[0].Message.Content.Content)
	assert.Equal(t, "stop", *result.Choices[0].FinishReason)
	assert.Equal(t, 10, result.Usage.PromptTokens)
	assert.Equal(t, 20, result.Usage.CompletionTokens)
	assert.Equal(t, 30, result.Usage.TotalTokens)
}
