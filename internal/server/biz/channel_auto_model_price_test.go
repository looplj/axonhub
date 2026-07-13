package biz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/ent/hook"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

func TestModelCardToChannelModelPrice(t *testing.T) {
	tests := []struct {
		name      string
		card      *objects.ModelCard
		wantOK    bool
		wantCodes []objects.PriceItemCode
		wantCosts []string
	}{
		{
			name: "all supported costs preserve deterministic order",
			card: &objects.ModelCard{Cost: objects.ModelCardCost{
				Input:      1.25,
				Output:     2.5,
				CacheRead:  0.125,
				CacheWrite: 0.25,
			}},
			wantOK: true,
			wantCodes: []objects.PriceItemCode{
				objects.PriceItemCodeUsage,
				objects.PriceItemCodeCompletion,
				objects.PriceItemCodePromptCachedToken,
				objects.PriceItemCodeWriteCachedTokens,
			},
			wantCosts: []string{"1.25", "2.5", "0.125", "0.25"},
		},
		{
			name: "only positive costs are included",
			card: &objects.ModelCard{Cost: objects.ModelCardCost{
				Input:      1,
				Output:     0,
				CacheRead:  -1,
				CacheWrite: 0.5,
			}},
			wantOK: true,
			wantCodes: []objects.PriceItemCode{
				objects.PriceItemCodeUsage,
				objects.PriceItemCodeWriteCachedTokens,
			},
			wantCosts: []string{"1", "0.5"},
		},
		{
			name:   "nil model card is not priceable",
			card:   nil,
			wantOK: false,
		},
		{
			name: "non-positive model card is not priceable",
			card: &objects.ModelCard{Cost: objects.ModelCardCost{
				Input:      0,
				Output:     -1,
				CacheRead:  -2,
				CacheWrite: 0,
			}},
			wantOK: false,
		},
		{
			name: "non-finite costs are ignored while finite costs remain",
			card: &objects.ModelCard{Cost: objects.ModelCardCost{
				Input:      math.NaN(),
				Output:     2,
				CacheRead:  math.Inf(1),
				CacheWrite: math.Inf(-1),
			}},
			wantOK:    true,
			wantCodes: []objects.PriceItemCode{objects.PriceItemCodeCompletion},
			wantCosts: []string{"2"},
		},
		{
			name: "all non-finite costs are not priceable",
			card: &objects.ModelCard{Cost: objects.ModelCardCost{
				Input:      math.NaN(),
				Output:     math.Inf(1),
				CacheRead:  math.Inf(-1),
				CacheWrite: math.NaN(),
			}},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, ok := modelCardToChannelModelPrice(tt.card)
			require.Equal(t, tt.wantOK, ok)
			if !tt.wantOK {
				require.Empty(t, price.Items)

				return
			}

			require.Len(t, price.Items, len(tt.wantCodes))
			for i, item := range price.Items {
				require.Equal(t, tt.wantCodes[i], item.ItemCode)
				require.Equal(t, objects.PricingModeUsagePerUnit, item.Pricing.Mode)
				require.NotNil(t, item.Pricing.UsagePerUnit)
				require.Equal(t, tt.wantCosts[i], item.Pricing.UsagePerUnit.String())
			}
		})
	}
}

func TestChannelService_EnsureChannelModelPrices_EligibilityAndIdempotence(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	ch := createAutoPriceTestChannel(t, ctx, client, "Eligibility", []string{
		"enabled-model",
		"disabled-model",
		"archived-model",
		"no-cost-model",
		"missing-model",
		"case-model",
		"deleted-model",
	})

	createModelLibraryEntry(t, ctx, client, "enabled-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	createModelLibraryEntry(t, ctx, client, "disabled-model", model.StatusDisabled, objects.ModelCardCost{Output: 2})
	createModelLibraryEntry(t, ctx, client, "archived-model", model.StatusArchived, objects.ModelCardCost{Input: 3})
	createModelLibraryEntry(t, ctx, client, "no-cost-model", model.StatusEnabled, objects.ModelCardCost{})
	createModelLibraryEntry(t, ctx, client, "Case-Model", model.StatusEnabled, objects.ModelCardCost{Input: 4})
	deletedModel := createModelLibraryEntry(t, ctx, client, "deleted-model", model.StatusEnabled, objects.ModelCardCost{Input: 5})
	require.NoError(t, client.Model.DeleteOne(deletedModel).Exec(ctx))

	candidates := []string{
		"enabled-model",
		"disabled-model",
		"archived-model",
		"no-cost-model",
		"missing-model",
		"case-model",
		"deleted-model",
		"enabled-model",
		"",
	}
	_, err := svc.ensureChannelModelPrices(ctx, ch.ID, candidates)
	require.NoError(t, err)

	prices := queryChannelModelPrices(t, ctx, client, ch.ID)
	require.Len(t, prices, 2)
	require.ElementsMatch(t, []string{"enabled-model", "disabled-model"}, lo.Map(prices, func(price *ent.ChannelModelPrice, _ int) string {
		return price.ModelID
	}))

	versions, err := client.ChannelModelPriceVersion.Query().
		Where(channelmodelpriceversion.ChannelID(ch.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, versions, 2)

	before := lo.SliceToMap(prices, func(price *ent.ChannelModelPrice) (string, *ent.ChannelModelPrice) {
		return price.ModelID, price
	})
	_, err = svc.ensureChannelModelPrices(ctx, ch.ID, candidates)
	require.NoError(t, err)

	after := queryChannelModelPrices(t, ctx, client, ch.ID)
	require.Len(t, after, 2)
	for _, price := range after {
		require.Equal(t, before[price.ModelID].ID, price.ID)
		require.Equal(t, before[price.ModelID].ReferenceID, price.ReferenceID)
		require.Equal(t, before[price.ModelID].Price, price.Price)
	}

	versionCount, err := client.ChannelModelPriceVersion.Query().
		Where(channelmodelpriceversion.ChannelID(ch.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, versionCount)
}

func TestChannelService_CreateChannel_AutoFillsEligibleInitialModels(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "priced-model", model.StatusDisabled, objects.ModelCardCost{Input: 1, Output: 2})
	createModelLibraryEntry(t, ctx, client, "zero-model", model.StatusEnabled, objects.ModelCardCost{})

	ch, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Auto Price Create",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "key"},
		SupportedModels:  []string{"priced-model", "zero-model", "missing-model"},
		DefaultTestModel: "priced-model",
	})
	require.NoError(t, err)

	prices := queryChannelModelPrices(t, ctx, client, ch.ID)
	require.Len(t, prices, 1)
	require.Equal(t, "priced-model", prices[0].ModelID)

	version, err := client.ChannelModelPriceVersion.Query().
		Where(channelmodelpriceversion.ChannelModelPriceID(prices[0].ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, channelmodelpriceversion.StatusActive, version.Status)
	require.Equal(t, prices[0].ReferenceID, version.ReferenceID)
	require.Equal(t, prices[0].Price, version.Price)

	returnedPrices, err := ch.QueryChannelModelPrices().All(ctx)
	require.NoError(t, err)
	require.Len(t, returnedPrices, 1)
}

func TestChannelService_CreateChannel_DefersReloadForCallerOwnedTransaction(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "outer-tx-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})

	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() {
		asyncReloadDisabled = previousAsyncReloadDisabled
	})

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := ent.NewTxContext(ctx, tx)
	txCtx = ent.NewContext(txCtx, tx.Client())

	ch, err := svc.CreateChannel(txCtx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Outer transaction create",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "key"},
		SupportedModels:  []string{"outer-tx-model"},
		DefaultTestModel: "outer-tx-model",
	})
	require.NoError(t, err)
	require.Zero(t, notifier.notifyCount)

	prices, err := ch.QueryChannelModelPrices().All(txCtx)
	require.NoError(t, err)
	require.Len(t, prices, 1)

	require.NoError(t, tx.Commit())
	ch.Unwrap()
	prices, err = ch.QueryChannelModelPrices().All(ctx)
	require.NoError(t, err)
	require.Len(t, prices, 1)
	require.Equal(t, 1, notifier.notifyCount)

	rollbackTx, err := client.Tx(ctx)
	require.NoError(t, err)
	rollbackCtx := ent.NewTxContext(ctx, rollbackTx)
	rollbackCtx = ent.NewContext(rollbackCtx, rollbackTx.Client())
	_, err = svc.CreateChannel(rollbackCtx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Rolled-back outer transaction create",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "key"},
		SupportedModels:  []string{"outer-tx-model"},
		DefaultTestModel: "outer-tx-model",
	})
	require.NoError(t, err)
	require.NoError(t, rollbackTx.Rollback())
	require.Equal(t, 1, notifier.notifyCount)
}

func TestChannelService_EnsureChannelModelPrices_RecreatesSoftDeletedPrice(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "recreated-model", model.StatusEnabled, objects.ModelCardCost{Input: 3})
	ch := createAutoPriceTestChannel(t, ctx, client, "Recreate soft-deleted price", []string{"recreated-model"})

	customPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{{
		ItemCode: objects.PriceItemCodeUsage,
		Pricing: objects.Pricing{
			Mode:         objects.PricingModeUsagePerUnit,
			UsagePerUnit: loToDecimalPtr("9"),
		},
	}}}
	created, err := svc.SaveChannelModelPrices(ctx, ch.ID, []SaveChannelModelPriceInput{{
		ModelID: "recreated-model",
		Price:   customPrice,
	}})
	require.NoError(t, err)
	require.Len(t, created, 1)

	_, err = svc.SaveChannelModelPrices(ctx, ch.ID, nil)
	require.NoError(t, err)
	require.False(t, channelModelPriceExists(t, ctx, client, ch.ID, "recreated-model"))

	_, err = svc.ensureChannelModelPrices(ctx, ch.ID, []string{"recreated-model"})
	require.NoError(t, err)
	recreated := queryChannelModelPrice(t, ctx, client, ch.ID, "recreated-model")
	require.NotEqual(t, created[0].ID, recreated.ID)
	require.NotEqual(t, created[0].ReferenceID, recreated.ReferenceID)
	require.Equal(t, "3", recreated.Price.Items[0].Pricing.UsagePerUnit.String())

	versions, err := client.ChannelModelPriceVersion.Query().
		Where(
			channelmodelpriceversion.ChannelID(ch.ID),
			channelmodelpriceversion.ModelID("recreated-model"),
		).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	require.ElementsMatch(t, []channelmodelpriceversion.Status{
		channelmodelpriceversion.StatusArchived,
		channelmodelpriceversion.StatusActive,
	}, lo.Map(versions, func(version *ent.ChannelModelPriceVersion, _ int) channelmodelpriceversion.Status {
		return version.Status
	}))
}

func TestChannelService_UpdateChannel_RetriesSupportedModelsAndPreservesExistingPrices(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "existing-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})

	ch, err := svc.CreateChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Auto Price Update",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "key"},
		SupportedModels:  []string{"existing-model", "late-library-model"},
		DefaultTestModel: "existing-model",
	})
	require.NoError(t, err)

	existingPrice := queryChannelModelPrice(t, ctx, client, ch.ID, "existing-model")
	existingVersionCount := countChannelModelPriceVersions(t, ctx, client, existingPrice.ID)

	createModelLibraryEntry(t, ctx, client, "late-library-model", model.StatusEnabled, objects.ModelCardCost{Input: 2})
	createModelLibraryEntry(t, ctx, client, "added-model", model.StatusEnabled, objects.ModelCardCost{Output: 3})

	_, err = svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{Name: lo.ToPtr("Auto Price Update Renamed")})
	require.NoError(t, err)
	require.False(t, channelModelPriceExists(t, ctx, client, ch.ID, "late-library-model"))

	updated, err := svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{
		SupportedModels: []string{"existing-model", "late-library-model", "added-model"},
	})
	require.NoError(t, err)
	require.True(t, channelModelPriceExists(t, ctx, client, ch.ID, "added-model"))
	require.True(t, channelModelPriceExists(t, ctx, client, ch.ID, "late-library-model"))
	returnedPrices, err := updated.QueryChannelModelPrices().All(ctx)
	require.NoError(t, err)
	require.Len(t, returnedPrices, 3)

	_, err = svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{
		SupportedModels: []string{"late-library-model", "added-model"},
	})
	require.NoError(t, err)
	require.True(t, channelModelPriceExists(t, ctx, client, ch.ID, "existing-model"))

	_, err = svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{
		SupportedModels: []string{"existing-model", "late-library-model", "added-model"},
	})
	require.NoError(t, err)

	preserved := queryChannelModelPrice(t, ctx, client, ch.ID, "existing-model")
	require.Equal(t, existingPrice.ID, preserved.ID)
	require.Equal(t, existingPrice.ReferenceID, preserved.ReferenceID)
	require.Equal(t, existingPrice.Price, preserved.Price)
	require.Equal(t, existingVersionCount, countChannelModelPriceVersions(t, ctx, client, preserved.ID))
}

func TestChannelService_DuplicateChannel_PreservesSourcePriceAndFillsGaps(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "custom-model", model.StatusEnabled, objects.ModelCardCost{Input: 9})
	createModelLibraryEntry(t, ctx, client, "gap-model", model.StatusDisabled, objects.ModelCardCost{Input: 2, Output: 4})
	createModelLibraryEntry(t, ctx, client, "no-cost-model", model.StatusEnabled, objects.ModelCardCost{})

	source := createAutoPriceTestChannel(t, ctx, client, "Duplicate Source", []string{"custom-model", "gap-model", "no-cost-model"})
	customPrice := objects.ModelPrice{Items: []objects.ModelPriceItem{
		{
			ItemCode: objects.PriceItemCodeUsage,
			Pricing: objects.Pricing{
				Mode:         objects.PricingModeUsagePerUnit,
				UsagePerUnit: loToDecimalPtr("42"),
			},
		},
	}}
	sourcePrices, err := svc.SaveChannelModelPrices(ctx, source.ID, []SaveChannelModelPriceInput{
		{ModelID: "custom-model", Price: customPrice},
	})
	require.NoError(t, err)
	require.Len(t, sourcePrices, 1)

	duplicated, err := svc.DuplicateChannel(ctx, source.ID, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Duplicate Target",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		Credentials:      objects.ChannelCredentials{APIKey: "target-key"},
		SupportedModels:  []string{"custom-model", "gap-model", "no-cost-model"},
		DefaultTestModel: "custom-model",
	})
	require.NoError(t, err)

	prices := queryChannelModelPrices(t, ctx, client, duplicated.ID)
	require.Len(t, prices, 2)

	copiedCustom := queryChannelModelPrice(t, ctx, client, duplicated.ID, "custom-model")
	require.Equal(t, customPrice, copiedCustom.Price)
	require.NotEqual(t, sourcePrices[0].ReferenceID, copiedCustom.ReferenceID)
	require.Equal(t, 1, countChannelModelPriceVersions(t, ctx, client, copiedCustom.ID))

	gap := queryChannelModelPrice(t, ctx, client, duplicated.ID, "gap-model")
	require.Len(t, gap.Price.Items, 2)
	require.Equal(t, objects.PriceItemCodeUsage, gap.Price.Items[0].ItemCode)
	require.Equal(t, "2", gap.Price.Items[0].Pricing.UsagePerUnit.String())
	require.Equal(t, objects.PriceItemCodeCompletion, gap.Price.Items[1].ItemCode)
	require.Equal(t, "4", gap.Price.Items[1].Pricing.UsagePerUnit.String())
	require.Equal(t, 1, countChannelModelPriceVersions(t, ctx, client, gap.ID))
	require.False(t, channelModelPriceExists(t, ctx, client, duplicated.ID, "no-cost-model"))
	returnedPrices, err := duplicated.QueryChannelModelPrices().All(ctx)
	require.NoError(t, err)
	require.Len(t, returnedPrices, 2)
}

func TestChannelService_UpdateChannel_RollsBackModelsWhenPriceVersionFails(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "rollback-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	ch := createAutoPriceTestChannel(t, ctx, client, "Update rollback", []string{"existing-model"})

	errInjected := errors.New("injected update price version failure")
	client.ChannelModelPriceVersion.Use(func(next ent.Mutator) ent.Mutator {
		return hook.ChannelModelPriceVersionFunc(func(ctx context.Context, mutation *ent.ChannelModelPriceVersionMutation) (ent.Value, error) {
			modelID, _ := mutation.ModelID()
			if mutation.Op() == ent.OpCreate && modelID == "rollback-model" {
				return nil, errInjected
			}

			return next.Mutate(ctx, mutation)
		})
	})

	updated, err := svc.UpdateChannel(ctx, ch.ID, &ent.UpdateChannelInput{
		SupportedModels: []string{"existing-model", "rollback-model"},
	})
	require.ErrorIs(t, err, errInjected)
	require.Nil(t, updated)

	persisted, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"existing-model"}, persisted.SupportedModels)
	require.Equal(t, 0, countAllChannelModelPrices(t, ctx, client))
	require.Equal(t, 0, countAllChannelModelPriceVersions(t, ctx, client))
}

func TestChannelService_BulkCreateChannels_RollsBackWholeBatchWhenPriceVersionFails(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "bulk-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})

	errInjected := errors.New("injected channel model price version failure")
	var versionCreates atomic.Int32
	client.ChannelModelPriceVersion.Use(func(next ent.Mutator) ent.Mutator {
		return hook.ChannelModelPriceVersionFunc(func(ctx context.Context, mutation *ent.ChannelModelPriceVersionMutation) (ent.Value, error) {
			if mutation.Op() == ent.OpCreate && versionCreates.Add(1) == 2 {
				return nil, errInjected
			}

			return next.Mutate(ctx, mutation)
		})
	})

	channels, err := svc.BulkCreateChannels(ctx, BulkCreateChannelsInput{
		Type:             channel.TypeOpenai,
		Name:             "Bulk Rollback",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		APIKeys:          []string{"key-1", "key-2", "key-3"},
		SupportedModels:  []string{"bulk-model"},
		DefaultTestModel: "bulk-model",
	})
	require.ErrorIs(t, err, errInjected)
	require.Empty(t, channels)
	require.Equal(t, int32(2), versionCreates.Load())
	require.Equal(t, 0, countAllChannels(t, ctx, client))
	require.Equal(t, 0, countAllChannelModelPrices(t, ctx, client))
	require.Equal(t, 0, countAllChannelModelPriceVersions(t, ctx, client))
}

func TestChannelService_BulkCreateChannels_ReturnsQueryableEntities(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "bulk-query-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})

	channels, err := svc.BulkCreateChannels(ctx, BulkCreateChannelsInput{
		Type:             channel.TypeOpenai,
		Name:             "Bulk Queryable",
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		APIKeys:          []string{"key-1", "key-2"},
		SupportedModels:  []string{"bulk-query-model"},
		DefaultTestModel: "bulk-query-model",
	})
	require.NoError(t, err)
	require.Len(t, channels, 2)

	for _, ch := range channels {
		prices, err := ch.QueryChannelModelPrices().All(ctx)
		require.NoError(t, err)
		require.Len(t, prices, 1)
	}
}

func TestChannelService_BulkImportChannels_RollsBackOnlyFailingItem(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	ctx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, ctx, client, "import-good-1", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	createModelLibraryEntry(t, ctx, client, "import-fail", model.StatusEnabled, objects.ModelCardCost{Input: 2})
	createModelLibraryEntry(t, ctx, client, "import-good-2", model.StatusEnabled, objects.ModelCardCost{Input: 3})

	errInjected := errors.New("injected import price version failure")
	client.ChannelModelPriceVersion.Use(func(next ent.Mutator) ent.Mutator {
		return hook.ChannelModelPriceVersionFunc(func(ctx context.Context, mutation *ent.ChannelModelPriceVersionMutation) (ent.Value, error) {
			modelID, _ := mutation.ModelID()
			if mutation.Op() == ent.OpCreate && modelID == "import-fail" {
				return nil, errInjected
			}

			return next.Mutate(ctx, mutation)
		})
	})

	items := []*BulkImportChannelItem{
		newBulkImportAutoPriceTestItem("Import Good 1", "import-good-1"),
		newBulkImportAutoPriceTestItem("Import Failure", "import-fail"),
		newBulkImportAutoPriceTestItem("Import Good 2", "import-good-2"),
	}
	result, err := svc.BulkImportChannels(ctx, items)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Success)
	require.Equal(t, 2, result.Created)
	require.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	require.Contains(t, result.Errors[0], errInjected.Error())
	require.Len(t, result.Channels, 2)
	require.ElementsMatch(t, []string{"Import Good 1", "Import Good 2"}, lo.Map(result.Channels, func(ch *ent.Channel, _ int) string {
		return ch.Name
	}))

	require.Equal(t, 2, countAllChannels(t, ctx, client))
	require.Equal(t, 2, countAllChannelModelPrices(t, ctx, client))
	require.Equal(t, 2, countAllChannelModelPriceVersions(t, ctx, client))

	failureExists, err := client.Channel.Query().Where(channel.Name("Import Failure")).Exist(ctx)
	require.NoError(t, err)
	require.False(t, failureExists)

	prices, err := client.ChannelModelPrice.Query().All(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"import-good-1", "import-good-2"}, lo.Map(prices, func(price *ent.ChannelModelPrice, _ int) string {
		return price.ModelID
	}))
	for _, ch := range result.Channels {
		returnedPrices, err := ch.QueryChannelModelPrices().All(ctx)
		require.NoError(t, err)
		require.Len(t, returnedPrices, 1)
	}
}

func TestChannelService_AutoPricingSupportsWriteOnlyChannelMutations(t *testing.T) {
	svc, client := setupTestChannelService(t)
	defer client.Close()

	setupCtx := channelAutoPriceTestContext(client)
	createModelLibraryEntry(t, setupCtx, client, "write-only-model", model.StatusEnabled, objects.ModelCardCost{Input: 1})
	channelToUpdate := createAutoPriceTestChannel(t, setupCtx, client, "Write-only update", []string{"old-model"})

	writeCtx := authz.NewUserContext(ent.NewContext(context.Background(), client), 42)
	writeCtx = contexts.WithUser(writeCtx, &ent.User{
		ID:     42,
		Scopes: []string{"write_channels"},
	})

	updated, err := svc.UpdateChannel(writeCtx, channelToUpdate.ID, &ent.UpdateChannelInput{
		SupportedModels: []string{"old-model", "write-only-model"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"old-model", "write-only-model"}, updated.SupportedModels)
	require.True(t, channelModelPriceExists(t, setupCtx, client, channelToUpdate.ID, "write-only-model"))

	result, err := svc.BulkImportChannels(writeCtx, []*BulkImportChannelItem{
		newBulkImportAutoPriceTestItem("Write-only import", "write-only-model"),
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 1, result.Created)
	require.Len(t, result.Channels, 1)
	require.True(t, channelModelPriceExists(t, setupCtx, client, result.Channels[0].ID, "write-only-model"))
}

func channelAutoPriceTestContext(client *ent.Client) context.Context {
	ctx := ent.NewContext(context.Background(), client)

	return authz.WithTestBypass(ctx)
}

func createModelLibraryEntry(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	modelID string,
	status model.Status,
	cost objects.ModelCardCost,
) *ent.Model {
	t.Helper()

	entity, err := client.Model.Create().
		SetDeveloper("test").
		SetModelID(modelID).
		SetName("Test " + modelID).
		SetIcon("test").
		SetGroup("test").
		SetModelCard(&objects.ModelCard{Cost: cost}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)

	return entity
}

func createAutoPriceTestChannel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	name string,
	supportedModels []string,
) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels(supportedModels).
		SetDefaultTestModel(supportedModels[0]).
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

func queryChannelModelPrices(t *testing.T, ctx context.Context, client *ent.Client, channelID int) []*ent.ChannelModelPrice {
	t.Helper()

	prices, err := client.ChannelModelPrice.Query().
		Where(channelmodelprice.ChannelID(channelID)).
		All(ctx)
	require.NoError(t, err)

	return prices
}

func queryChannelModelPrice(t *testing.T, ctx context.Context, client *ent.Client, channelID int, modelID string) *ent.ChannelModelPrice {
	t.Helper()

	price, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(channelID),
			channelmodelprice.ModelID(modelID),
		).
		Only(ctx)
	require.NoError(t, err)

	return price
}

func channelModelPriceExists(t *testing.T, ctx context.Context, client *ent.Client, channelID int, modelID string) bool {
	t.Helper()

	exists, err := client.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(channelID),
			channelmodelprice.ModelID(modelID),
		).
		Exist(ctx)
	require.NoError(t, err)

	return exists
}

func countChannelModelPriceVersions(t *testing.T, ctx context.Context, client *ent.Client, priceID int) int {
	t.Helper()

	count, err := client.ChannelModelPriceVersion.Query().
		Where(channelmodelpriceversion.ChannelModelPriceID(priceID)).
		Count(ctx)
	require.NoError(t, err)

	return count
}

func countAllChannels(t *testing.T, ctx context.Context, client *ent.Client) int {
	t.Helper()

	count, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)

	return count
}

func countAllChannelModelPrices(t *testing.T, ctx context.Context, client *ent.Client) int {
	t.Helper()

	count, err := client.ChannelModelPrice.Query().Count(ctx)
	require.NoError(t, err)

	return count
}

func countAllChannelModelPriceVersions(t *testing.T, ctx context.Context, client *ent.Client) int {
	t.Helper()

	count, err := client.ChannelModelPriceVersion.Query().Count(ctx)
	require.NoError(t, err)

	return count
}

func newBulkImportAutoPriceTestItem(name, modelID string) *BulkImportChannelItem {
	return &BulkImportChannelItem{
		Type:             channel.TypeOpenai.String(),
		Name:             name,
		BaseURL:          lo.ToPtr("https://api.openai.com/v1"),
		APIKey:           lo.ToPtr(fmt.Sprintf("key-%s", modelID)),
		SupportedModels:  []string{modelID},
		DefaultTestModel: modelID,
	}
}
