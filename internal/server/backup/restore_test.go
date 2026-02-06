package backup

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

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
