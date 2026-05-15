package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestDefaultSelector_Select_InheritsDeveloperAssociations(t *testing.T) {
	ctx, client := setupTest(t)
	_, err := client.Channel.Create().
		SetType(channel.TypeAnthropic).
		SetName("Anthropic Primary").
		SetBaseURL("https://api.anthropic.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key-anthropic"}).
		SetSupportedModels([]string{"claude-opus-4-6", "claude-sonnet-4-6"}).
		SetDefaultTestModel("claude-sonnet-4-6").
		SetTags([]string{"anthropic"}).
		SetOrderingWeight(100).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)

	err = systemService.SetModelSettings(ctx, biz.SystemModelSettings{
		FallbackToChannelsOnModelNotFound: true,
		QueryAllChannelModels:             true,
		DeveloperSettings: []*biz.DeveloperModelSettings{
			{
				Developer: "anthropic",
				Associations: []*objects.ModelAssociation{
					{
						Type:     "channel_tags_model",
						Priority: 0,
						ChannelTagsModel: &objects.ChannelTagsModelAssociation{
							ChannelTags: []string{"anthropic"},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = client.Model.Create().
		SetDeveloper("anthropic").
		SetModelID("claude-opus-4-6").
		SetType(model.TypeChat).
		SetName("Claude Opus 4.6").
		SetIcon("anthropic").
		SetGroup("claude").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.Model.Create().
		SetDeveloper("anthropic").
		SetModelID("claude-sonnet-4-6").
		SetType(model.TypeChat).
		SetName("Claude Sonnet 4.6").
		SetIcon("anthropic").
		SetGroup("claude").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	selector := NewDefaultSelector(channelService, modelService, systemService)
	candidates, err := selector.Select(ctx, &llm.Request{Model: "claude-opus-4-6"})
	require.NoError(t, err)
	require.NotEmpty(t, candidates)
	require.Equal(t, "claude-opus-4-6", candidates[0].Models[0].ActualModel)

	candidates, err = selector.Select(ctx, &llm.Request{Model: "claude-sonnet-4-6"})
	require.NoError(t, err)
	require.NotEmpty(t, candidates)
	require.Equal(t, "claude-sonnet-4-6", candidates[0].Models[0].ActualModel)
}

func TestDefaultSelector_Select_InvalidatesCacheWhenDeveloperAssociationsChange(t *testing.T) {
	ctx, client := setupTest(t)
	channels := createTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)
	selector := NewDefaultSelector(channelService, modelService, systemService)

	err := systemService.SetModelSettings(ctx, biz.SystemModelSettings{
		QueryAllChannelModels: true,
		DeveloperSettings: []*biz.DeveloperModelSettings{
			{
				Developer: "openai",
				Associations: []*objects.ModelAssociation{
					{
						Type:     "channel_model",
						Priority: 0,
						ChannelModel: &objects.ChannelModelAssociation{
							ChannelID: channels[0].ID,
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = client.Model.Create().
		SetDeveloper("openai").
		SetModelID("gpt-4").
		SetType(model.TypeChat).
		SetName("Cached Developer Model").
		SetIcon("openai").
		SetGroup("gpt").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	_, err = selector.selectModelCandidates(ctx, &llm.Request{Model: "gpt-4"})
	require.NoError(t, err)

	selector.cacheMu.RLock()
	initialEntry := selector.associationCache["gpt-4"]
	selector.cacheMu.RUnlock()
	require.NotNil(t, initialEntry)

	err = systemService.SetModelSettings(ctx, biz.SystemModelSettings{
		QueryAllChannelModels: true,
		DeveloperSettings: []*biz.DeveloperModelSettings{
			{
				Developer: "openai",
				Associations: []*objects.ModelAssociation{
					{
						Type:     "channel_model",
						Priority: 0,
						ChannelModel: &objects.ChannelModelAssociation{
							ChannelID: channels[1].ID,
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = selector.selectModelCandidates(ctx, &llm.Request{Model: "gpt-4"})
	require.NoError(t, err)

	selector.cacheMu.RLock()
	currentEntry := selector.associationCache["gpt-4"]
	selector.cacheMu.RUnlock()
	require.NotSame(t, initialEntry, currentEntry)
}

func TestModelAssociationSignature_IncludesNestedCondition(t *testing.T) {
	associations := []*objects.ModelAssociation{
		{
			Type:     "channel_model",
			Priority: 1,
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
			ChannelModel: &objects.ChannelModelAssociation{
				ChannelID: 1,
				ModelID:   "gpt-4",
			},
		},
	}

	signature := modelAssociationSignature(associations)
	associations[0].When.Condition.Conditions[0].Value = 200

	require.NotEqual(t, signature, modelAssociationSignature(associations))
}
