package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xregexp"
	"github.com/looplj/axonhub/llm/httpclient"
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

	for _, ch := range channels {
		if _, err := svc.syncChannelModelsForChannel(ctx, ch, nil); err != nil {
			log.Warn(ctx, "failed to sync models for channel",
				log.Int("channel_id", ch.ID),
				log.String("channel_name", ch.Name),
				log.Cause(err))

			failureCount++
		} else {
			successCount++
		}
	}

	log.Info(ctx, "completed model sync for channels",
		log.Int("success", successCount),
		log.Int("failure", failureCount))
}

// syncChannelModelsForChannel syncs supported models for a single channel.
func (svc *ChannelService) syncChannelModelsForChannel(ctx context.Context, ch *ent.Channel, patternOverride *string) (*ent.Channel, error) {
	httpClient := httpclient.NewHttpClient()
	modelFetcher := NewModelFetcher(httpClient, svc)

	result, err := modelFetcher.FetchModels(ctx, FetchModelsInput{
		ChannelType: ch.Type.String(),
		BaseURL:     ch.BaseURL,
		ChannelID:   lo.ToPtr(ch.ID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}

	// Check if there was an error in the result
	if result.Error != nil {
		return nil, fmt.Errorf("model fetch returned error: %s", *result.Error)
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

	// Read existing manual models from the channel
	manualModels := ch.ManualModels
	if manualModels == nil {
		manualModels = []string{}
	}

	// Merge fetched models with manual models, removing duplicates
	mergedModels := lo.Uniq(append(manualModels, fetchedModelIDs...))

	if len(mergedModels) == 0 {
		log.Warn(ctx, "no models to sync for channel (both fetched and manual are empty)",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name))

		return ch, nil
	}

	// Update channel's supported models with merged list
	// Keep manual_models unchanged (preserve user's manually added models)
	updatedCh, err := svc.entFromContext(ctx).Channel.
		UpdateOneID(ch.ID).
		SetSupportedModels(mergedModels).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update channel supported models: %w", err)
	}

	log.Info(ctx, "successfully synced models for channel",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
		log.Int("fetched_count", len(fetchedModelIDs)),
		log.Int("manual_count", len(manualModels)),
		log.Int("total_count", len(mergedModels)))

	return updatedCh, nil
}

func (svc *ChannelService) SyncChannelModels(ctx context.Context, channelID int, patternOverride *string) (*ent.Channel, error) {
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return svc.syncChannelModelsForChannel(ctx, ch, patternOverride)
}
