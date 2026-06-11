package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// DuplicateChannel creates a new channel from input and copies current model prices
// from the source channel in the same transaction.
func (svc *ChannelService) DuplicateChannel(ctx context.Context, sourceID int, input ent.CreateChannelInput) (*ent.Channel, error) {
	var duplicated *ent.Channel

	err := svc.RunInTransaction(ctx, func(ctx context.Context) error {
		db := svc.entFromContext(ctx)

		if _, err := db.Channel.Get(ctx, sourceID); err != nil {
			return fmt.Errorf("failed to get source channel: %w", err)
		}

		existing, err := db.Channel.Query().
			Where(channel.Name(input.Name)).
			First(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("failed to check channel name: %w", err)
		}

		if existing != nil {
			return xerrors.DuplicateNameError("channel", input.Name)
		}

		ch, err := svc.createChannel(ctx, input)
		if err != nil {
			return err
		}

		prices, err := db.ChannelModelPrice.Query().
			Where(
				channelmodelprice.ChannelID(sourceID),
			).
			All(ctx)
		if err != nil {
			return fmt.Errorf("failed to query source channel model prices: %w", err)
		}

		now := time.Now()
		for _, price := range prices {
			refID := generateReferenceID()

			copiedPrice, err := db.ChannelModelPrice.Create().
				SetChannelID(ch.ID).
				SetModelID(price.ModelID).
				SetPrice(price.Price).
				SetReferenceID(refID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to copy channel model price: %w", err)
			}

			_, err = db.ChannelModelPriceVersion.Create().
				SetChannelID(ch.ID).
				SetModelID(price.ModelID).
				SetChannelModelPriceID(copiedPrice.ID).
				SetPrice(price.Price).
				SetStatus(channelmodelpriceversion.StatusActive).
				SetEffectiveStartAt(now).
				SetReferenceID(refID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to copy channel model price version: %w", err)
			}
		}

		duplicated = ch

		return nil
	})
	if err != nil {
		return nil, err
	}

	svc.asyncReloadChannels()

	return duplicated, nil
}
