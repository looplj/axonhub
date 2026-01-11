package biz

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestPromptMatcher_MatchConditions(t *testing.T) {
	matcher := NewPromptMatcher()

	tests := []struct {
		name       string
		conditions []objects.PromptActivationConditionComposite
		model      string
		expected   bool
	}{
		{
			name:       "empty conditions should always match",
			conditions: nil,
			model:      "gpt-4",
			expected:   true,
		},
		{
			name:       "empty composite list should match",
			conditions: []objects.PromptActivationConditionComposite{},
			model:      "gpt-4",
			expected:   true,
		},
		{
			name: "model_id exact match",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-4")},
					},
				},
			},
			model:    "gpt-4",
			expected: true,
		},
		{
			name: "model_id mismatch",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-4")},
					},
				},
			},
			model:    "gpt-3.5-turbo",
			expected: false,
		},
		{
			name: "model_pattern match",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("gpt-4.*")},
					},
				},
			},
			model:    "gpt-4-turbo",
			expected: true,
		},
		{
			name: "model_pattern mismatch",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("gpt-4.*")},
					},
				},
			},
			model:    "claude-3-opus",
			expected: false,
		},
		{
			name: "composite OR - one matches",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-4")},
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-3.5-turbo")},
					},
				},
			},
			model:    "gpt-3.5-turbo",
			expected: true,
		},
		{
			name: "composite OR - none matches",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-4")},
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("gpt-3.5-turbo")},
					},
				},
			},
			model:    "claude-3-opus",
			expected: false,
		},
		{
			name: "multiple composites AND - all match",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("gpt-.*")},
					},
				},
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr(".*-4")},
					},
				},
			},
			model:    "gpt-4",
			expected: true,
		},
		{
			name: "multiple composites AND - one fails",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("gpt-.*")},
					},
				},
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr(".*-turbo")},
					},
				},
			},
			model:    "gpt-4",
			expected: false,
		},
		{
			name: "nil model_id should not match",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelID, ModelID: nil},
					},
				},
			},
			model:    "gpt-4",
			expected: false,
		},
		{
			name: "empty model_pattern should not match",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("")},
					},
				},
			},
			model:    "gpt-4",
			expected: false,
		},
		{
			name: "unknown condition type should not match",
			conditions: []objects.PromptActivationConditionComposite{
				{
					Conditions: []objects.PromptActivationCondition{
						{Type: "unknown_type"},
					},
				},
			},
			model:    "gpt-4",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.MatchConditions(tt.conditions, tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptMatcher_MatchPrompt(t *testing.T) {
	matcher := NewPromptMatcher()

	tests := []struct {
		name     string
		prompt   *ent.Prompt
		model    string
		expected bool
	}{
		{
			name:     "nil prompt should not match",
			prompt:   nil,
			model:    "gpt-4",
			expected: false,
		},
		{
			name: "prompt with no conditions should match",
			prompt: &ent.Prompt{
				ID:      1,
				Role:    "system",
				Content: "You are a helpful assistant.",
				Settings: objects.PromptSettings{
					Action:     objects.PromptAction{Type: objects.PromptActionTypePrepend},
					Conditions: nil,
				},
			},
			model:    "gpt-4",
			expected: true,
		},
		{
			name: "prompt with matching condition",
			prompt: &ent.Prompt{
				ID:      2,
				Role:    "system",
				Content: "You are a coding assistant.",
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
			model:    "gpt-4",
			expected: true,
		},
		{
			name: "prompt with non-matching condition",
			prompt: &ent.Prompt{
				ID:      3,
				Role:    "system",
				Content: "You are a coding assistant.",
				Settings: objects.PromptSettings{
					Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
					Conditions: []objects.PromptActivationConditionComposite{
						{
							Conditions: []objects.PromptActivationCondition{
								{Type: objects.PromptActivationConditionTypeModelID, ModelID: lo.ToPtr("claude-3-opus")},
							},
						},
					},
				},
			},
			model:    "gpt-4",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.MatchPrompt(tt.prompt, tt.model)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptMatcher_FilterMatchingPrompts(t *testing.T) {
	matcher := NewPromptMatcher()

	prompts := []*ent.Prompt{
		{
			ID:      1,
			Role:    "system",
			Content: "Prompt 1 - no conditions",
			Settings: objects.PromptSettings{
				Action:     objects.PromptAction{Type: objects.PromptActionTypePrepend},
				Conditions: nil,
			},
		},
		{
			ID:      2,
			Role:    "system",
			Content: "Prompt 2 - GPT only",
			Settings: objects.PromptSettings{
				Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
				Conditions: []objects.PromptActivationConditionComposite{
					{
						Conditions: []objects.PromptActivationCondition{
							{Type: objects.PromptActivationConditionTypeModelPattern, ModelPattern: lo.ToPtr("gpt-.*")},
						},
					},
				},
			},
		},
		{
			ID:      3,
			Role:    "system",
			Content: "Prompt 3 - Claude only",
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

	t.Run("filter for gpt-4", func(t *testing.T) {
		result := matcher.FilterMatchingPrompts(prompts, "gpt-4")
		require.Len(t, result, 2)
		assert.Equal(t, 1, result[0].ID)
		assert.Equal(t, 2, result[1].ID)
	})

	t.Run("filter for claude-3-opus", func(t *testing.T) {
		result := matcher.FilterMatchingPrompts(prompts, "claude-3-opus")
		require.Len(t, result, 2)
		assert.Equal(t, 1, result[0].ID)
		assert.Equal(t, 3, result[1].ID)
	})

	t.Run("filter for unknown model", func(t *testing.T) {
		result := matcher.FilterMatchingPrompts(prompts, "unknown-model")
		require.Len(t, result, 1)
		assert.Equal(t, 1, result[0].ID)
	})
}

func TestPromptMatcher_ApplyPrompts(t *testing.T) {
	matcher := NewPromptMatcher()

	userContent := "Hello, how are you?"
	request := &llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &userContent}},
		},
	}

	tests := []struct {
		name          string
		prompts       []*ent.Prompt
		expectedLen   int
		expectedOrder []string // roles in expected order
		expectedFirst string   // first message content
		expectedLast  string   // last message content
	}{
		{
			name:          "no prompts",
			prompts:       []*ent.Prompt{},
			expectedLen:   1,
			expectedOrder: []string{"user"},
		},
		{
			name: "prepend single prompt",
			prompts: []*ent.Prompt{
				{
					Role:    "system",
					Content: "You are a helpful assistant.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
					},
				},
			},
			expectedLen:   2,
			expectedOrder: []string{"system", "user"},
			expectedFirst: "You are a helpful assistant.",
		},
		{
			name: "append single prompt",
			prompts: []*ent.Prompt{
				{
					Role:    "system",
					Content: "Remember to be concise.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: objects.PromptActionTypeAppend},
					},
				},
			},
			expectedLen:   2,
			expectedOrder: []string{"user", "system"},
			expectedLast:  "Remember to be concise.",
		},
		{
			name: "prepend and append",
			prompts: []*ent.Prompt{
				{
					Role:    "system",
					Content: "You are a helpful assistant.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
					},
				},
				{
					Role:    "system",
					Content: "Be concise.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: objects.PromptActionTypeAppend},
					},
				},
			},
			expectedLen:   3,
			expectedOrder: []string{"system", "user", "system"},
			expectedFirst: "You are a helpful assistant.",
			expectedLast:  "Be concise.",
		},
		{
			name: "multiple prepends maintain order",
			prompts: []*ent.Prompt{
				{
					Role:    "system",
					Content: "First prepend.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
					},
				},
				{
					Role:    "system",
					Content: "Second prepend.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: objects.PromptActionTypePrepend},
					},
				},
			},
			expectedLen:   3,
			expectedOrder: []string{"system", "system", "user"},
			expectedFirst: "First prepend.",
		},
		{
			name: "default action is prepend",
			prompts: []*ent.Prompt{
				{
					Role:    "system",
					Content: "Default action prompt.",
					Settings: objects.PromptSettings{
						Action: objects.PromptAction{Type: ""},
					},
				},
			},
			expectedLen:   2,
			expectedOrder: []string{"system", "user"},
			expectedFirst: "Default action prompt.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqCopy := &llm.Request{
				Model:    request.Model,
				Messages: make([]llm.Message, len(request.Messages)),
			}
			copy(reqCopy.Messages, request.Messages)

			result := matcher.ApplyPrompts(reqCopy, tt.prompts)
			require.Len(t, result.Messages, tt.expectedLen)

			for i, role := range tt.expectedOrder {
				assert.Equal(t, role, result.Messages[i].Role, "message %d role mismatch", i)
			}

			if tt.expectedFirst != "" {
				assert.Equal(t, tt.expectedFirst, *result.Messages[0].Content.Content)
			}

			if tt.expectedLast != "" {
				assert.Equal(t, tt.expectedLast, *result.Messages[len(result.Messages)-1].Content.Content)
			}
		})
	}
}
