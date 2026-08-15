package backup

import (
	"encoding/json"
	"testing"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestBackupService_Restore_SystemConfigs(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	_, err := client.System.Create().
		SetKey(biz.SystemKeyRetryPolicy).
		SetValue(`{"max_retries":1}`).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.System.Create().
		SetKey(biz.SystemKeySecretKey).
		SetValue("target-secret").
		Save(ctx)
	require.NoError(t, err)

	data, err := json.Marshal(BackupData{
		Version: BackupVersion,
		SystemConfigs: []*BackupSystemConfig{
			{Key: biz.SystemKeyRetryPolicy, Value: `{"max_retries":4}`},
			{Key: biz.SystemKeySecretKey, Value: "source-secret"},
		},
	})
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{IncludeSystemConfigs: true})
	require.NoError(t, err)

	retryPolicy, err := client.System.Query().Where(system.KeyEQ(biz.SystemKeyRetryPolicy)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, `{"max_retries":4}`, retryPolicy.Value)

	secretKey, err := client.System.Query().Where(system.KeyEQ(biz.SystemKeySecretKey)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "target-secret", secretKey.Value)
}

func TestBackupService_Restore(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	existingPrice := createBackupTestChannelModelPrice(t, client, ctx, ch1.ID, "gpt-4")
	m1 := createBackupTestModel(t, client, ctx, "openai", "gpt-4")

	data, err := service.Backup(ctx, BackupOptions{
		IncludeChannels:    true,
		IncludeModels:      true,
		IncludeModelPrices: true,
	})
	require.NoError(t, err)

	channelsBefore, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)

	modelsBefore, err := client.Model.Query().Count(ctx)
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		IncludeModelPrices:      true,
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

	restoredPrice, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch1.ID),
			channelmodelprice.ModelID("gpt-4"),
		).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, existingPrice.ReferenceID, restoredPrice.ReferenceID)
}

func TestBackupService_Restore_ModelPricesOnly(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)

	backupData := BackupData{
		Version:  BackupVersion,
		Channels: []*BackupChannel{},
		Models:   []*BackupModel{},
		APIKeys:  []*BackupAPIKey{},
		ChannelModelPrices: []*BackupChannelModelPrice{
			{
				ChannelName: ch1.Name,
				ModelID:     "gpt-4",
				Price: objects.ModelPrice{
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode: objects.PricingModeFlatFee,
								FlatFee: func() *decimal.Decimal {
									d := decimal.NewFromFloat(1)
									return &d
								}(),
							},
						},
					},
				},
				ReferenceID: "ref-gpt-4",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         false,
		IncludeModels:           false,
		IncludeAPIKeys:          false,
		IncludeModelPrices:      true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
		APIKeyConflictStrategy:  ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	restoredPrice, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch1.ID),
			channelmodelprice.ModelID("gpt-4"),
		).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "ref-gpt-4", restoredPrice.ReferenceID)
}

func TestBackupService_Restore_RemapChannelIDsInModelSettingsAndAPIKeyProfiles(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	createBackupTestProject(t, client, ctx, "Default", "Default Project")

	oldChannelID := 123
	backupData := BackupData{
		Version: BackupVersion,
		Channels: []*BackupChannel{
			{
				Channel: ent.Channel{
					ID:      oldChannelID,
					Type:    channel.TypeOpenai,
					Name:    "Channel From Backup",
					BaseURL: "https://api.example.com",
					Status:  channel.StatusEnabled,
				},
				Credentials: objects.ChannelCredentials{APIKey: "backup-api-key"},
			},
		},
		Models: []*BackupModel{
			{
				Model: ent.Model{
					Developer: "openai",
					ModelID:   "gpt-4",
					Type:      model.TypeChat,
					Name:      "GPT-4",
					Icon:      "test-icon",
					Group:     "test",
					Settings: &objects.ModelSettings{
						Associations: []*objects.ModelAssociation{
							{
								Type:     "channel_model",
								Priority: 0,
								ChannelModel: &objects.ChannelModelAssociation{
									ChannelID: oldChannelID,
									ModelID:   "gpt-4",
								},
								Regex: &objects.RegexAssociation{
									Pattern: ".*",
									Exclude: []*objects.ExcludeAssociation{
										{ChannelIds: []int{oldChannelID}},
									},
								},
							},
						},
					},
					Status: model.StatusEnabled,
				},
			},
		},
		APIKeys: []*BackupAPIKey{
			{
				APIKey: ent.APIKey{
					Key:    "sk-backup-key",
					Name:   "Backup API Key",
					Type:   "user",
					Status: "enabled",
					Scopes: []string{"chat"},
					Profiles: &objects.APIKeyProfiles{
						ActiveProfile: "default",
						Profiles: []objects.APIKeyProfile{
							{
								Name:       "default",
								ChannelIDs: []int{oldChannelID},
								ModelIDs:   []string{"gpt-4"},
							},
						},
					},
				},
				ProjectName: "Default",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		IncludeAPIKeys:          true,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
		ModelConflictStrategy:   ConflictStrategyOverwrite,
		APIKeyConflictStrategy:  ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	restoredChannel, err := client.Channel.Query().Where(channel.Name("Channel From Backup")).First(ctx)
	require.NoError(t, err)
	require.NotEqual(t, oldChannelID, restoredChannel.ID)

	restoredModel, err := client.Model.Query().Where(model.ModelID("gpt-4")).First(ctx)
	require.NoError(t, err)
	require.NotNil(t, restoredModel.Settings)
	require.Len(t, restoredModel.Settings.Associations, 1)
	require.NotNil(t, restoredModel.Settings.Associations[0].ChannelModel)
	require.Equal(t, restoredChannel.ID, restoredModel.Settings.Associations[0].ChannelModel.ChannelID)
	require.NotNil(t, restoredModel.Settings.Associations[0].Regex)
	require.Len(t, restoredModel.Settings.Associations[0].Regex.Exclude, 1)
	require.Equal(t, []int{restoredChannel.ID}, restoredModel.Settings.Associations[0].Regex.Exclude[0].ChannelIds)

	restoredKey, err := client.APIKey.Query().Where(apikey.Key("sk-backup-key")).First(ctx)
	require.NoError(t, err)
	require.NotNil(t, restoredKey.Profiles)
	require.Len(t, restoredKey.Profiles.Profiles, 1)
	require.Equal(t, []int{restoredChannel.ID}, restoredKey.Profiles.Profiles[0].ChannelIDs)
}

func TestBackupService_Restore_RemapChannelIDsInProjectProfiles(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	oldChannelID := 456
	backupData := BackupData{
		Version: BackupVersion,
		Projects: []*BackupProject{
			{
				Project: ent.Project{
					Name:        "Project With Profiles",
					Description: "project with channel restrictions",
					Status:      project.StatusActive,
					Profiles: &objects.ProjectProfiles{
						ActiveProfile: "production",
						Profiles: []objects.ProjectProfile{
							{
								Name:        "production",
								ChannelIDs:  []int{oldChannelID},
								ChannelTags: []string{"allowed"},
							},
						},
					},
				},
			},
		},
		Channels: []*BackupChannel{
			{
				Channel: ent.Channel{
					ID:      oldChannelID,
					Type:    channel.TypeOpenai,
					Name:    "Project Channel From Backup",
					BaseURL: "https://api.example.com",
					Status:  channel.StatusEnabled,
				},
				Credentials: objects.ChannelCredentials{APIKey: "backup-api-key"},
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeProjects:         true,
		IncludeChannels:         true,
		ProjectConflictStrategy: ConflictStrategyOverwrite,
		ChannelConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	restoredChannel, err := client.Channel.Query().Where(channel.Name("Project Channel From Backup")).First(ctx)
	require.NoError(t, err)
	require.NotEqual(t, oldChannelID, restoredChannel.ID)

	restoredProject, err := client.Project.Query().Where(project.Name("Project With Profiles")).First(ctx)
	require.NoError(t, err)
	require.NotNil(t, restoredProject.Profiles)
	require.Len(t, restoredProject.Profiles.Profiles, 1)
	require.Equal(t, []int{restoredChannel.ID}, restoredProject.Profiles.Profiles[0].ChannelIDs)
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
				Credentials: objects.ChannelCredentials{
					APIKey: "test-api-key",
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
		ChannelModelPrices: []*BackupChannelModelPrice{
			{
				ChannelName: "New Channel",
				ModelID:     "new-model-1",
				Price: objects.ModelPrice{
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode: objects.PricingModeFlatFee,
								FlatFee: func() *decimal.Decimal {
									d := decimal.NewFromFloat(1)
									return &d
								}(),
							},
						},
					},
				},
				ReferenceID: "ref-new-model-1",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:         true,
		IncludeModels:           true,
		IncludeModelPrices:      true,
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

	priceCount, err := client.ChannelModelPrice.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, priceCount)
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
				Credentials: objects.ChannelCredentials{
					APIKey: "test-api-key",
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
		IncludeModelPrices:      true,
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

func TestBackupService_Restore_ModelPriceConflictStrategy_Skip(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	existingPrice := createBackupTestChannelModelPrice(t, client, ctx, ch1.ID, "gpt-4")

	newPricePerUnit := decimal.NewFromFloat(999.99)
	backupData := BackupData{
		Version:  BackupVersion,
		Channels: []*BackupChannel{},
		Models:   []*BackupModel{},
		ChannelModelPrices: []*BackupChannelModelPrice{
			{
				ChannelName: ch1.Name,
				ModelID:     "gpt-4",
				Price: objects.ModelPrice{
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: &newPricePerUnit,
							},
						},
					},
				},
				ReferenceID: "new-ref-id",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:            false,
		IncludeModels:              false,
		IncludeAPIKeys:             false,
		IncludeModelPrices:         true,
		ModelPriceConflictStrategy: ConflictStrategySkip,
	})
	require.NoError(t, err)

	restoredPrice, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch1.ID),
			channelmodelprice.ModelID("gpt-4"),
		).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, existingPrice.ReferenceID, restoredPrice.ReferenceID)
}

func TestBackupService_Restore_ModelPriceConflictStrategy_Overwrite(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	_ = createBackupTestChannelModelPrice(t, client, ctx, ch1.ID, "gpt-4")

	newPricePerUnit := decimal.NewFromFloat(999.99)
	backupData := BackupData{
		Version:  BackupVersion,
		Channels: []*BackupChannel{},
		Models:   []*BackupModel{},
		ChannelModelPrices: []*BackupChannelModelPrice{
			{
				ChannelName: ch1.Name,
				ModelID:     "gpt-4",
				Price: objects.ModelPrice{
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: &newPricePerUnit,
							},
						},
					},
				},
				ReferenceID: "overwritten-ref-id",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:            false,
		IncludeModels:              false,
		IncludeAPIKeys:             false,
		IncludeModelPrices:         true,
		ModelPriceConflictStrategy: ConflictStrategyOverwrite,
	})
	require.NoError(t, err)

	restoredPrice, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(ch1.ID),
			channelmodelprice.ModelID("gpt-4"),
		).
		First(ctx)
	require.NoError(t, err)
	require.Equal(t, "overwritten-ref-id", restoredPrice.ReferenceID)
}

func TestBackupService_Restore_ModelPriceConflictStrategy_Error(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	ch1 := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	_ = createBackupTestChannelModelPrice(t, client, ctx, ch1.ID, "gpt-4")

	newPricePerUnit := decimal.NewFromFloat(999.99)
	backupData := BackupData{
		Version:  BackupVersion,
		Channels: []*BackupChannel{},
		Models:   []*BackupModel{},
		ChannelModelPrices: []*BackupChannelModelPrice{
			{
				ChannelName: ch1.Name,
				ModelID:     "gpt-4",
				Price: objects.ModelPrice{
					Items: []objects.ModelPriceItem{
						{
							ItemCode: objects.PriceItemCodeUsage,
							Pricing: objects.Pricing{
								Mode:         objects.PricingModeUsagePerUnit,
								UsagePerUnit: &newPricePerUnit,
							},
						},
					},
				},
				ReferenceID: "new-ref-id",
			},
		},
	}

	data, err := json.MarshalIndent(backupData, "", "  ")
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeChannels:            false,
		IncludeModels:              false,
		IncludeAPIKeys:             false,
		IncludeModelPrices:         true,
		ModelPriceConflictStrategy: ConflictStrategyError,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "channel model price already exists")
}

func TestBackupService_Restore_UsageStats(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	proj := createBackupTestProject(t, client, ctx, "Project1", "Test Project")
	ch := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	ak := createBackupTestAPIKey(t, client, ctx, user, proj, "API Key 1", "sk-test-key-1")
	_, usage := createBackupTestUsage(t, client, ctx, proj, ch, ak)

	data, err := service.Backup(ctx, BackupOptions{
		IncludeAPIKeys:    true,
		IncludeUsageStats: true,
	})
	require.NoError(t, err)

	_, err = client.UsageLog.Delete().Exec(ctx)
	require.NoError(t, err)

	_, err = client.Request.Delete().Exec(ctx)
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeUsageStats: true,
	})
	require.NoError(t, err)

	requestsCount, err := client.Request.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, requestsCount)

	usageLogs, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, usageLogs, 1)
	require.Equal(t, int64(150), usageLogs[0].TotalTokens)
	require.Equal(t, int64(20), usageLogs[0].PromptCachedTokens)
	require.NotNil(t, usageLogs[0].TotalCost)
	require.Equal(t, *usage.TotalCost, *usageLogs[0].TotalCost)
	require.Equal(t, "price-ref", usageLogs[0].CostPriceReferenceID)

	restoredRequest, err := client.Request.Get(ctx, usageLogs[0].RequestID)
	require.NoError(t, err)
	require.Equal(t, "gpt-4", restoredRequest.ModelID)
	require.Equal(t, proj.ID, restoredRequest.ProjectID)
	require.Equal(t, ch.ID, restoredRequest.ChannelID)
	require.Equal(t, ak.ID, restoredRequest.APIKeyID)
	require.JSONEq(t, `{}`, string(restoredRequest.RequestBody))

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeUsageStats: true,
	})
	require.NoError(t, err)

	requestsCount, err = client.Request.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, requestsCount)

	usageLogsAfterSecondRestore, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, usageLogsAfterSecondRestore, 1)
	require.Equal(t, restoredRequest.ID, usageLogsAfterSecondRestore[0].RequestID)
}

func TestBackupService_Restore_MultipleUsageLogsPerRequest(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, err := client.User.Query().First(ctx)
	require.NoError(t, err)
	proj := createBackupTestProject(t, client, ctx, "Project1", "Test Project")
	ch := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	ak := createBackupTestAPIKey(t, client, ctx, user, proj, "API Key 1", "sk-test-key-1")
	req, originalUsage := createBackupTestUsage(t, client, ctx, proj, ch, ak)
	require.NoError(t, client.UsageLog.DeleteOne(originalUsage).Exec(ctx))

	primaryExecution, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetChannelID(ch.ID).
		SetExternalID("primary-external-secret").
		SetModelID("gpt-4").
		SetPurpose(requestexecution.PurposePrimary).
		SetReasoningEffort("high").
		SetRequestBody(objects.JSONRawMessage(`{"prompt":"primary-execution-secret"}`)).
		SetResponseBody(objects.JSONRawMessage(`{"answer":"primary-response-secret"}`)).
		SetResponseChunks([]objects.JSONRawMessage{objects.JSONRawMessage(`{"delta":"primary-chunk-secret"}`)}).
		SetRequestHeaders(objects.JSONRawMessage(`{"X-Private":"primary-header-secret"}`)).
		SetErrorMessage("primary-error-secret").
		SetResponseStatusCode(502).
		SetMetricsLatencyMs(1234).
		SetMetricsFirstTokenLatencyMs(234).
		SetMetricsReasoningDurationMs(345).
		SetRequestURL("https://primary-url-secret.example/v1/chat/completions").
		SetPassThroughApplied(true).
		SetStatus(requestexecution.StatusCompleted).
		Save(ctx)
	require.NoError(t, err)

	visionExecution, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetChannelID(ch.ID).
		SetModelID("vision-model").
		SetPurpose(requestexecution.PurposeVisionDelegation).
		SetRequestBody(objects.JSONRawMessage(`{"prompt":"vision-execution-secret"}`)).
		SetStatus(requestexecution.StatusCompleted).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(req.ID).
		SetRequestExecutionID(primaryExecution.ID).
		SetAPIKeyID(ak.ID).
		SetProjectID(proj.ID).
		SetChannelID(ch.ID).
		SetModelID("gpt-4").
		SetPromptTokens(40).
		SetCompletionTokens(60).
		SetTotalTokens(100).
		SetTotalCost(1).
		SetSource(usagelog.SourceAPI).
		SetFormat("openai/chat_completions").
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(req.ID).
		SetRequestExecutionID(visionExecution.ID).
		SetAPIKeyID(ak.ID).
		SetProjectID(proj.ID).
		SetChannelID(ch.ID).
		SetModelID("vision-model").
		SetPromptTokens(20).
		SetCompletionTokens(30).
		SetTotalTokens(50).
		SetTotalCost(2).
		SetSource(usagelog.SourceAPI).
		SetFormat("openai/chat_completions").
		Save(ctx)
	require.NoError(t, err)

	data, err := service.Backup(ctx, BackupOptions{
		IncludeAPIKeys:    true,
		IncludeUsageStats: true,
	})
	require.NoError(t, err)
	require.NotContains(t, string(data), "primary-execution-secret")
	require.NotContains(t, string(data), "primary-response-secret")
	require.NotContains(t, string(data), "primary-chunk-secret")
	require.NotContains(t, string(data), "primary-header-secret")
	require.NotContains(t, string(data), "primary-external-secret")
	require.NotContains(t, string(data), "primary-error-secret")
	require.NotContains(t, string(data), "primary-url-secret")
	require.NotContains(t, string(data), "vision-execution-secret")

	var backupData BackupData
	require.NoError(t, json.Unmarshal(data, &backupData))
	require.Len(t, backupData.UsageLogs, 2)
	require.Len(t, backupData.RequestExecutions, 2)
	for _, execution := range backupData.RequestExecutions {
		require.False(t, execution.DetailsIncluded)
		require.Empty(t, execution.RequestBody)
		require.Empty(t, execution.ResponseBody)
		require.Empty(t, execution.ResponseChunks)
		require.Empty(t, execution.RequestHeaders)
	}
	require.ElementsMatch(t,
		[]int{primaryExecution.ID, visionExecution.ID},
		[]int{backupData.UsageLogs[0].RequestExecutionID, backupData.UsageLogs[1].RequestExecutionID},
	)

	requestLogData, err := service.Backup(ctx, BackupOptions{
		IncludeUsageStats:  true,
		IncludeRequestLogs: true,
	})
	require.NoError(t, err)
	require.Contains(t, string(requestLogData), "primary-execution-secret")
	require.Contains(t, string(requestLogData), "primary-response-secret")
	require.Contains(t, string(requestLogData), "primary-chunk-secret")
	require.Contains(t, string(requestLogData), "primary-header-secret")
	require.Contains(t, string(requestLogData), "primary-external-secret")
	require.Contains(t, string(requestLogData), "primary-error-secret")
	require.Contains(t, string(requestLogData), "primary-url-secret")
	require.Contains(t, string(requestLogData), "vision-execution-secret")
	var requestLogBackupData BackupData
	require.NoError(t, json.Unmarshal(requestLogData, &requestLogBackupData))
	require.ElementsMatch(t,
		[]requestexecution.Purpose{requestexecution.PurposePrimary, requestexecution.PurposeVisionDelegation},
		lo.Map(requestLogBackupData.RequestExecutions, func(execution *BackupRequestExecution, _ int) requestexecution.Purpose {
			return execution.Purpose
		}),
	)
	for _, execution := range requestLogBackupData.RequestExecutions {
		require.True(t, execution.DetailsIncluded)
	}

	_, err = client.UsageLog.Delete().Exec(ctx)
	require.NoError(t, err)
	_, err = client.RequestExecution.Delete().Exec(ctx)
	require.NoError(t, err)
	_, err = client.Request.Delete().Exec(ctx)
	require.NoError(t, err)

	restore := func() {
		t.Helper()
		require.NoError(t, service.Restore(ctx, requestLogData, RestoreOptions{IncludeUsageStats: true}))

		usageLogs, queryErr := client.UsageLog.Query().All(ctx)
		require.NoError(t, queryErr)
		require.Len(t, usageLogs, 2)

		usageByModel := make(map[string]*ent.UsageLog, len(usageLogs))
		for _, usageLog := range usageLogs {
			usageByModel[usageLog.ModelID] = usageLog
		}
		require.Equal(t, int64(100), usageByModel["gpt-4"].TotalTokens)
		require.Equal(t, int64(50), usageByModel["vision-model"].TotalTokens)
		require.Equal(t, 1.0, *usageByModel["gpt-4"].TotalCost)
		require.Equal(t, 2.0, *usageByModel["vision-model"].TotalCost)

		executions, queryErr := client.RequestExecution.Query().All(ctx)
		require.NoError(t, queryErr)
		require.Len(t, executions, 2)
		executionPurposeByID := make(map[int]requestexecution.Purpose, len(executions))
		for _, execution := range executions {
			executionPurposeByID[execution.ID] = execution.Purpose
			require.JSONEq(t, `{}`, string(execution.RequestBody))
			require.Empty(t, execution.ResponseBody)
			require.Empty(t, execution.ResponseChunks)
			require.Empty(t, execution.RequestHeaders)
			require.Empty(t, execution.ExternalID)
			require.Nil(t, execution.ReasoningEffort)
			require.Empty(t, execution.ErrorMessage)
			require.Nil(t, execution.ResponseStatusCode)
			require.Nil(t, execution.MetricsLatencyMs)
			require.Nil(t, execution.MetricsFirstTokenLatencyMs)
			require.Nil(t, execution.MetricsReasoningDurationMs)
			require.Empty(t, execution.RequestURL)
			require.False(t, execution.PassThroughApplied)
		}
		require.Equal(t, requestexecution.PurposePrimary, executionPurposeByID[usageByModel["gpt-4"].RequestExecutionID])
		require.Equal(t, requestexecution.PurposeVisionDelegation, executionPurposeByID[usageByModel["vision-model"].RequestExecutionID])
	}

	restore()
	usageLogs, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	for _, usageLog := range usageLogs {
		_, err = client.UsageLog.UpdateOneID(usageLog.ID).
			SetUpdatedAt(usageLog.UpdatedAt).
			Modify(func(update *sql.UpdateBuilder) {
				update.SetNull(usagelog.FieldRequestExecutionID)
			}).
			Save(ctx)
		require.NoError(t, err)
	}
	unlinkedUsageCount, err := client.UsageLog.Query().
		Where(usagelog.RequestExecutionIDIsNil()).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, unlinkedUsageCount)

	restore()
	require.NoError(t, service.Restore(ctx, requestLogData, RestoreOptions{
		IncludeUsageStats:  true,
		IncludeRequestLogs: true,
	}))
	executionCount, err := client.RequestExecution.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, executionCount)
	usageCount, err := client.UsageLog.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, usageCount)
	requestCount, err := client.Request.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, requestCount)
}

func TestBackupService_Restore_UsageStatsWithRequestLogs(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	user, _ := client.User.Query().First(ctx)
	proj := createBackupTestProject(t, client, ctx, "Project1", "Test Project")
	ch := createBackupTestChannel(t, client, ctx, "Channel 1", channel.TypeOpenai)
	ak := createBackupTestAPIKey(t, client, ctx, user, proj, "API Key 1", "sk-test-key-1")
	_, usage := createBackupTestUsage(t, client, ctx, proj, ch, ak)

	data, err := service.Backup(ctx, BackupOptions{
		IncludeAPIKeys:     true,
		IncludeUsageStats:  true,
		IncludeRequestLogs: true,
	})
	require.NoError(t, err)

	_, err = client.UsageLog.Delete().Exec(ctx)
	require.NoError(t, err)

	_, err = client.Request.Delete().Exec(ctx)
	require.NoError(t, err)

	err = service.Restore(ctx, data, RestoreOptions{
		IncludeUsageStats:  true,
		IncludeRequestLogs: true,
	})
	require.NoError(t, err)

	requests, err := client.Request.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, requests, 1)
	require.JSONEq(t, `{"model":"gpt-4"}`, string(requests[0].RequestBody))
	require.Equal(t, "127.0.0.1", requests[0].ClientIP)

	usageLogs, err := client.UsageLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, usageLogs, 1)
	require.Equal(t, requests[0].ID, usageLogs[0].RequestID)
	require.Equal(t, int64(150), usageLogs[0].TotalTokens)
	require.NotNil(t, usageLogs[0].TotalCost)
	require.Equal(t, *usage.TotalCost, *usageLogs[0].TotalCost)
}
