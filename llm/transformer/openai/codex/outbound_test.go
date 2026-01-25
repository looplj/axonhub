package codex

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/llm"
)

func TestIsCodexRequest(t *testing.T) {
	tests := []struct {
		name     string
		msgs     []llm.Message
		expected bool
	}{
		{
			name: "contains codex prefix in system message",
			msgs: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr(CodexInstructionPrefix + " some instructions")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			expected: true,
		},
		{
			name: "contains 'You are Codex' in developer message",
			msgs: []llm.Message{
				{Role: "developer", Content: llm.MessageContent{Content: lo.ToPtr("You are Codex, a helpful assistant")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			expected: true,
		},
		{
			name: "does not contain codex prefix",
			msgs: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("You are a helpful assistant")}},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			expected: false,
		},
		{
			name: "no system messages",
			msgs: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
			},
			expected: false,
		},
		{
			name:     "empty messages",
			msgs:     []llm.Message{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isCodexRequest(tt.msgs))
		})
	}
}

func TestSetCodexSystemInstruction(t *testing.T) {
	msgs := []llm.Message{
		{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("Existing system instruction")}},
		{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}},
	}

	result := appendCodexSystemInstruction(msgs)

	assert.Len(t, result, 3)
	assert.Equal(t, "system", result[0].Role)
	assert.Equal(t, CodexInstructions, *result[0].Content.Content)
	assert.Equal(t, msgs[0], result[1])
	assert.Equal(t, msgs[1], result[2])
}
