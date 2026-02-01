package responses

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
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

func TestConvertStreamOptions(t *testing.T) {
	tests := []struct {
		name     string
		src      *llm.StreamOptions
		metadata map[string]any
		expected *StreamOptions
	}{
		{
			name:     "nil stream options",
			src:      nil,
			metadata: nil,
			expected: nil,
		},
		{
			name: "include obfuscation false",
			src: &llm.StreamOptions{
				IncludeUsage: true,
			},
			metadata: map[string]any{
				"include_obfuscation": lo.ToPtr(false),
			},
			expected: &StreamOptions{
				IncludeObfuscation: lo.ToPtr(false),
			},
		},
		{
			name: "include obfuscation true",
			src: &llm.StreamOptions{
				IncludeUsage: false,
			},
			metadata: map[string]any{
				"include_obfuscation": lo.ToPtr(true),
			},
			expected: &StreamOptions{
				IncludeObfuscation: lo.ToPtr(true),
			},
		},
		{
			name: "no include obfuscation in metadata",
			src: &llm.StreamOptions{
				IncludeUsage: true,
			},
			metadata: map[string]any{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertStreamOptions(tt.src, tt.metadata)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToTextOptions(t *testing.T) {
	tests := []struct {
		name     string
		req      *llm.Request
		expected *TextOptions
	}{
		{
			name:     "nil request",
			req:      nil,
			expected: nil,
		},
		{
			name:     "empty request",
			req:      &llm.Request{},
			expected: nil,
		},
		{
			name: "only response format",
			req: &llm.Request{
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_object",
				},
			},
			expected: &TextOptions{
				Format: &TextFormat{
					Type: "json_object",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToTextOptions(tt.req)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToLLMRequest_TransformerMetadata(t *testing.T) {
	tests := []struct {
		name     string
		req      *Request
		validate func(t *testing.T, chatReq *llm.Request)
	}{
		{
			name: "converts MaxToolCalls to TransformerMetadata",
			req: &Request{
				Model:        "gpt-4o",
				MaxToolCalls: lo.ToPtr(int64(10)),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.TransformerMetadata)
				v, ok := chatReq.TransformerMetadata["max_tool_calls"]
				require.True(t, ok)
				require.Equal(t, int64(10), *v.(*int64))
			},
		},
		{
			name: "converts PromptCacheKey to PromptCacheKey field",
			req: &Request{
				Model:          "gpt-4o",
				PromptCacheKey: lo.ToPtr("cache-key-123"),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.PromptCacheKey)
				require.Equal(t, "cache-key-123", *chatReq.PromptCacheKey)
			},
		},
		{
			name: "converts PromptCacheRetention to TransformerMetadata",
			req: &Request{
				Model:                "gpt-4o",
				PromptCacheRetention: lo.ToPtr("24h"),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.TransformerMetadata)
				v, ok := chatReq.TransformerMetadata["prompt_cache_retention"]
				require.True(t, ok)
				require.Equal(t, "24h", *v.(*string))
			},
		},
		{
			name: "converts Truncation to TransformerMetadata",
			req: &Request{
				Model:      "gpt-4o",
				Truncation: lo.ToPtr("auto"),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.TransformerMetadata)
				v, ok := chatReq.TransformerMetadata["truncation"]
				require.True(t, ok)
				require.Equal(t, "auto", *v.(*string))
			},
		},
		{
			name: "converts TextVerbosity to Verbosity",
			req: &Request{
				Model: "gpt-4o",
				Text: &TextOptions{
					Verbosity: lo.ToPtr("high"),
				},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.Equal(t, "high", lo.FromPtr(chatReq.Verbosity))
			},
		},
		{
			name: "converts Include to TransformerMetadata",
			req: &Request{
				Model:   "gpt-4o",
				Include: []string{"file_search_call.results", "reasoning.encrypted_content"},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.TransformerMetadata)
				v, ok := chatReq.TransformerMetadata["include"]
				require.True(t, ok)
				require.Equal(t, []string{"file_search_call.results", "reasoning.encrypted_content"}, v.([]string))
			},
		},
		{
			name: "initializes TransformerMetadata",
			req: &Request{
				Model: "gpt-4o",
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.TransformerMetadata)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToLLMRequest(tt.req)
			require.NoError(t, err)
			tt.validate(t, result)
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
				ReasoningEffort:  "",
				ReasoningBudget:  nil,
				ReasoningSummary: nil,
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
		{
			name: "with summary specified",
			req: &llm.Request{
				ReasoningEffort:  "high",
				ReasoningSummary: lo.ToPtr("detailed"),
				ReasoningBudget:  lo.ToPtr(int64(5000)),
			},
			expected: &Reasoning{
				Effort:    "high",
				MaxTokens: nil, // effort takes priority
				Summary:   "detailed",
			},
		},
		{
			name: "with only summary specified (no effort or budget)",
			req: &llm.Request{
				ReasoningSummary: lo.ToPtr("concise"),
			},
			expected: &Reasoning{
				Summary: "concise",
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
