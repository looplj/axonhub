package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

func TestInboundTransformer_TransformRequest(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		httpReq     *httpclient.Request
		expected    *llm.Request
		expectError bool
	}{
		{
			name: "valid simple text request",
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": "Hello, Claude!"
						}
					]
				}`),
			},
			expected: &llm.Request{
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
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"system": "You are a helpful assistant.",
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expected: &llm.Request{
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
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": [
								{
									"type": "text",
									"text": "What's in this image?"
								},
								{
									"type": "image",
									"source": {
										"type": "base64",
										"media_type": "image/jpeg",
										"data": "/9j/4AAQSkZJRg..."
									}
								}
							]
						}
					]
				}`),
			},
			expected: &llm.Request{
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
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"temperature": 0.7,
					"stop_sequences": ["Human:", "Assistant:"],
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expected: &llm.Request{
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
			name:        "nil request",
			httpReq:     nil,
			expectError: true,
		},
		{
			name: "empty body",
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte{},
			},
			expectError: true,
		},
		{
			name: "missing model",
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expectError: true,
		},
		{
			name: "missing max_tokens",
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expectError: true,
		},
		{
			name: "missing messages",
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024
				}`),
			},
			expectError: true,
		},
		{
			name: "invalid content type",
			httpReq: &httpclient.Request{
				Headers: http.Header{
					"Content-Type": []string{"text/plain"},
				},
				Body: []byte(`{
					"model": "claude-3-sonnet-20240229",
					"max_tokens": 1024,
					"messages": [
						{
							"role": "user",
							"content": "Hello!"
						}
					]
				}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformRequest(t.Context(), tt.httpReq)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tt.expected.Model, result.Model)
				require.Equal(t, tt.expected.MaxTokens, result.MaxTokens)
				require.Equal(t, tt.expected.Temperature, result.Temperature)
				require.Equal(t, len(tt.expected.Messages), len(result.Messages))

				for i, expectedMsg := range tt.expected.Messages {
					require.Equal(t, expectedMsg.Role, result.Messages[i].Role)
					require.Equal(t, expectedMsg.Content.Content, result.Messages[i].Content.Content)
					require.Equal(t, len(expectedMsg.Content.MultipleContent), len(result.Messages[i].Content.MultipleContent))
				}

				if tt.expected.Stop != nil {
					require.NotNil(t, result.Stop)
					require.Equal(t, tt.expected.Stop.Stop, result.Stop.Stop)
					require.Equal(t, tt.expected.Stop.MultipleStop, result.Stop.MultipleStop)
				}
			}
		})
	}
}

func TestInboundTransformer_TransformResponse(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, resp *Message)
	}{
		{
			name: "valid response",
			chatResp: &llm.Response{
				ID:      "msg_123",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: func() *string { s := "Hello! How can I help you?"; return &s }(),
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "Hello! How can I help you?", resp.Content[0].Text)
				require.Equal(t, "end_turn", *resp.StopReason)
			},
		},
		{
			name: "response with multimodal content",
			chatResp: &llm.Response{
				ID:      "msg_456",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: func() *string { s := "I can see an image."; return &s }(),
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "I can see an image.", resp.Content[0].Text)
			},
		},
		{
			name: "response with image content",
			chatResp: &llm.Response{
				ID:      "msg_image_123",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: func() *string { s := "Here's an image for you:"; return &s }(),
									},
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAYABgAAD//gA7Q1JFQVR",
										},
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 2)

				// First content block should be text
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "Here's an image for you:", resp.Content[0].Text)

				// Second content block should be image
				require.Equal(t, "image", resp.Content[1].Type)
				require.NotNil(t, resp.Content[1].Source)
				require.Equal(t, "base64", resp.Content[1].Source.Type)
				require.Equal(t, "image/jpeg", resp.Content[1].Source.MediaType)
				require.Equal(t, "/9j/4AAQSkZJRgABAQEAYABgAAD//gA7Q1JFQVR", resp.Content[1].Source.Data)
			},
		},
		{
			name: "response with multiple images and text",
			chatResp: &llm.Response{
				ID:      "msg_multi_image_456",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: func() *string { s := "Here are two different images:"; return &s }(),
									},
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
										},
									},
									{
										Type: "text",
										Text: func() *string { s := "and"; return &s }(),
									},
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "data:image/webp;base64,UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
										},
									},
									{
										Type: "text",
										Text: func() *string { s := "Both images show different content."; return &s }(),
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 5)

				// First content block should be text
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "Here are two different images:", resp.Content[0].Text)

				// Second content block should be PNG image
				require.Equal(t, "image", resp.Content[1].Type)
				require.NotNil(t, resp.Content[1].Source)
				require.Equal(t, "base64", resp.Content[1].Source.Type)
				require.Equal(t, "image/png", resp.Content[1].Source.MediaType)
				require.Equal(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==", resp.Content[1].Source.Data)

				// Third content block should be text
				require.Equal(t, "text", resp.Content[2].Type)
				require.Equal(t, "and", resp.Content[2].Text)

				// Fourth content block should be WebP image
				require.Equal(t, "image", resp.Content[3].Type)
				require.NotNil(t, resp.Content[3].Source)
				require.Equal(t, "base64", resp.Content[3].Source.Type)
				require.Equal(t, "image/webp", resp.Content[3].Source.MediaType)
				require.Equal(t, "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA", resp.Content[3].Source.Data)

				// Fifth content block should be text
				require.Equal(t, "text", resp.Content[4].Type)
				require.Equal(t, "Both images show different content.", resp.Content[4].Text)
			},
		},
		{
			name: "response with thinking, text, and image content",
			chatResp: &llm.Response{
				ID:      "msg_think_image_789",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ReasoningContent: func() *string {
								s := "I should analyze this image and provide a helpful response with a visual example."
								return &s
							}(),
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: func() *string { s := "Based on my analysis, here's a relevant image:"; return &s }(),
									},
									{
										Type: "image_url",
										ImageURL: &llm.ImageURL{
											URL: "data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
										},
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 3)

				// First content block should be thinking
				require.Equal(t, "thinking", resp.Content[0].Type)
				require.Equal(t, "I should analyze this image and provide a helpful response with a visual example.", resp.Content[0].Thinking)

				// Second content block should be text
				require.Equal(t, "text", resp.Content[1].Type)
				require.Equal(t, "Based on my analysis, here's a relevant image:", resp.Content[1].Text)

				// Third content block should be GIF image
				require.Equal(t, "image", resp.Content[2].Type)
				require.NotNil(t, resp.Content[2].Source)
				require.Equal(t, "base64", resp.Content[2].Source.Type)
				require.Equal(t, "image/gif", resp.Content[2].Source.MediaType)
				require.Equal(t, "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7", resp.Content[2].Source.Data)
			},
		},
		{
			name: "response with thinking content",
			chatResp: &llm.Response{
				ID:      "msg_789",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ReasoningContent: func() *string {
								s := "Let me think about this step by step. First, I need to understand the problem..."
								return &s
							}(),
							Content: llm.MessageContent{
								Content: func() *string { s := "Based on my analysis, the answer is 42."; return &s }(),
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 2)

				// First content block should be thinking
				require.Equal(t, "thinking", resp.Content[0].Type)
				require.Equal(t, "Let me think about this step by step. First, I need to understand the problem...", resp.Content[0].Thinking)

				// Second content block should be text
				require.Equal(t, "text", resp.Content[1].Type)
				require.Equal(t, "Based on my analysis, the answer is 42.", resp.Content[1].Text)
			},
		},
		{
			name: "response with tool calls",
			chatResp: &llm.Response{
				ID:      "msg_tool_123",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: func() *string { s := "I'll help you with that calculation."; return &s }(),
							},
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "calculate",
										Arguments: `{"operation": "add", "a": 5, "b": 3}`,
									},
								},
							},
						},
						FinishReason: func() *string { s := "tool_calls"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 2)

				// First content block should be text
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "I'll help you with that calculation.", resp.Content[0].Text)

				// Second content block should be tool_use
				require.Equal(t, "tool_use", resp.Content[1].Type)
				require.Equal(t, "call_123", resp.Content[1].ID)
				require.Equal(t, "calculate", *resp.Content[1].Name)

				// Verify tool input JSON
				var input map[string]interface{}
				err := json.Unmarshal(resp.Content[1].Input, &input)
				require.NoError(t, err)
				require.Equal(t, "add", input["operation"])
				require.Equal(t, float64(5), input["a"])
				require.Equal(t, float64(3), input["b"])

				require.Equal(t, "tool_use", *resp.StopReason)
			},
		},
		{
			name: "response with thinking and tool calls",
			chatResp: &llm.Response{
				ID:      "msg_think_tool_456",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ReasoningContent: func() *string {
								s := "The user wants me to calculate something. I should use the calculator tool."
								return &s
							}(),
							Content: llm.MessageContent{
								Content: func() *string { s := "Let me calculate that for you."; return &s }(),
							},
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_456",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "multiply",
										Arguments: `{"x": 7, "y": 8}`,
									},
								},
								{
									ID:   "call_789",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "format_result",
										Arguments: `{"value": 56, "format": "decimal"}`,
									},
								},
							},
						},
						FinishReason: func() *string { s := "tool_calls"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 4)

				// First content block should be thinking
				require.Equal(t, "thinking", resp.Content[0].Type)
				require.Equal(t, "The user wants me to calculate something. I should use the calculator tool.", resp.Content[0].Thinking)

				// Second content block should be text
				require.Equal(t, "text", resp.Content[1].Type)
				require.Equal(t, "Let me calculate that for you.", resp.Content[1].Text)

				// Third content block should be first tool_use
				require.Equal(t, "tool_use", resp.Content[2].Type)
				require.Equal(t, "call_456", resp.Content[2].ID)
				require.Equal(t, "multiply", *resp.Content[2].Name)

				// Fourth content block should be second tool_use
				require.Equal(t, "tool_use", resp.Content[3].Type)
				require.Equal(t, "call_789", resp.Content[3].ID)
				require.Equal(t, "format_result", *resp.Content[3].Name)
			},
		},
		{
			name: "response with empty tool arguments",
			chatResp: &llm.Response{
				ID:      "msg_empty_args",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_empty",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "get_time",
										Arguments: "", // Empty arguments
									},
								},
							},
						},
						FinishReason: func() *string { s := "tool_calls"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "tool_use", resp.Content[0].Type)
				require.Equal(t, "call_empty", resp.Content[0].ID)
				require.Equal(t, "get_time", *resp.Content[0].Name)

				// Should default to empty JSON object
				require.Equal(t, json.RawMessage("{}"), resp.Content[0].Input)
			},
		},
		{
			name: "response with different finish reasons",
			chatResp: &llm.Response{
				ID:      "msg_length",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: func() *string { s := "This is a long response that was cut off..."; return &s }(),
							},
						},
						FinishReason: func() *string { s := "length"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Equal(t, "max_tokens", *resp.StopReason)
			},
		},
		{
			name: "response with usage details",
			chatResp: &llm.Response{
				ID:      "msg_usage",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: func() *string { s := "Response with detailed usage."; return &s }(),
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
				Usage: &llm.Usage{
					PromptTokens:     100,
					CompletionTokens: 50,
					TotalTokens:      150,
					PromptTokensDetails: &llm.PromptTokensDetails{
						CachedTokens: 20,
					},
					CompletionTokensDetails: &llm.CompletionTokensDetails{
						ReasoningTokens: 10,
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.NotNil(t, resp.Usage)
				require.Equal(t, int64(100), resp.Usage.InputTokens)
				require.Equal(t, int64(50), resp.Usage.OutputTokens)
				require.Equal(t, int64(20), resp.Usage.CacheReadInputTokens)
			},
		},
		{
			name:        "nil response",
			chatResp:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(t.Context(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.NotEmpty(t, result.Body)

				// Verify the response can be unmarshaled to AnthropicResponse
				var anthropicResp Message

				err := json.Unmarshal(result.Body, &anthropicResp)
				require.NoError(t, err)
				require.Equal(t, tt.chatResp.ID, anthropicResp.ID)
				require.Equal(t, "message", anthropicResp.Type)
				require.Equal(t, "assistant", anthropicResp.Role)
				require.Equal(t, tt.chatResp.Model, anthropicResp.Model)

				// Run custom validation if provided
				if tt.validate != nil {
					tt.validate(t, &anthropicResp)
				}
			}
		})
	}
}

func TestInboundTransformer_ErrorHandling(t *testing.T) {
	transformer := NewInboundTransformer()

	t.Run("TransformRequest error cases", func(t *testing.T) {
		tests := []struct {
			name        string
			httpReq     *httpclient.Request
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil request",
				httpReq:     nil,
				expectError: true,
				errorMsg:    "http request is nil",
			},
			{
				name: "empty body",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte{},
				},
				expectError: true,
				errorMsg:    "request body is empty",
			},
			{
				name: "invalid content type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"text/plain"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": []}`,
					),
				},
				expectError: true,
				errorMsg:    "unsupported content type",
			},
			{
				name: "no content type header",
				httpReq: &httpclient.Request{
					Headers: http.Header{},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": []}`,
					),
				},
				expectError: true,
				errorMsg:    "unsupported content type",
			},
			{
				name: "invalid JSON",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(`{invalid json}`),
				},
				expectError: true,
				errorMsg:    "failed to decode anthropic request",
			},
			{
				name: "missing model field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "model is required",
			},
			{
				name: "empty model field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "", "max_tokens": 1024, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "model is required",
			},
			{
				name: "missing messages field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024}`),
				},
				expectError: true,
				errorMsg:    "messages are required",
			},
			{
				name: "empty messages array",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": []}`,
					),
				},
				expectError: true,
				errorMsg:    "messages are required",
			},
			{
				name: "invalid max_tokens (negative)",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": -1, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "max_tokens is required and must be positive",
			},
			{
				name: "invalid max_tokens (zero)",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 0, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "max_tokens is required and must be positive",
			},
			{
				name: "missing max_tokens field",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true,
				errorMsg:    "max_tokens is required and must be positive",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformRequest(t.Context(), tt.httpReq)
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
			chatResp    *llm.Response
			expectError bool
			errorMsg    string
		}{
			{
				name:        "nil response",
				chatResp:    nil,
				expectError: true,
				errorMsg:    "chat completion response is nil",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformResponse(t.Context(), tt.chatResp)
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

func TestInboundTransformer_ValidationEdgeCases(t *testing.T) {
	transformer := NewInboundTransformer()

	t.Run("Message content validation", func(t *testing.T) {
		tests := []struct {
			name        string
			httpReq     *httpclient.Request
			expectError bool
		}{
			{
				name: "null content in message",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": [{"role": "user", "content": null}]}`,
					),
				},
				expectError: true, // Should error on null content
			},
			{
				name: "invalid content type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "messages": [{"role": "user", "content": 123}]}`,
					),
				},
				expectError: true, // Should error on invalid content type
			},
			{
				name: "invalid system prompt type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "system": 123, "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true, // Should error on invalid system type
			},
			{
				name: "invalid system prompt array type",
				httpReq: &httpclient.Request{
					Headers: http.Header{
						"Content-Type": []string{"application/json"},
					},
					Body: []byte(
						`{"model": "claude-3-sonnet-20240229", "max_tokens": 1024, "system": [{"type": "invalid"}], "messages": [{"role": "user", "content": "Hello"}]}`,
					),
				},
				expectError: true, // Should error on invalid system prompt array
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := transformer.TransformRequest(t.Context(), tt.httpReq)
				if tt.expectError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestInboundTransformer_TransformError(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name     string
		llmErr   *llm.ResponseError
		expected *httpclient.Error
	}{
		{
			name: "generic error",
			llmErr: &llm.ResponseError{
				Detail: llm.ErrorDetail{
					Message:   "some error",
					Type:      "test_error",
					RequestID: "123456",
				},
			},
			expected: &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     "Internal Server Error",
				Body:       []byte(`{"message":"some error","request_id":"123456"}`),
			},
		},
		{
			name:   "nil error",
			llmErr: nil,
			expected: &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Status:     "Internal Server Error",
				Body:       []byte(`{"message":"internal server error","request_id":""}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformer.TransformError(context.Background(), tt.llmErr)
			require.NotNil(t, result)
			require.Equal(t, tt.expected.StatusCode, result.StatusCode)
			require.JSONEq(t, string(tt.expected.Body), string(result.Body))
		})
	}
}

func TestAnthropicMessageContent_MarshalUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		content MessageContent
		jsonStr string
	}{
		{
			name: "string content",
			content: MessageContent{
				Content: func() *string { s := "Hello, world!"; return &s }(),
			},
			jsonStr: `"Hello, world!"`,
		},
		{
			name: "array content",
			content: MessageContent{
				MultipleContent: []ContentBlock{
					{
						Type: "text",
						Text: "Hello",
					},
				},
			},
			jsonStr: `[{"type":"text","text":"Hello"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Marshal
			data, err := json.Marshal(tt.content)
			require.NoError(t, err)
			require.JSONEq(t, tt.jsonStr, string(data))

			// Test Unmarshal
			var content MessageContent

			err = json.Unmarshal([]byte(tt.jsonStr), &content)
			require.NoError(t, err)
			require.Equal(t, tt.content.Content, content.Content)
			require.Equal(t, len(tt.content.MultipleContent), len(content.MultipleContent))
		})
	}
}

func TestInboundTransformer_TransformResponse_EdgeCases(t *testing.T) {
	transformer := NewInboundTransformer()

	tests := []struct {
		name        string
		chatResp    *llm.Response
		expectError bool
		validate    func(t *testing.T, resp *Message)
	}{
		{
			name: "response with only thinking content",
			chatResp: &llm.Response{
				ID:      "msg_only_thinking",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ReasoningContent: func() *string {
								s := "I need to think about this carefully..."
								return &s
							}(),
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "thinking", resp.Content[0].Type)
				require.Equal(t, "I need to think about this carefully...", resp.Content[0].Thinking)
			},
		},
		{
			name: "response with only tool calls",
			chatResp: &llm.Response{
				ID:      "msg_only_tools",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_only",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "search",
										Arguments: `{"query": "test"}`,
									},
								},
							},
						},
						FinishReason: func() *string { s := "tool_calls"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "tool_use", resp.Content[0].Type)
				require.Equal(t, "call_only", resp.Content[0].ID)
				require.Equal(t, "search", *resp.Content[0].Name)
			},
		},
		{
			name: "response with empty thinking content",
			chatResp: &llm.Response{
				ID:      "msg_empty_thinking",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ReasoningContent: func() *string {
								s := ""
								return &s
							}(),
							Content: llm.MessageContent{
								Content: func() *string { s := "Direct answer."; return &s }(),
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				// Empty thinking content should be ignored
				require.Len(t, resp.Content, 1)
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "Direct answer.", resp.Content[0].Text)
			},
		},
		{
			name: "response with no choices",
			chatResp: &llm.Response{
				ID:      "msg_no_choices",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Empty(t, resp.Content)
				require.Nil(t, resp.StopReason)
			},
		},
		{
			name: "response with choice but no message",
			chatResp: &llm.Response{
				ID:      "msg_no_message",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index:        0,
						Message:      nil,
						Delta:        nil,
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Empty(t, resp.Content)
				require.Equal(t, "end_turn", *resp.StopReason)
			},
		},
		{
			name: "response with delta instead of message",
			chatResp: &llm.Response{
				ID:      "msg_delta",
				Object:  "chat.completion.chunk",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Delta: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: func() *string { s := "Delta content"; return &s }(),
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "Delta content", resp.Content[0].Text)
			},
		},
		{
			name: "response with unknown finish reason",
			chatResp: &llm.Response{
				ID:      "msg_unknown_finish",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								Content: func() *string { s := "Some content"; return &s }(),
							},
						},
						FinishReason: func() *string { s := "unknown_reason"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Equal(t, "unknown_reason", *resp.StopReason)
			},
		},
		{
			name: "response with malformed tool arguments",
			chatResp: &llm.Response{
				ID:      "msg_malformed_args",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							ToolCalls: []llm.ToolCall{
								{
									ID:   "call_malformed",
									Type: "function",
									Function: llm.FunctionCall{
										Name:      "test_func",
										Arguments: `{"invalid": json}`, // Invalid JSON
									},
								},
							},
						},
						FinishReason: func() *string { s := "tool_calls"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				require.Len(t, resp.Content, 1)
				require.Equal(t, "tool_use", resp.Content[0].Type)
				require.Equal(t, "call_malformed", resp.Content[0].ID)
				require.Equal(t, "test_func", *resp.Content[0].Name)

				// Malformed JSON should be wrapped in raw_arguments field
				var input map[string]interface{}
				err := json.Unmarshal(resp.Content[0].Input, &input)
				require.NoError(t, err)
				require.Equal(t, `{"invalid": json}`, input["raw_arguments"])
			},
		},
		{
			name: "response with multiple content parts including non-text",
			chatResp: &llm.Response{
				ID:      "msg_mixed_content",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: func() *string { s := "First text part"; return &s }(),
									},
									{
										Type: "image_url", // Non-text type, should be ignored
										ImageURL: &llm.ImageURL{
											URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==",
										},
									},
									{
										Type: "text",
										Text: func() *string { s := "Second text part"; return &s }(),
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				// Should only include text parts
				require.Len(t, resp.Content, 3)
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "First text part", resp.Content[0].Text)

				require.Equal(t, "image", resp.Content[1].Type)
				require.NotNil(t, resp.Content[1].Source)
				require.Equal(t, "image/png", resp.Content[1].Source.MediaType)
				require.Equal(t, "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg==", resp.Content[1].Source.Data)

				require.Equal(t, "text", resp.Content[2].Type)
				require.Equal(t, "Second text part", resp.Content[2].Text)
			},
		},
		{
			name: "response with nil text in content part",
			chatResp: &llm.Response{
				ID:      "msg_nil_text",
				Object:  "chat.completion",
				Model:   "claude-3-sonnet-20240229",
				Created: 1234567890,
				Choices: []llm.Choice{
					{
						Index: 0,
						Message: &llm.Message{
							Role: "assistant",
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{
										Type: "text",
										Text: nil, // Nil text should be ignored
									},
									{
										Type: "text",
										Text: func() *string { s := "Valid text"; return &s }(),
									},
								},
							},
						},
						FinishReason: func() *string { s := "stop"; return &s }(),
					},
				},
			},
			expectError: false,
			validate: func(t *testing.T, resp *Message) {
				t.Helper()
				// Should only include the valid text part
				require.Len(t, resp.Content, 1)
				require.Equal(t, "text", resp.Content[0].Type)
				require.Equal(t, "Valid text", resp.Content[0].Text)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := transformer.TransformResponse(t.Context(), tt.chatResp)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, http.StatusOK, result.StatusCode)
				require.Equal(t, "application/json", result.Headers.Get("Content-Type"))
				require.NotEmpty(t, result.Body)

				// Verify the response can be unmarshaled to AnthropicResponse
				var anthropicResp Message

				err := json.Unmarshal(result.Body, &anthropicResp)
				require.NoError(t, err)
				require.Equal(t, tt.chatResp.ID, anthropicResp.ID)
				require.Equal(t, "message", anthropicResp.Type)
				require.Equal(t, "assistant", anthropicResp.Role)
				require.Equal(t, tt.chatResp.Model, anthropicResp.Model)

				// Run custom validation if provided
				if tt.validate != nil {
					tt.validate(t, &anthropicResp)
				}
			}
		})
	}
}
