package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/objects"
)

func setupBackupTest(t *testing.T) (*ent.Client, *BackupService, context.Context) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	executor := executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1))

	t.Cleanup(func() {
		_ = executor.Shutdown(context.Background())
	})

	channelService := NewChannelService(ChannelServiceParams{
		Executor: executor,
		Ent:      client,
	})

	systemService := NewSystemService(SystemServiceParams{})

	modelService := NewModelService(ModelServiceParams{
		ChannelService: channelService,
		SystemService:  systemService,
		Ent:            client,
	})

	service := NewBackupService(BackupServiceParams{
		ChannelService: channelService,
		ModelService:   modelService,
		Ent:            client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)

	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	user, err := client.User.Create().
		SetEmail("test@example.com").
		SetPassword("password").
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)

	ctx = contexts.WithUser(ctx, user)

	return client, service, ctx
}

func createBackupTestChannel(t *testing.T, client *ent.Client, ctx context.Context, name string, chType channel.Type) *ent.Channel {
	credentials := &objects.ChannelCredentials{
		APIKey: "test-api-key",
	}

	settings := &objects.ChannelSettings{
		ExtraModelPrefix: "test",
	}

	ch, err := client.Channel.Create().
		SetType(chType).
		SetName(name).
		SetBaseURL("https://api.example.com").
		SetStatus(channel.StatusEnabled).
		SetCredentials(credentials).
		SetSupportedModels([]string{"model-1", "model-2"}).
		SetAutoSyncSupportedModels(true).
		SetTags([]string{"test"}).
		SetDefaultTestModel("model-1").
		SetSettings(settings).
		SetOrderingWeight(1).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func createBackupTestModel(t *testing.T, client *ent.Client, ctx context.Context, developer, modelID string) *ent.Model {
	modelCard := &objects.ModelCard{
		Reasoning: objects.ModelCardReasoning{
			Supported: true,
			Default:   false,
		},
		ToolCall:    true,
		Temperature: true,
		Vision:      false,
		Cost: objects.ModelCardCost{
			Input:  0.001,
			Output: 0.002,
		},
		Limit: objects.ModelCardLimit{
			Context: 8192,
			Output:  4096,
		},
	}

	settings := &objects.ModelSettings{
		Associations: []*objects.ModelAssociation{},
	}

	m, err := client.Model.Create().
		SetDeveloper(developer).
		SetModelID(modelID).
		SetType(model.TypeChat).
		SetName(fmt.Sprintf("Test Model %s", modelID)).
		SetIcon("test-icon").
		SetGroup("test-group").
		SetModelCard(modelCard).
		SetSettings(settings).
		SetStatus(model.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return m
}

func TestBackupService_Backup(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	ch2 := createBackupTestChannel(t, client, ctx, "Channel 2", channel.TypeAnthropic)

	m1 := createBackupTestModel(t, client, ctx, "openai", "gpt-4")
	m2 := createBackupTestModel(t, client, ctx, "anthropic", "claude-3")

	data, err := service.Backup(ctx, BackupOptions{
		IncludeChannels: true,
		IncludeModels:   true,
	})
	require.NoError(t, err)
	require.NotNil(t, data)
	require.NotEmpty(t, data)

	var backupData BackupData

	err = json.Unmarshal(data, &backupData)
	require.NoError(t, err)

	require.Equal(t, BackupVersion, backupData.Version)
	require.Len(t, backupData.Channels, 2)
	require.Len(t, backupData.Models, 2)

	require.Equal(t, ch1.Name, backupData.Channels[0].Name)
	require.Equal(t, ch2.Name, backupData.Channels[1].Name)
	require.Equal(t, m1.Name, backupData.Models[0].Name)
	require.Equal(t, m2.Name, backupData.Models[1].Name)
}

func TestBackupService_Backup_Empty(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	data, err := service.Backup(ctx, BackupOptions{
		IncludeChannels: true,
		IncludeModels:   true,
	})
	require.NoError(t, err)
	require.NotNil(t, data)

	var backupData BackupData

	err = json.Unmarshal(data, &backupData)
	require.NoError(t, err)

	require.Equal(t, BackupVersion, backupData.Version)
	require.Len(t, backupData.Channels, 0)
	require.Len(t, backupData.Models, 0)
}

func TestBackupService_Restore(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	m1 := createBackupTestModel(t, client, ctx, "openai", "gpt-4")

	data, err := service.Backup(ctx, BackupOptions{
		IncludeChannels: true,
		IncludeModels:   true,
	})
	require.NoError(t, err)

	channelsBefore, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)

	modelsBefore, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	channelsAfter, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)

	modelsAfter, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)

	require.Equal(t, channelsBefore, channelsAfter)
	require.Equal(t, modelsBefore, modelsAfter)

	restoredChannel, err := client.Channel.Query().
		Where(channel.Name(ch1.Name)).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, ch1.Name, restoredChannel.Name)
	require.Equal(t, ch1.BaseURL, restoredChannel.BaseURL)

	restoredModel, err := client.Model.Query().
		Where(model.ModelID(m1.ModelID)).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, m1.Name, restoredModel.Name)
	require.Equal(t, m1.Developer, restoredModel.Developer)
}

func TestBackupService_Restore_NewData(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	baseURL := "https://new-api.example.com"
	autoSync := true

	backupData := BackupData{
		Version: BackupVersion,
		Channels: []*BackupChannel{
			{
				Channel: ent.Channel{
					Type:                    channel.TypeOpenai,
					Name:                    "New Channel",
					BaseURL:                 baseURL,
					Status:                  channel.StatusEnabled,
					SupportedModels:         []string{"new-model-1"},
					AutoSyncSupportedModels: autoSync,
					Tags:                    []string{"new"},
					DefaultTestModel:        "new-model-1",
					OrderingWeight:          10,
				},
			},
		},
		Models: []*BackupModel{
			{
				Model: ent.Model{
					Developer: "new-developer",
					ModelID:   "new-model",
					Type:      model.TypeChat,
					Name:      "New Model",
					Icon:      "new-icon",
					Group:     "new-group",
					Status:    model.StatusEnabled,
				},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	channels, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, channels)

	models, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, models)

	newChannel, err := client.Channel.Query().
		Where(channel.Name("New Channel")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "New Channel", newChannel.Name)

	newModel, err := client.Model.Query().
		Where(model.ModelID("new-model")).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "New Model", newModel.Name)
}

func TestBackupService_Restore_UpdateExisting(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	m1 := createBackupTestModel(t, client, ctx, "openai", "gpt-4")

	baseURL := "https://updated-api.example.com"
	autoSync := false

	backupData := BackupData{
		Version: BackupVersion,
		Channels: []*BackupChannel{
			{
				Channel: ent.Channel{
					Type:                    ch1.Type,
					Name:                    ch1.Name,
					BaseURL:                 baseURL,
					Status:                  channel.StatusDisabled,
					SupportedModels:         []string{"updated-model"},
					AutoSyncSupportedModels: autoSync,
					Tags:                    []string{"updated"},
					DefaultTestModel:        "updated-model",
					OrderingWeight:          20,
				},
			},
		},
		Models: []*BackupModel{
			{
				Model: ent.Model{
					Developer: m1.Developer,
					ModelID:   m1.ModelID,
					Type:      m1.Type,
					Name:      "Updated Model",
					Icon:      "updated-icon",
					Group:     "updated-group",
					Status:    model.StatusDisabled,
				},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	channels, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, channels)

	models, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, models)

	updatedChannel, err := client.Channel.Query().
		Where(channel.Name(ch1.Name)).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, ch1.Name, updatedChannel.Name)
	require.Equal(t, "https://updated-api.example.com", updatedChannel.BaseURL)
	require.Equal(t, channel.StatusDisabled, updatedChannel.Status)
	require.Equal(t, []string{"updated-model"}, updatedChannel.SupportedModels)
	require.Equal(t, false, updatedChannel.AutoSyncSupportedModels)
	require.Equal(t, []string{"updated"}, updatedChannel.Tags)
	require.Equal(t, "updated-model", updatedChannel.DefaultTestModel)
	require.Equal(t, 20, updatedChannel.OrderingWeight)

	updatedModel, err := client.Model.Query().
		Where(model.ModelID(m1.ModelID)).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "Updated Model", updatedModel.Name)
	require.Equal(t, model.StatusDisabled, updatedModel.Status)
	require.Equal(t, "updated-icon", updatedModel.Icon)
	require.Equal(t, "updated-group", updatedModel.Group)
}

func TestBackupService_Restore_InvalidJSON(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	invalidData := []byte("invalid json")

	err := service.Restore(ctx, invalidData, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
	})
	require.Error(t, err)
}

func TestBackupService_Restore_InvalidVersion(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	backupData := BackupData{
		Version:  "invalid-version",
		Channels: []*BackupChannel{},
		Models:   []*BackupModel{},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
	})
	require.Error(t, err)
}
