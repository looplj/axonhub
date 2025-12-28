package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// syncChannelModels syncs supported models for all channels with auto_sync_supported_models enabled.
// This function is called periodically (every hour) to keep model lists up to date.
func (svc *ChannelService) syncChannelModels(ctx context.Context) {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
		if err := svc.syncChannelModelsForChannel(ctx, ch); err != nil {
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
func (svc *ChannelService) syncChannelModelsForChannel(ctx context.Context, ch *ent.Channel) error {
	// Create a default HTTP client for model fetching
	httpClient := httpclient.NewHttpClient()
	modelFetcher := NewModelFetcher(httpClient, svc)

	result, err := modelFetcher.FetchModels(ctx, FetchModelsInput{
		ChannelType: ch.Type.String(),
		BaseURL:     ch.BaseURL,
		ChannelID:   lo.ToPtr(ch.ID),
	})
	if err != nil {
		return fmt.Errorf("failed to fetch models: %w", err)
	}

	// Check if there was an error in the result
	if result.Error != nil {
		return fmt.Errorf("model fetch returned error: %s", *result.Error)
	}

	// Extract model IDs
	modelIDs := lo.Map(result.Models, func(m ModelIdentify, _ int) string {
		return m.ID
	})

	if len(modelIDs) == 0 {
		log.Warn(ctx, "no models fetched for channel",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name))

		return nil
	}

	// Update channel's supported models
	ctx = privacy.DecisionContext(ctx, privacy.Allow)
	if err := svc.entFromContext(ctx).Channel.
		UpdateOneID(ch.ID).
		SetSupportedModels(modelIDs).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to update channel supported models: %w", err)
	}

	log.Info(ctx, "successfully synced models for channel",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
		log.Int("model_count", len(modelIDs)))

	return nil
}
