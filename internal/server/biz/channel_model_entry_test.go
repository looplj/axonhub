package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannel_GetUnifiedModels(t *testing.T) {
	tests := []struct {
		name     string
		channel  *Channel
		expected []ChannelModelEntry
	}{
		{
			name: "direct models only",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"},
				{RequestModel: "gpt-3.5-turbo", ActualModel: "gpt-3.5-turbo", Source: "direct"},
			},
		},
		{
			name: "with extra model prefix",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "deepseek-chat", ActualModel: "deepseek-chat", Source: "direct"},
				{RequestModel: "deepseek-reasoner", ActualModel: "deepseek-reasoner", Source: "direct"},
				{RequestModel: "deepseek/deepseek-chat", ActualModel: "deepseek-chat", Source: "prefix"},
				{RequestModel: "deepseek/deepseek-reasoner", ActualModel: "deepseek-reasoner", Source: "prefix"},
			},
		},
		{
			name: "with auto-trimmed prefixes",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"openai/gpt-4", "deepseek-ai/deepseek-chat"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"openai", "deepseek-ai"},
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "openai/gpt-4", ActualModel: "openai/gpt-4", Source: "direct"},
				{RequestModel: "deepseek-ai/deepseek-chat", ActualModel: "deepseek-ai/deepseek-chat", Source: "direct"},
				{RequestModel: "gpt-4", ActualModel: "openai/gpt-4", Source: "auto_trim"},
				{RequestModel: "deepseek-chat", ActualModel: "deepseek-ai/deepseek-chat", Source: "auto_trim"},
			},
		},
		{
			name: "with model mappings",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "gpt-4-turbo"},
							{From: "gpt4", To: "gpt-4-turbo"},
						},
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4-turbo", ActualModel: "gpt-4-turbo", Source: "direct"},
				{RequestModel: "gpt-4", ActualModel: "gpt-4-turbo", Source: "mapping"},
				{RequestModel: "gpt4", ActualModel: "gpt-4-turbo", Source: "mapping"},
			},
		},
		{
			name: "combined: all features",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"openai/gpt-4", "deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix:        "custom",
						AutoTrimedModelPrefixes: []string{"openai"},
						ModelMappings: []objects.ModelMapping{
							{From: "gpt4", To: "openai/gpt-4"},
						},
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "openai/gpt-4", ActualModel: "openai/gpt-4", Source: "direct"},
				{RequestModel: "deepseek-chat", ActualModel: "deepseek-chat", Source: "direct"},
				{RequestModel: "custom/openai/gpt-4", ActualModel: "openai/gpt-4", Source: "prefix"},
				{RequestModel: "custom/deepseek-chat", ActualModel: "deepseek-chat", Source: "prefix"},
				{RequestModel: "gpt-4", ActualModel: "openai/gpt-4", Source: "auto_trim"},
				{RequestModel: "gpt4", ActualModel: "openai/gpt-4", Source: "mapping"},
			},
		},
		{
			name: "no duplicates",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "gpt-4"},
						},
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"},
			},
		},
		{
			name: "nil settings",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4"},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "gpt-4", Source: "direct"},
			},
		},
		{
			name: "hideOriginalModels: with model mappings only",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "gpt-4-turbo"},
							{From: "gpt4", To: "gpt-4-turbo"},
						},
						HideOriginalModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "gpt-4-turbo", Source: "mapping"},
				{RequestModel: "gpt4", ActualModel: "gpt-4-turbo", Source: "mapping"},
			},
		},
		{
			name: "hideOriginalModels: with extra prefix",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix:   "deepseek",
						HideOriginalModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "deepseek/deepseek-chat", ActualModel: "deepseek-chat", Source: "prefix"},
				{RequestModel: "deepseek/deepseek-reasoner", ActualModel: "deepseek-reasoner", Source: "prefix"},
			},
		},
		{
			name: "hideOriginalModels: with auto-trimmed prefixes",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"openai/gpt-4", "deepseek-ai/deepseek-chat"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"openai", "deepseek-ai"},
						HideOriginalModels:      true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "openai/gpt-4", Source: "auto_trim"},
				{RequestModel: "deepseek-chat", ActualModel: "deepseek-ai/deepseek-chat", Source: "auto_trim"},
			},
		},
		{
			name: "hideOriginalModels: combined features",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"openai/gpt-4", "deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix:        "custom",
						AutoTrimedModelPrefixes: []string{"openai"},
						ModelMappings: []objects.ModelMapping{
							{From: "gpt4", To: "openai/gpt-4"},
						},
						HideOriginalModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "custom/openai/gpt-4", ActualModel: "openai/gpt-4", Source: "prefix"},
				{RequestModel: "custom/deepseek-chat", ActualModel: "deepseek-chat", Source: "prefix"},
				{RequestModel: "gpt-4", ActualModel: "openai/gpt-4", Source: "auto_trim"},
				{RequestModel: "gpt4", ActualModel: "openai/gpt-4", Source: "mapping"},
			},
		},
		{
			name: "hideOriginalModels: false keeps direct models",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "gpt-4-turbo"},
						},
						HideOriginalModels: false,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4-turbo", ActualModel: "gpt-4-turbo", Source: "direct"},
				{RequestModel: "gpt-4", ActualModel: "gpt-4-turbo", Source: "mapping"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.channel.GetModelEntries()
			// Convert map to slice for comparison
			resultSlice := make([]ChannelModelEntry, 0, len(result))
			for _, entry := range result {
				resultSlice = append(resultSlice, entry)
			}

			require.ElementsMatch(t, tt.expected, resultSlice)
		})
	}
}

func TestChannel_GetUnifiedModels_CachesResult(t *testing.T) {
	ch := &Channel{
		Channel: &ent.Channel{
			SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
		},
	}

	// First call computes the result
	result1 := ch.GetModelEntries()
	require.Len(t, result1, 2)
	require.NotNil(t, ch.cachedModelEntries)

	// Second call should return the same cached map
	result2 := ch.GetModelEntries()
	require.Equal(t, result1, result2)
}
