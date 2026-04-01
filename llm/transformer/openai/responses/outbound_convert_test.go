package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestConvertToolMessage(t *testing.T) {
	tests := []struct {
		name     string
		msg      llm.Message
		expected Item
	}{
		{
			name: "custom tool output uses custom_tool_call_output",
			msg: llm.Message{
				Role:       "tool",
				ToolCallID: lo.ToPtr("call_patch_001"),
				Content: llm.MessageContent{
					Content: lo.ToPtr("Patch applied successfully."),
				},
			},
			expected: Item{
				Type:   "custom_tool_call_output",
				CallID: "call_patch_001",
				Output: &Input{Text: lo.ToPtr("Patch applied successfully.")},
			},
		},
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
							InputAudio: &llm.InputAudio{
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
			itemType := "function_call_output"
			if tt.expected.Type != "" {
				itemType = tt.expected.Type
			}
			result := convertToolMessageWithType(tt.msg, itemType)
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
		{
			name: "json_schema with name and schema",
			req: &llm.Request{
				ResponseFormat: &llm.ResponseFormat{
					Type:       "json_schema",
					JSONSchema: json.RawMessage(`{"name":"ping_response","schema":{"type":"object","properties":{"pong":{"type":"boolean"}},"required":["pong"],"additionalProperties":false}}`),
				},
			},
			expected: &TextOptions{
				Format: &TextFormat{
					Type:   "json_schema",
					Name:   "ping_response",
					Schema: json.RawMessage(`{"type":"object","properties":{"pong":{"type":"boolean"}},"required":["pong"],"additionalProperties":false}`),
				},
			},
		},
		{
			name: "json_schema with strict",
			req: &llm.Request{
				ResponseFormat: &llm.ResponseFormat{
					Type:       "json_schema",
					JSONSchema: json.RawMessage(`{"name":"test","strict":true,"schema":{"type":"object"}}`),
				},
			},
			expected: &TextOptions{
				Format: &TextFormat{
					Type:   "json_schema",
					Name:   "test",
					Schema: json.RawMessage(`{"type":"object"}`),
					Strict: lo.ToPtr(true),
				},
			},
		},
		{
			name: "json_schema type without json_schema field",
			req: &llm.Request{
				ResponseFormat: &llm.ResponseFormat{
					Type: "json_schema",
				},
			},
			expected: &TextOptions{
				Format: &TextFormat{
					Type: "json_schema",
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

func TestConvertInstructionsFromMessages(t *testing.T) {
	tests := []struct {
		name     string
		msgs     []llm.Message
		expected string
	}{
		{
			name:     "empty messages",
			msgs:     []llm.Message{},
			expected: "",
		},
		{
			name: "system message",
			msgs: []llm.Message{
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: lo.ToPtr("system instruction"),
					},
				},
			},
			expected: "system instruction",
		},
		{
			name: "developer message should be ignored in instructions",
			msgs: []llm.Message{
				{
					Role: "developer",
					Content: llm.MessageContent{
						Content: lo.ToPtr("developer instruction"),
					},
				},
			},
			expected: "",
		},
		{
			name: "mixed system and developer messages",
			msgs: []llm.Message{
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: lo.ToPtr("system 1"),
					},
				},
				{
					Role: "developer",
					Content: llm.MessageContent{
						Content: lo.ToPtr("developer 1"),
					},
				},
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: lo.ToPtr("system 2"),
					},
				},
			},
			expected: "system 1\nsystem 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertInstructionsFromMessages(tt.msgs)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertInputFromMessages(t *testing.T) {
	tests := []struct {
		name             string
		msgs             []llm.Message
		transformOptions llm.TransformOptions
		expected         Input
	}{
		{
			name: "single developer message",
			msgs: []llm.Message{
				{
					Role: "developer",
					Content: llm.MessageContent{
						Content: lo.ToPtr("dev content"),
					},
				},
			},
			transformOptions: llm.TransformOptions{
				ArrayInputs: lo.ToPtr(true),
			},
			expected: Input{
				Items: []Item{
					{
						Type: "message",
						Role: "developer",
						Content: &Input{
							Items: []Item{
								{
									Type: "input_text",
									Text: lo.ToPtr("dev content"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mixed developer and user messages",
			msgs: []llm.Message{
				{
					Role: "developer",
					Content: llm.MessageContent{
						Content: lo.ToPtr("dev 1"),
					},
				},
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("user 1"),
					},
				},
			},
			expected: Input{
				Items: []Item{
					{
						Type: "message",
						Role: "developer",
						Content: &Input{
							Items: []Item{
								{
									Type: "input_text",
									Text: lo.ToPtr("dev 1"),
								},
							},
						},
					},
					{
						Type: "message",
						Role: "user",
						Content: &Input{
							Items: []Item{
								{
									Type: "input_text",
									Text: lo.ToPtr("user 1"),
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
			result := convertInputFromMessages(tt.msgs, tt.transformOptions, shared.TransportScope{})
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

func TestConvertOutputToMessage(t *testing.T) {
	tests := []struct {
		name                string
		output              []Item
		transformerMetadata map[string]any
		validate            func(t *testing.T, msg llm.Message)
	}{
		{
			name:   "empty output",
			output: nil,
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "assistant", msg.Role)
				require.Nil(t, msg.Content.Content)
				require.Nil(t, msg.Content.MultipleContent)
			},
		},
		{
			name: "text message output",
			output: []Item{
				{
					ID:   "msg_001",
					Type: "message",
					Content: &Input{Items: []Item{
						{Type: "output_text", Text: lo.ToPtr("Hello world")},
					}},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "msg_001", msg.ID)
				require.NotNil(t, msg.Content.Content)
				require.Equal(t, "Hello world", *msg.Content.Content)
			},
		},
		{
			name: "direct output_text item",
			output: []Item{
				{Type: "output_text", Text: lo.ToPtr("Direct text")},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.NotNil(t, msg.Content.Content)
				require.Equal(t, "Direct text", *msg.Content.Content)
			},
		},
		{
			name: "function call output",
			output: []Item{
				{
					Type:      "function_call",
					CallID:    "call_123",
					Name:      "get_weather",
					Arguments: `{"location":"NYC"}`,
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.ToolCalls, 1)
				require.Equal(t, "call_123", msg.ToolCalls[0].ID)
				require.Equal(t, "function", msg.ToolCalls[0].Type)
				require.Equal(t, "get_weather", msg.ToolCalls[0].Function.Name)
				require.Equal(t, `{"location":"NYC"}`, msg.ToolCalls[0].Function.Arguments)
			},
		},
		{
			name: "custom tool call output",
			output: []Item{
				{
					Type:   "custom_tool_call",
					CallID: "call_custom_1",
					Name:   "patch_tool",
					Input:  lo.ToPtr("some input"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.ToolCalls, 1)
				tc := msg.ToolCalls[0]
				require.Equal(t, "call_custom_1", tc.ID)
				require.Equal(t, llm.ToolTypeResponsesCustomTool, tc.Type)
				require.NotNil(t, tc.ResponseCustomToolCall)
				require.Equal(t, "patch_tool", tc.ResponseCustomToolCall.Name)
				require.Equal(t, "some input", tc.ResponseCustomToolCall.Input)
			},
		},
		{
			name: "reasoning output with encrypted content",
			output: []Item{
				{
					Type:             "reasoning",
					Summary:          []ReasoningSummary{{Type: "summary_text", Text: "Thinking step"}},
					EncryptedContent: lo.ToPtr("encrypted_data"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.NotNil(t, msg.ReasoningContent)
				require.Equal(t, "Thinking step", *msg.ReasoningContent)
				require.NotNil(t, msg.ReasoningSignature)
				require.Equal(t, "encrypted_data", *msg.ReasoningSignature)
			},
		},
		{
			name: "image generation output with custom format",
			output: []Item{
				{
					Type:   "image_generation_call",
					Result: lo.ToPtr("base64data"),
				},
			},
			transformerMetadata: map[string]any{"image_output_format": "webp"},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 1)
				part := msg.Content.MultipleContent[0]
				require.Equal(t, "image_url", part.Type)
				require.Equal(t, "data:image/webp;base64,base64data", part.ImageURL.URL)
			},
		},
		{
			name: "image generation output with default png format",
			output: []Item{
				{
					Type:   "image_generation_call",
					Result: lo.ToPtr("pngdata"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 1)
				require.Contains(t, msg.Content.MultipleContent[0].ImageURL.URL, "data:image/png;base64,")
			},
		},
		{
			name: "compaction output",
			output: []Item{
				{
					ID:               "cmp_001",
					Type:             "compaction",
					EncryptedContent: lo.ToPtr("enc_data"),
					CreatedBy:        lo.ToPtr("assistant"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 1)
				part := msg.Content.MultipleContent[0]
				require.Equal(t, "compaction", part.Type)
				require.NotNil(t, part.Compact)
				require.Equal(t, "cmp_001", part.Compact.ID)
				require.Equal(t, "enc_data", part.Compact.EncryptedContent)
				require.Equal(t, "assistant", *part.Compact.CreatedBy)
			},
		},
		{
			name: "compaction_summary output",
			output: []Item{
				{
					ID:               "cmp_sum_001",
					Type:             "compaction_summary",
					EncryptedContent: lo.ToPtr("summary_enc"),
					CreatedBy:        lo.ToPtr("system"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 1)
				part := msg.Content.MultipleContent[0]
				require.Equal(t, "compaction_summary", part.Type)
				require.NotNil(t, part.Compact)
				require.Equal(t, "cmp_sum_001", part.Compact.ID)
				require.Equal(t, "summary_enc", part.Compact.EncryptedContent)
				require.Equal(t, "system", *part.Compact.CreatedBy)
			},
		},
		{
			name: "mixed text and compaction",
			output: []Item{
				{
					ID:   "msg_mix",
					Type: "message",
					Content: &Input{Items: []Item{
						{Type: "output_text", Text: lo.ToPtr("Some text")},
					}},
				},
				{
					ID:               "cmp_002",
					Type:             "compaction",
					EncryptedContent: lo.ToPtr("enc_mixed"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 2)
				require.Equal(t, "text", msg.Content.MultipleContent[0].Type)
				require.Equal(t, "Some text", *msg.Content.MultipleContent[0].Text)
				require.Equal(t, "compaction", msg.Content.MultipleContent[1].Type)
				require.Equal(t, "enc_mixed", msg.Content.MultipleContent[1].Compact.EncryptedContent)
			},
		},
		{
			name: "text compaction text preserves order",
			output: []Item{
				{
					ID:   "msg_before",
					Type: "message",
					Content: &Input{Items: []Item{
						{Type: "output_text", Text: lo.ToPtr("before")},
					}},
				},
				{
					ID:               "cmp_mid",
					Type:             "compaction",
					EncryptedContent: lo.ToPtr("enc_mid"),
				},
				{
					ID:   "msg_after",
					Type: "message",
					Content: &Input{Items: []Item{
						{Type: "output_text", Text: lo.ToPtr("after")},
					}},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 3)
				require.Equal(t, "text", msg.Content.MultipleContent[0].Type)
				require.Equal(t, "before", *msg.Content.MultipleContent[0].Text)
				require.Equal(t, "compaction", msg.Content.MultipleContent[1].Type)
				require.Equal(t, "enc_mid", msg.Content.MultipleContent[1].Compact.EncryptedContent)
				require.Equal(t, "text", msg.Content.MultipleContent[2].Type)
				require.Equal(t, "after", *msg.Content.MultipleContent[2].Text)
			},
		},
		{
			name: "input_image output",
			output: []Item{
				{
					Type:     "input_image",
					ImageURL: lo.ToPtr("https://example.com/img.png"),
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.Content.MultipleContent, 1)
				require.Equal(t, "image_url", msg.Content.MultipleContent[0].Type)
				require.Equal(t, "https://example.com/img.png", msg.Content.MultipleContent[0].ImageURL.URL)
			},
		},
		{
			name: "multiple function calls",
			output: []Item{
				{Type: "function_call", CallID: "c1", Name: "fn1", Arguments: "{}"},
				{Type: "function_call", CallID: "c2", Name: "fn2", Arguments: `{"a":1}`},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Len(t, msg.ToolCalls, 2)
				require.Equal(t, "fn1", msg.ToolCalls[0].Function.Name)
				require.Equal(t, "fn2", msg.ToolCalls[1].Function.Name)
			},
		},
		{
			name: "reasoning with text and function call",
			output: []Item{
				{
					Type:             "reasoning",
					Summary:          []ReasoningSummary{{Type: "summary_text", Text: "Thought"}},
					EncryptedContent: lo.ToPtr("enc_reason"),
				},
				{
					ID:   "msg_r",
					Type: "message",
					Content: &Input{Items: []Item{
						{Type: "output_text", Text: lo.ToPtr("Answer")},
					}},
				},
				{Type: "function_call", CallID: "c1", Name: "fn1", Arguments: "{}"},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "msg_r", msg.ID)
				require.NotNil(t, msg.ReasoningContent)
				require.Equal(t, "Thought", *msg.ReasoningContent)
				require.Len(t, msg.Content.MultipleContent, 1)
				require.Equal(t, "text", msg.Content.MultipleContent[0].Type)
				require.Equal(t, "Answer", *msg.Content.MultipleContent[0].Text)
				require.Len(t, msg.ToolCalls, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := convertOutputToMessage(tt.output, shared.TransportScope{}, tt.transformerMetadata)
			tt.validate(t, msg)
		})
	}
}

func TestConvertAssistantMessage_WithCompactContent(t *testing.T) {
	tests := []struct {
		name     string
		msg      llm.Message
		validate func(t *testing.T, items []Item)
	}{
		{
			name: "assistant message with compaction content part",
			msg: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "compaction_out_123",
								EncryptedContent: "outbound_encrypted",
								CreatedBy:        lo.ToPtr("assistant"),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, items []Item) {
				require.Len(t, items, 1)
				require.Equal(t, "compaction", items[0].Type)
				require.Equal(t, "compaction_out_123", items[0].ID)
				require.NotNil(t, items[0].EncryptedContent)
				require.Equal(t, "outbound_encrypted", *items[0].EncryptedContent)
				require.NotNil(t, items[0].CreatedBy)
				require.Equal(t, "assistant", *items[0].CreatedBy)
			},
		},
		{
			name: "assistant message with mixed text and compaction content",
			msg: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("Here is some text"),
						},
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "compaction_mixed_456",
								EncryptedContent: "mixed_encrypted_data",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, items []Item) {
				require.Len(t, items, 2)

				require.Equal(t, "message", items[0].Type)
				require.Equal(t, "assistant", items[0].Role)
				require.Len(t, items[0].Content.Items, 1)
				require.Equal(t, "output_text", items[0].Content.Items[0].Type)
				require.Equal(t, "Here is some text", *items[0].Content.Items[0].Text)

				require.Equal(t, "compaction", items[1].Type)
				require.Equal(t, "compaction_mixed_456", items[1].ID)
				require.NotNil(t, items[1].EncryptedContent)
				require.Equal(t, "mixed_encrypted_data", *items[1].EncryptedContent)
				require.Nil(t, items[1].CreatedBy)
			},
		},
		{
			name: "assistant message with text and tool calls emits message before tool calls",
			msg: llm.Message{
				Role: "assistant",
				ToolCalls: []llm.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "fn1",
							Arguments: "{}",
						},
					},
					{
						ID:   "call_2",
						Type: "function",
						Function: llm.FunctionCall{
							Name:      "fn2",
							Arguments: `{"a":1}`,
						},
					},
				},
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("msg 1"),
						},
					},
				},
			},
			validate: func(t *testing.T, items []Item) {
				require.Len(t, items, 3)
				require.Equal(t, "message", items[0].Type)
				require.Equal(t, "assistant", items[0].Role)
				require.Len(t, items[0].Content.Items, 1)
				require.Equal(t, "msg 1", *items[0].Content.Items[0].Text)
				require.Equal(t, "function_call", items[1].Type)
				require.Equal(t, "call_1", items[1].CallID)
				require.Equal(t, "function_call", items[2].Type)
				require.Equal(t, "call_2", items[2].CallID)
			},
		},
		{
			name: "assistant message with compaction content without created_by",
			msg: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "compaction_no_created",
								EncryptedContent: "no_created_by_data",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, items []Item) {
				require.Len(t, items, 1)
				require.Equal(t, "compaction", items[0].Type)
				require.Equal(t, "compaction_no_created", items[0].ID)
				require.NotNil(t, items[0].EncryptedContent)
				require.Equal(t, "no_created_by_data", *items[0].EncryptedContent)
				require.Nil(t, items[0].CreatedBy)
			},
		},
		{
			name: "assistant message with text compaction text preserves order",
			msg: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "text",
							Text: lo.ToPtr("before"),
						},
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "compaction_mid",
								EncryptedContent: "enc_mid",
							},
						},
						{
							Type: "text",
							Text: lo.ToPtr("after"),
						},
					},
				},
			},
			validate: func(t *testing.T, items []Item) {
				require.Len(t, items, 3)
				require.Equal(t, "message", items[0].Type)
				require.Len(t, items[0].Content.Items, 1)
				require.Equal(t, "before", *items[0].Content.Items[0].Text)
				require.Equal(t, "compaction", items[1].Type)
				require.Equal(t, "compaction_mid", items[1].ID)
				require.Equal(t, "message", items[2].Type)
				require.Len(t, items[2].Content.Items, 1)
				require.Equal(t, "after", *items[2].Content.Items[0].Text)
			},
		},
		{
			name: "assistant message with multiple compaction content parts",
			msg: llm.Message{
				Role: "assistant",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "compaction_multi_1",
								EncryptedContent: "encrypted_1",
								CreatedBy:        lo.ToPtr("user_a"),
							},
						},
						{
							Type: "compaction",
							Compact: &llm.CompactContent{
								ID:               "compaction_multi_2",
								EncryptedContent: "encrypted_2",
								CreatedBy:        lo.ToPtr("user_b"),
							},
						},
					},
				},
			},
			validate: func(t *testing.T, items []Item) {
				require.Len(t, items, 2)

				require.Equal(t, "compaction", items[0].Type)
				require.Equal(t, "compaction_multi_1", items[0].ID)
				require.Equal(t, "encrypted_1", *items[0].EncryptedContent)
				require.Equal(t, "user_a", *items[0].CreatedBy)

				require.Equal(t, "compaction", items[1].Type)
				require.Equal(t, "compaction_multi_2", items[1].ID)
				require.Equal(t, "encrypted_2", *items[1].EncryptedContent)
				require.Equal(t, "user_b", *items[1].CreatedBy)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertAssistantMessage(tt.msg, shared.TransportScope{})
			tt.validate(t, result)
		})
	}
}
