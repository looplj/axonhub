package responses

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestConvertToolMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      llm.Message
		expected Item
	}{
		{
			name: "tool message with simple content",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_123"),
				Content: llm.MessageContent{
					Content: lo.ToPtr("Simple tool result"),
				},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "call_123",
				Output: &Input{Text: lo.ToPtr("Simple tool result")},
			},
		},
		{
			name: "tool message with multiple content - single text part",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_cmN7LOSh5GhF7h0m5KfWuGEI"),
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("I located"),
							CacheControl: &llm.CacheControl{
								Type: "ephemeral",
							},
						},
					},
				},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "call_cmN7LOSh5GhF7h0m5KfWuGEI",
				Output: &Input{Items: []Item{
					{
						Type: "input_text",
						Text: lo.ToPtr("I located"),
					},
				}},
			},
		},
		{
			name: "tool message with multiple content - multiple text parts",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_456"),
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("First part"),
						},
						{
							Type: "text",
							Text: lo.ToPtr("Second part"),
						},
					},
				},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "call_456",
				Output: &Input{Items: []Item{
					{
						Type: "input_text",
						Text: lo.ToPtr("First part"),
					},
					{
						Type: "input_text",
						Text: lo.ToPtr("Second part"),
					},
				}},
			},
		},
		{
			name: "tool message with multiple content - mixed types (only text extracted)",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_789"),
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("Text result"),
						},
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: "https://example.com/image.jpg",
							},
						},
						{
							Type: "text",
							Text: lo.ToPtr("More text"),
						},
					},
				},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "call_789",
				Output: &Input{Items: []Item{
					{
						Type: "input_text",
						Text: lo.ToPtr("Text result"),
					},
					{
						Type: "input_text",
						Text: lo.ToPtr("More text"),
					},
				}},
			},
		},
		{
			name: "tool message with no content",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_empty"),
				Content:    llm.MessageContent{},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "call_empty",
				Output: &Input{
					Text: lo.ToPtr(""),
				},
			},
		},
		{
			name: "tool message with no tool call ID",
			msg: llm.Message{
				Role: "tool",
				Content: llm.MessageContent{
					Content: lo.ToPtr("Result without call ID"),
				},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "",
				Output: &Input{Text: lo.ToPtr("Result without call ID")},
			},
		},
		{
			name: "tool message with multiple content but no text parts",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_no_text"),
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "image_url",
							ImageURL: &llm.ImageURL{
								URL: "https://example.com/image.jpg",
							},
						},
						{
							Type: "input_audio",
							Audio: &llm.Audio{
								Data:   "audio-data",
								Format: "wav",
							},
						},
					},
				},
			},
			expected: Item{
				Type:   "function_call_output",
				CallID: "call_no_text",
				Output: &Input{
					Text: lo.ToPtr(""),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToolMessage(tt.msg)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertReasoning(t *testing.T) {
	tests := []struct {
		name     string
		req      *llm.Request
		expected *Reasoning
	}{
		{
			name: "nil reasoning fields",
			req: &llm.Request{
				ReasoningEffort: "",
				ReasoningBudget: nil,
			},
			expected: nil,
		},
		{
			name: "only effort specified",
			req: &llm.Request{
				ReasoningEffort: "high",
				ReasoningBudget: nil,
			},
			expected: &Reasoning{
				Effort:    "high",
				MaxTokens: nil,
			},
		},
		{
			name: "only budget specified",
			req: &llm.Request{
				ReasoningEffort: "",
				ReasoningBudget: lo.ToPtr(int64(5000)),
			},
			expected: &Reasoning{
				Effort:    "",
				MaxTokens: lo.ToPtr(int64(5000)),
			},
		},
		{
			name: "both effort and budget specified - effort takes priority",
			req: &llm.Request{
				ReasoningEffort: "medium",
				ReasoningBudget: lo.ToPtr(int64(3000)),
			},
			expected: &Reasoning{
				Effort:    "medium",
				MaxTokens: nil, // Should be nil when effort is specified
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertReasoning(tt.req)
			require.Equal(t, tt.expected, result)
		})
	}
}
