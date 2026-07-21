package responses

import (
	"encoding/json"
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
						Type:     "input_image",
						ImageURL: lo.ToPtr("https://example.com/image.jpg"),
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
					Items: []Item{
						{
							Type:     "input_image",
							ImageURL: lo.ToPtr("https://example.com/image.jpg"),
						},
						{
							Type:       "input_audio",
							InputAudio: &llm.InputAudio{
								Data:   "audio-data",
								Format: "wav",
							},
						},
					},
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

func TestConvertAssistantMessage_WithImage(t *testing.T) {
	msgs := []llm.Message{
		{
			Role: "assistant",
			Content: llm.MessageContent{
				MultipleContent: []llm.MessageContentPart{
					{
						Type: "text",
						Text: lo.ToPtr("here is the generated image"),
					},
					{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: "https://example.com/generated.png",
						},
					},
				},
			},
		},
	}

	result := convertInputFromMessages(msgs, llm.TransformOptions{ArrayInputs: lo.ToPtr(true)}, nil)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	require.Equal(t, "message", item.Type)
	require.Equal(t, "assistant", item.Role)
	require.NotNil(t, item.Content)
	require.Len(t, item.Content.Items, 2)
	require.Equal(t, "output_text", item.Content.Items[0].Type)
	require.Equal(t, "input_image", item.Content.Items[1].Type)
	require.NotNil(t, item.Content.Items[1].ImageURL)
	require.Equal(t, "https://example.com/generated.png", *item.Content.Items[1].ImageURL)
}

func TestConvertWebSearchToTool(t *testing.T) {
	tests := []struct {
		name     string
		src      llm.Tool
		expected Tool
	}{
		{
			name: "minimal web search tool preserves type without asserting nil internals",
			src: llm.Tool{
				Type: llm.ToolTypeWebSearch,
			},
			expected: Tool{
				Type: "web_search",
			},
		},
		{
			name: "web search maps explicit allowed domains and user location fields",
			src: llm.Tool{
				Type: llm.ToolTypeWebSearch,
				WebSearch: &llm.WebSearch{
					AllowedDomains: []string{"example.com", "docs.example.com"},
					UserLocation: llm.WebSearchToolUserLocation{
						Type:     "approximate",
						City:     "San Francisco",
						Region:   "CA",
						Country:  "US",
						Timezone: "America/Los_Angeles",
					},
				},
			},
			expected: Tool{
				Type: "web_search",
				Filters: &WebSearchFilters{
					AllowedDomains: []string{"example.com", "docs.example.com"},
				},
				UserLocation: &WebSearchUserLocation{
					Type:     "approximate",
					City:     "San Francisco",
					Region:   "CA",
					Country:  "US",
					Timezone: "America/Los_Angeles",
				},
			},
		},
		{
			name: "web search defaults location type to approximate when omitted",
			src: llm.Tool{
				Type: llm.ToolTypeWebSearch,
				WebSearch: &llm.WebSearch{
					UserLocation: llm.WebSearchToolUserLocation{
						Country: "US",
					},
				},
			},
			expected: Tool{
				Type: "web_search",
				UserLocation: &WebSearchUserLocation{
					Type:    "approximate",
					Country: "US",
				},
			},
		},
		{
			name: "anthropic only strict and max uses are ignored when they are the only fields",
			src: llm.Tool{
				Type: llm.ToolTypeWebSearch,
				WebSearch: &llm.WebSearch{
					Strict:  lo.ToPtr(true),
					MaxUses: lo.ToPtr(int64(3)),
				},
			},
			expected: Tool{
				Type: "web_search",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertWebSearchToTool(tt.src)
			require.Equal(t, tt.expected, result)
			require.Equal(t, "web_search", result.Type)
			require.Empty(t, result.Parameters)
		})
	}
}

func TestConvertStreamOptions(t *testing.T) {
	tests := []struct {
		name     string
		raw      json.RawMessage
		expected *StreamOptions
	}{
		{
			name:     "empty raw",
			raw:      nil,
			expected: nil,
		},
		{
			name:     "include obfuscation false",
			raw:      json.RawMessage(`{"include_obfuscation":false}`),
			expected: &StreamOptions{IncludeObfuscation: lo.ToPtr(false)},
		},
		{
			name:     "include obfuscation true",
			raw:      json.RawMessage(`{"include_obfuscation":true}`),
			expected: &StreamOptions{IncludeObfuscation: lo.ToPtr(true)},
		},
		{
			name:     "no include obfuscation key",
			raw:      json.RawMessage(`{"include_usage":true}`),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertStreamOptions(tt.raw)
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
			name: "converts MaxToolCalls to PE.Request",
			req: &Request{
				Model:        "gpt-4o",
				MaxToolCalls: lo.ToPtr(int64(10)),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.ProviderExtensions.OpenAIResponses.Request.MaxToolCalls)
				require.Equal(t, int64(10), *chatReq.ProviderExtensions.OpenAIResponses.Request.MaxToolCalls)
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
			name: "converts PromptCacheRetention to PE.Request",
			req: &Request{
				Model:                "gpt-4o",
				PromptCacheRetention: lo.ToPtr("24h"),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.ProviderExtensions.OpenAIResponses.Request.PromptCacheRetention)
				require.Equal(t, "24h", *chatReq.ProviderExtensions.OpenAIResponses.Request.PromptCacheRetention)
			},
		},
		{
			name: "converts Truncation to PE.Request",
			req: &Request{
				Model:      "gpt-4o",
				Truncation: lo.ToPtr("auto"),
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.ProviderExtensions.OpenAIResponses.Request.Truncation)
				require.Equal(t, "auto", *chatReq.ProviderExtensions.OpenAIResponses.Request.Truncation)
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
			name: "converts Include to PE.OpenAIResponses.Request",
			req: &Request{
				Model:   "gpt-4o",
				Include: []string{"file_search_call.results", "reasoning.encrypted_content"},
			},
			validate: func(t *testing.T, chatReq *llm.Request) {
				require.NotNil(t, chatReq.ProviderExtensions)
				require.NotNil(t, chatReq.ProviderExtensions.OpenAIResponses)
				require.NotNil(t, chatReq.ProviderExtensions.OpenAIResponses.Request)
				require.Equal(t, []string{"file_search_call.results", "reasoning.encrypted_content"},
					chatReq.ProviderExtensions.OpenAIResponses.Request.Include)
				if chatReq.TransformerMetadata != nil {
					_, ok := chatReq.TransformerMetadata["include"]
					require.False(t, ok)
				}
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
		{
			name: "user message with input_audio",
			msgs: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{
								Type: "input_audio",
								InputAudio: &llm.InputAudio{
									Data:   "audio-base64-data",
									Format: "wav",
								},
							},
						},
					},
				},
			},
			expected: Input{
				Items: []Item{
					{
						Type: "message",
						Role: "user",
						Content: &Input{
							Items: []Item{
								{
									Type: "input_audio",
									InputAudio: &llm.InputAudio{
										Data:   "audio-base64-data",
										Format: "wav",
									},
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
			result := convertInputFromMessages(tt.msgs, tt.transformOptions, nil)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStructurallyRepresentedInputItem(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		expected bool
	}{
		{"empty", "", true},
		{"message", "message", true},
		{"input_text", "input_text", true},
		{"input_image", "input_image", true},
		{"input_audio", "input_audio", true},
		{"function_call", "function_call", true},
		{"function_call_output", "function_call_output", true},
		{"reasoning", "reasoning", true},
		{"compaction", "compaction", true},
		{"unknown_type", "unknown_type", false},
		{"file_search", "file_search", false},
	}


	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStructurallyRepresentedInputItem(tt.itemType)
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
			name: "both effort and budget specified - effort wins per requirement",
			req: &llm.Request{
				ReasoningEffort: "medium",
				ReasoningBudget: lo.ToPtr(int64(3000)),
			},
			expected: &Reasoning{
				Effort:    "medium",
				MaxTokens: nil, // effort takes priority; max_tokens omitted to avoid mutually-exclusive fields
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
				MaxTokens: nil, // effort present, so budget omitted per mutual-exclusion rule
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
		{
			name: "with only responses reasoning context",
			req: &llm.Request{
				ProviderExtensions: &llm.ProviderExtensions{
					OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
						Request: &llm.OpenAIResponsesRequestExtensions{
							ReasoningContext: "all_turns",
						},
					},
				},
			},
			expected: &Reasoning{
				Context: "all_turns",
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
			name: "message output with nil content",
			output: []Item{
				{
					ID:      "msg_nil",
					Type:    "message",
					Content: nil,
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "msg_nil", msg.ID)
				require.Nil(t, msg.Content.Content)
				require.Nil(t, msg.Content.MultipleContent)
				require.Empty(t, msg.Annotations)
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
			name: "output_text annotations are converted",
			output: []Item{
				{
					ID:   "msg_annotated",
					Type: "message",
					Content: &Input{Items: []Item{
						{
							Type: "output_text",
							Text: lo.ToPtr("Alpha"),
							Annotations: []Annotation{
								{
									Type:       "url_citation",
									StartIndex: lo.ToPtr(int64(0)),
									EndIndex:   lo.ToPtr(int64(5)),
									URLCitation: &URLCitation{
										URL:   "https://example.com/alpha",
										Title: "Alpha Source",
									},
								},
							},
						},
					}},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.Equal(t, "msg_annotated", msg.ID)
				require.NotNil(t, msg.Content.Content)
				require.Equal(t, "Alpha", *msg.Content.Content)
				require.Len(t, msg.Annotations, 1)
				require.Equal(t, "url_citation", msg.Annotations[0].Type)
				require.NotNil(t, msg.Annotations[0].StartIndex)
				require.Equal(t, int64(0), *msg.Annotations[0].StartIndex)
				require.NotNil(t, msg.Annotations[0].EndIndex)
				require.Equal(t, int64(5), *msg.Annotations[0].EndIndex)
				require.NotNil(t, msg.Annotations[0].URLCitation)
				require.Equal(t, "https://example.com/alpha", msg.Annotations[0].URLCitation.URL)
				require.Equal(t, "Alpha Source", msg.Annotations[0].URLCitation.Title)
			},
		},
		{
			name: "output_text annotations are rebased across items using rune length",
			output: []Item{
				{
					Type: "message",
					Content: &Input{Items: []Item{
						{
							Type: "output_text",
							Text: lo.ToPtr("你好"),
							Annotations: []Annotation{
								{
									Type:       "url_citation",
									StartIndex: lo.ToPtr(int64(0)),
									EndIndex:   lo.ToPtr(int64(2)),
									URLCitation: &URLCitation{
										URL:   "https://example.com/nihao",
										Title: "Ni Hao",
									},
								},
							},
						},
					}},
				},
				{
					Type: "output_text",
					Text: lo.ToPtr("世界"),
					Annotations: []Annotation{
						{
							Type:       "url_citation",
							StartIndex: lo.ToPtr(int64(0)),
							EndIndex:   lo.ToPtr(int64(2)),
							URLCitation: &URLCitation{
								URL:   "https://example.com/shijie",
								Title: "Shi Jie",
							},
						},
					},
				},
			},
			validate: func(t *testing.T, msg llm.Message) {
				require.NotNil(t, msg.Content.Content)
				require.Equal(t, "你好世界", *msg.Content.Content)
				require.Len(t, msg.Annotations, 2)
				require.NotNil(t, msg.Annotations[0].StartIndex)
				require.Equal(t, int64(0), *msg.Annotations[0].StartIndex)
				require.NotNil(t, msg.Annotations[0].EndIndex)
				require.Equal(t, int64(2), *msg.Annotations[0].EndIndex)
				require.NotNil(t, msg.Annotations[1].StartIndex)
				require.Equal(t, int64(2), *msg.Annotations[1].StartIndex)
				require.NotNil(t, msg.Annotations[1].EndIndex)
				require.Equal(t, int64(4), *msg.Annotations[1].EndIndex)
				require.NotNil(t, msg.Annotations[1].URLCitation)
				require.Equal(t, "https://example.com/shijie", msg.Annotations[1].URLCitation.URL)
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
			msg := convertOutputToMessage(tt.output, tt.transformerMetadata)
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
			result := convertAssistantMessage(tt.msg, nil)
			tt.validate(t, result)
		})
	}
}

// TestConvertToLLMRequest_SamplingPenalties covers #13: Responses inbound
// must read frequency_penalty/presence_penalty from the request body.
func TestConvertToLLMRequest_SamplingPenalties(t *testing.T) {
	req := &Request{
		Model:            "gpt-4o",
		FrequencyPenalty: lo.ToPtr(0.5),
		PresencePenalty:  lo.ToPtr(0.3),
	}

	result, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.NotNil(t, result.FrequencyPenalty)
	require.Equal(t, 0.5, *result.FrequencyPenalty)
	require.NotNil(t, result.PresencePenalty)
	require.Equal(t, 0.3, *result.PresencePenalty)
}

// TestConvertToLLMRequest_Background covers #15: Responses inbound must
// preserve the top-level background flag on PE.OpenAIResponses.Request.
func TestConvertToLLMRequest_Background(t *testing.T) {
	t.Run("background true preserved", func(t *testing.T) {
		req := &Request{
			Model:      "gpt-4o",
			Background: lo.ToPtr(true),
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		require.NotNil(t, result.ProviderExtensions.OpenAIResponses.Request.Background)
		require.Equal(t, true, *result.ProviderExtensions.OpenAIResponses.Request.Background)
	})

	t.Run("background absent stays absent", func(t *testing.T) {
		req := &Request{
			Model: "gpt-4o",
		}

		result, err := convertToLLMRequest(req)
		require.NoError(t, err)
		if result.ProviderExtensions != nil && result.ProviderExtensions.OpenAIResponses != nil && result.ProviderExtensions.OpenAIResponses.Request != nil {
			require.Nil(t, result.ProviderExtensions.OpenAIResponses.Request.Background)
		}
	})
}

// TestConvertToLLMRequest_Modalities: Responses request model has no modalities
// field; modalities live on common llm.Request for Chat. Kept as documentation
// that convertToLLMRequest does not invent Responses.Modalities.
func TestConvertToLLMRequest_ModalitiesAbsentOnResponsesRequest(t *testing.T) {
	req := &Request{
		Model: "gpt-4o",
		Input: Input{Text: lo.ToPtr("hi")},
	}
	result, err := convertToLLMRequest(req)
	require.NoError(t, err)
	require.Empty(t, result.Modalities)
}

// TestCustomToolCall_NamespaceRoundTrip covers #10: custom_tool_call must
// carry namespace through canonical ResponseCustomToolCall on both the
// inbound (outputItem -> ToolCall) and outbound (ToolCall -> Item) paths.
func TestCustomToolCall_NamespaceRoundTrip(t *testing.T) {
	t.Run("inbound preserves namespace", func(t *testing.T) {
		outputItem := Item{
			ID:        "ctc_ns",
			Type:      "custom_tool_call",
			CallID:    "call_ns_1",
			Name:      "apply_patch",
			Namespace: "mcp__myserver",
			Input:     lo.ToPtr("*** Begin Patch\n*** End Patch"),
			Status:    lo.ToPtr("completed"),
		}

		msg := convertOutputToMessage([]Item{outputItem}, nil)
		require.Len(t, msg.ToolCalls, 1)
		tc := msg.ToolCalls[0]
		require.NotNil(t, tc.ResponseCustomToolCall)
		require.Equal(t, "mcp__myserver", tc.ResponseCustomToolCall.Namespace)
		require.Equal(t, "apply_patch", tc.ResponseCustomToolCall.Name)
	})

	t.Run("outbound preserves namespace", func(t *testing.T) {
		chatResp := &llm.Response{
			ID:    "resp_ns",
			Model: "gpt-4o",
			Choices: []llm.Choice{{
				Message: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{{
						ID:   "call_ns_2",
						Type: llm.ToolTypeResponsesCustomTool,
						ResponseCustomToolCall: &llm.ResponseCustomToolCall{
							CallID:    "call_ns_2",
							Name:      "apply_patch",
							Namespace: "mcp__myserver",
							Input:     "*** Begin Patch\n*** End Patch",
						},
					}},
				},
				FinishReason: lo.ToPtr("tool_calls"),
			}},
		}

		resp := convertToResponsesAPIResponse(chatResp)
		require.Len(t, resp.Output, 1)
		item := resp.Output[0]
		require.Equal(t, "custom_tool_call", item.Type)
		require.Equal(t, "mcp__myserver", item.Namespace)
		require.Equal(t, "apply_patch", item.Name)
	})
}

// TestFunctionCallItem_IDAndStatusRoundTrip covers #19: function_call items
// must preserve their item ID (distinct from call_id) and status across the
// Responses inbound (outputItem -> ToolCall) and outbound (ToolCall -> Item)
// conversion, instead of overwriting ID with call_id and hardcoding status.
func TestFunctionCallItem_IDAndStatusRoundTrip(t *testing.T) {
	t.Run("inbound preserves item id and status", func(t *testing.T) {
		outputItem := Item{
			ID:        "fc_item_1",
			Type:      "function_call",
			CallID:    "call_fc_1",
			Name:      "get_weather",
			Namespace: "",
			Arguments: `{"city":"sf"}`,
			Status:    lo.ToPtr("incomplete"),
		}

		msg := convertOutputToMessage([]Item{outputItem}, nil)
		require.Len(t, msg.ToolCalls, 1)
		tc := msg.ToolCalls[0]
		require.Equal(t, "call_fc_1", tc.ID)
		require.Equal(t, "fc_item_1", tc.ResponseItemID)
		require.Equal(t, "incomplete", tc.Status)
	})

	t.Run("outbound restores item id and status", func(t *testing.T) {
		chatResp := &llm.Response{
			ID:    "resp_fc",
			Model: "gpt-4o",
			Choices: []llm.Choice{{
				Message: &llm.Message{
					Role: "assistant",
					ToolCalls: []llm.ToolCall{{
						ID:             "call_fc_2",
						Type:           "function",
						ResponseItemID: "fc_item_2",
						Status:         "incomplete",
						Function: llm.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"sf"}`,
						},
					}},
				},
				FinishReason: lo.ToPtr("tool_calls"),
			}},
		}

		resp := convertToResponsesAPIResponse(chatResp)
		require.Len(t, resp.Output, 1)
		item := resp.Output[0]
		require.Equal(t, "function_call", item.Type)
		require.Equal(t, "fc_item_2", item.ID)
		require.Equal(t, "call_fc_2", item.CallID)
		require.NotNil(t, item.Status)
		require.Equal(t, "incomplete", *item.Status)
	})
}

func TestConvertAssistantMessage_CustomToolCallEmitsNamespace(t *testing.T) {
	msg := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:   "call_ns_3",
			Type: llm.ToolTypeResponsesCustomTool,
			ResponseCustomToolCall: &llm.ResponseCustomToolCall{CallID: "call_ns_3", Name: "apply_patch", Namespace: "mcp__myserver", Input: "patch"},
		}},
	}
	items := convertAssistantMessage(msg, nil)
	var found bool
	for _, it := range items {
		if it.Type == "custom_tool_call" {
			found = true
			require.Equal(t, "mcp__myserver", it.Namespace, "D11(iii): convertAssistantMessage must emit custom_tool_call item with namespace")
			require.Equal(t, "call_ns_3", it.CallID)
		}
	}
	require.True(t, found, "expected a custom_tool_call item in output")
}

// TestConvertAssistantMessage_NamespaceFunctionCallRestored covers #1a/D1:
// when building function_call Items from an assistant message whose tool call
// carries a flattened composite name, convertAssistantMessage must look up the
// namespace tool map in metadata and restore {name:leaf, namespace:group}.
func TestConvertAssistantMessage_NamespaceFunctionCallRestored(t *testing.T) {
	msg := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:   "call_ns_1",
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "mcp__node_repl__run",
				Arguments: `{"x":1}`,
			},
		}},
	}
	metadata := map[string]any{
		responsesNamespaceToolMapTransformerMetadataKey: map[string]namespaceToolEntry{
			"mcp__node_repl__run": {Leaf: "run", Namespace: "mcp__node_repl"},
		},
	}
	items := convertAssistantMessage(msg, metadata)
	var found bool
	for _, it := range items {
		if it.Type == "function_call" {
			found = true
			require.Equal(t, "run", it.Name, "name must be restored to leaf")
			require.Equal(t, "mcp__node_repl", it.Namespace, "namespace must be restored to group")
		}
	}
	require.True(t, found, "expected a function_call item")
}

// TestConvertAssistantMessage_FlatFunctionCallUnchanged ensures flat tools
// with no map entry keep their original name and empty namespace.
func TestConvertAssistantMessage_FlatFunctionCallUnchanged(t *testing.T) {
	msg := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{{
			ID:   "call_flat",
			Type: "function",
			Function: llm.FunctionCall{
				Name:      "get_weather",
				Arguments: `{}`,
			},
		}},
	}
	items := convertAssistantMessage(msg, nil)
	var found bool
	for _, it := range items {
		if it.Type == "function_call" {
			found = true
			require.Equal(t, "get_weather", it.Name)
			require.Equal(t, "", it.Namespace)
		}
	}
	require.True(t, found)
}
