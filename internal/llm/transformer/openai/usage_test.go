package openai

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/llm"
)

func TestUsage_ToLLMUsage(t *testing.T) {
	tests := []struct {
		name     string
		usage    *Usage
		expected *llm.Usage
	}{
		{
			name:     "nil usage returns nil",
			usage:    nil,
			expected: nil,
		},
		{
			name: "basic usage without cached tokens",
			usage: &Usage{
				Usage: llm.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
			},
			expected: &llm.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		},
		{
			name: "usage with cached tokens and no existing details",
			usage: &Usage{
				Usage: llm.Usage{
					PromptTokens:     15,
					CompletionTokens: 25,
					TotalTokens:      40,
				},
				CachedTokens: 5,
			},
			expected: &llm.Usage{
				PromptTokens:     15,
				CompletionTokens: 25,
				TotalTokens:      40,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 5,
				},
			},
		},
		{
			name: "usage with cached tokens and existing details - cached tokens not overwritten",
			usage: &Usage{
				Usage: llm.Usage{
					PromptTokens:     20,
					CompletionTokens: 30,
					TotalTokens:      50,
					PromptTokensDetails: &llm.PromptTokensDetails{
						CachedTokens: 2,
					},
				},
				CachedTokens: 8,
			},
			expected: &llm.Usage{
				PromptTokens:     20,
				CompletionTokens: 30,
				TotalTokens:      50,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 2,
				},
			},
		},
		{
			name: "usage with zero cached tokens",
			usage: &Usage{
				Usage: llm.Usage{
					PromptTokens:     12,
					CompletionTokens: 18,
					TotalTokens:      30,
				},
				CachedTokens: 0,
			},
			expected: &llm.Usage{
				PromptTokens:     12,
				CompletionTokens: 18,
				TotalTokens:      30,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.usage.ToLLMUsage()
			require.Equal(t, tt.expected, result)
		})
	}
}
