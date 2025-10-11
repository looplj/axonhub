package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelService_ListAllModels(t *testing.T) {
	tests := []struct {
		name     string
		channels []*Channel
		expected []string
	}{
		{
			name: "single channel with no mappings",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-3.5-turbo"},
		},
		{
			name: "single channel with model mappings",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"claude-3-opus-20240229"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "claude-3-opus", To: "claude-3-opus-20240229"},
								{From: "claude-opus", To: "claude-3-opus-20240229"},
							},
						},
					},
				},
			},
			expected: []string{"claude-3-opus-20240229", "claude-3-opus", "claude-opus"},
		},
		{
			name: "multiple channels with overlapping models",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4", "gpt-3.5-turbo"},
					},
				},
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4", "gpt-4-turbo"},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-3.5-turbo", "gpt-4-turbo"},
		},
		{
			name: "multiple channels with model mappings",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4-latest", To: "gpt-4"},
							},
						},
					},
				},
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"claude-3-opus-20240229"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "claude-3-opus", To: "claude-3-opus-20240229"},
							},
						},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-4-latest", "claude-3-opus-20240229", "claude-3-opus"},
		},
		{
			name: "mapping to unsupported model should be ignored",
			channels: []*Channel{
				{
					Channel: &ent.Channel{
						SupportedModels: []string{"gpt-4"},
						Settings: &objects.ChannelSettings{
							ModelMappings: []objects.ModelMapping{
								{From: "gpt-4-latest", To: "gpt-4"},
								{From: "invalid-mapping", To: "unsupported-model"},
							},
						},
					},
				},
			},
			expected: []string{"gpt-4", "gpt-4-latest"},
		},
		{
			name:     "empty channels",
			channels: []*Channel{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ChannelService{
				EnabledChannels: tt.channels,
			}

			result := svc.ListAllModels(context.Background())

			// Convert to map for easier comparison (order doesn't matter)
			resultMap := make(map[string]bool)
			for _, model := range result {
				resultMap[model.ID] = true
			}

			expectedMap := make(map[string]bool)
			for _, model := range tt.expected {
				expectedMap[model] = true
			}

			assert.Equal(t, expectedMap, resultMap, "Model lists should match")
			assert.Equal(t, len(tt.expected), len(result), "Should have same number of models")
		})
	}
}
