package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestMessageContent_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		content  MessageContent
		expected string
	}{
		{
			name:     "string content",
			content:  MessageContent{Content: lo.ToPtr("Hello")},
			expected: `"Hello"`,
		},
		{
			name:     "nil content",
			content:  MessageContent{Content: nil},
			expected: `null`,
		},
		{
			name: "single text part collapses to string",
			content: MessageContent{
				MultipleContent: []MessageContentPart{
					{Type: "text", Text: lo.ToPtr("Hello")},
				},
			},
			expected: `"Hello"`,
		},
		{
			name: "multiple parts as array",
			content: MessageContent{
				MultipleContent: []MessageContentPart{
					{Type: "text", Text: lo.ToPtr("Look at this")},
					{Type: "image_url", ImageURL: &ImageURL{URL: "https://example.com/image.png"}},
				},
			},
			expected: `[{"type":"text","text":"Look at this"},{"type":"image_url","image_url":{"url":"https://example.com/image.png"}}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.content)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestMessageContent_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, MessageContent)
	}{
		{
			name:  "string content",
			input: `"Hello world"`,
			validate: func(t *testing.T, c MessageContent) {
				require.NotNil(t, c.Content)
				require.Equal(t, "Hello world", *c.Content)
				require.Empty(t, c.MultipleContent)
			},
		},
		{
			name:  "array content",
			input: `[{"type":"text","text":"Hello"},{"type":"image_url","image_url":{"url":"https://example.com/img.png"}}]`,
			validate: func(t *testing.T, c MessageContent) {
				require.Nil(t, c.Content)
				require.Len(t, c.MultipleContent, 2)
				require.Equal(t, "text", c.MultipleContent[0].Type)
				require.Equal(t, "Hello", *c.MultipleContent[0].Text)
				require.Equal(t, "image_url", c.MultipleContent[1].Type)
				require.Equal(t, "https://example.com/img.png", c.MultipleContent[1].ImageURL.URL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content MessageContent

			err := json.Unmarshal([]byte(tt.input), &content)
			require.NoError(t, err)
			tt.validate(t, content)
		})
	}
}

func TestStop_MarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		stop     Stop
		expected string
	}{
		{
			name:     "single stop",
			stop:     Stop{Stop: lo.ToPtr("END")},
			expected: `"END"`,
		},
		{
			name:     "multiple stops",
			stop:     Stop{MultipleStop: []string{"END", "STOP", "DONE"}},
			expected: `["END","STOP","DONE"]`,
		},
		{
			name:     "empty stop",
			stop:     Stop{},
			expected: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.stop)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(data))

			// Test round-trip
			var unmarshaled Stop

			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			if tt.stop.Stop != nil {
				require.Equal(t, *tt.stop.Stop, *unmarshaled.Stop)
			}

			if len(tt.stop.MultipleStop) > 0 {
				require.Equal(t, tt.stop.MultipleStop, unmarshaled.MultipleStop)
			}
		})
	}
}

func TestToolChoice_MarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		choice   ToolChoice
		expected string
	}{
		{
			name:     "string choice",
			choice:   ToolChoice{ToolChoice: lo.ToPtr("auto")},
			expected: `"auto"`,
		},
		{
			name: "named choice",
			choice: ToolChoice{
				NamedToolChoice: &NamedToolChoice{
					Type:     "function",
					Function: ToolFunction{Name: "get_weather"},
				},
			},
			expected: `{"type":"function","function":{"name":"get_weather"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.choice)
			require.NoError(t, err)
			require.JSONEq(t, tt.expected, string(data))

			var unmarshaled ToolChoice

			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err)

			if tt.choice.ToolChoice != nil {
				require.NotNil(t, unmarshaled.ToolChoice)
				require.Equal(t, *tt.choice.ToolChoice, *unmarshaled.ToolChoice)
			}

			if tt.choice.NamedToolChoice != nil {
				require.NotNil(t, unmarshaled.NamedToolChoice)
				require.Equal(t, tt.choice.NamedToolChoice.Type, unmarshaled.NamedToolChoice.Type)
				require.Equal(t, tt.choice.NamedToolChoice.Function.Name, unmarshaled.NamedToolChoice.Function.Name)
			}
		})
	}
}

func TestRoundTrip_Request(t *testing.T) {
	original := &Request{
		Model: "gpt-4",
		Messages: []Message{
			{Role: "system", Content: MessageContent{Content: lo.ToPtr("You are helpful.")}},
			{Role: "user", Content: MessageContent{Content: lo.ToPtr("Hello")}},
		},
		Temperature:         lo.ToPtr(0.7),
		MaxTokens:           lo.ToPtr(int64(100)),
		MaxCompletionTokens: lo.ToPtr(int64(200)),
		Stream:              lo.ToPtr(true),
		StreamOptions:       &StreamOptions{IncludeUsage: true},
		Tools: []Tool{
			{Type: "function", Function: Function{Name: "test_func", Parameters: json.RawMessage(`{}`)}},
		},
	}

	// Convert to llm.Request and back
	llmReq := original.ToLLMRequest()
	roundTripped := RequestFromLLM(llmReq)

	require.Equal(t, original.Model, roundTripped.Model)
	require.Len(t, roundTripped.Messages, len(original.Messages))
	require.Equal(t, *original.Temperature, *roundTripped.Temperature)
	require.Equal(t, *original.MaxTokens, *roundTripped.MaxTokens)
	require.Equal(t, *original.Stream, *roundTripped.Stream)
	require.Equal(t, original.StreamOptions.IncludeUsage, roundTripped.StreamOptions.IncludeUsage)
	require.Len(t, roundTripped.Tools, len(original.Tools))
}

func TestRoundTrip_Response(t *testing.T) {
	original := &Response{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "gpt-4",
		Choices: []Choice{
			{
				Index:        0,
				Message:      &Message{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("Hi!")}},
				FinishReason: lo.ToPtr("stop"),
			},
		},
		Usage: &Usage{
			Usage: llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		},
		SystemFingerprint: "fp_123",
		ServiceTier:       "default",
	}

	// Convert to llm.Response and back
	llmResp := original.ToLLMResponse()
	roundTripped := ResponseFromLLM(llmResp)

	require.Equal(t, original.ID, roundTripped.ID)
	require.Equal(t, original.Object, roundTripped.Object)
	require.Equal(t, original.Created, roundTripped.Created)
	require.Equal(t, original.Model, roundTripped.Model)
	require.Len(t, roundTripped.Choices, len(original.Choices))
	require.Equal(t, *original.Choices[0].Message.Content.Content, *roundTripped.Choices[0].Message.Content.Content)
	require.Equal(t, original.SystemFingerprint, roundTripped.SystemFingerprint)
	require.Equal(t, original.ServiceTier, roundTripped.ServiceTier)
	require.NotNil(t, roundTripped.Usage)
	require.Equal(t, original.Usage.PromptTokens, roundTripped.Usage.PromptTokens)
}
