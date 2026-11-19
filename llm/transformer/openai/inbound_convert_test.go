package openai

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
)

// TestToLLMMessage_ReasoningField tests reasoning field conversion from client format to unified format.
// Both Reasoning and ReasoningContent are preserved and synced.
func TestToLLMMessage_ReasoningField(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		want    llm.Message
	}{
		{
			name: "Only reasoning field - should sync to ReasoningContent",
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
			name: "Only reasoning_text field (GitHub Copilot) - should normalize to both",
			message: Message{
				Role:          "assistant",
				ReasoningText: lo.ToPtr("I'm thinking about this step by step"),
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
		},
		{
			name: "Only reasoning_content field - should sync to Reasoning",
			message: Message{
				Role:             "assistant",
				Reasoning:        nil,
				ReasoningContent: lo.ToPtr("I'm thinking about this step by step"),
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking about this step by step"),
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
			name: "Empty reasoning field - should not sync to ReasoningContent",
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
			name: "Empty reasoning_content with non-empty reasoning - should sync from Reasoning",
			message: Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking"),
				ReasoningContent: lo.ToPtr(""),
			},
			want: llm.Message{
				Role:             "assistant",
				Reasoning:        lo.ToPtr("I'm thinking"),
				ReasoningContent: lo.ToPtr(""),
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

func TestCopilotReasoningOpaqueToLLM(t *testing.T) {
	opaque := base64.StdEncoding.EncodeToString([]byte{0x57, 0xe9, 0x79, 0x46})
	source := Message{
		Role:            "assistant",
		ReasoningText:   lo.ToPtr("thinking"),
		ReasoningOpaque: lo.ToPtr(opaque),
	}

	got := source.ToLLMMessage()
	assert.Equal(t, opaque, lo.FromPtr(got.ReasoningSignature))
	assert.Equal(t, "thinking", lo.FromPtr(got.ReasoningContent))

	// Generic Chat Completions upstreams do not receive Copilot's field.
	req := RequestFromLLM(t.Context(), &llm.Request{
		Model:    "gpt-4o",
		Messages: []llm.Message{got},
	}, ReasoningFieldAll)
	assert.Nil(t, req.Messages[0].ReasoningOpaque)
}

func TestReasoningOpaqueGoogleSignatureFallback(t *testing.T) {
	const googleSignature = "CgR0ZXN0"
	for _, tt := range []struct {
		name   string
		opaque *string
		want   string
	}{
		{"absent", nil, googleSignature},
		{"empty", lo.ToPtr(""), googleSignature},
		{"whitespace", lo.ToPtr(" \t\n"), googleSignature},
		{"nonempty stays unchanged", lo.ToPtr("  V+l5RhjeGPTC  "), "  V+l5RhjeGPTC  "},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := Message{
				Role: "assistant", ReasoningOpaque: tt.opaque,
				ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: FunctionCall{Name: "Read", Arguments: "{}"},
					ExtraContent: &ToolCallExtraContent{Google: &ToolCallGoogleExtraContent{ThoughtSignature: googleSignature}},
				}},
			}
			got := source.ToLLMMessage()
			assert.Equal(t, tt.want, lo.FromPtr(got.ReasoningSignature))
			assert.Equal(t, googleSignature, got.ToolCalls[0].TransformerMetadata[TransformerMetadataKeyGoogleThoughtSignature])
		})
	}
}
