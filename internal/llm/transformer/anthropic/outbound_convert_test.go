package anthropic

import (
	"encoding/json"
	"reflect"
	"testing"

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

	result := convertToChatCompletionResponse(anthropicResp)

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
				Content: []MessageContentBlock{
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
				Content: []MessageContentBlock{
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
					Content: []MessageContentBlock{{Type: "text", Text: "Test"}},
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
						Content:    []MessageContentBlock{{Type: "text", Text: "Test"}},
						Model:      "claude-3-sonnet-20240229",
						StopReason: func() *string { s := anthropicReason; return &s }(),
					}

					result := convertToChatCompletionResponse(msg)
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
				// Verify detailed token information
				require.NotNil(t, result.Usage.PromptTokensDetails)
				require.Equal(t, 30, result.Usage.PromptTokensDetails.CachedTokens)
				require.NotNil(t, result.Usage.CompletionTokensDetails)
				require.Equal(t, 0, result.Usage.CompletionTokensDetails.ReasoningTokens)
			},
		},
		{
			name: "usage with detailed token breakdown",
			input: &Message{
				ID:   "msg_detailed",
				Type: "message",
				Role: "assistant",
				Content: []MessageContentBlock{
					{Type: "text", Text: "Detailed response"},
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
				require.Equal(t, 200, result.Usage.PromptTokens)
				require.Equal(t, 75, result.Usage.CompletionTokens)
				require.Equal(t, 275, result.Usage.TotalTokens)
				// Verify detailed prompt token information
				require.NotNil(t, result.Usage.PromptTokensDetails)
				require.Equal(t, 100, result.Usage.PromptTokensDetails.CachedTokens)
				// Verify detailed completion token information
				require.NotNil(t, result.Usage.CompletionTokensDetails)
				require.Equal(t, 0, result.Usage.CompletionTokensDetails.ReasoningTokens)
			},
		},
		{
			name: "usage without cache tokens",
			input: &Message{
				ID:   "msg_no_cache",
				Type: "message",
				Role: "assistant",
				Content: []MessageContentBlock{
					{Type: "text", Text: "No cache response"},
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
			name: "nil usage",
			input: &Message{
				ID:      "msg_nusage",
				Type:    "message",
				Role:    "assistant",
				Content: []MessageContentBlock{{Type: "text", Text: "No usage"}},
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
			result := convertToChatCompletionResponse(tt.input)
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
		{
			name: "request with image content",
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
									Text: "What's in this image?",
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
				MaxTokens: func() *int64 { v := int64(1024); return &v }(),
				Messages: []llm.Message{
					{
						Role: "user",
						Content: llm.MessageContent{
							MultipleContent: []llm.MessageContentPart{
								{
									Type: "text",
									Text: func() *string { s := "Compare these two images:"; return &s }(),
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
									Text: func() *string { s := "What are the differences?"; return &s }(),
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
									Text: "Compare these two images:",
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
									Text: "and",
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
									Text: "What are the differences?",
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

func Test_convertUsage(t *testing.T) {
	type args struct {
		usage Usage
	}

	tests := []struct {
		name string
		args args
		want llm.Usage
	}{
		{
			name: "base case",
			args: args{
				usage: Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     30,
					ServiceTier:              "standard",
				},
			},
			want: llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 30,
				},
			},
		},
		{
			name: "cache read tokens greater than input tokens",
			args: args{
				usage: Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     150,
					ServiceTier:              "standard",
				},
			},
			want: llm.Usage{
				PromptTokens:     250,
				CompletionTokens: 50,
				TotalTokens:      300,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 150,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertToLlmUsage(tt.args.usage); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertUsage() = %v, want %v", got, tt.want)
			}
		})
	}
}
