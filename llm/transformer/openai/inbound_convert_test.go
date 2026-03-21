package openai

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
)

// TestToLLMMessage_ReasoningField tests parsing of reasoning field from JSON
func TestToLLMMessage_ReasoningField(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		want    llm.Message
	}{
		{
			name: "Only reasoning field - should populate both Reasoning and ReasoningContent",
			message: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: nil,
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Only reasoning_content field - normal behavior",
			message: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Both fields present - both preserved",
			message: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Neither field present - both nil",
			message: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
		},
		{
			name: "Empty reasoning field - should not populate ReasoningContent",
			message: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr(""),
				ReasoningContent: nil,
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr(""),
				ReasoningContent: nil,
			},
		},
		{
			name: "Nil reasoning field - should not populate ReasoningContent",
			message: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.message.ToLLMMessage()
			assert.Equal(t, tt.want.Role, got.Role)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Refusal, got.Refusal)
			assert.Equal(t, tt.want.ToolCallID, got.ToolCallID)
			assert.Equal(t, tt.want.Reasoning, got.Reasoning)
			assert.Equal(t, tt.want.ReasoningContent, got.ReasoningContent)
		})
	}
}

func TestMessageContent_VideoURLRoundTrip(t *testing.T) {
	raw := []byte(`[{"type":"video_url","video_url":{"url":"https://example.com/example.mp4"}}]`)

	var content MessageContent
	err := json.Unmarshal(raw, &content)
	assert.NoError(t, err)
	assert.Len(t, content.MultipleContent, 1)
	assert.Equal(t, "video_url", content.MultipleContent[0].Type)
	if assert.NotNil(t, content.MultipleContent[0].VideoURL) {
		assert.Equal(t, "https://example.com/example.mp4", content.MultipleContent[0].VideoURL.URL)
	}

	llmContent := content.ToLLMContent()
	assert.Len(t, llmContent.MultipleContent, 1)
	if assert.NotNil(t, llmContent.MultipleContent[0].VideoURL) {
		assert.Equal(t, "https://example.com/example.mp4", llmContent.MultipleContent[0].VideoURL.URL)
	}

	roundTrip := MessageContentFromLLM(llmContent)
	if assert.NotNil(t, roundTrip.MultipleContent[0].VideoURL) {
		assert.Equal(t, "https://example.com/example.mp4", roundTrip.MultipleContent[0].VideoURL.URL)
	}
}

// TestMessageFromLLM_ReasoningSync tests outbound sync of reasoning_content to reasoning
func TestMessageFromLLM_ReasoningSync(t *testing.T) {
	tests := []struct {
		name    string
		message llm.Message
		want    Message
	}{
		{
			name: "Only reasoning_content - should sync to Reasoning",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Only reasoning - should sync to ReasoningContent via fallback",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: nil,
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Both fields - both preserved",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Neither field - both nil",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
		},
		{
			name: "Empty reasoning_content - should not sync",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: lo.ToPtr(""),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: lo.ToPtr(""),
			},
		},
		{
			name: "Nil reasoning_content - should not sync",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MessageFromLLM(tt.message)
			assert.Equal(t, tt.want.Role, got.Role)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Refusal, got.Refusal)
			assert.Equal(t, tt.want.ToolCallID, got.ToolCallID)
			assert.Equal(t, tt.want.Reasoning, got.Reasoning)
			assert.Equal(t, tt.want.ReasoningContent, got.ReasoningContent)
		})
	}
}

// TestMessageFromLLM_ReasoningFallback tests fallback logic for reasoning field
func TestMessageFromLLM_ReasoningFallback(t *testing.T) {
	tests := []struct {
		name    string
		message llm.Message
		want    Message
	}{
		{
			name: "ReasoningSignature with foreign signature - should clear both fields",
			message: llm.Message{
				Role:               "assistant",
				Reasoning:          lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent:   lo.ToPtr("I'm thinking about this step by step"),
				ReasoningSignature: lo.ToPtr("foreign-signature"),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "ReasoningSignature with OpenAI encrypted content - should preserve reasoning_content",
			message: llm.Message{
				Role:               "assistant",
				Reasoning:          lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent:   lo.ToPtr("I'm thinking about this step by step"),
				ReasoningSignature: lo.ToPtr("QVhOMTAz"), // AXN103 base64 encoded prefix for OpenAI encrypted content
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "ReasoningSignature empty - should preserve reasoning_content",
			message: llm.Message{
				Role:               "assistant",
				Reasoning:          lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent:   lo.ToPtr("I'm thinking about this step by step"),
				ReasoningSignature: lo.ToPtr(""),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "ReasoningSignature nil - should preserve reasoning_content",
			message: llm.Message{
				Role:               "assistant",
				Reasoning:          lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent:   lo.ToPtr("I'm thinking about this step by step"),
				ReasoningSignature: nil,
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Foreign signature with only ReasoningContent - should drop reasoning_content",
			message: llm.Message{
				Role:               "assistant",
				Reasoning:          nil,
				ReasoningContent:   lo.ToPtr("foreign reasoning content"),
				ReasoningSignature: lo.ToPtr("foreign-sig"),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("foreign reasoning content"),
				ReasoningContent: lo.ToPtr("foreign reasoning content"),
			},
		},
		{
			name: "Empty string ReasoningContent with Reasoning - should sync from Reasoning",
			message: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("valid reasoning"),
				ReasoningContent: lo.ToPtr(""),
			},
			want: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("valid reasoning"),
				ReasoningContent: lo.ToPtr(""),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MessageFromLLM(tt.message)
			assert.Equal(t, tt.want.Role, got.Role)
			assert.Equal(t, tt.want.Name, got.Name)
			assert.Equal(t, tt.want.Refusal, got.Refusal)
			assert.Equal(t, tt.want.ToolCallID, got.ToolCallID)
			assert.Equal(t, tt.want.Reasoning, got.Reasoning)
			assert.Equal(t, tt.want.ReasoningContent, got.ReasoningContent)
		})
	}
}
