package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestOutboundTransformer_TransformRequest(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

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
			result, err := transformer.TransformRequest(t.Context(), tt.chatReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.anthropic.com/v1/messages", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.Equal(t, "2023-06-01", result.Headers.Get("Anthropic-Version"))
				require.NotEmpty(t, result.Body)

				// Verify the request can be unmarshaled to AnthropicRequest
				var anthropicReq MessageRequest

				err := json.Unmarshal(result.Body, &anthropicReq)
				require.NoError(t, err)
				require.Equal(t, tt.chatReq.Model, anthropicReq.Model)
				require.Greater(t, anthropicReq.MaxTokens, int64(0))

				// Verify auth
				if result.Auth != nil {
					require.Equal(t, "api_key", result.Auth.Type)
					require.Equal(t, "test-api-key", result.Auth.APIKey)
				}
			}
		})
	}
}

func TestOutboundTransformer_TransformResponse(t *testing.T) {
	transformer, _ := NewOutboundTransformer("", "")

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
			result, err := transformer.TransformResponse(t.Context(), tt.httpResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, "chat.completion", result.Object)
				require.NotEmpty(t, result.ID)
				require.NotEmpty(t, result.Model)
				require.NotEmpty(t, result.Choices)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
			}
		})
	}
}

func TestOutboundTransformer_ErrorHandling(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-key")

	t.Run("TransformRequest error cases", func(t *testing.T) {
		tests := []struct {
			name        string
			chatReq     *llm.Request
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil request",
				chatReq:     nil,
				expectError: true,
				errorMsg:    "chat completion request is nil",
			},
			{
				name: "empty model",
				chatReq: &llm.Request{
					Model:     "",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Hello"; return &s }(),
							},
						},
					},
				},
				expectError: true,
				errorMsg:    "model is required",
			},
			{
				name: "no messages",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages:  []llm.Message{},
				},
				expectError: true,
				errorMsg:    "messages are required",
			},
			{
				name: "negative max tokens",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(-1); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Hello"; return &s }(),
							},
						},
					},
				},
				expectError: true,
				errorMsg:    "max_tokens must be positive",
			},
			{
				name: "zero max tokens",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(0); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Hello"; return &s }(),
							},
						},
					},
				},
				expectError: true,
				errorMsg:    "max_tokens must be positive",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformRequest(t.Context(), tt.chatReq)
				if tt.expectError {
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.errorMsg)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("TransformResponse error cases", func(t *testing.T) {
		tests := []struct {
			name        string
			httpResp    *httpclient.Response
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil response",
				httpResp:    nil,
				expectError: true,
				errorMsg:    "http response is nil",
			},
			{
				name: "HTTP error status",
				httpResp: &httpclient.Response{
					StatusCode: http.StatusBadRequest,
					Body:       []byte(`{"error": {"message": "Bad request"}}`),
				},
				expectError: true,
				errorMsg:    "HTTP error 400",
			},
			{
				name: "empty response body",
				httpResp: &httpclient.Response{
					StatusCode: http.StatusOK,
					Body:       []byte{},
				},
				expectError: true,
				errorMsg:    "response body is empty",
			},
			{
				name: "invalid JSON response",
				httpResp: &httpclient.Response{
					StatusCode: http.StatusOK,
					Body:       []byte(`{invalid json}`),
				},
				expectError: true,
				errorMsg:    "failed to unmarshal anthropic response",
			},
			{
				name: "malformed JSON response",
				httpResp: &httpclient.Response{
					StatusCode: http.StatusOK,
					Body:       []byte(`{"id": 123, "type": "message"}`), // ID should be string
				},
				expectError: true,
				errorMsg:    "failed to unmarshal anthropic response",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformResponse(t.Context(), tt.httpResp)
				if tt.expectError {
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.errorMsg)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestOutboundTransformer_ToolUse(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.example.com", "test-api-key")

	t.Run("Tool conversion and handling", func(t *testing.T) {
		tests := []struct {
			name        string
			chatReq     *llm.Request
			expectError bool
			validate    func(t *testing.T, result *httpclient.Request)
		}{
			{
				name: "request with single tool",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "What's the weather?"; return &s }(),
							},
						},
					},
					Tools: []llm.Tool{
						{
							Type: "function",
							Function: llm.Function{
								Name:        "get_weather",
								Description: "Get the current weather for a location",
								Parameters: json.RawMessage(
									`{"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}`,
								),
							},
						},
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *httpclient.Request) {
					t.Helper()

					var anthropicReq MessageRequest

					err := json.Unmarshal(result.Body, &anthropicReq)
					require.NoError(t, err)
					require.NotNil(t, anthropicReq.Tools)
					require.Len(t, anthropicReq.Tools, 1)
					require.Equal(t, "get_weather", anthropicReq.Tools[0].Name)
					require.Equal(
						t,
						"Get the current weather for a location",
						anthropicReq.Tools[0].Description,
					)
					// Compare JSON content flexibly (ignore whitespace differences)
					expectedSchema := map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type": "string",
							},
						},
						"required": []any{"location"},
					}

					var actualSchema map[string]any

					unmarshalErr := json.Unmarshal(anthropicReq.Tools[0].InputSchema, &actualSchema)
					require.NoError(t, unmarshalErr)
					require.Equal(t, expectedSchema, actualSchema)
				},
			},
			{
				name: "request with multiple tools",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Help me calculate and check weather"; return &s }(),
							},
						},
					},
					Tools: []llm.Tool{
						{
							Type: "function",
							Function: llm.Function{
								Name:        "calculator",
								Description: "Perform mathematical calculations",
								Parameters: json.RawMessage(
									`{"type": "object", "properties": {"expression": {"type": "string"}}, "required": ["expression"]}`,
								),
							},
						},
						{
							Type: "function",
							Function: llm.Function{
								Name:        "get_weather",
								Description: "Get the current weather for a location",
								Parameters: json.RawMessage(
									`{"type": "object", "properties": {"location": {"type": "string"}}, "required": ["location"]}`,
								),
							},
						},
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *httpclient.Request) {
					t.Helper()

					var anthropicReq MessageRequest

					err := json.Unmarshal(result.Body, &anthropicReq)
					require.NoError(t, err)
					require.NotNil(t, anthropicReq.Tools)
					require.Len(t, anthropicReq.Tools, 2)

					// Check first tool
					require.Equal(t, "calculator", anthropicReq.Tools[0].Name)
					require.Equal(
						t,
						"Perform mathematical calculations",
						anthropicReq.Tools[0].Description,
					)

					// Check second tool
					require.Equal(t, "get_weather", anthropicReq.Tools[1].Name)
					require.Equal(
						t,
						"Get the current weather for a location",
						anthropicReq.Tools[1].Description,
					)
				},
			},
			{
				name: "request with non-function tool (should be filtered out)",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Use any tool available"; return &s }(),
							},
						},
					},
					Tools: []llm.Tool{
						{
							Type: "function",
							Function: llm.Function{
								Name:        "valid_function",
								Description: "A valid function",
								Parameters:  json.RawMessage(`{"type": "object"}`),
							},
						},
						{
							Type: "code_interpreter", // This should be filtered out
							Function: llm.Function{
								Name:        "invalid_tool",
								Description: "This should not be included",
								Parameters:  json.RawMessage(`{"type": "object"}`),
							},
						},
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *httpclient.Request) {
					t.Helper()

					var anthropicReq MessageRequest

					err := json.Unmarshal(result.Body, &anthropicReq)
					require.NoError(t, err)
					require.NotNil(t, anthropicReq.Tools)
					require.Len(
						t,
						anthropicReq.Tools,
						1,
					) // Only the function tool should be included
					require.Equal(t, "valid_function", anthropicReq.Tools[0].Name)
				},
			},
			{
				name: "request with empty tools array",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Hello"; return &s }(),
							},
						},
					},
					Tools: []llm.Tool{},
				},
				expectError: false,
				validate: func(t *testing.T, result *httpclient.Request) {
					t.Helper()

					var anthropicReq MessageRequest

					err := json.Unmarshal(result.Body, &anthropicReq)
					require.NoError(t, err)
					require.Nil(t, anthropicReq.Tools) // Should not include tools field if empty
				},
			},
			{
				name: "request with web_search tool (native Anthropic)",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Search the web for latest AI news"; return &s }(),
							},
						},
					},
					Tools: []llm.Tool{
						{
							Type: "function",
							Function: llm.Function{
								Name:        "web_search",
								Description: "Search the web for information",
								Parameters: json.RawMessage(
									`{"type": "object", "properties": {"query": {"type": "string"}}}`,
								),
							},
						},
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *httpclient.Request) {
					t.Helper()

					var anthropicReq MessageRequest

					err := json.Unmarshal(result.Body, &anthropicReq)
					require.NoError(t, err)
					require.NotNil(t, anthropicReq.Tools)
					require.Len(t, anthropicReq.Tools, 1)
					require.Equal(t, "web_search", anthropicReq.Tools[0].Name)
					require.Equal(t, ToolTypeWebSearch20250305, anthropicReq.Tools[0].Type)
					require.Empty(t, anthropicReq.Tools[0].Description)
					require.Empty(t, anthropicReq.Tools[0].InputSchema)

					// Verify beta header is set
					require.Equal(t, "web-search-2025-03-05", result.Headers.Get("Anthropic-Beta"))
				},
			},
			{
				name: "request with tool choice",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								Content: func() *string { s := "Use the calculator"; return &s }(),
							},
						},
					},
					Tools: []llm.Tool{
						{
							Type: "function",
							Function: llm.Function{
								Name:        "calculator",
								Description: "Perform calculations",
								Parameters: json.RawMessage(
									`{"type": "object", "properties": {"expression": {"type": "string"}}, "required": ["expression"]}`,
								),
							},
						},
					},
					ToolChoice: &llm.ToolChoice{
						NamedToolChoice: &llm.NamedToolChoice{
							Type: "function",
							Function: llm.ToolFunction{
								Name: "calculator",
							},
						},
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *httpclient.Request) {
					t.Helper()

					var anthropicReq MessageRequest

					err := json.Unmarshal(result.Body, &anthropicReq)
					require.NoError(t, err)
					// Note: Tool choice is not directly supported in current implementation
					// but should not cause errors
					require.NotNil(t, anthropicReq.Tools)
					require.Len(t, anthropicReq.Tools, 1)

					// Verify beta header is NOT set for regular function tools
					require.Empty(t, result.Headers.Get("Anthropic-Beta"))
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := transformer.TransformRequest(t.Context(), tt.chatReq)
				if tt.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					tt.validate(t, result)
				}
			})
		}
	})
}

func TestOutboundTransformer_ValidationEdgeCases(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.example.com", "test-api-key")

	t.Run("Message content validation", func(t *testing.T) {
		tests := []struct {
			name        string
			chatReq     *llm.Request
			expectError bool
		}{
			{
				name: "message with nil content",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role:    "user",
							Content: llm.MessageContent{}, // Empty content
						},
					},
				},
				expectError: false, // Should handle gracefully
			},
			{
				name: "message with empty multiple content",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{},
							},
						},
					},
				},
				expectError: false, // Should handle gracefully
			},
			{
				name: "message with invalid image URL",
				chatReq: &llm.Request{
					Model:     "claude-3-sonnet-20240229",
					MaxTokens: func() *int64 { v := int64(1024); return &v }(),
					Messages: []llm.Message{
						{
							Role: "user",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "invalid-url-format", // Not a data URL
										},
									},
								},
							},
						},
					},
				},
				expectError: false, // Should handle gracefully, not convert to image block
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformRequest(t.Context(), tt.chatReq)
				if tt.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestOutboundTransformer_TransformError(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://example.com", "xxx")

	tests := []struct {
		name     string
		httpErr  *httpclient.Error
		expected *llm.ResponseError
	}{
		{
			name: "http error with json body",
			httpErr: &httpclient.Error{
				StatusCode: http.StatusBadRequest,
				Body:       []byte(`{"type": "api_error", "message": "bad request", "request_id": "req_123"}`),
			},
			expected: &llm.ResponseError{
				Detail: llm.ErrorDetail{
					Type:    "api_error",
					Message: `{"type": "api_error", "message": "bad request", "request_id": "req_123"}`,
				},
			},
		},
		{
			name: "http error with non-json body",
			httpErr: &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Body:       []byte("internal server error"),
			},
			expected: &llm.ResponseError{
				Detail: llm.ErrorDetail{
					Type:    "api_error",
					Message: "internal server error",
				},
			},
		},
		{
			name:    "nil error",
			httpErr: nil,
			expected: &llm.ResponseError{
				Detail: llm.ErrorDetail{
					Type:    "api_error",
					Message: "Request failed.",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.TransformError(context.Background(), tt.httpErr)
			require.NotNil(t, result)
			require.Equal(t, tt.expected.Detail.Type, result.Detail.Type)
			require.Equal(t, tt.expected.Detail.Message, result.Detail.Message)
		})
	}
}

func TestOutboundTransformer_TransformRequest_WithTestData(t *testing.T) {
	tests := []struct {
		name         string
		requestFile  string
		expectedFile string
		validate     func(t *testing.T, result *httpclient.Request, llmRequest *llm.Request)
	}{
		{
			name:         "tool use request transformation",
			requestFile:  "llm-tool.request.json",
			expectedFile: "anthropic-tool.request.json",
			validate: func(t *testing.T, result *httpclient.Request, llmRequest *llm.Request) {
				t.Helper()

				// Verify basic HTTP request properties
				require.Equal(t, http.MethodPost, result.Method)
				require.Equal(t, "https://api.anthropic.com/v1/messages", result.URL)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.Equal(t, "2023-06-01", result.Headers.Get("Anthropic-Version"))
				require.NotEmpty(t, result.Body)

				// Verify auth
				require.NotNil(t, result.Auth)
				require.Equal(t, "api_key", result.Auth.Type)
				require.Equal(t, "test-api-key", result.Auth.APIKey)

				// Parse the transformed Anthropic request
				var anthropicReq MessageRequest

				err := json.Unmarshal(result.Body, &anthropicReq)
				require.NoError(t, err)

				// Verify model and max_tokens
				require.Equal(t, llmRequest.Model, anthropicReq.Model)
				require.Equal(t, *llmRequest.MaxTokens, anthropicReq.MaxTokens)

				// Verify messages
				require.Len(t, anthropicReq.Messages, len(llmRequest.Messages))
				require.Equal(t, llmRequest.Messages[0].Role, anthropicReq.Messages[0].Role)

				// Verify tools transformation
				require.NotNil(t, anthropicReq.Tools)
				require.Len(t, anthropicReq.Tools, len(llmRequest.Tools))

				// Verify first tool (get_coordinates)
				require.Equal(t, "get_coordinates", anthropicReq.Tools[0].Name)
				require.Equal(t, "Accepts a place as an address, then returns the latitude and longitude coordinates.", anthropicReq.Tools[0].Description)

				// Verify tool input schema
				var schema map[string]any

				err = json.Unmarshal(anthropicReq.Tools[0].InputSchema, &schema)
				require.NoError(t, err)
				require.Equal(t, "object", schema["type"])

				properties, ok := schema["properties"].(map[string]any)
				require.True(t, ok)
				location, ok := properties["location"].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "string", location["type"])
				require.Equal(t, "The location to look up.", location["description"])

				// Verify second tool (get_temperature_unit)
				require.Equal(t, "get_temperature_unit", anthropicReq.Tools[1].Name)

				// Verify third tool (get_weather)
				require.Equal(t, "get_weather", anthropicReq.Tools[2].Name)
				require.Equal(t, "Get the weather at a specific location", anthropicReq.Tools[2].Description)
			},
		},
		{
			name:         "llm-parallel_multiple_tool.request",
			requestFile:  "llm-parallel_multiple_tool.request.json",
			expectedFile: "anthropic-parallel_multiple_tool.request.json",
			validate:     func(t *testing.T, result *httpclient.Request, llmRequest *llm.Request) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the test request data
			var llmReqquest llm.Request

			err := xtest.LoadTestData(t, tt.requestFile, &llmReqquest)
			require.NoError(t, err)

			// Create transformer
			transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
			require.NoError(t, err)

			// Transform the request
			result, err := transformer.TransformRequest(t.Context(), &llmReqquest)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Run validation
			tt.validate(t, result, &llmReqquest)

			if tt.expectedFile != "" {
				gotReq, err := xjson.To[MessageRequest](result.Body)
				require.NoError(t, err)

				var expectedReq MessageRequest

				err = xtest.LoadTestData(t, tt.expectedFile, &expectedReq)
				require.NoError(t, err)

				// Verify the transformed request matches the expected request
				if !xtest.Equal(expectedReq, gotReq) {
					t.Fatalf("requests are not equal %s", cmp.Diff(expectedReq, gotReq))
				}
			}
		})
	}
}

func TestOutboundTransformer_WebSearchBetaHeader(t *testing.T) {
	t.Run("Direct Anthropic API with web_search tool", func(t *testing.T) {
		transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		require.NoError(t, err)

		chatReq := &llm.Request{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: func() *string { s := "What's the weather?"; return &s }(),
					},
				},
			},
			Tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:        "web_search",
						Description: "Search the web",
					},
				},
			},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		require.Equal(t, "web-search-2025-03-05", result.Headers.Get("Anthropic-Beta"))
	})

	t.Run("Direct Anthropic API with web_search_20250305 type input", func(t *testing.T) {
		transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		require.NoError(t, err)

		chatReq := &llm.Request{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: func() *string { s := "What's the weather?"; return &s }(),
					},
				},
			},
			Tools: []llm.Tool{
				{
					Type: llm.ToolTypeAnthropicWebSearch, // Already transformed type
				},
			},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		// Should still set Beta header for already-transformed tool type
		require.Equal(t, "web-search-2025-03-05", result.Headers.Get("Anthropic-Beta"))

		// Verify tool is passed through correctly
		var anthropicReq MessageRequest

		err = json.Unmarshal(result.Body, &anthropicReq)
		require.NoError(t, err)
		require.Len(t, anthropicReq.Tools, 1)
		require.Equal(t, ToolTypeWebSearch20250305, anthropicReq.Tools[0].Type)
	})

	t.Run("Direct Anthropic API without web_search tool - no Beta header", func(t *testing.T) {
		transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		require.NoError(t, err)

		chatReq := &llm.Request{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: func() *string { s := "Hello"; return &s }(),
					},
				},
			},
			Tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:        "calculator",
						Description: "Perform calculations",
					},
				},
			},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		// Regular function tools should NOT trigger Beta header
		require.Empty(t, result.Headers.Get("Anthropic-Beta"))
	})

	t.Run("Direct Anthropic API with mixed tools including web_search", func(t *testing.T) {
		transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
		require.NoError(t, err)

		chatReq := &llm.Request{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: func() *string { s := "Search and calculate"; return &s }(),
					},
				},
			},
			Tools: []llm.Tool{
				{
					Type: "function",
					Function: llm.Function{
						Name:        "calculator",
						Description: "Perform calculations",
					},
				},
				{
					Type: "function",
					Function: llm.Function{
						Name:        "web_search",
						Description: "Search the web",
					},
				},
			},
		}

		result, err := transformer.TransformRequest(t.Context(), chatReq)
		require.NoError(t, err)
		// Mixed tools with web_search should trigger Beta header
		require.Equal(t, "web-search-2025-03-05", result.Headers.Get("Anthropic-Beta"))

		// Verify tools are converted correctly
		var anthropicReq MessageRequest

		err = json.Unmarshal(result.Body, &anthropicReq)
		require.NoError(t, err)
		require.Len(t, anthropicReq.Tools, 2)

		// Find web_search tool and verify conversion
		var hasWebSearch bool

		for _, tool := range anthropicReq.Tools {
			if tool.Type == ToolTypeWebSearch20250305 {
				hasWebSearch = true

				require.Equal(t, "web_search", tool.Name)
			}
		}

		require.True(t, hasWebSearch, "web_search tool should be converted to web_search_20250305")
	})
}

func TestOutboundTransformer_RawURL(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		request        *llm.Request
		expectedURL    string
		expectedRawURL bool
	}{
		{
			name: "raw URL enabled with Config",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://custom.api.com/v1",
				APIKey:  "test-key",
				RawURL:  true,
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://custom.api.com/v1/messages",
			expectedRawURL: true,
		},
		{
			name: "raw URL auto-enabled with # suffix",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://custom.api.com/v100#",
				APIKey:  "test-key",
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://custom.api.com/v100/messages",
			expectedRawURL: true,
		},
		{
			name: "raw URL with full path",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://custom.api.com/v1/messages#",
				APIKey:  "test-key",
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://custom.api.com/v1/messages/messages",
			expectedRawURL: true,
		},
		{
			name: "raw URL false with standard URL",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://api.anthropic.com",
				APIKey:  "test-key",
				RawURL:  false,
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://api.anthropic.com/v1/messages",
			expectedRawURL: false,
		},
		{
			name: "raw URL false with v1 already in URL",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://api.anthropic.com/v1",
				APIKey:  "test-key",
				RawURL:  false,
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://api.anthropic.com/v1/messages",
			expectedRawURL: false,
		},
		{
			name: "raw URL with custom endpoint without version",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://custom-endpoint.com/api/llm",
				APIKey:  "test-key",
				RawURL:  true,
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://custom-endpoint.com/api/llm/messages",
			expectedRawURL: true,
		},
		{
			name: "raw URL with streaming enabled",
			config: &Config{
				Type:    PlatformDirect,
				BaseURL: "https://custom.api.com/v1#",
				APIKey:  "test-key",
			},
			request: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
				Stream: func() *bool { v := true; return &v }(),
			},
			expectedURL:    "https://custom.api.com/v1/messages",
			expectedRawURL: true,
		},
		{
			name: "raw URL with DeepSeek platform",
			config: &Config{
				Type:    PlatformDeepSeek,
				BaseURL: "https://api.deepseek.com/v1#",
				APIKey:  "test-key",
			},
			request: &llm.Request{
				Model:     "deepseek-chat",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://api.deepseek.com/v1/messages",
			expectedRawURL: true,
		},
		{
			name: "raw URL with Doubao platform",
			config: &Config{
				Type:    PlatformDoubao,
				BaseURL: "https://ark.cn-beijing.volces.com/v20#",
				APIKey:  "test-key",
			},
			request: &llm.Request{
				Model:     "doubao-pro-4k",
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: func() *string { s := "Hello, world!"; return &s }(),
						},
					},
				},
			},
			expectedURL:    "https://ark.cn-beijing.volces.com/v20/messages",
			expectedRawURL: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transformerInterface, err := NewOutboundTransformerWithConfig(tt.config)
			if err != nil {
				t.Fatalf("Failed to create transformer: %v", err)
			}

			transformer := transformerInterface.(*OutboundTransformer)

			if transformer.config.RawURL != tt.expectedRawURL {
				t.Errorf("Expected RawURL to be %v, got %v", tt.expectedRawURL, transformer.config.RawURL)
			}

			result, err := transformer.TransformRequest(t.Context(), tt.request)
			if err != nil {
				t.Fatalf("TransformRequest() unexpected error = %v", err)
			}

			if result.URL != tt.expectedURL {
				t.Errorf("Expected URL %s, got %s", tt.expectedURL, result.URL)
			}
		})
	}
}
