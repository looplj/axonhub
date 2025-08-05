package anthropic

import (
	"encoding/json"
	"net/http"
	"testing"

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

func TestOutboundTransformer_AggregateStreamChunks(t *testing.T) {
	transformer := NewOutboundTransformer("", "")

	tests := []struct {
		name     string
		chunks   []*httpclient.StreamEvent
		expected string
	}{
		{
			name:     "empty chunks",
			chunks:   []*httpclient.StreamEvent{},
			expected: "",
		},
		{
			name: "single chunk",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte(`{
						"type": "message_start",
						"message": {
							"id": "msg_123",
							"type": "message",
							"role": "assistant",
							"content": [],
							"model": "claude-3-sonnet-20240229"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "content_block_delta",
						"index": 0,
						"delta": {
							"type": "text_delta",
							"text": "Hello!"
						}
					}`),
				},
				{
					Data: []byte(`{
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
			},
			expected: "Hello!",
		},
		{
			name: "multiple content chunks",
			chunks: []*httpclient.StreamEvent{
				{
					Data: []byte(`{
						"type": "message_start",
						"message": {
							"id": "msg_456",
							"type": "message",
							"role": "assistant",
							"content": [],
							"model": "claude-3-sonnet-20240229"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "content_block_delta",
						"index": 0,
						"delta": {
							"type": "text_delta",
							"text": "Hello"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "content_block_delta",
						"index": 0,
						"delta": {
							"type": "text_delta",
							"text": ", world!"
						}
					}`),
				},
				{
					Data: []byte(`{
						"type": "message_delta",
						"delta": {
							"stop_reason": "end_turn"
						}
					}`),
				},
			},
			expected: "Hello, world!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultBytes, err := transformer.AggregateStreamChunks(t.Context(), tt.chunks)
			require.NoError(t, err)
			require.NotNil(t, resultBytes)

			// Parse the response
			var result llm.Response

			err = json.Unmarshal(resultBytes, &result)
			require.NoError(t, err)

			if tt.expected == "" {
				require.Empty(t, result.Choices)
			} else {
				require.NotEmpty(t, result.Choices)
				require.Equal(t, tt.expected, *result.Choices[0].Message.Content.Content)
				require.Equal(t, "assistant", result.Choices[0].Message.Role)
			}
		})
	}
}

func TestOutboundTransformer_AggregateStreamChunks_EdgeCases(t *testing.T) {
	transformer := NewOutboundTransformer("", "")

	t.Run("Streaming edge cases", func(t *testing.T) {
		tests := []struct {
			name        string
			chunks      []*httpclient.StreamEvent
			expectError bool
			validate    func(t *testing.T, result *llm.Response)
			errorMsg    string
		}{
			{
				name:        "nil chunks",
				chunks:      nil,
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.Empty(t, result.Choices)
				},
			},
			{
				name: "chunks with invalid JSON",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_123",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{invalid json}`), // This should be skipped
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Hello"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "Hello", *result.Choices[0].Message.Content.Content)
				},
			},
			{
				name: "chunks with unknown event types",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_123",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "unknown_event",
							"some_field": "value"
						}`), // Should be skipped
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Hello"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "Hello", *result.Choices[0].Message.Content.Content)
				},
			},
			{
				name: "chunks missing message_start",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Hello"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					// Should handle gracefully, might have empty fields
				},
			},
			{
				name: "chunks with all event types",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_complete",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229",
								"usage": {"input_tokens": 5, "output_tokens": 0}
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "text",
								"text": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Complete"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_stop",
							"index": 0
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							},
							"usage": {"input_tokens": 5, "output_tokens": 8}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_stop"
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "Complete", *result.Choices[0].Message.Content.Content)
					require.Equal(t, "msg_complete", result.ID)
					require.Equal(t, "stop", *result.Choices[0].FinishReason)
					require.NotNil(t, result.Usage)
					require.Equal(t, 5, result.Usage.PromptTokens)
					require.Equal(t, 8, result.Usage.CompletionTokens)
				},
			},
			{
				name: "chunks with thinking blocks",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_thinking",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "thinking",
								"thinking": "Let me think about this..."
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "thinking_delta",
								"thinking": " some more"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 1,
							"content_block": {
								"type": "text",
								"text": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 1,
							"delta": {
								"type": "text_delta",
								"text": "Final answer"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					// Should contain both thinking and text content
					require.NotNil(t, result.Choices[0].Message.Content.MultipleContent)
					require.Len(t, result.Choices[0].Message.Content.MultipleContent, 2)
				},
			},
			{
				name: "chunks with tool use",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_tool",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "text",
								"text": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "I'll use a tool"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 1,
							"content_block": {
								"type": "tool_use",
								"id": "tool_123",
								"name": "calculator",
								"input": "{\"expression\": \"2+2\"}"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "tool_use"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
					require.NotNil(t, result.Choices[0].Message.ToolCalls)
					require.Len(t, result.Choices[0].Message.ToolCalls, 1)
					require.Equal(t, "tool_123", result.Choices[0].Message.ToolCalls[0].ID)
				},
			},
			{
				name: "chunks with partial JSON",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_partial",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "tool_use",
								"id": "tool_456",
								"name": "json_tool"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "input_json_delta",
								"partial_json": "{\"key\":"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "input_json_delta",
								"partial_json": "\"value\"}"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "tool_use"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "tool_calls", *result.Choices[0].FinishReason)
					require.NotNil(t, result.Choices[0].Message.ToolCalls)
					require.Len(t, result.Choices[0].Message.ToolCalls, 1)
					require.Equal(t, "tool_456", result.Choices[0].Message.ToolCalls[0].ID)
					require.Equal(
						t,
						`{"key":"value"}`,
						result.Choices[0].Message.ToolCalls[0].Function.Arguments,
					)
				},
			},
			{
				name: "chunks with ping events",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_ping",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "ping"
						}`), // Should be ignored
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "After ping"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "After ping", *result.Choices[0].Message.Content.Content)
				},
			},
			{
				name: "chunks with signature delta",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_sig",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_start",
							"index": 0,
							"content_block": {
								"type": "thinking",
								"thinking": ""
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "thinking_delta",
								"thinking": "Thinking..."
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "signature_delta",
								"signature": "abc123"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "end_turn"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					// Should handle signature delta gracefully
					require.NotNil(t, result.Choices[0].Message.Content.MultipleContent)
				},
			},
			{
				name: "chunks with multiple stop reasons",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(`{
							"type": "message_start",
							"message": {
								"id": "msg_multi_stop",
								"type": "message",
								"role": "assistant",
								"content": [],
								"model": "claude-3-sonnet-20240229"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "content_block_delta",
							"index": 0,
							"delta": {
								"type": "text_delta",
								"text": "Test"
							}
						}`),
					},
					{
						Data: []byte(`{
							"type": "message_delta",
							"delta": {
								"stop_reason": "max_tokens"
							}
						}`),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(t, "length", *result.Choices[0].FinishReason)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resultBytes, err := transformer.AggregateStreamChunks(t.Context(), tt.chunks)
				if tt.expectError {
					require.Error(t, err)

					if tt.errorMsg != "" {
						require.Contains(t, err.Error(), tt.errorMsg)
					}
				} else {
					require.NoError(t, err)
					// Parse the response
					var result llm.Response

					err = json.Unmarshal(resultBytes, &result)
					require.NoError(t, err)
					tt.validate(t, &result)
				}
			})
		}
	})
}

func TestOutboundTransformer_SetAPIKey(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	newAPIKey := "new-api-key"
	transformer.SetAPIKey(newAPIKey)
	require.Equal(t, newAPIKey, transformer.apiKey)
}

func TestOutboundTransformer_SetBaseURL(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	newBaseURL := "https://custom.api.com"
	transformer.SetBaseURL(newBaseURL)
	require.Equal(t, newBaseURL, transformer.baseURL)
}

func TestOutboundTransformer_ErrorHandling(t *testing.T) {
	transformer := NewOutboundTransformer("https://api.anthropic.com", "test-key")

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
				name: "HTTP error with error object",
				httpResp: &httpclient.Response{
					StatusCode: http.StatusTooManyRequests,
					Body:       []byte(`{"error": {"message": "Rate limit exceeded"}}`),
					Error: &httpclient.ResponseError{
						Message: "Rate limit exceeded",
						Type:    "rate_limit_error",
					},
				},
				expectError: true,
				errorMsg:    "HTTP error 429",
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

	t.Run("TransformStreamChunk error cases", func(t *testing.T) {
		tests := []struct {
			name        string
			event       *httpclient.StreamEvent
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil event",
				event:       nil,
				expectError: true,
				errorMsg:    "stream event is nil",
			},
			{
				name: "empty event data",
				event: &httpclient.StreamEvent{
					Type: "message_start",
					Data: []byte{},
				},
				expectError: true,
				errorMsg:    "event data is empty",
			},
			{
				name: "invalid JSON in event data",
				event: &httpclient.StreamEvent{
					Type: "message_start",
					Data: []byte(`{invalid json}`),
				},
				expectError: true,
				errorMsg:    "failed to unmarshal anthropic stream event",
			},
			{
				name: "malformed stream event structure",
				event: &httpclient.StreamEvent{
					Type: "message_start",
					Data: []byte(`{"message": {"id": 123}}`), // ID should be string
				},
				expectError: true,
				errorMsg:    "failed to unmarshal anthropic stream event",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformStreamChunk(t.Context(), tt.event)
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
	transformer := NewOutboundTransformer("", "")

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
					expectedSchema := map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type": "string",
							},
						},
						"required": []interface{}{"location"},
					}
					var actualSchema map[string]interface{}
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
	transformer := NewOutboundTransformer("", "")

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
				Messages: []MessageParam{
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
				System: &SystemPrompt{
					Prompt: func() *string { s := "You are helpful."; return &s }(),
				},
				Messages: []MessageParam{
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
			require.Equal(t, tt.expected.Model, result.Model)
			require.Equal(t, tt.expected.MaxTokens, result.MaxTokens)
			require.Equal(t, tt.expected.System, result.System)
			require.Equal(t, len(tt.expected.Messages), len(result.Messages))
		})
	}
}

func TestConvertToChatCompletionResponse(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	anthropicResp := &Message{
		ID:   "msg_123",
		Type: "message",
		Role: "assistant",
		Content: []ContentBlock{
			{
				Type: "text",
				Text: "Hello! How can I help you?",
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

	require.Equal(t, "msg_123", result.ID)
	require.Equal(t, "chat.completion", result.Object)
	require.Equal(t, "claude-3-sonnet-20240229", result.Model)
	require.Equal(t, 1, len(result.Choices))
	require.Equal(t, "assistant", result.Choices[0].Message.Role)
	require.Equal(t, "Hello! How can I help you?", *result.Choices[0].Message.Content.Content)
	require.Equal(t, "stop", *result.Choices[0].FinishReason)
	require.Equal(t, 10, result.Usage.PromptTokens)
	require.Equal(t, 20, result.Usage.CompletionTokens)
	require.Equal(t, 30, result.Usage.TotalTokens)
}

func TestConvertToChatCompletionResponse_EdgeCases(t *testing.T) {
	transformer := NewOutboundTransformer("", "").(*OutboundTransformer)

	tests := []struct {
		name     string
		input    *Message
		validate func(t *testing.T, result *llm.Response)
	}{
		{
			name:  "nil response",
			input: nil,
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				// Should handle nil gracefully or panic appropriately
				if result != nil {
					require.Empty(t, result.ID)
					require.Empty(t, result.Choices)
				}
			},
		},
		{
			name: "empty content blocks",
			input: &Message{
				ID:      "msg_empty",
				Type:    "message",
				Role:    "assistant",
				Content: []ContentBlock{},
				Model:   "claude-3-sonnet-20240229",
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_empty", result.ID)
				require.Equal(t, "chat.completion", result.Object)
				require.NotNil(t, result.Choices)
				if len(result.Choices) > 0 {
					require.Nil(t, result.Choices[0].Message.Content.Content)
					require.Empty(t, result.Choices[0].Message.Content.MultipleContent)
				}
			},
		},
		{
			name: "multiple text content blocks",
			input: &Message{
				ID:   "msg_multi",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Hello"},
					{Type: "text", Text: " world!"},
					{Type: "text", Text: " How are you?"},
				},
				Model: "claude-3-sonnet-20240229",
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_multi", result.ID)
				require.NotNil(t, result.Choices[0].Message.Content.Content)
				require.Equal(
					t,
					"Hello world! How are you?",
					*result.Choices[0].Message.Content.Content,
				)
			},
		},
		{
			name: "mixed content types",
			input: &Message{
				ID:   "msg_mixed",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Check this image: "},
					{Type: "image", Source: &ImageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      "/9j/4AAQSkZJRg==",
					}},
					{Type: "text", Text: " and this text"},
				},
				Model: "claude-3-sonnet-20240229",
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_mixed", result.ID)
				require.Nil(
					t,
					result.Choices[0].Message.Content.Content,
				) // Should use MultipleContent for mixed types
				require.Len(t, result.Choices[0].Message.Content.MultipleContent, 3)
			},
		},
		{
			name: "tool use content",
			input: &Message{
				ID:   "msg_tool",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{
						Type: "text",
						Text: "I'll help you with that calculation.",
					},
					{
						Type:  "tool_use",
						ID:    "tool_123",
						Name:  func() *string { s := "calculator"; return &s }(),
						Input: json.RawMessage(`{"expression": "2+2"}`),
					},
				},
				Model: "claude-3-sonnet-20240229",
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_tool", result.ID)
				require.NotNil(t, result.Choices[0].Message.ToolCalls)
				require.Len(t, result.Choices[0].Message.ToolCalls, 1)
				require.Equal(t, "tool_123", result.Choices[0].Message.ToolCalls[0].ID)
				require.Equal(t, "calculator", result.Choices[0].Message.ToolCalls[0].Function.Name)
				require.Equal(
					t,
					`{"expression": "2+2"}`,
					result.Choices[0].Message.ToolCalls[0].Function.Arguments,
				)
			},
		},
		{
			name: "all stop reasons",
			input: func() *Message {
				return &Message{
					ID:      "msg_stop",
					Type:    "message",
					Role:    "assistant",
					Content: []ContentBlock{{Type: "text", Text: "Test"}},
					Model:   "claude-3-sonnet-20240229",
				}
			}(),
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				// Test each stop reason
				stopReasons := map[string]string{
					"end_turn":      "stop",
					"max_tokens":    "length",
					"stop_sequence": "stop",
					"tool_use":      "tool_calls",
					"pause_turn":    "pause_turn",
					"refusal":       "refusal",
				}

				for anthropicReason, expectedReason := range stopReasons {
					msg := &Message{
						ID:         "msg_stop",
						Type:       "message",
						Role:       "assistant",
						Content:    []ContentBlock{{Type: "text", Text: "Test"}},
						Model:      "claude-3-sonnet-20240229",
						StopReason: func() *string { s := anthropicReason; return &s }(),
					}

					result := transformer.convertToChatCompletionResponse(msg)
					if expectedReason == "stop" {
						require.Equal(t, expectedReason, *result.Choices[0].FinishReason)
					} else {
						require.Equal(t, expectedReason, *result.Choices[0].FinishReason)
					}
				}
			},
		},
		{
			name: "usage with cache tokens",
			input: &Message{
				ID:   "msg_cache",
				Type: "message",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "Cached response"},
				},
				Model: "claude-3-sonnet-20240229",
				Usage: &Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     30,
					ServiceTier:              "standard",
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_cache", result.ID)
				require.Equal(t, 100, result.Usage.PromptTokens)
				require.Equal(t, 50, result.Usage.CompletionTokens)
				require.Equal(t, 150, result.Usage.TotalTokens)
				// Cache tokens should be included in input tokens
			},
		},
		{
			name: "nil usage",
			input: &Message{
				ID:      "msg_nusage",
				Type:    "message",
				Role:    "assistant",
				Content: []ContentBlock{{Type: "text", Text: "No usage"}},
				Model:   "claude-3-sonnet-20240229",
				Usage:   nil,
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_nusage", result.ID)
				require.Nil(t, result.Usage)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.convertToChatCompletionResponse(tt.input)
			tt.validate(t, result)
		})
	}
}
