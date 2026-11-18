package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/internal/scopes"
)

// syncChannelModels syncs supported models for all channels with auto_sync_supported_models enabled.
// This function is called periodically (every hour) to keep model lists up to date.
func (svc *ChannelService) syncChannelModels(ctx context.Context) {
	// Query all enabled channels with auto_sync_supported_models = true
	channels, err := svc.entFromContext(ctx).Channel.
		Query().
		Where(
			channel.StatusEQ(channel.StatusEnabled),
			channel.AutoSyncSupportedModelsEQ(true),
		).
		All(ctx)
	if err != nil {
		log.Error(ctx, "failed to query channels for model sync", log.Cause(err))
		return
	}

	if len(channels) == 0 {
		log.Debug(ctx, "no channels with auto_sync_supported_models enabled")
		return
	}

	log.Info(ctx, "starting model sync for channels", log.Int("count", len(channels)))

	successCount := 0
	failureCount := 0
	changedCount := 0

	for _, ch := range channels {
		_, changed, err := svc.syncChannelModelsForChannel(ctx, ch, nil)
		if err != nil {
			log.Warn(ctx, "failed to sync models for channel",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", ch.Name),
				log.Cause(err))

			failureCount++
		} else {
			successCount++
			if changed {
				changedCount++
			}
		}
	}

	if changedCount > 0 {
		svc.asyncReloadChannels()
	}

	log.Info(ctx, "completed model sync for channels",
		log.Int("success", successCount),
		log.Int("failure", failureCount),
		log.Int("changed", changedCount))
}

// syncChannelModelsForChannel syncs supported models for a single channel.
func (svc *ChannelService) syncChannelModelsForChannel(ctx context.Context, ch *ent.Channel, patternOverride *string) (*ent.Channel, bool, error) {
	fetchType := ch.Type
	fetchBaseURL := ch.BaseURL
	modelFetcher := NewModelFetcher(svc.httpClient, svc)

	result, err := modelFetcher.FetchModels(ctx, FetchModelsInput{
		ChannelType: fetchType.String(),
		BaseURL:     fetchBaseURL,
		ChannelID:   lo.ToPtr(ch.ID),
	})
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch models: %w", err)
	}

	// Check if there was an error in the result
	if result.Error != nil {
		return nil, false, fmt.Errorf("model fetch returned error: %s", *result.Error)
	}
	if result.Fallback {
		return nil, false, fmt.Errorf("model fetch returned fallback models")
	}

	pricedModels := make(map[string]*objects.ModelPrice)
	for _, fetchedModel := range result.Models {
		if fetchedModel.Pricing != nil {
			pricedModels[fetchedModel.ID] = fetchedModel.Pricing
		}
	}

	// Extract model IDs from fetched models
	fetchedModelIDs := lo.Map(result.Models, func(m ModelIdentify, _ int) string {
		return m.ID
	})

	pattern := ch.AutoSyncModelPattern
	if patternOverride != nil {
		pattern = *patternOverride
	}

	// Filter by auto_sync_model_pattern if set
	if pattern != "" {
		if err := xregexp.ValidateRegex(pattern); err != nil {
			log.Warn(ctx, "invalid auto_sync_model_pattern, skipping filter",
				log.Int("channel_id", ch.ID),
				log.String("pattern", pattern),
				log.Cause(err))
		} else {
			before := len(fetchedModelIDs)
			fetchedModelIDs = xregexp.Filter(fetchedModelIDs, pattern)
			log.Info(ctx, "filtered models by pattern",
				log.Int("channel_id", ch.ID),
				log.String("pattern", pattern),
				log.Int("before", before),
				log.Int("after", len(fetchedModelIDs)))
		}
	}

	var (
		updatedCh     *ent.Channel
		changed       bool
		modelsChanged bool
		manualCount   int
		totalCount    int
	)

	err = svc.RunInTransaction(ctx, func(ctx context.Context) error {
		db := svc.entFromContext(ctx)

		// Re-read the channel inside the transaction: the provider fetch above can
		// take seconds, so manual models saved meanwhile must not be overwritten by
		// the stale snapshot passed into this function.
		current, err := db.Channel.Get(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("failed to reload channel for model sync: %w", err)
		}

		updatedCh = current

		manualModels := current.ManualModels
		if manualModels == nil {
			manualModels = []string{}
		}
		manualCount = len(manualModels)

		// Merge fetched models with manual models, removing duplicates
		mergedModels := lo.Uniq(append(manualModels, fetchedModelIDs...))
		totalCount = len(mergedModels)

		if len(mergedModels) == 0 {
			log.Warn(ctx, "no models to sync for channel (both fetched and manual are empty)",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", ch.Name))

			return nil
		}

		addedModels := lo.Without(mergedModels, current.SupportedModels...)
		removedModels := lo.Without(current.SupportedModels, mergedModels...)
		modelsChanged = len(addedModels) > 0 || len(removedModels) > 0
		modelProtocolsChanged := modelsChanged && RemoveRemovedModelProtocolOverrides(current.Settings, mergedModels)

		if modelsChanged {
			update := db.Channel.
				UpdateOneID(ch.ID).
				SetSupportedModels(mergedModels)
			if modelProtocolsChanged {
				update.SetSettings(current.Settings)
			}
			updated, err := update.Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to update channel supported models: %w", err)
			}

			updatedCh = updated
		}

		var (
			endpointPricesChanged bool
			endpointTemplates     []channelModelPriceTemplate
		)
		if current.Type != fetchType || current.BaseURL != fetchBaseURL {
			log.Info(ctx, "endpoint price sync skipped, provider identity changed",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", current.Name),
			)
		} else {
			endpointPricesChanged, endpointTemplates, err = svc.syncChannelModelPricesFromEndpoint(ctx, current, pricedModels, mergedModels)
			if err != nil {
				return err
			}
		}

		pricesChanged, err := svc.ensureChannelModelPrices(ctx, ch.ID, mergedModels, endpointTemplates)
		if err != nil {
			return err
		}

		changed = modelsChanged || modelProtocolsChanged || pricesChanged || endpointPricesChanged

		return nil
	})
	if err != nil {
		return nil, false, err
	}
	if ent.TxFromContext(ctx) == nil && updatedCh != nil {
		updatedCh.Unwrap()
	}

	log.Info(ctx, "successfully synced models for channel",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
		log.Int("fetched_count", len(fetchedModelIDs)),
		log.Int("manual_count", manualCount),
		log.Int("total_count", totalCount))

	return updatedCh, changed, nil
}

func (svc *ChannelService) syncChannelModelPricesFromEndpoint(
	ctx context.Context,
	ch *ent.Channel,
	pricedModels map[string]*objects.ModelPrice,
	mergedModels []string,
) (bool, []channelModelPriceTemplate, error) {
	if !supportsEndpointPricing(ch.Type) || len(pricedModels) == 0 || ch.Settings == nil || !ch.Settings.SyncModelPrices {
		return false, nil, nil
	}

	modelIDs := make([]string, 0, len(pricedModels))
	for _, modelID := range uniqueNonEmptyModelIDs(mergedModels) {
		if pricedModels[modelID] != nil {
			modelIDs = append(modelIDs, modelID)
		}
	}
	if len(modelIDs) == 0 {
		return false, nil, nil
	}

	db := svc.entFromContext(ctx)
	existingPrices := make(map[string]*ent.ChannelModelPrice, len(modelIDs))
	for start := 0; start < len(modelIDs); start += channelModelPriceQueryBatchSize {
		end := min(start+channelModelPriceQueryBatchSize, len(modelIDs))
		prices, err := authz.RunWithScopeDecision(ctx, scopes.ScopeWriteChannels, func(queryCtx context.Context) ([]*ent.ChannelModelPrice, error) {
			return db.ChannelModelPrice.Query().
				Where(
					channelmodelprice.ChannelID(ch.ID),
					channelmodelprice.ModelIDIn(modelIDs[start:end]...),
				).
				All(queryCtx)
		})
		if err != nil {
			return false, nil, fmt.Errorf("failed to query existing channel model prices: %w", err)
		}
		for _, price := range prices {
			existingPrices[price.ModelID] = price
		}
	}

	var (
		created   int
		updated   int
		skipped   int
		changed   bool
		templates = make([]channelModelPriceTemplate, 0, len(modelIDs))
	)
	for _, modelID := range modelIDs {
		endpointPrice := *pricedModels[modelID]
		existing, ok := existingPrices[modelID]
		if !ok {
			templates = append(templates, channelModelPriceTemplate{ModelID: modelID, Price: endpointPrice})
			created++
			changed = true
			continue
		}

		mergedPrice := mergeChannelModelPriceItems(existing.Price, endpointPrice, true)
		templates = append(templates, channelModelPriceTemplate{ModelID: modelID, Price: mergedPrice})
		if existing.Price.Equals(mergedPrice) {
			skipped++
			continue
		}
		updated++
		changed = true
	}

	log.Info(ctx, "synced endpoint model prices",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
		log.Int("created", created),
		log.Int("updated", updated),
		log.Int("skipped", skipped))

	return changed, templates, nil
}

func (svc *ChannelService) SyncChannelModels(ctx context.Context, channelID int, patternOverride *string) (*ent.Channel, error) {
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	updated, changed, err := svc.syncChannelModelsForChannel(ctx, ch, patternOverride)
	if err != nil {
		return nil, err
	}

	if changed {
		svc.reloadChannelsAfterCommit(ctx)
	}

	return updated, nil
}
