package anthropic

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/looplj/axonhub/internal/pkg/xtest"
	"github.com/looplj/axonhub/llm"
)

func Test_convertUsage(t *testing.T) {
	type args struct {
		usage        *Usage
		platformType PlatformType
	}

	tests := []struct {
		name string
		args args
		want *llm.Usage
	}{
		{
			name: "base case - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     30,
					ServiceTier:              "standard",
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     150, // 100 + 20 + 30
				CompletionTokens: 50,
				TotalTokens:      200, // 150 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens:      30,
					WriteCachedTokens: 20,
				},
			},
		},
		{
			name: "cache read tokens greater than input tokens - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     150,
					ServiceTier:              "standard",
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     270, // 100 + 20 + 150
				CompletionTokens: 50,
				TotalTokens:      320, // 270 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens:      150,
					WriteCachedTokens: 20,
				},
			},
		},
		{
			name: "nil usage",
			args: args{
				usage:        nil,
				platformType: PlatformDirect,
			},
			want: nil,
		},
		{
			name: "zero values - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:              0,
					OutputTokens:             0,
					CacheCreationInputTokens: 0,
					CacheReadInputTokens:     0,
					ServiceTier:              "",
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
		},
		{
			name: "moonshot cached tokens conversion - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CachedTokens:             75,
					CacheCreationInputTokens: 0,
					CacheReadInputTokens:     0,
					ServiceTier:              "standard",
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     175, // 100 + 75
				CompletionTokens: 50,
				TotalTokens:      225, // 175 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 75,
				},
			},
		},
		{
			name: "only input and output tokens - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:  100,
					OutputTokens: 50,
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
			},
		},
		{
			name: "only cache creation tokens - Anthropic official",
			args: args{
				usage: &Usage{
					CacheCreationInputTokens: 100,
					OutputTokens:             50,
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     100, // only cache creation tokens
				CompletionTokens: 50,
				TotalTokens:      150, // 100 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					WriteCachedTokens: 100,
				},
			},
		},
		{
			name: "only cache read tokens - Anthropic official",
			args: args{
				usage: &Usage{
					CacheReadInputTokens: 100,
					OutputTokens:         50,
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     100, // only cache read tokens
				CompletionTokens: 50,
				TotalTokens:      150, // 100 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 100,
				},
			},
		},
		{
			name: "large numbers - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:              1000000,
					OutputTokens:             500000,
					CacheCreationInputTokens: 200000,
					CacheReadInputTokens:     300000,
					ServiceTier:              "priority",
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     1500000, // 1000000 + 200000 + 300000
				CompletionTokens: 500000,
				TotalTokens:      2000000, // 1500000 + 500000
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens:      300000,
					WriteCachedTokens: 200000,
				},
			},
		},
		{
			name: "empty usage struct - Anthropic official",
			args: args{
				usage:        &Usage{},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
			},
		},
		{
			name: "moonshot with cache creation tokens - cached tokens ignored",
			args: args{
				usage: &Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CachedTokens:             75,
					CacheCreationInputTokens: 0,
					CacheReadInputTokens:     0,
					ServiceTier:              "standard",
				},
				platformType: PlatformMoonshot,
			},
			want: &llm.Usage{
				PromptTokens:     100, // 100 input tokens.
				CompletionTokens: 50,
				TotalTokens:      150, // 100 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens: 75, //
				},
			},
		},
		{
			name: "only output tokens - Anthropic official",
			args: args{
				usage: &Usage{
					OutputTokens: 50,
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     0,
				CompletionTokens: 50,
				TotalTokens:      50,
			},
		},
		{
			name: "Moonshot - cache read tokens included in input tokens",
			args: args{
				usage: &Usage{
					InputTokens:              100, // Already includes 30 cached tokens
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     30,
					ServiceTier:              "standard",
				},
				platformType: PlatformMoonshot,
			},
			want: &llm.Usage{
				PromptTokens:     100, // 100 input tokens.
				CompletionTokens: 50,
				TotalTokens:      150, // 100 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens:      30,
					WriteCachedTokens: 20,
				},
			},
		},
		{
			name: "cache creation with ttl - Anthropic official",
			args: args{
				usage: &Usage{
					InputTokens:              100,
					OutputTokens:             50,
					CacheCreationInputTokens: 20,
					CacheReadInputTokens:     30,
					CacheCreation: CacheCreation{
						Ephemeral5mInputTokens: 10,
						Ephemeral1hInputTokens: 10,
					},
					ServiceTier: "standard",
				},
				platformType: PlatformDirect,
			},
			want: &llm.Usage{
				PromptTokens:     150, // 100 + 20 + 30
				CompletionTokens: 50,
				TotalTokens:      200, // 150 + 50
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens:           30,
					WriteCachedTokens:      20,
					WriteCached5MinTokens:  10,
					WriteCached1HourTokens: 10,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToLlmUsage(tt.args.usage, tt.args.platformType)

			if !cmp.Equal(tt.want, got,
				xtest.NilPromptTokensDetails,
				xtest.NilCompletionTokensDetails,
			) {
				t.Fatalf("diff: %v", cmp.Diff(tt.want, got))
			}
			// require.Equal(t, tt.want, got)
		})
	}
}

func Test_convertToAnthropicUsage(t *testing.T) {
	tests := []struct {
		name     string
		llmUsage *llm.Usage
		want     *Usage
	}{
		{
			name: "base case",
			llmUsage: &llm.Usage{
				PromptTokens:     150,
				CompletionTokens: 50,
				PromptTokensDetails: &llm.PromptTokensDetails{
					CachedTokens:           30,
					WriteCachedTokens:      20,
					WriteCached5MinTokens:  10,
					WriteCached1HourTokens: 10,
				},
			},
			want: &Usage{
				InputTokens:              100, // 150 - 30 - 20
				OutputTokens:             50,
				CacheReadInputTokens:     30,
				CacheCreationInputTokens: 20,
				CacheCreation: CacheCreation{
					Ephemeral5mInputTokens: 10,
					Ephemeral1hInputTokens: 10,
				},
			},
		},
		{
			name: "no details",
			llmUsage: &llm.Usage{
				PromptTokens:     100,
				CompletionTokens: 50,
			},
			want: &Usage{
				InputTokens:  100,
				OutputTokens: 50,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToAnthropicUsage(tt.llmUsage)
			if !cmp.Equal(tt.want, got) {
				t.Fatalf("diff: %v", cmp.Diff(tt.want, got))
			}
		})
	}
}
