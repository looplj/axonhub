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

func TestChannel_ChooseModel_RemoveModelPrefixes_Symmetric(t *testing.T) {
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

func TestChannel_resolveAutoTrimedModel(t *testing.T) {
	tests := []struct {
		name     string
		channel  *Channel
		model    string
		expected string
		found    bool
	}{
		{
			name: "no prefixes configured",
			channel: &Channel{
				Channel: &ent.Channel{
					Settings: &objects.ChannelSettings{},
				},
			},
			model:    "openai/gpt-4",
			expected: "",
			found:    false,
		},
		{
			name: "settings is nil",
			channel: &Channel{
				Channel: &ent.Channel{
					Settings: nil,
				},
			},
			model:    "openai/gpt-4",
			expected: "",
			found:    false,
		},
		{
			name: "model without slash - no removal",
			channel: &Channel{
				Channel: &ent.Channel{
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"openai"},
					},
				},
			},
			model:    "gpt-4",
			expected: "",
			found:    false,
		},
		{
			name: "prefix without slash in model - no removal",
			channel: &Channel{
				Channel: &ent.Channel{
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"gpt"},
					},
				},
			},
			model:    "gpt-4",
			expected: "",
			found:    false,
		},
		{
			name: "request without prefix but supported model has prefix",
			channel: &Channel{
				Channel: &ent.Channel{
					SupportedModels: []string{"deepseek-ai/DeepSeek-V3.2"},
					Settings: &objects.ChannelSettings{
						AutoTrimedModelPrefixes: []string{"deepseek-ai"},
					},
				},
			},
			model:    "DeepSeek-V3.2",
			expected: "deepseek-ai/DeepSeek-V3.2",
			found:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := tt.channel.resolveAutoTrimedModel(tt.model)
			require.Equal(t, tt.found, found)
			require.Equal(t, tt.expected, result)
		})
	}
}
