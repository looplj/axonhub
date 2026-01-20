package biz

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// ChannelOrderingItem represents a channel ordering update.
type ChannelOrderingItem struct {
	ID             int
	OrderingWeight int
}

// BulkUpdateChannelOrdering updates the ordering weight for multiple channels in a single transaction.
func (svc *ChannelService) BulkUpdateChannelOrdering(ctx context.Context, items []*ChannelOrderingItem) ([]*ent.Channel, error) {
	client := svc.entFromContext(ctx)

	updatedChannels := make([]*ent.Channel, 0, len(items))
	for _, update := range items {
		channel, err := client.Channel.
			UpdateOneID(update.ID).
			SetOrderingWeight(update.OrderingWeight).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update channel %d: %w", update.ID, err)
		}

		updatedChannels = append(updatedChannels, channel)
	}

	svc.asyncReloadChannels()

	return updatedChannels, nil
}

// BulkCreateChannelsInput represents input for bulk creating channels.
type BulkCreateChannelsInput struct {
	Type             channel.Type
	Name             string
	Tags             []string
	BaseURL          *string
	APIKeys          []string
	SupportedModels  []string
	DefaultTestModel string
	Policies         *objects.ChannelPolicies
	Settings         *objects.ChannelSettings
}

// BulkCreateChannels creates multiple channels with the same configuration but different API keys.
// Returns error if any channel creation fails (transaction will rollback).
func (svc *ChannelService) BulkCreateChannels(ctx context.Context, input BulkCreateChannelsInput) ([]*ent.Channel, error) {
	if len(input.APIKeys) == 0 {
		return nil, fmt.Errorf("no API keys provided")
	}

	if input.BaseURL == nil {
		return nil, fmt.Errorf("base URL is required")
	}

	if err := channel.TypeValidator(input.Type); err != nil {
		return nil, fmt.Errorf("invalid channel type '%s': %w", input.Type, err)
	}

	var createdChannels []*ent.Channel

	// Get all existing channel names to check for conflicts
	existingChannels, err := svc.entFromContext(ctx).Channel.Query().Select(channel.FieldName).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing channels: %w", err)
	}

	existingNames := lo.SliceToMap(existingChannels, func(ch *ent.Channel) (string, bool) {
		return ch.Name, true
	})

	// All channels use numbered format: "base - (1)", "base - (2)", etc.
	counter := 1

	tagsToUse := input.Tags
	if len(tagsToUse) == 0 {
		tagsToUse = []string{input.Name} // Use base name as tag (backward compatible)
	}

	for _, apiKey := range input.APIKeys {
		// Generate unique channel name with numbering
		channelName := fmt.Sprintf("%s - (%d)", input.Name, counter)
		// Find next available counter
		for existingNames[channelName] {
			counter++
			channelName = fmt.Sprintf("%s - (%d)", input.Name, counter)
		}

		counter++
		existingNames[channelName] = true

		// Create channel input
		createInput := ent.CreateChannelInput{
			Type:             input.Type,
			BaseURL:          input.BaseURL,
			Name:             channelName,
			Credentials:      &objects.ChannelCredentials{APIKey: apiKey},
			SupportedModels:  input.SupportedModels,
			Tags:             tagsToUse,
			DefaultTestModel: input.DefaultTestModel,
			Policies:         input.Policies,
			Settings:         input.Settings,
		}

		// Create the channel without reload
		ch, err := svc.createChannel(ctx, createInput)
		if err != nil {
			return nil, fmt.Errorf("failed to create channel '%s': %w", channelName, err)
		}

		createdChannels = append(createdChannels, ch)
	}

	// Reload channels once after all successful creations
	svc.asyncReloadChannels()

	return createdChannels, nil
}

func (svc *ChannelService) bulkUpdateChannelStatus(ctx context.Context, ids []int, status channel.Status, action string) error {
	if len(ids) == 0 {
		return nil
	}

	client := svc.entFromContext(ctx)

	// Verify all channels exist
	count, err := client.Channel.Query().
		Where(channel.IDIn(ids...)).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to query channels: %w", err)
	}

	if count != len(ids) {
		return fmt.Errorf("expected to find %d channels, but found %d", len(ids), count)
	}

	// Update status
	if _, err = client.Channel.Update().
		Where(channel.IDIn(ids...)).
		SetStatus(status).
		Save(ctx); err != nil {
		return fmt.Errorf("failed to %s channels: %w", action, err)
	}

	svc.asyncReloadChannels()

	return nil
}

// BulkArchiveChannels updates the status of multiple channels to archived.
func (svc *ChannelService) BulkArchiveChannels(ctx context.Context, ids []int) error {
	return svc.bulkUpdateChannelStatus(ctx, ids, channel.StatusArchived, "archive")
}

// BulkDisableChannels updates the status of multiple channels to disabled.
func (svc *ChannelService) BulkDisableChannels(ctx context.Context, ids []int) error {
	return svc.bulkUpdateChannelStatus(ctx, ids, channel.StatusDisabled, "disable")
}

// BulkEnableChannels updates the status of multiple channels to enabled.
func (svc *ChannelService) BulkEnableChannels(ctx context.Context, ids []int) error {
	return svc.bulkUpdateChannelStatus(ctx, ids, channel.StatusEnabled, "enable")
}

// BulkDeleteChannels deletes multiple channels by their IDs.
func (svc *ChannelService) BulkDeleteChannels(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	deleted, err := svc.entFromContext(ctx).Channel.Delete().Where(channel.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to bulk delete channels: %w", err)
	}

	log.Info(ctx, "bulk deleted channels", log.Int("count", deleted))
	svc.asyncReloadChannels()

	return nil
}

// BulkImportChannelItem represents a single channel to be imported.
type BulkImportChannelItem struct {
	Type             string
	Name             string
	BaseURL          *string
	APIKey           *string
	SupportedModels  []string
	DefaultTestModel string
}

// BulkImportChannelsResult represents the result of bulk importing channels.
type BulkImportChannelsResult struct {
	Success  bool
	Created  int
	Failed   int
	Errors   []string
	Channels []*ent.Channel
}

// BulkImportChannels imports multiple channels at once.
func (svc *ChannelService) BulkImportChannels(ctx context.Context, items []*BulkImportChannelItem) (*BulkImportChannelsResult, error) {
	var (
		createdChannels []*ent.Channel
		errors          []string
	)

	created := 0
	failed := 0

	for i, item := range items {
		// Validate channel type
		channelType := channel.Type(item.Type)
		if err := channel.TypeValidator(channelType); err != nil {
			errors = append(errors, fmt.Sprintf("Row %d: Invalid channel type '%s'", i+1, item.Type))
			failed++

			continue
		}

		// Validate required fields
		if item.BaseURL == nil || *item.BaseURL == "" {
			errors = append(errors, fmt.Sprintf("Row %d (%s): Base URL is required", i+1, item.Name))
			failed++

			continue
		}

		if item.APIKey == nil || *item.APIKey == "" {
			errors = append(errors, fmt.Sprintf("Row %d (%s): API Key is required", i+1, item.Name))
			failed++

			continue
		}

		// Prepare credentials (API key is now required)
		credentials := &objects.ChannelCredentials{
			APIKey: *item.APIKey,
		}

		// Create the channel (baseURL is now required)
		channelBuilder := svc.entFromContext(ctx).Channel.Create().
			SetType(channelType).
			SetName(item.Name).
			SetBaseURL(*item.BaseURL).
			SetCredentials(credentials).
			SetSupportedModels(item.SupportedModels).
			SetDefaultTestModel(item.DefaultTestModel)

		ch, err := channelBuilder.Save(ctx)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Row %d (%s): %s", i+1, item.Name, err.Error()))
			failed++

			continue
		}

		createdChannels = append(createdChannels, ch)
		created++
	}

	success := failed == 0
	result := &BulkImportChannelsResult{
		Success:  success,
		Created:  created,
		Failed:   failed,
		Errors:   errors,
		Channels: createdChannels,
	}

	svc.asyncReloadChannels()

	return result, nil
}
