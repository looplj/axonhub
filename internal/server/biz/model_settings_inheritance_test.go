package biz

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
)

func TestEffectiveModelAssociations_InheritsDeveloperSettings(t *testing.T) {
	modelAssociation := &objects.ModelAssociation{
		Type:     "model",
		Priority: 1,
		ModelID:  &objects.ModelIDAssociation{ModelID: "model-specific"},
	}
	developerAssociationSamePriority := &objects.ModelAssociation{
		Type:         "channel_model",
		Priority:     1,
		ChannelModel: &objects.ChannelModelAssociation{ChannelID: 10},
	}
	developerAssociationHigherPriority := &objects.ModelAssociation{
		Type:     "channel_tags_model",
		Priority: 0,
		ChannelTagsModel: &objects.ChannelTagsModelAssociation{
			ChannelTags: []string{"anthropic"},
		},
	}

	result := EffectiveModelAssociations(&SystemModelSettings{
		DeveloperSettings: []*DeveloperModelSettings{
			{
				Developer: "openai",
				Associations: []*objects.ModelAssociation{
					developerAssociationSamePriority,
					developerAssociationHigherPriority,
				},
			},
		},
	}, &ent.Model{
		Developer: "openai",
		ModelID:   "claude-opus-4-6",
		Settings: &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{modelAssociation},
		},
	})

	require.Len(t, result, 3)
	require.Equal(t, "channel_tags_model", result[0].Type)
	require.Equal(t, "claude-opus-4-6", result[0].ChannelTagsModel.ModelID)
	require.Same(t, modelAssociation, result[1])
	require.Equal(t, "channel_model", result[2].Type)
	require.Equal(t, "claude-opus-4-6", result[2].ChannelModel.ModelID)
	require.Empty(t, developerAssociationSamePriority.ChannelModel.ModelID)
	require.Empty(t, developerAssociationHigherPriority.ChannelTagsModel.ModelID)
}

func TestEffectiveModelAssociations_DisablesDeveloperSettingsInheritance(t *testing.T) {
	modelAssociation := &objects.ModelAssociation{
		Type:     "model",
		Priority: 1,
		ModelID:  &objects.ModelIDAssociation{ModelID: "model-specific"},
	}
	developerAssociation := &objects.ModelAssociation{
		Type:         "channel_model",
		Priority:     0,
		ChannelModel: &objects.ChannelModelAssociation{ChannelID: 10},
	}

	result := EffectiveModelAssociations(&SystemModelSettings{
		DeveloperSettings: []*DeveloperModelSettings{
			{
				Developer: "openai",
				Associations: []*objects.ModelAssociation{
					developerAssociation,
				},
			},
		},
	}, &ent.Model{
		Developer: "openai",
		ModelID:   "gpt-4",
		Settings: &objects.ModelSettings{
			DisableDeveloperSettingsInheritance: true,
			Associations:                        []*objects.ModelAssociation{modelAssociation},
		},
	})

	require.Equal(t, []*objects.ModelAssociation{modelAssociation}, result)
	require.Empty(t, developerAssociation.ChannelModel.ModelID)
}

func TestEffectiveModelAssociations_LegacyModelSettingsInheritByDefault(t *testing.T) {
	var legacySettings objects.ModelSettings
	err := json.Unmarshal([]byte(`{"associations":[]}`), &legacySettings)
	require.NoError(t, err)
	require.False(t, legacySettings.DisableDeveloperSettingsInheritance)

	result := EffectiveModelAssociations(&SystemModelSettings{
		DeveloperSettings: []*DeveloperModelSettings{
			{
				Developer: "openai",
				Associations: []*objects.ModelAssociation{
					{
						Type:         "channel_model",
						ChannelModel: &objects.ChannelModelAssociation{ChannelID: 10},
					},
				},
			},
		},
	}, &ent.Model{
		Developer: "openai",
		ModelID:   "gpt-4",
		Settings:  &legacySettings,
	})

	require.Len(t, result, 1)
	require.Equal(t, "gpt-4", result[0].ChannelModel.ModelID)
}

func TestValidateSystemModelSettings_RejectsDuplicateDevelopers(t *testing.T) {
	err := validateSystemModelSettings(&SystemModelSettings{
		DeveloperSettings: []*DeveloperModelSettings{
			{Developer: "openai"},
			{Developer: "openai"},
		},
	})
	require.ErrorContains(t, err, "duplicate model developer")
}

func TestValidateSystemModelSettings_RejectsDeveloperModelSelection(t *testing.T) {
	err := validateSystemModelSettings(&SystemModelSettings{
		DeveloperSettings: []*DeveloperModelSettings{
			{
				Developer: "anthropic",
				Associations: []*objects.ModelAssociation{
					{
						Type:    "model",
						ModelID: &objects.ModelIDAssociation{ModelID: "claude-opus-4-6"},
					},
				},
			},
		},
	})
	require.ErrorContains(t, err, "developer association type")
}
