package biz

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// newAssociationCleanupFixture builds a model service wired to an in-memory
// database, so the cascade can be exercised exactly as the channel service
// invokes it.
func newAssociationCleanupFixture(t *testing.T) (context.Context, *ent.Client, *ModelService, *SystemService) {
	t.Helper()

	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	systemSvc := &SystemService{
		AbstractService: &AbstractService{db: client},
		Cache:           xcache.NewFromConfig[ent.System](xcache.Config{Mode: xcache.ModeMemory}),
	}

	modelSvc := &ModelService{
		AbstractService: &AbstractService{db: client},
		channelService:  NewChannelServiceForTest(client),
		systemService:   systemSvc,
	}

	return ctx, client, modelSvc, systemSvc
}

func TestPurgeChannelFromModelAssociations_DropsDirectRules(t *testing.T) {
	ctx, client, svc, _ := newAssociationCleanupFixture(t)

	doomed, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Doomed").
		SetBaseURL("https://api.doomed.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-doomed"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	keeper, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Keeper").
		SetBaseURL("https://api.keeper.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-keeper"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	created, err := client.Model.Create().
		SetDeveloper("openai").
		SetModelID("gpt-4").
		SetGroup("gpt").
		SetType("chat").
		SetName("GPT-4").
		SetIcon("icon").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type:         "channel_model",
					Priority:     0,
					ChannelModel: &objects.ChannelModelAssociation{ChannelID: doomed.ID, ModelID: "gpt-4"},
				},
				{
					Type:         "channel_regex",
					Priority:     1,
					ChannelRegex: &objects.ChannelRegexAssociation{ChannelID: doomed.ID, Pattern: "gpt-.*"},
				},
				{
					Type:         "channel_model",
					Priority:     2,
					ChannelModel: &objects.ChannelModelAssociation{ChannelID: keeper.ID, ModelID: "gpt-4"},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.PurgeChannelFromModelAssociations(ctx, []int{doomed.ID}))

	stored, err := client.Model.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, stored.Settings.Associations, 1)
	require.Equal(t, "channel_model", stored.Settings.Associations[0].Type)
	require.Equal(t, keeper.ID, stored.Settings.Associations[0].ChannelModel.ChannelID)
}

func TestPurgeChannelFromModelAssociations_LeavesUnrelatedModelsUntouched(t *testing.T) {
	ctx, client, svc, _ := newAssociationCleanupFixture(t)

	doomed, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Doomed").
		SetBaseURL("https://api.doomed.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-doomed"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	other, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Other").
		SetBaseURL("https://api.other.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-other"}).
		SetSupportedModels([]string{"claude-3"}).
		SetDefaultTestModel("claude-3").
		Save(ctx)
	require.NoError(t, err)

	created, err := client.Model.Create().
		SetDeveloper("anthropic").
		SetModelID("claude-3").
		SetGroup("claude").
		SetType("chat").
		SetName("Claude 3").
		SetIcon("icon").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{
			Associations: []*objects.ModelAssociation{
				{
					Type:         "channel_model",
					Priority:     0,
					ChannelModel: &objects.ChannelModelAssociation{ChannelID: other.ID, ModelID: "claude-3"},
				},
				{
					Type:     "regex",
					Priority: 1,
					Regex:    &objects.RegexAssociation{Pattern: "claude-.*"},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.PurgeChannelFromModelAssociations(ctx, []int{doomed.ID}))

	stored, err := client.Model.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, stored.Settings.Associations, 2)
	require.Equal(t, other.ID, stored.Settings.Associations[0].ChannelModel.ChannelID)
	require.Equal(t, "regex", stored.Settings.Associations[1].Type)
}

func TestPurgeChannelFromModelAssociations_AlsoCleansDeveloperSettings(t *testing.T) {
	ctx, client, svc, systemSvc := newAssociationCleanupFixture(t)

	doomed, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Doomed").
		SetBaseURL("https://api.doomed.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-doomed"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		Save(ctx)
	require.NoError(t, err)

	require.NoError(t, systemSvc.SetModelSettings(ctx, SystemModelSettings{
		DeveloperSettings: []*DeveloperModelSettings{
			{
				Developer: "openai",
				Associations: []*objects.ModelAssociation{
					{
						Type:         "channel_model",
						Priority:     0,
						ChannelModel: &objects.ChannelModelAssociation{ChannelID: doomed.ID, ModelID: "gpt-4"},
					},
					{
						Type:             "channel_tags_model",
						Priority:         1,
						ChannelTagsModel: &objects.ChannelTagsModelAssociation{ChannelTags: []string{"公益"}, ModelID: "gpt-4"},
					},
				},
			},
		},
	}))

	require.NoError(t, svc.PurgeChannelFromModelAssociations(ctx, []int{doomed.ID}))

	settings, err := systemSvc.ModelSettings(ctx)
	require.NoError(t, err)
	require.Len(t, settings.DeveloperSettings, 1)
	require.Len(t, settings.DeveloperSettings[0].Associations, 1)
	require.Equal(t, "channel_tags_model", settings.DeveloperSettings[0].Associations[0].Type)
}

func TestPurgeChannelFromModelAssociations_NoopWithoutIDs(t *testing.T) {
	ctx, _, svc, _ := newAssociationCleanupFixture(t)

	require.NoError(t, svc.PurgeChannelFromModelAssociations(ctx, nil))
	require.NoError(t, svc.PurgeChannelFromModelAssociations(ctx, []int{}))
}
