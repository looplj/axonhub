package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestChannelService_SaveChannelModelPrices(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	// Create a test channel
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Test Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-3.5-turbo"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	price1 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: loToDecimalPtr("0.01"),
				},
			},
		},
	}

	price2 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: loToDecimalPtr("0.02"),
				},
			},
		},
	}

	t.Run("batch create", func(t *testing.T) {
		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-4",
				Price:   price1,
			},
			{
				ModelID: "gpt-3.5-turbo",
				Price:   price2,
			},
		}

		results, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.NoError(t, err)
		require.Len(t, results, 2)

		for _, res := range results {
			// Check if version is created
			version, err := client.ChannelModelPriceVersion.Query().
				Where(channelmodelpriceversion.ChannelModelPriceID(res.ID)).
				Only(ctx)
			require.NoError(t, err)
			require.Equal(t, channelmodelpriceversion.StatusActive, version.Status)
			require.Equal(t, res.ReferenceID, version.ReferenceID)
			require.Len(t, res.ReferenceID, 8)
		}
	})

	t.Run("batch update and archive old version", func(t *testing.T) {
		// First update gpt-4
		newPrice1 := price1
		newPrice1.Items[0].Pricing.UsagePerUnit = loToDecimalPtr("0.015")

		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-4",
				Price:   newPrice1,
			},
		}

		// Store old ref id
		oldPrice, err := client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID("gpt-4"),
			).Only(ctx)
		require.NoError(t, err)

		oldRefID := oldPrice.ReferenceID

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		results, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.NoError(t, err)
		require.Len(t, results, 1)

		updatedPrice := results[0]
		require.NotEqual(t, oldRefID, updatedPrice.ReferenceID)
		require.Equal(t, newPrice1, updatedPrice.Price)

		// Check versions
		versions, err := client.ChannelModelPriceVersion.Query().
			Where(channelmodelpriceversion.ChannelModelPriceID(updatedPrice.ID)).
			Order(ent.Asc(channelmodelpriceversion.FieldEffectiveStartAt)).
			All(ctx)
		require.NoError(t, err)
		require.Len(t, versions, 2)

		// Old version should be archived
		require.Equal(t, channelmodelpriceversion.StatusArchived, versions[0].Status)
		require.NotNil(t, versions[0].EffectiveEndAt)
		require.Equal(t, oldRefID, versions[0].ReferenceID)

		// New version should be active
		require.Equal(t, channelmodelpriceversion.StatusActive, versions[1].Status)
		require.Nil(t, versions[1].EffectiveEndAt)
		require.Equal(t, updatedPrice.ReferenceID, versions[1].ReferenceID)
	})

	t.Run("delete missing models", func(t *testing.T) {
		// Only send gpt-3.5-turbo, gpt-4 should be deleted
		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-3.5-turbo",
				Price:   price2,
			},
		}

		// Verify gpt-4 exists before delete
		exists, err := client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID("gpt-4"),
			).Exist(ctx)
		require.NoError(t, err)
		require.True(t, exists)

		results, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.NoError(t, err)
		require.Len(t, results, 1) // Only gpt-3.5-turbo remains (as skip/update)

		// Verify gpt-4 is deleted
		exists, err = client.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(ch.ID),
				channelmodelprice.ModelID("gpt-4"),
			).Exist(ctx)
		require.NoError(t, err)
		require.False(t, exists)

		// Verify gpt-4 versions are archived
		versions, err := client.ChannelModelPriceVersion.Query().
			Where(
				channelmodelpriceversion.ChannelID(ch.ID),
				channelmodelpriceversion.ModelID("gpt-4"),
			).All(ctx)
		require.NoError(t, err)

		for _, v := range versions {
			require.Equal(t, channelmodelpriceversion.StatusArchived, v.Status)
			require.NotNil(t, v.EffectiveEndAt)
		}
	})

	t.Run("duplicate model id should error", func(t *testing.T) {
		inputs := []SaveChannelModelPriceInput{
			{
				ModelID: "gpt-4",
				Price:   price1,
			},
			{
				ModelID: "gpt-4",
				Price:   price2,
			},
		}

		_, err := svc.SaveChannelModelPrices(ctx, ch.ID, inputs)
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate model price input")
		require.Contains(t, err.Error(), "model_id=gpt-4")
	})
}

func TestChannelService_UpdateChannelModelPriceEntityKeepsOneActiveVersion(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Version transition lock").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"locked-model"}).
		SetDefaultTestModel("locked-model").
		Save(ctx)
	require.NoError(t, err)

	initialPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{{
		ItemCode: objects.PriceItemCodeUsage,
		Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: loToDecimalPtr("1")},
	}}}
	created, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{{
		ModelID: "locked-model",
		Price:   initialPrice,
	}})
	require.NoError(t, err)
	require.Len(t, created, 1)

	now := time.Now().UTC()
	err = svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		for index, value := range []string{"2", "3"} {
			newPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: loToDecimalPtr(value)},
			}}}
			if _, err := svc.updateChannelModelPriceEntity(txCtx, created[0], newPrice, true, nil, now.Add(time.Duration(index+1)*time.Second)); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	price, err := client.ChannelModelPrice.Query().Where(
		channelmodelprice.ChannelID(ch.ID),
		channelmodelprice.ModelID("locked-model"),
	).Only(ctx)
	require.NoError(t, err)
	versions, err := price.QueryVersions().All(ctx)
	require.NoError(t, err)
	activeCount := 0
	for _, version := range versions {
		if version.Status == channelmodelpriceversion.StatusActive {
			activeCount++
			require.Equal(t, price.ReferenceID, version.ReferenceID)
		}
	}
	require.Equal(t, 1, activeCount)
	require.Equal(t, "3", price.Price.Items[0].Pricing.UsagePerUnit.String())
}

func usageItem(value string) objects.ModelPriceItem {
	return objects.ModelPriceItem{
		ItemCode: objects.PriceItemCodeUsage,
		Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: loToDecimalPtr(value)},
	}
}

func completionItem(value string) objects.ModelPriceItem {
	return objects.ModelPriceItem{
		ItemCode: objects.PriceItemCodeCompletion,
		Pricing:  objects.Pricing{Mode: objects.PricingModeUsagePerUnit, UsagePerUnit: loToDecimalPtr(value)},
	}
}

// Simulates the endpoint-sync vs manual-save race: the sync snapshot is taken
// before a concurrent manual save changes item X to "7"; the sync then updates
// the row reporting only item X. The merge must run against the locked row
// (which also carries the concurrent manual edit to item Y), so Y is not lost.
func TestChannelService_UpdateChannelModelPriceEntityMergesAgainstLockedRow(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Locked row merge").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"race-model"}).
		SetDefaultTestModel("race-model").
		Save(ctx)
	require.NoError(t, err)

	initialPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{
		usageItem("1"),
		completionItem("2"),
	}}
	created, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{{
		ModelID: "race-model",
		Price:   initialPrice,
	}})
	require.NoError(t, err)
	require.Len(t, created, 1)

	err = svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Sync snapshot taken here, before the concurrent manual save.
		syncPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{usageItem("1")}}

		// Concurrent manual save: whole-row price with X changed to 7.
		manualPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{
			usageItem("7"),
			completionItem("2"),
		}}
		_, err := svc.SaveChannelModelPrices(txCtx, ch.ID, []SaveChannelModelPriceInput{{
			ModelID: "race-model",
			Price:   manualPrice,
		}})
		if err != nil {
			return err
		}

		// Sync now commits; it only reported item X, and the merge must run
		// against the row state the sync sees under the lock, not the snapshot.
		_, err = svc.updateChannelModelPriceEntity(txCtx, created[0], syncPrice, false, nil, time.Now().UTC())
		return err
	})
	require.NoError(t, err)

	price, err := client.ChannelModelPrice.Query().Where(
		channelmodelprice.ChannelID(ch.ID),
		channelmodelprice.ModelID("race-model"),
	).Only(ctx)
	require.NoError(t, err)
	items := lo.KeyBy(price.Price.Items, func(item objects.ModelPriceItem) objects.PriceItemCode {
		return item.ItemCode
	})
	// Item X is the reported code, so the sync value wins over the manual "7".
	require.Equal(t, "1", items[objects.PriceItemCodeUsage].Pricing.UsagePerUnit.String())
	// Item Y was not reported, so the concurrent manual save survives.
	require.Equal(t, "2", items[objects.PriceItemCodeCompletion].Pricing.UsagePerUnit.String())
}

// Same race but where the sync only reports item Y: the concurrent manual edit
// to item X must survive untouched.
func TestChannelService_UpdateChannelModelPriceEntityPreservesUnreportedItems(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Unreported item merge").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"race-model-2"}).
		SetDefaultTestModel("race-model-2").
		Save(ctx)
	require.NoError(t, err)

	created, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{{
		ModelID: "race-model-2",
		Price:   objects.ModelPrice{Items: []objects.ModelPriceItem{usageItem("1")}},
	}})
	require.NoError(t, err)
	require.Len(t, created, 1)

	err = svc.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Concurrent manual save changes item X to 7 before the sync update.
		_, err := svc.SaveChannelModelPrices(txCtx, ch.ID, []SaveChannelModelPriceInput{{
			ModelID: "race-model-2",
			Price:   objects.ModelPrice{Items: []objects.ModelPriceItem{usageItem("7")}},
		}})
		if err != nil {
			return err
		}

		// Sync reports only item Y; the locked row's item X must be preserved.
		syncPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{completionItem("5")}}
		_, err = svc.updateChannelModelPriceEntity(txCtx, created[0], syncPrice, false, nil, time.Now().UTC())
		return err
	})
	require.NoError(t, err)

	price, err := client.ChannelModelPrice.Query().Where(
		channelmodelprice.ChannelID(ch.ID),
		channelmodelprice.ModelID("race-model-2"),
	).Only(ctx)
	require.NoError(t, err)
	items := lo.KeyBy(price.Price.Items, func(item objects.ModelPriceItem) objects.PriceItemCode {
		return item.ItemCode
	})
	require.Len(t, items, 2)
	require.Equal(t, "7", items[objects.PriceItemCodeUsage].Pricing.UsagePerUnit.String())
	require.Equal(t, "5", items[objects.PriceItemCodeCompletion].Pricing.UsagePerUnit.String())
}

// Regression: the endpoint-sync update path must also fill library gaps for
// codes the endpoint did not report. A row with only a prompt rate, plus a
// prompt-only endpoint quote, must gain the library's completion rate (billing
// would otherwise charge zero for completion tokens).
func TestChannelService_EnsureChannelModelPrices_UpdateFillsLibraryGaps(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	ch := createAutoPriceTestChannel(t, ctx, client, "Library gap fill", []string{"gap-model"})

	createModelLibraryEntry(t, ctx, client, "gap-model", model.StatusEnabled, objects.ModelCardCost{
		Input:  1,
		Output: 2,
	})

	// Existing row carries only the prompt rate.
	created, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{{
		ModelID: "gap-model",
		Price:   objects.ModelPrice{Items: []objects.ModelPriceItem{usageItem("1")}},
	}})
	require.NoError(t, err)
	require.Len(t, created, 1)

	endpointTemplates := []channelModelPriceTemplate{{
		ModelID: "gap-model",
		Price:   objects.ModelPrice{Items: []objects.ModelPriceItem{usageItem("1")}},
	}}
	_, err = svc.ensureChannelModelPrices(ctx, ch.ID, []string{"gap-model"}, endpointTemplates)
	require.NoError(t, err)

	updated, err := client.ChannelModelPrice.Query().Where(
		channelmodelprice.ChannelID(ch.ID),
		channelmodelprice.ModelID("gap-model"),
	).Only(ctx)
	require.NoError(t, err)

	items := lo.KeyBy(updated.Price.Items, func(item objects.ModelPriceItem) objects.PriceItemCode {
		return item.ItemCode
	})
	// The endpoint value wins for the reported code; the library fills the gap.
	require.Equal(t, "1", items[objects.PriceItemCodeUsage].Pricing.UsagePerUnit.String())
	require.Equal(t, "2", items[objects.PriceItemCodeCompletion].Pricing.UsagePerUnit.String())
}

func loToDecimalPtr(s string) *decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return &d
}

func TestChannelService_DuplicateChannelCopiesModelPrices(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	source, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Source Channel").
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key1"}).
		SetSupportedModels([]string{"gpt-4", "gpt-4o"}).
		SetDefaultTestModel("gpt-4").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	price := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: loToDecimalPtr("0.01"),
				},
			},
		},
	}

	sourcePrices, err := svc.SaveChannelModelPrices(ctx, source.ID, []SaveChannelModelPriceInput{
		{ModelID: "gpt-4", Price: price},
	})
	require.NoError(t, err)
	require.Len(t, sourcePrices, 1)

	duplicated, err := svc.DuplicateChannel(ctx, source.ID, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		Name:             "Source Channel (1)",
		Credentials:      objects.ChannelCredentials{APIKey: "key2"},
		SupportedModels:  []string{"gpt-4", "gpt-4o"},
		DefaultTestModel: "gpt-4",
	})
	require.NoError(t, err)

	copiedPrices, err := client.ChannelModelPrice.Query().
		Where(channelmodelprice.ChannelID(duplicated.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, copiedPrices, 1)

	copiedPrice := copiedPrices[0]
	require.Equal(t, "gpt-4", copiedPrice.ModelID)
	require.Equal(t, price, copiedPrice.Price)
	require.NotEqual(t, sourcePrices[0].ReferenceID, copiedPrice.ReferenceID)
	require.Len(t, copiedPrice.ReferenceID, 8)

	copiedVersion, err := client.ChannelModelPriceVersion.Query().
		Where(channelmodelpriceversion.ChannelModelPriceID(copiedPrice.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, duplicated.ID, copiedVersion.ChannelID)
	require.Equal(t, "gpt-4", copiedVersion.ModelID)
	require.Equal(t, price, copiedVersion.Price)
	require.Equal(t, channelmodelpriceversion.StatusActive, copiedVersion.Status)
	require.Nil(t, copiedVersion.EffectiveEndAt)
	require.Equal(t, copiedPrice.ReferenceID, copiedVersion.ReferenceID)
}

func TestCalculatePriceChanges(t *testing.T) {
	price1 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{ItemCode: objects.PriceItemCodeUsage},
		},
	}
	price2 := objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{ItemCode: objects.PriceItemCodeCompletion},
		},
	}

	existingPrices := []*ent.ChannelModelPrice{
		{
			ID:      1,
			ModelID: "gpt-4",
			Price:   price1,
		},
	}

	tests := []struct {
		name           string
		existingPrices []*ent.ChannelModelPrice
		inputs         []SaveChannelModelPriceInput
		want           []PriceChangeAction
	}{
		{
			name:           "create and update",
			existingPrices: existingPrices,
			inputs: []SaveChannelModelPriceInput{
				{
					ModelID: "gpt-4",
					Price:   price1,
				},
				{
					ModelID: "gpt-3.5-turbo",
					Price:   price2,
				},
			},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeSkip,
					ModelID:       "gpt-4",
					Price:         price1,
					ExistingPrice: existingPrices[0],
				},
				{
					Type:          ActionTypeCreate,
					ModelID:       "gpt-3.5-turbo",
					Price:         price2,
					ExistingPrice: nil,
				},
			},
		},
		{
			name:           "all create",
			existingPrices: []*ent.ChannelModelPrice{},
			inputs: []SaveChannelModelPriceInput{
				{
					ModelID: "gpt-4",
					Price:   price1,
				},
			},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeCreate,
					ModelID:       "gpt-4",
					Price:         price1,
					ExistingPrice: nil,
				},
			},
		},
		{
			name:           "all update",
			existingPrices: existingPrices,
			inputs: []SaveChannelModelPriceInput{
				{
					ModelID: "gpt-4",
					Price:   price2,
				},
			},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeUpdate,
					ModelID:       "gpt-4",
					Price:         price2,
					ExistingPrice: existingPrices[0],
				},
			},
		},
		{
			name:           "delete missing",
			existingPrices: existingPrices,
			inputs:         []SaveChannelModelPriceInput{},
			want: []PriceChangeAction{
				{
					Type:          ActionTypeDelete,
					ModelID:       "gpt-4",
					ExistingPrice: existingPrices[0],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePriceChanges(tt.existingPrices, tt.inputs)
			assert.Equal(t, tt.want, got)
		})
	}
}
