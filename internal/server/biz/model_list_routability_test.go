package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/llm"
)

func setupListEnabledModelsTest(t *testing.T) (context.Context, *ent.Client, *ModelService, *ChannelService, *SystemService) {
	t.Helper()

	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	channelSvc := NewChannelServiceForTest(client)
	systemSvc := &SystemService{
		AbstractService: &AbstractService{db: client},
		Cache:           xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}
	modelSvc := &ModelService{
		AbstractService: &AbstractService{db: client},
		channelService:  channelSvc,
		systemService:   systemSvc,
	}

	return ctx, client, modelSvc, channelSvc, systemSvc
}

func reloadEnabledChannels(t *testing.T, ctx context.Context, client *ent.Client, channelSvc *ChannelService) {
	t.Helper()

	enabledEntities, err := client.Channel.Query().
		Where(channel.StatusEQ(channel.StatusEnabled)).
		All(ctx)
	require.NoError(t, err)

	enabledChannels := make([]*Channel, 0, len(enabledEntities))
	for _, e := range enabledEntities {
		built, buildErr := channelSvc.buildChannelWithTransformer(e)
		require.NoError(t, buildErr)
		enabledChannels = append(enabledChannels, built)
	}

	channelSvc.SetEnabledChannelsForTest(enabledChannels)
}

func createConfiguredModel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	modelID string,
	modelType model.Type,
	assoc *objects.ModelAssociation,
) {
	t.Helper()

	_, err := client.Model.Create().
		SetDeveloper("openai").
		SetModelID(modelID).
		SetName(modelID).
		SetType(modelType).
		SetGroup("test").
		SetIcon("icon").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{assoc},
		}).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)
}

func listedModelIDs(t *testing.T, ctx context.Context, modelSvc *ModelService) map[string]bool {
	t.Helper()

	result, err := modelSvc.ListEnabledModels(ctx)
	require.NoError(t, err)

	ids := make(map[string]bool, len(result))
	for _, item := range result {
		ids[item.ID] = true
	}

	return ids
}

func TestListEnabledModels_HideUnroutableModelsInList(t *testing.T) {
	ctx, client, modelSvc, channelSvc, systemSvc := setupListEnabledModelsTest(t)

	chatChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Chat Channel").
		SetBaseURL("https://api.chat.example/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "chat-key"}).
		SetSupportedModels([]string{"routable-chat"}).
		SetDefaultTestModel("routable-chat").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	responsesChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenaiResponses).
		SetName("Responses Only Channel").
		SetBaseURL("https://api.responses.example/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "responses-key"}).
		SetSupportedModels([]string{"unroutable-embedding"}).
		SetDefaultTestModel("unroutable-embedding").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	reloadEnabledChannels(t, ctx, client, channelSvc)

	createConfiguredModel(t, ctx, client, "routable-chat", model.TypeChat, &objects.ModelAssociation{
		Type: "channel_model",
		ChannelModel: &objects.ChannelModelAssociation{
			ChannelID: chatChannel.ID,
			ModelID:   "routable-chat",
		},
	})
	createConfiguredModel(t, ctx, client, "unroutable-embedding", model.TypeEmbedding, &objects.ModelAssociation{
		Type: "channel_model",
		ChannelModel: &objects.ChannelModelAssociation{
			ChannelID: responsesChannel.ID,
			ModelID:   "unroutable-embedding",
		},
	})

	err = systemSvc.SetModelSettings(ctx, SystemModelSettings{
		QueryAllChannelModels:      false,
		HideUnroutableModelsInList: false,
	})
	require.NoError(t, err)

	listed := listedModelIDs(t, ctx, modelSvc)
	require.True(t, listed["routable-chat"], "chat model should remain visible when the switch is off")
	require.True(t, listed["unroutable-embedding"], "embedding model with an association should remain visible when the switch is off")

	err = systemSvc.SetModelSettings(ctx, SystemModelSettings{
		QueryAllChannelModels:      false,
		HideUnroutableModelsInList: true,
	})
	require.NoError(t, err)

	listed = listedModelIDs(t, ctx, modelSvc)
	require.True(t, listed["routable-chat"], "chat model with a capable channel should stay visible")
	require.False(t, listed["unroutable-embedding"], "embedding model on a responses-only channel should be hidden")

	err = systemSvc.SetModelSettings(ctx, SystemModelSettings{
		QueryAllChannelModels:      true,
		HideUnroutableModelsInList: true,
	})
	require.NoError(t, err)

	listed = listedModelIDs(t, ctx, modelSvc)
	require.True(t, listed["routable-chat"], "chat model should stay visible when query-all is enabled")
	require.False(t, listed["unroutable-embedding"], "hidden configured model must not reappear as a channel-derived facade")
}

func TestListEnabledModels_HideUnroutableRespectsExcludedProjectTags(t *testing.T) {
	ctx, client, modelSvc, channelSvc, systemSvc := setupListEnabledModelsTest(t)

	excludedChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Excluded Tagged Channel").
		SetBaseURL("https://api.excluded.example/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "excluded-key"}).
		SetSupportedModels([]string{"tagged-only-model"}).
		SetDefaultTestModel("tagged-only-model").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"AAA"}).
		Save(ctx)
	require.NoError(t, err)

	reloadEnabledChannels(t, ctx, client, channelSvc)

	createConfiguredModel(t, ctx, client, "tagged-only-model", model.TypeChat, &objects.ModelAssociation{
		Type: "channel_model",
		ChannelModel: &objects.ChannelModelAssociation{
			ChannelID: excludedChannel.ID,
			ModelID:   "tagged-only-model",
		},
	})

	err = systemSvc.SetModelSettings(ctx, SystemModelSettings{
		QueryAllChannelModels:      false,
		HideUnroutableModelsInList: true,
	})
	require.NoError(t, err)

	apiKey := &ent.APIKey{
		ID:   1,
		Name: "project-a-key",
		Edges: ent.APIKeyEdges{
			Project: &ent.Project{
				ID: 1,
				Profiles: &objects.ProjectProfiles{
					ActiveProfile: "default",
					Profiles: []objects.ProjectProfile{
						{
							Name:                 "default",
							ChannelTags:          []string{"AAA"},
							ChannelTagsMatchMode: objects.ChannelTagsMatchModeNone,
						},
					},
				},
			},
		},
	}

	listed := listedModelIDs(t, contexts.WithAPIKey(ctx, apiKey), modelSvc)
	require.False(t, listed["tagged-only-model"], "model only served by an excluded-tag channel must not appear")

	listed = listedModelIDs(t, ctx, modelSvc)
	require.True(t, listed["tagged-only-model"], "the same model remains visible without the project tag filter")
}

func TestListEnabledModels_HideUnroutableKeepsChannelDerivedModels(t *testing.T) {
	ctx, client, modelSvc, channelSvc, systemSvc := setupListEnabledModelsTest(t)

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Derived Channel").
		SetBaseURL("https://api.derived.example/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "derived-key"}).
		SetSupportedModels([]string{"channel-only-model"}).
		SetDefaultTestModel("channel-only-model").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	reloadEnabledChannels(t, ctx, client, channelSvc)

	err = systemSvc.SetModelSettings(ctx, SystemModelSettings{
		QueryAllChannelModels:      true,
		HideUnroutableModelsInList: true,
	})
	require.NoError(t, err)

	listed := listedModelIDs(t, ctx, modelSvc)
	require.True(t, listed["channel-only-model"], "channel-derived models remain listed when query-all is enabled")
}

func TestListEnabledModels_HideUnroutableDoesNotUseWhenConditions(t *testing.T) {
	ctx, client, modelSvc, channelSvc, systemSvc := setupListEnabledModelsTest(t)

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("When Channel").
		SetBaseURL("https://api.when.example/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "when-key"}).
		SetSupportedModels([]string{"when-model"}).
		SetDefaultTestModel("when-model").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	reloadEnabledChannels(t, ctx, client, channelSvc)

	createConfiguredModel(t, ctx, client, "when-model", model.TypeChat, &objects.ModelAssociation{
		Type: "channel_model",
		ChannelModel: &objects.ChannelModelAssociation{
			ChannelID: ch.ID,
			ModelID:   "when-model",
		},
		When: &objects.ModelAssociationWhen{
			Enabled: true,
			Condition: &objects.Condition{
				Type:     "condition",
				Field:    "request_format",
				Operator: "eq",
				Value:    llm.APIFormatOpenAIModeration.String(),
			},
		},
	})

	err = systemSvc.SetModelSettings(ctx, SystemModelSettings{
		QueryAllChannelModels:      false,
		HideUnroutableModelsInList: true,
	})
	require.NoError(t, err)

	listed := listedModelIDs(t, ctx, modelSvc)
	require.True(t, listed["when-model"], "When conditions must not hide models from the list")
}
