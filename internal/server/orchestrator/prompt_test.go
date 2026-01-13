package orchestrator

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestInjectPrompts_NoProjectID(t *testing.T) {
	ctx := context.Background()

	state := &PersistenceState{
		PromptProvider: nil,
	}
	inbound := &PersistentInboundTransformer{state: state}

	middleware := injectPrompts(inbound)

	userContent := "Hello"
	request := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &userContent}},
		},
	}

	result, err := middleware.OnInboundLlmRequest(ctx, request)
	require.NoError(t, err)
	assert.Len(t, result.Messages, 1)
	assert.Equal(t, "user", result.Messages[0].Role)
}

func TestInjectPrompts_WithMatchingPrompts(t *testing.T) {
	ctx := contexts.WithProjectID(context.Background(), 1)

	prompts := []*ent.Prompt{
		{
			ID:      1,
			Role:    "system",
			Content: "You are a helpful assistant.",
			Settings: objects.PromptSettings{
				Action:     objects.PromptAction{Type: objects.PromptActionTypePrepend},
				Conditions: nil,
			},
		},
	}

	state := &PersistenceState{
		PromptProvider: &stubPromptProvider{prompts: prompts},
	}
	inbound := &PersistentInboundTransformer{state: state}

	middleware := injectPrompts(inbound)

	userContent := "Hello"
	request := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &userContent}},
		},
	}

	result, err := middleware.OnInboundLlmRequest(ctx, request)
	require.NoError(t, err)
	require.Len(t, result.Messages, 2)
	assert.Equal(t, "system", result.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", *result.Messages[0].Content.Content)
	assert.Equal(t, "user", result.Messages[1].Role)
}

func TestInjectPrompts_WithModelCondition(t *testing.T) {
	ctx := contexts.WithProjectID(context.Background(), 1)

	prompts := []*ent.Prompt{
		{
			ID:      1,
			Role:    "system",
			Content: "GPT-4 specific prompt",
			Settings: objects.PromptSettings{
				Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
				Conditions: []objects.PromptActivationConditionComposite{
					{
						Conditions: []objects.PromptActivationCondition{
							{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-4")},
						},
					},
				},
			},
		},
		{
			ID:      2,
			Role:    "system",
			Content: "Claude specific prompt",
			Settings: objects.PromptSettings{
				Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
				Conditions: []objects.PromptActivationConditionComposite{
					{
						Conditions: []objects.PromptActivationCondition{
							{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("claude-.*")},
						},
					},
				},
			},
		},
	}

	state := &PersistenceState{
		PromptProvider: &stubPromptProvider{prompts: prompts},
	}
	inbound := &PersistentInboundTransformer{state: state}

	middleware := injectPrompts(inbound)

	t.Run("gpt-4 request gets gpt-4 prompt", func(t *testing.T) {
		userContent := "Hello"
		request := &llm.Request{
			Model: "gpt-4",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: &userContent}},
			},
		}

		result, err := middleware.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2)
		assert.Equal(t, "GPT-4 specific prompt", *result.Messages[0].Content.Content)
	})

	t.Run("claude request gets claude prompt", func(t *testing.T) {
		userContent := "Hello"
		request := &llm.Request{
			Model: "claude-3-opus",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: &userContent}},
			},
		}

		result, err := middleware.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Messages, 2)
		assert.Equal(t, "Claude specific prompt", *result.Messages[0].Content.Content)
	})

	t.Run("unknown model gets no prompt", func(t *testing.T) {
		userContent := "Hello"
		request := &llm.Request{
			Model: "unknown-model",
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: &userContent}},
			},
		}

		result, err := middleware.OnInboundLlmRequest(ctx, request)
		require.NoError(t, err)
		require.Len(t, result.Messages, 1)
	})
}

func TestInjectPrompts_PrependAndAppend(t *testing.T) {
	ctx := contexts.WithProjectID(context.Background(), 1)

	prompts := []*ent.Prompt{
		{
			ID:      1,
			Role:    "system",
			Content: "Prepend prompt",
			Settings: objects.PromptSettings{
				Action:     objects.PromptAction{Type: objects.PromptActionTypePrepend},
				Conditions: nil,
			},
		},
		{
			ID:      2,
			Role:    "system",
			Content: "Append prompt",
			Settings: objects.PromptSettings{
				Action:     objects.PromptAction{Type: objects.PromptActionTypeAppend},
				Conditions: nil,
			},
		},
	}

	state := &PersistenceState{
		PromptProvider: &stubPromptProvider{prompts: prompts},
	}
	inbound := &PersistentInboundTransformer{state: state}

	middleware := injectPrompts(inbound)

	userContent := "Hello"
	request := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &userContent}},
		},
	}

	result, err := middleware.OnInboundLlmRequest(ctx, request)
	require.NoError(t, err)
	require.Len(t, result.Messages, 3)
	assert.Equal(t, "Prepend prompt", *result.Messages[0].Content.Content)
	assert.Equal(t, "Hello", *result.Messages[1].Content.Content)
	assert.Equal(t, "Append prompt", *result.Messages[2].Content.Content)
}
