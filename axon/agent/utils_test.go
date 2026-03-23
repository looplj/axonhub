package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloneMessages_DeepCopiesContent(t *testing.T) {
	text := "original"
	toolID := "tool-1"
	isErr := true
	msgs := []Message{
		{
			Role:    RoleUser,
			Content: &Content{Text: &text},
		},
		{
			Role:      RoleAssistant,
			ToolCall:  &ToolCall{ID: "tu-1", Name: "search", Input: `{"q":"test"}`},
			ToolUseID: &toolID,
			IsError:   &isErr,
		},
	}

	cloned := cloneMessages(msgs)
	require.Len(t, cloned, 2)

	// Mutate originals
	*msgs[0].Content.Text = "mutated"
	msgs[1].ToolCall.Name = "mutated"
	*msgs[1].ToolUseID = "mutated"
	*msgs[1].IsError = false

	// Cloned values should be unaffected
	assert.Equal(t, "original", *cloned[0].Content.Text)
	assert.Equal(t, "search", cloned[1].ToolCall.Name)
	assert.Equal(t, "tool-1", *cloned[1].ToolUseID)
	assert.Equal(t, true, *cloned[1].IsError)
}

func TestCloneMessages_NilReturnsNil(t *testing.T) {
	assert.Nil(t, cloneMessages(nil))
}

func TestCloneMessages_EmptyReturnsEmpty(t *testing.T) {
	result := cloneMessages([]Message{})
	require.NotNil(t, result)
	assert.Empty(t, result)
}

func TestCloneMessages_DeepCopiesParts(t *testing.T) {
	msgs := []Message{
		{
			Role: RoleAssistant,
			Content: &Content{
				Parts: []ContentPart{
					{Type: ContentPartText, Text: "hello"},
					{Type: ContentPartThinking, Thinking: "thinking..."},
				},
			},
		},
	}

	cloned := cloneMessages(msgs)
	msgs[0].Content.Parts[0].Text = "mutated"

	assert.Equal(t, "hello", cloned[0].Content.Parts[0].Text)
}
