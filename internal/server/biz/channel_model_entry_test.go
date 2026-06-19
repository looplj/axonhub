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
		{
			name: "hideMappedModels: with model mappings only",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "gpt-4-turbo"},
						},
						HideMappedModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "gpt-4-turbo", Source: "mapping"},
			},
		},
		{
			name: "hideMappedModels: with extra prefix hides prefixed target too",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deep-deepseek-v4-pro"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "AIHubMix",
						ModelMappings: []objects.ModelMapping{
							{From: "deepseek-v4-pro", To: "deep-deepseek-v4-pro"},
						},
						HideMappedModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "deepseek-v4-pro", ActualModel: "deep-deepseek-v4-pro", Source: "mapping"},
			},
		},
		{
			name: "hideMappedModels: with extra prefix and multiple models",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4-turbo", "claude-3"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "proxy",
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "gpt-4-turbo"},
						},
						HideMappedModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "claude-3", ActualModel: "claude-3", Source: "direct"},
				{RequestModel: "proxy/claude-3", ActualModel: "claude-3", Source: "prefix"},
				{RequestModel: "gpt-4", ActualModel: "gpt-4-turbo", Source: "mapping"},
			},
		},
		{
			name: "hideMappedModels: with auto-trim hides trimmed target too",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"openai/gpt-4-turbo"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"openai"},
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "openai/gpt-4-turbo"},
						},
						HideMappedModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "openai/gpt-4-turbo", Source: "mapping"},
			},
		},
		{
			name: "hideMappedModels: with prefix and auto-trim combined",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"openai/gpt-4-turbo", "claude-3"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix:        "proxy",
						AutoTrimedModelPrefixes: []string{"openai"},
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "openai/gpt-4-turbo"},
						},
						HideMappedModels: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "claude-3", ActualModel: "claude-3", Source: "direct"},
				{RequestModel: "proxy/claude-3", ActualModel: "claude-3", Source: "prefix"},
				{RequestModel: "gpt-4", ActualModel: "openai/gpt-4-turbo", Source: "mapping"},
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

func TestChannel_GetUnifiedModels_LowercaseModelID(t *testing.T) {
	tests := []struct {
		name     string
		channel  *Channel
		expected []ChannelModelEntry
	}{
		{
			name: "basic lowercase — ActualModel preserves original casing",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"Pro/zai-org/GLM-5.1"},
					Settings: &objects.ChannelSettings{
						LowercaseModelID: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "pro/zai-org/glm-5.1", ActualModel: "Pro/zai-org/GLM-5.1", Source: "direct"},
			},
		},
		{
			name: "with prefix — ActualModel preserves original casing",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"GLM-5.1"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "zai-org",
						LowercaseModelID: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "glm-5.1", ActualModel: "GLM-5.1", Source: "direct"},
				{RequestModel: "zai-org/glm-5.1", ActualModel: "GLM-5.1", Source: "prefix"},
			},
		},
		{
			name: "with model mappings — mapping To preserves original casing",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"GPT-4-Turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "GPT-4", To: "GPT-4-Turbo"},
						},
						LowercaseModelID: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4-turbo", ActualModel: "GPT-4-Turbo", Source: "direct"},
				{RequestModel: "gpt-4", ActualModel: "GPT-4-Turbo", Source: "mapping"},
			},
		},
		{
			name: "false preserves case",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"Pro/zai-org/GLM-5"},
					Settings: &objects.ChannelSettings{
						LowercaseModelID: false,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "Pro/zai-org/GLM-5", ActualModel: "Pro/zai-org/GLM-5", Source: "direct"},
			},
		},
		{
			name: "with auto-trim — ActualModel keeps original casing",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"OpenAI/GPT-4"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"OpenAI"},
						LowercaseModelID:        true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "openai/gpt-4", ActualModel: "OpenAI/GPT-4", Source: "direct"},
				{RequestModel: "gpt-4", ActualModel: "OpenAI/GPT-4", Source: "auto_trim"},
			},
		},
		{
			name: "with auto-trim and prefix — combined features",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"Pro/zai-org/GLM-5.1"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"Pro", "Pro/zai-org"},
						LowercaseModelID:        true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "pro/zai-org/glm-5.1", ActualModel: "Pro/zai-org/GLM-5.1", Source: "direct"},
				{RequestModel: "zai-org/glm-5.1", ActualModel: "Pro/zai-org/GLM-5.1", Source: "auto_trim"},
				{RequestModel: "glm-5.1", ActualModel: "Pro/zai-org/GLM-5.1", Source: "auto_trim"},
			},
		},
		{
			name: "collision: direct beats mapping",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"GPT-4"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4", To: "GPT-4"},
						},
						LowercaseModelID: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "gpt-4", ActualModel: "GPT-4", Source: "direct"},
			},
		},
		{
			name: "collision: mapping beats prefix",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"GLM-5.1"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "ZAI",
						ModelMappings: []objects.ModelMapping{
							{From: "zai/GLM-5.1", To: "GLM-5.1"},
						},
						LowercaseModelID: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "glm-5.1", ActualModel: "GLM-5.1", Source: "direct"},
				{RequestModel: "zai/glm-5.1", ActualModel: "GLM-5.1", Source: "mapping"},
			},
		},
		{
			name: "collision: auto_trim beats mapping",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"ZAI/GLM-5.1"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"ZAI"},
						ModelMappings: []objects.ModelMapping{
							{From: "glm-5.1", To: "ZAI/GLM-5.1"},
						},
						LowercaseModelID: true,
					},
				},
			},
			expected: []ChannelModelEntry{
				{RequestModel: "zai/glm-5.1", ActualModel: "ZAI/GLM-5.1", Source: "direct"},
				{RequestModel: "glm-5.1", ActualModel: "ZAI/GLM-5.1", Source: "auto_trim"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.channel.GetModelEntries()
			resultSlice := make([]ChannelModelEntry, 0, len(result))
			for _, entry := range result {
				resultSlice = append(resultSlice, entry)
			}

			require.ElementsMatch(t, tt.expected, resultSlice)
		})
	}
}
