package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannel_IsModelSupported_WithExtraModelPrefix(t *testing.T) {
	tests := []struct {
		name      string
		channel   *Channel
		model     string
		supported bool
	}{
		{
			name: "model without prefix is supported",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			model:     "deepseek-chat",
			supported: true,
		},
		{
			name: "model with prefix is supported",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			model:     "deepseek/deepseek-chat",
			supported: true,
		},
		{
			name: "model with prefix but not in supported models",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			model:     "deepseek/gpt-4",
			supported: false,
		},
		{
			name: "model with wrong prefix",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			model:     "openai/deepseek-chat",
			supported: false,
		},
		{
			name: "no extra prefix configured",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"gpt-4"},
					Settings:        &objects.ChannelSettings{},
				},
			},
			model:     "openai/gpt-4",
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.channel.IsModelSupported(tt.model)
			require.Equal(t, tt.supported, result)
		})
	}
}

func TestChannel_ChooseModel_WithExtraModelPrefix(t *testing.T) {
	tests := []struct {
		name          string
		channel       *Channel
		inputModel    string
		expectedModel string
		expectError   bool
	}{
		{
			name: "model without prefix returns as-is",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			inputModel:    "deepseek-chat",
			expectedModel: "deepseek-chat",
			expectError:   false,
		},
		{
			name: "model with prefix strips prefix",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"deepseek-chat", "deepseek-reasoner"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			inputModel:    "deepseek/deepseek-chat",
			expectedModel: "deepseek-chat",
			expectError:   false,
		},
		{
			name: "model with prefix but not supported returns error",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			inputModel:  "deepseek/gpt-4",
			expectError: true,
		},
		{
			name: "unsupported model returns error",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			inputModel:  "gpt-4",
			expectError: true,
		},
		{
			name: "model with wrong prefix returns error",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"deepseek-chat"},
					Settings: &objects.ChannelSettings{
						ExtraModelPrefix: "deepseek",
					},
				},
			},
			inputModel:  "openai/deepseek-chat",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.channel.ChooseModel(tt.inputModel)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedModel, result)
			}
		})
	}
}

func TestChannel_GetModelEntries_HideMappedModels(t *testing.T) {
	tests := []struct {
		name             string
		channel          *Channel
		expectedModels   map[string]ChannelModelEntry
		shouldNotContain []string
	}{
		{
			name: "HideMappedModels=false shows both mapped and original models",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4-alias", To: "gpt-4"},
							{From: "gpt-3.5-alias", To: "gpt-3.5-turbo"},
						},
						HideMappedModels: false,
					},
				},
			},
			expectedModels: map[string]ChannelModelEntry{
				"gpt-4": {
					RequestModel: "gpt-4",
					ActualModel:  "gpt-4",
					Source:       "direct",
				},
				"gpt-3.5-turbo": {
					RequestModel: "gpt-3.5-turbo",
					ActualModel:  "gpt-3.5-turbo",
					Source:       "direct",
				},
				"gpt-4-alias": {
					RequestModel: "gpt-4-alias",
					ActualModel:  "gpt-4",
					Source:       "mapping",
				},
				"gpt-3.5-alias": {
					RequestModel: "gpt-3.5-alias",
					ActualModel:  "gpt-3.5-turbo",
					Source:       "mapping",
				},
			},
		},
		{
			name: "HideMappedModels=true hides original models but shows mapped models",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4-alias", To: "gpt-4"},
							{From: "gpt-3.5-alias", To: "gpt-3.5-turbo"},
						},
						HideMappedModels: true,
					},
				},
			},
			expectedModels: map[string]ChannelModelEntry{
				"gpt-4-alias": {
					RequestModel: "gpt-4-alias",
					ActualModel:  "gpt-4",
					Source:       "mapping",
				},
				"gpt-3.5-alias": {
					RequestModel: "gpt-3.5-alias",
					ActualModel:  "gpt-3.5-turbo",
					Source:       "mapping",
				},
			},
			shouldNotContain: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name: "HideMappedModels=true with partial mappings hides only mapped original models",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo", "claude-3"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4-alias", To: "gpt-4"},
						},
						HideMappedModels: true,
					},
				},
			},
			expectedModels: map[string]ChannelModelEntry{
				"gpt-3.5-turbo": {
					RequestModel: "gpt-3.5-turbo",
					ActualModel:  "gpt-3.5-turbo",
					Source:       "direct",
				},
				"claude-3": {
					RequestModel: "claude-3",
					ActualModel:  "claude-3",
					Source:       "direct",
				},
				"gpt-4-alias": {
					RequestModel: "gpt-4-alias",
					ActualModel:  "gpt-4",
					Source:       "mapping",
				},
			},
			shouldNotContain: []string{"gpt-4"},
		},
		{
			name: "HideMappedModels=true with mapping to unsupported model does not hide original",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4-alias", To: "gpt-4"},
							{From: "invalid-alias", To: "claude-3"},
						},
						HideMappedModels: true,
					},
				},
			},
			expectedModels: map[string]ChannelModelEntry{
				"gpt-4-alias": {
					RequestModel: "gpt-4-alias",
					ActualModel:  "gpt-4",
					Source:       "mapping",
				},
			},
			shouldNotContain: []string{"gpt-4", "invalid-alias", "claude-3"},
		},
		{
			name: "HideMappedModels=true with no mappings shows all original models",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings:    []objects.ModelMapping{},
						HideMappedModels: true,
					},
				},
			},
			expectedModels: map[string]ChannelModelEntry{
				"gpt-4": {
					RequestModel: "gpt-4",
					ActualModel:  "gpt-4",
					Source:       "direct",
				},
				"gpt-3.5-turbo": {
					RequestModel: "gpt-3.5-turbo",
					ActualModel:  "gpt-3.5-turbo",
					Source:       "direct",
				},
			},
		},
		{
			name: "HideMappedModels=true with HideOriginalModels=true shows only mapped models",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "Test Channel",
					SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					Settings: &objects.ChannelSettings{
						ModelMappings: []objects.ModelMapping{
							{From: "gpt-4-alias", To: "gpt-4"},
							{From: "gpt-3.5-alias", To: "gpt-3.5-turbo"},
						},
						HideMappedModels:   true,
						HideOriginalModels: true,
					},
				},
			},
			expectedModels: map[string]ChannelModelEntry{
				"gpt-4-alias": {
					RequestModel: "gpt-4-alias",
					ActualModel:  "gpt-4",
					Source:       "mapping",
				},
				"gpt-3.5-alias": {
					RequestModel: "gpt-3.5-alias",
					ActualModel:  "gpt-3.5-turbo",
					Source:       "mapping",
				},
			},
			shouldNotContain: []string{"gpt-4", "gpt-3.5-turbo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.channel.GetModelEntries()

			for expectedModel, expectedEntry := range tt.expectedModels {
				entry, exists := result[expectedModel]
				require.True(t, exists, "model %s should exist in result", expectedModel)
				require.Equal(t, expectedEntry.RequestModel, entry.RequestModel)
				require.Equal(t, expectedEntry.ActualModel, entry.ActualModel)
				require.Equal(t, expectedEntry.Source, entry.Source)
			}

			for _, shouldNotContain := range tt.shouldNotContain {
				_, exists := result[shouldNotContain]
				require.False(t, exists, "model %s should not exist in result", shouldNotContain)
			}
		})
	}
}

func TestChannel_ChooseModel_AutoTrimedModelPrefixes(t *testing.T) {
	tests := []struct {
		name          string
		channel       *Channel
		inputModel    string
		expectedModel string
		expectError   bool
	}{
		{
			name: "request has prefix, channel supports trimmed",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "DeepSeek",
					SupportedModels: []string{"DeepSeek-V3.2"},
					Settings:        &objects.ChannelSettings{AutoTrimedModelPrefixes: []string{"deepseek-ai"}},
				},
			},
			inputModel:    "deepseek-ai/DeepSeek-V3.2",
			expectedModel: "",
			expectError:   true,
		},
		{
			name: "request trimmed, channel supports prefixed",
			channel: &Channel{
				Channel: &ent.Channel{
					Name:            "DeepSeek",
					SupportedModels: []string{"deepseek-ai/DeepSeek-V3.2"},
					Settings:        &objects.ChannelSettings{AutoTrimedModelPrefixes: []string{"deepseek-ai"}},
				},
			},
			inputModel:    "DeepSeek-V3.2",
			expectedModel: "deepseek-ai/DeepSeek-V3.2",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.channel.ChooseModel(tt.inputModel)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedModel, result)
			}
		})
	}
}
