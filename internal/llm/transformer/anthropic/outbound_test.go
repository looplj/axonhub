package anthropic

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
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
		name      string
		chunks    []*httpclient.StreamEvent
		expected  string
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name:     "empty chunks",
			chunks:   []*httpclient.StreamEvent{},
			expected: "",
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.ErrorContains(t, err, "empty stream chunks")
			},
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
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.NoError(t, err)
			},
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
			assertErr: func(t assert.TestingT, err error, args ...interface{}) bool {
				return assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultBytes, err := transformer.AggregateStreamChunks(t.Context(), tt.chunks)
			tt.assertErr(t, err)

			if tt.expected == "" {
				if err == nil {
					var result llm.Response

					err := json.Unmarshal(resultBytes, &result)
					require.NoError(t, err)
					require.Empty(t, result.Choices)
				}
			} else {
				require.NotNil(t, resultBytes)

				var result llm.Response

				err := json.Unmarshal(resultBytes, &result)
				require.NoError(t, err)
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
				expectError: true,
				errorMsg:    "empty stream chunks",
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
				name: "chunks with detailed usage information",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(
							`{"type": "message_start", "message": {"id": "msg_detailed_usage", "type": "message", "role": "assistant", "content": [], "model": "claude-3-sonnet-20240229", "usage": {"input_tokens": 100, "output_tokens": 0, "cache_creation_input_tokens": 20, "cache_read_input_tokens": 50}}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Response with detailed usage"}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"input_tokens": 100, "output_tokens": 25, "cache_creation_input_tokens": 20, "cache_read_input_tokens": 50}}`,
						),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(
						t,
						"Response with detailed usage",
						*result.Choices[0].Message.Content.Content,
					)
					require.Equal(t, "msg_detailed_usage", result.ID)
					require.Equal(t, "stop", *result.Choices[0].FinishReason)
					require.NotNil(t, result.Usage)
					require.Equal(t, 100, result.Usage.PromptTokens)
					require.Equal(t, 25, result.Usage.CompletionTokens)
					require.Equal(t, 125, result.Usage.TotalTokens)
					// Verify detailed token information
					require.NotNil(t, result.Usage.PromptTokensDetails)
					require.Equal(t, 50, result.Usage.PromptTokensDetails.CachedTokens)
					require.NotNil(t, result.Usage.CompletionTokensDetails)
					require.Equal(t, 0, result.Usage.CompletionTokensDetails.ReasoningTokens)
				},
			},
			{
				name: "chunks with usage but no cache tokens",
				chunks: []*httpclient.StreamEvent{
					{
						Data: []byte(
							`{"type": "message_start", "message": {"id": "msg_no_cache_stream", "type": "message", "role": "assistant", "content": [], "model": "claude-3-sonnet-20240229"}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "No cache response"}}`,
						),
					},
					{
						Data: []byte(
							`{"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"input_tokens": 80, "output_tokens": 40, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0}}`,
						),
					},
				},
				expectError: false,
				validate: func(t *testing.T, result *llm.Response) {
					t.Helper()
					require.NotNil(t, result)
					require.NotEmpty(t, result.Choices)
					require.Equal(
						t,
						"No cache response",
						*result.Choices[0].Message.Content.Content,
					)
					require.Equal(t, "msg_no_cache_stream", result.ID)
					require.Equal(t, "stop", *result.Choices[0].FinishReason)
					require.NotNil(t, result.Usage)
					require.Equal(t, 80, result.Usage.PromptTokens)
					require.Equal(t, 40, result.Usage.CompletionTokens)
					require.Equal(t, 120, result.Usage.TotalTokens)
					// Verify no detailed token information when cache tokens are 0
					require.Nil(t, result.Usage.PromptTokensDetails)
					require.NotNil(t, result.Usage.CompletionTokensDetails)
					require.Equal(t, 0, result.Usage.CompletionTokensDetails.ReasoningTokens)
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
	transformer := NewOutboundTransformer("", "")

	newAPIKey := "new-api-key"
	transformer.SetAPIKey(newAPIKey)
	require.Equal(t, newAPIKey, transformer.apiKey)
}

func TestOutboundTransformer_SetBaseURL(t *testing.T) {
	transformer := NewOutboundTransformer("", "")

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
	transformer := NewOutboundTransformer("", "")

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

func TestOutboundTransformer_StreamTransformation_WithTestData_Stop(t *testing.T) {
	transformer := NewOutboundTransformer("", "")

	// Load test data from files
	anthropicData, err := os.ReadFile("testdata/anthropic-stop.stream.jsonl")
	require.NoError(t, err)

	expectedData, err := os.ReadFile("testdata/response-stop.stream.jsonl")
	require.NoError(t, err)

	// Parse anthropic stream events
	anthropicLines := strings.Split(strings.TrimSpace(string(anthropicData)), "\n")

	var streamEvents []*httpclient.StreamEvent

	for _, line := range anthropicLines {
		if line != "" {
			var event struct {
				Type string `json:"Type"`
				Data string `json:"Data"`
			}

			err := json.Unmarshal([]byte(line), &event)
			require.NoError(t, err)

			streamEvents = append(streamEvents, &httpclient.StreamEvent{
				Type: event.Type,
				Data: []byte(event.Data),
			})
		}
	}

	// Parse expected responses
	expectedLines := strings.Split(strings.TrimSpace(string(expectedData)), "\n")

	var expectedResponses []*llm.Response

	for _, line := range expectedLines {
		if line != "" {
			// Check if this is a DONE event
			if strings.Contains(line, `"Data":"[DONE]"`) {
				// This is a DONE event, add the DoneResponse
				expectedResponses = append(expectedResponses, llm.DoneResponse)
			} else {
				// Parse the StreamEvent to get the Data field
				var event struct {
					Type string `json:"Type"`
					Data string `json:"Data"`
				}

				err := json.Unmarshal([]byte(line), &event)
				require.NoError(t, err)

				// Parse the Data field as llm.Response
				var resp llm.Response

				err = json.Unmarshal([]byte(event.Data), &resp)
				require.NoError(t, err)

				expectedResponses = append(expectedResponses, &resp)
			}
		}
	}

	// Create a mock stream
	mockStream := &mockStreamEvent{
		events: streamEvents,
		index:  0,
	}

	// Transform the stream
	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	// Collect all transformed responses
	var actualResponses []*llm.Response

	for transformedStream.Next() {
		resp := transformedStream.Current()
		actualResponses = append(actualResponses, resp)
	}

	require.NoError(t, transformedStream.Err())

	// Verify the number of responses matches
	require.Equal(t, len(expectedResponses), len(actualResponses), "Number of responses should match")

	// Verify each response
	for i, expected := range expectedResponses {
		actual := actualResponses[i]

		// Verify basic fields
		assert.Equal(t, expected.ID, actual.ID, "Response %d: ID should match", i)
		assert.Equal(t, expected.Object, actual.Object, "Response %d: Object should match", i)
		assert.Equal(t, expected.Model, actual.Model, "Response %d: Model should match", i)
		assert.Equal(t, expected.Created, actual.Created, "Response %d: Created should match", i)

		// Verify choices
		assert.Equal(t, len(expected.Choices), len(actual.Choices), "Response %d: Number of choices should match", i)

		if len(expected.Choices) > 0 && len(actual.Choices) > 0 {
			expectedChoice := expected.Choices[0]
			actualChoice := actual.Choices[0]

			assert.Equal(t, expectedChoice.Index, actualChoice.Index, "Response %d: Choice index should match", i)
			assert.Equal(t, expectedChoice.FinishReason, actualChoice.FinishReason, "Response %d: Finish reason should match", i)

			// Verify delta content
			if expectedChoice.Delta != nil && actualChoice.Delta != nil {
				assert.Equal(t, expectedChoice.Delta.Role, actualChoice.Delta.Role, "Response %d: Delta role should match", i)

				if expectedChoice.Delta.Content.Content != nil && actualChoice.Delta.Content.Content != nil {
					assert.Equal(t, *expectedChoice.Delta.Content.Content, *actualChoice.Delta.Content.Content, "Response %d: Delta content should match", i)
				}
			}
		}

		// Verify usage information
		if expected.Usage != nil && actual.Usage != nil {
			assert.Equal(t, expected.Usage.PromptTokens, actual.Usage.PromptTokens, "Response %d: Prompt tokens should match", i)
			assert.Equal(t, expected.Usage.CompletionTokens, actual.Usage.CompletionTokens, "Response %d: Completion tokens should match", i)
			assert.Equal(t, expected.Usage.TotalTokens, actual.Usage.TotalTokens, "Response %d: Total tokens should match", i)
		}
	}

	// Test aggregation as well
	aggregatedBytes, err := transformer.AggregateStreamChunks(t.Context(), streamEvents)
	require.NoError(t, err)

	var aggregatedResp llm.Response

	err = json.Unmarshal(aggregatedBytes, &aggregatedResp)
	require.NoError(t, err)

	// Verify aggregated response
	assert.Equal(t, "msg_bdrk_01Fbg5HKuVfmtT6mAMxQoCSn", aggregatedResp.ID)
	assert.Equal(t, "chat.completion", aggregatedResp.Object)
	assert.Equal(t, "claude-3-7-sonnet-20250219", aggregatedResp.Model)
	assert.NotEmpty(t, aggregatedResp.Choices)
	assert.Equal(t, "assistant", aggregatedResp.Choices[0].Message.Role)

	// Verify the complete content
	expectedContent := "1 2 3 4 5\n6 7 8 9 10\n11 12 13 14 15\n16 17 18 19 20"
	assert.Equal(t, expectedContent, *aggregatedResp.Choices[0].Message.Content.Content)
}

// mockStreamEvent implements streams.Stream[*httpclient.StreamEvent] for testing.
type mockStreamEvent struct {
	events []*httpclient.StreamEvent
	index  int
	err    error
}

func (m *mockStreamEvent) Next() bool {
	return m.index < len(m.events)
}

func (m *mockStreamEvent) Current() *httpclient.StreamEvent {
	if m.index < len(m.events) {
		event := m.events[m.index]
		m.index++

		return event
	}

	return nil
}

func (m *mockStreamEvent) Err() error {
	return m.err
}

func (m *mockStreamEvent) Close() error {
	return nil
}
