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
		ModelID:   "gpt-4o",
		Settings: &objects.ModelSettings{
			Associations: []*objects.ModelAssociation{modelAssociation},
		},
	})

	require.Len(t, result, 3)
	require.Equal(t, "channel_tags_model", result[0].Type)
	require.Equal(t, "gpt-4o", result[0].ChannelTagsModel.ModelID)
	require.Same(t, modelAssociation, result[1])
	require.Equal(t, "channel_model", result[2].Type)
	require.Equal(t, "gpt-4o", result[2].ChannelModel.ModelID)
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

func TestCloneModelAssociation_DeepCopiesWhenCondition(t *testing.T) {
	assoc := &objects.ModelAssociation{
		Type: "channel_model",
		When: &objects.ModelAssociationWhen{
			Enabled: true,
			Condition: &objects.Condition{
				Type:  objects.ConditionTypeGroup,
				Logic: "and",
				Conditions: []objects.Condition{
					{
						Type:     objects.ConditionTypeCondition,
						Field:    "prompt_tokens",
						Operator: "gt",
						Value:    100,
					},
				},
			},
		},
		ChannelModel: &objects.ChannelModelAssociation{ChannelID: 10},
	}

	clone := cloneModelAssociation(assoc)
	require.NotSame(t, assoc.When, clone.When)
	require.NotSame(t, assoc.When.Condition, clone.When.Condition)

	clone.When.Enabled = false
	clone.When.Condition.Conditions[0].Field = "stream"
	clone.When.Condition.Conditions[0].Value = true

	require.True(t, assoc.When.Enabled)
	require.Equal(t, "prompt_tokens", assoc.When.Condition.Conditions[0].Field)
	require.Equal(t, 100, assoc.When.Condition.Conditions[0].Value)
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
