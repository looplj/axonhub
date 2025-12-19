package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestConvertToChatCompletionResponse(t *testing.T) {
	anthropicResp := &Message{
		ID:   "msg_123",
		Type: "message",
		Role: "assistant",
		Content: []MessageContentBlock{
			{
				Type: "text",
				Text: lo.ToPtr("Hello! How can I help you?"),
			},
		},
		Model:      "claude-3-sonnet-20240229",
		StopReason: func() *string { s := "end_turn"; return &s }(),
		Usage: &Usage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}
	result := convertToLlmResponse(anthropicResp, PlatformDirect)

	require.Equal(t, "msg_123", result.ID)
	require.Equal(t, "chat.completion", result.Object)
	require.Equal(t, "claude-3-sonnet-20240229", result.Model)
	require.Equal(t, 1, len(result.Choices))
	require.Equal(t, "assistant", result.Choices[0].Message.Role)
	require.Equal(t, "Hello! How can I help you?", *result.Choices[0].Message.Content.Content)
	require.Equal(t, "stop", *result.Choices[0].FinishReason)
	require.Equal(t, int64(10), result.Usage.PromptTokens)
	require.Equal(t, int64(20), result.Usage.CompletionTokens)
	require.Equal(t, int64(30), result.Usage.TotalTokens)
}

func TestOutboundTransformer_ToolArgsRepair(t *testing.T) {
	transformer, _ := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")

	t.Run("reparable invalid json becomes valid", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_repair",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "test_func",
								Arguments: `{"invalid": json}`,
							},
						},
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)
		require.NotEmpty(t, anthropicReq.Messages)

		// Find the tool_use block
		var found bool

		for _, msg := range anthropicReq.Messages {
			for _, blk := range msg.Content.MultipleContent {
				if blk.Type == "tool_use" {
					found = true

					require.Equal(t, "call_repair", blk.ID)
					// Input should be valid JSON and repaired to {"invalid":"json"}
					require.True(t, json.Valid(blk.Input))

					var m map[string]any

					err := json.Unmarshal(blk.Input, &m)
					require.NoError(t, err)
					require.Equal(t, "json", m["invalid"])
				}
			}
		}

		require.True(t, found, "expected tool_use block to be present")
	})

	t.Run("empty args becomes empty object", func(t *testing.T) {
		req := &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: func() *int64 { v := int64(1024); return &v }(),
			Messages: []llm.Message{
				{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{
						{
							ID:   "call_empty",
							Type: "function",
							Function: llm.FunctionCall{
								Name:      "test_func",
								Arguments: "",
							},
						},
					},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)
		require.NotEmpty(t, anthropicReq.Messages)

		var found bool

		for _, msg := range anthropicReq.Messages {
			for _, blk := range msg.Content.MultipleContent {
				if blk.Type == "tool_use" {
					found = true

					require.Equal(t, "call_empty", blk.ID)
					require.Equal(t, json.RawMessage("{}"), blk.Input)
				}
			}
		}

		require.True(t, found, "expected tool_use block to be present")
	})
}

func TestConvertToChatCompletionResponse_EdgeCases(t *testing.T) {
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
				Content: []MessageContentBlock{},
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
				Content: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("Hello")},
					{Type: "text", Text: lo.ToPtr(" world!")},
					{Type: "text", Text: lo.ToPtr(" How are you?")},
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
				Content: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("Check this image: ")},
					{Type: "image", Source: &ImageSource{
						Type:      "base64",
						MediaType: "image/jpeg",
						Data:      "/9j/4AAQSkZJRg==",
					}},
					{Type: "text", Text: lo.ToPtr(" and this text")},
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
				Content: []MessageContentBlock{
					{
						Type: "text",
						Text: lo.ToPtr("I'll help you with that calculation."),
					},
					{
						Type:  "tool_use",
						ID:    "tool_123",
						Name:  lo.ToPtr("calculator"),
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
					Content: []MessageContentBlock{{Type: "text", Text: lo.ToPtr("Test")}},
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
					"pause_turn":    "stop",
					"refusal":       "content_filter",
				}

				for anthropicReason, expectedReason := range stopReasons {
					msg := &Message{
						ID:         "msg_stop",
						Type:       "message",
						Role:       "assistant",
						Content:    []MessageContentBlock{{Type: "text", Text: lo.ToPtr("Test")}},
						Model:      "claude-3-sonnet-20240229",
						StopReason: lo.ToPtr(anthropicReason),
					}

					result := convertToLlmResponse(msg, PlatformDirect)
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
				Content: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("Cached response")},
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
				require.Equal(t, int64(150), result.Usage.PromptTokens)
				require.Equal(t, int64(50), result.Usage.CompletionTokens)
				require.Equal(t, int64(200), result.Usage.TotalTokens)
				require.Equal(t, int64(30), result.Usage.PromptTokensDetails.CachedTokens)
				require.Equal(t, int64(20), result.Usage.PromptTokensDetails.WriteCachedTokens)
			},
		},
		{
			name: "usage with detailed token breakdown",
			input: &Message{
				ID:   "msg_detailed",
				Type: "message",
				Role: "assistant",
				Content: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("Detailed response")},
				},
				Model: "claude-3-sonnet-20240229",
				Usage: &Usage{
					InputTokens:              200,
					OutputTokens:             75,
					CacheCreationInputTokens: 50,
					CacheReadInputTokens:     100,
					ServiceTier:              "premium",
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_detailed", result.ID)
				require.Equal(t, int64(350), result.Usage.PromptTokens)
				require.Equal(t, int64(75), result.Usage.CompletionTokens)
				require.Equal(t, int64(425), result.Usage.TotalTokens)
				// Verify detailed prompt token information
				require.NotNil(t, result.Usage.PromptTokensDetails)
				require.Equal(t, int64(100), result.Usage.PromptTokensDetails.CachedTokens)
				require.Equal(t, int64(50), result.Usage.PromptTokensDetails.WriteCachedTokens)
			},
		},
		{
			name: "usage without cache tokens",
			input: &Message{
				ID:   "msg_no_cache",
				Type: "message",
				Role: "assistant",
				Content: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("No cache response")},
				},
				Model: "claude-3-sonnet-20240229",
				Usage: &Usage{
					InputTokens:              80,
					OutputTokens:             40,
					CacheCreationInputTokens: 0,
					CacheReadInputTokens:     0,
					ServiceTier:              "standard",
				},
			},
			validate: func(t *testing.T, result *llm.Response) {
				t.Helper()
				require.Equal(t, "msg_no_cache", result.ID)
				require.Equal(t, int64(80), result.Usage.PromptTokens)
				require.Equal(t, int64(40), result.Usage.CompletionTokens)
				require.Equal(t, int64(120), result.Usage.TotalTokens)
			},
		},
		{
			name: "nil usage",
			input: &Message{
				ID:      "msg_nusage",
				Type:    "message",
				Role:    "assistant",
				Content: []MessageContentBlock{{Type: "text", Text: lo.ToPtr("No usage")}},
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
			result := convertToLlmResponse(tt.input, PlatformDirect)
			tt.validate(t, result)
		})
	}
}

func TestConvertToAnthropicRequest(t *testing.T) {
	tests := []struct {
		name     string
		chatReq  *llm.Request
		expected *MessageRequest
	}{
		{
			name: "simple request",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
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
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
		},
		{
			name: "request with system message",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "system",
						Content: llm.MessageContent{
							Content: lo.ToPtr("You are helpful."),
						},
					},
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
			expected: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 1024,
				System: &SystemPrompt{
					Prompt: lo.ToPtr("You are helpful."),
				},
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							Content: lo.ToPtr("Hello!"),
						},
					},
				},
			},
		},
		{
			name: "request with image content",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("What's in this image?"),
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
				},
			},
			expected: &MessageRequest{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: 1024,
				Messages: []MessageParam{
					{
						Role: "user",
						Content: MessageContent{
							MultipleContent: []MessageContentBlock{
								{
									Type: "text",
									Text: lo.ToPtr("What's in this image?"),
								},
								{
									Type: "image",
									Source: &ImageSource{
										Type:      "base64",
										MediaType: "image/jpeg",
										Data:      "/9j/4AAQSkZJRgABAQEAYABgAAD//gA7Q1JFQVR",
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "request with multiple images and text",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Compare these two images:"),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("and"),
								},
								{
									Type: "image_url",
									ImageURL: &llm.ImageURL{
										URL: "data:image/webp;base64,UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("What are the differences?"),
								},
							},
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
							MultipleContent: []MessageContentBlock{
								{
									Type: "text",
									Text: lo.ToPtr("Compare these two images:"),
								},
								{
									Type: "image",
									Source: &ImageSource{
										Type:      "base64",
										MediaType: "image/png",
										Data:      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("and"),
								},
								{
									Type: "image",
									Source: &ImageSource{
										Type:      "base64",
										MediaType: "image/webp",
										Data:      "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA",
									},
								},
								{
									Type: "text",
									Text: lo.ToPtr("What are the differences?"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "request with reasoning content (simple content)",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("What is 2+2?"),
						},
					},
					{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("The answer is 4."),
						},
						ReasoningContent: lo.ToPtr("Let me calculate: 2 + 2 = 4"),
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
							Content: lo.ToPtr("What is 2+2?"),
						},
					},
					{
						Role: "assistant",
						Content: MessageContent{
							MultipleContent: []MessageContentBlock{
								{
									Type:     "thinking",
									Thinking: lo.ToPtr("Let me calculate: 2 + 2 = 4"),
								},
								{
									Type: "text",
									Text: lo.ToPtr("The answer is 4."),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "request with reasoning content (multiple content)",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Solve this problem"),
						},
					},
					{
						Role: "assistant",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: lo.ToPtr("Here is the solution."),
								},
							},
						},
						ReasoningContent: lo.ToPtr("First, I need to analyze the problem..."),
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
							Content: lo.ToPtr("Solve this problem"),
						},
					},
					{
						Role: "assistant",
						Content: MessageContent{
							MultipleContent: []MessageContentBlock{
								{
									Type:     "thinking",
									Thinking: lo.ToPtr("First, I need to analyze the problem..."),
								},
								{
									Type: "text",
									Text: lo.ToPtr("Here is the solution."),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "request with reasoning content and tool calls",
			chatReq: &llm.Request{
				Model:     "claude-3-sonnet-20240229",
				MaxTokens: lo.ToPtr(int64(1024)),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							Content: lo.ToPtr("Calculate 5 * 10"),
						},
					},
					{
						Role: "assistant",
						Content: llm.MessageContent{
							Content: lo.ToPtr("I'll use the calculator."),
						},
						ReasoningContent: lo.ToPtr("I need to use the calculator tool for this."),
						ToolCalls: []llm.ToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: llm.FunctionCall{
									Name:      "calculator",
									Arguments: `{"expression":"5*10"}`,
								},
							},
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
							Content: lo.ToPtr("Calculate 5 * 10"),
						},
					},
					{
						Role: "assistant",
						Content: MessageContent{
							MultipleContent: []MessageContentBlock{
								{
									Type: "text",
									Text: lo.ToPtr("I'll use the calculator."),
								},
								{
									Type:     "thinking",
									Thinking: lo.ToPtr("I need to use the calculator tool for this."),
								},
								{
									Type: "tool_use",
									ID:   "call_123",
									Name: lo.ToPtr("calculator"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToAnthropicRequest(tt.chatReq)
			require.Equal(t, tt.expected.Model, result.Model)
			require.Equal(t, tt.expected.MaxTokens, result.MaxTokens)
			require.Equal(t, tt.expected.System, result.System)
			require.Equal(t, len(tt.expected.Messages), len(result.Messages))
		})
	}
}
