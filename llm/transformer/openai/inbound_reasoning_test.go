package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
)

func TestIsReasoningSignatureEvent(t *testing.T) {
	signature := "test-signature"
	reasoningContent := "test-reasoning-content"
	content := "test-content"

	tests := []struct {
		name     string
		response *llm.Response
		want     bool
	}{
		{
			name: "Anthropic pure signature event - should be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Gemini mixed chunk with signature and reasoning content - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
							ReasoningContent:   &reasoningContent,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Gemini mixed chunk with signature and content - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
							Content: llm.MessageContent{
								Content: &content,
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Standard content without signature - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							Content: llm.MessageContent{
								Content: &content,
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Empty signature - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: func() *string { s := ""; return &s }(),
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Nil signature - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							Content: llm.MessageContent{
								Content: &content,
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Multiple choices - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
						},
					},
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Nil delta - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: nil,
					},
				},
			},
			want: false,
		},
		{
			name: "No choices - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{},
			},
			want: false,
		},
		{
			name: "Chunk with signature and tool calls - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
							ToolCalls: []llm.ToolCall{
								{ID: "call_123", Type: "function"},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Chunk with signature and refusal - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
							Refusal:            "I cannot answer this",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Chunk with signature and multiple content - should NOT be skipped",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: &signature,
							Content: llm.MessageContent{
								MultipleContent: []llm.MessageContentPart{
									{Type: "text", Text: func() *string { s := "test"; return &s }()},
								},
							},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReasoningSignatureEvent(tt.response)
			assert.Equal(t, tt.want, got)
		})
	}
}
