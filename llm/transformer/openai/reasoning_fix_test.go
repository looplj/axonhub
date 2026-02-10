package openai

import (
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestIsReasoningSignatureEvent_Fix(t *testing.T) {
	tests := []struct {
		name     string
		response *llm.Response
		wantSkip bool // true means isReasoningSignatureEvent returns true (skip this chunk)
	}{
		{
			name: "Anthropic Pure Signature (Should Skip)",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: lo.ToPtr("signature_123"),
							// No content
						},
					},
				},
			},
			wantSkip: true,
		},
		{
			name: "Gemini Mixed Chunk - Signature + Content (Should Keep)",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: lo.ToPtr("signature_gemini"),
							Content: llm.MessageContent{
								Content: lo.ToPtr("Thinking done. Hello!"),
							},
						},
					},
				},
			},
			wantSkip: false, // Fix: Before valid check this was true (skipped)
		},
		{
			name: "Gemini Mixed Chunk - Signature + Reasoning (Should Keep)",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							ReasoningSignature: lo.ToPtr("signature_gemini"),
							ReasoningContent:   lo.ToPtr("I am thinking..."),
						},
					},
				},
			},
			wantSkip: false, // Fix: Before valid check this was true (skipped)
		},
		{
			name: "Standard Content (Should Keep)",
			response: &llm.Response{
				Choices: []llm.Choice{
					{
						Delta: &llm.Message{
							Content: llm.MessageContent{
								Content: lo.ToPtr("Just content"),
							},
						},
					},
				},
			},
			wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isReasoningSignatureEvent(tt.response)
			assert.Equal(t, tt.wantSkip, got, "isReasoningSignatureEvent mismatch")
		})
	}
}
