package biz

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channelmodelprice"
	"github.com/looplj/axonhub/internal/ent/channelmodelpriceversion"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

type SaveChannelModelPriceInput struct {
	ModelID string             `json:"modelId"`
	Price   objects.ModelPrice `json:"price"`
}

type ActionType string

const (
	ActionTypeCreate ActionType = "create"
	ActionTypeUpdate ActionType = "update"
	ActionTypeDelete ActionType = "delete"
	ActionTypeSkip   ActionType = "skip"
)

type PriceChangeAction struct {
	Type          ActionType
	ModelID       string
	Price         objects.ModelPrice
	ExistingPrice *ent.ChannelModelPrice // nil if create
}

func calculatePriceChanges(prices []*ent.ChannelModelPrice, inputs []SaveChannelModelPriceInput) []PriceChangeAction {
	existingMap := lo.KeyBy(prices, func(p *ent.ChannelModelPrice) string {
		return p.ModelID
	})

	// Use a map for inputs to handle duplicates in the inputs slice
	inputMap := lo.KeyBy(inputs, func(i SaveChannelModelPriceInput) string {
		return i.ModelID
	})

	var actions []PriceChangeAction

	// 1. Identify updates and creates
	// We iterate over the unique inputs from inputMap to avoid duplicate actions
	for modelID, input := range inputMap {
		existing, ok := existingMap[modelID]
		if !ok {
			actions = append(actions, PriceChangeAction{
				Type:          ActionTypeCreate,
				ModelID:       modelID,
				Price:         input.Price,
				ExistingPrice: nil,
			})
		} else {
			// Only update if price changed
			if existing.Price.Equals(input.Price) {
				actions = append(actions, PriceChangeAction{
					Type:          ActionTypeSkip,
					ModelID:       modelID,
					Price:         input.Price,
					ExistingPrice: existing,
				})
			} else {
				actions = append(actions, PriceChangeAction{
					Type:          ActionTypeUpdate,
					ModelID:       modelID,
					Price:         input.Price,
					ExistingPrice: existing,
				})
			}
		}
	}

	// 2. Identify deletes: present in existing but not in inputs
	for _, existing := range prices {
		if _, ok := inputMap[existing.ModelID]; !ok {
			actions = append(actions, PriceChangeAction{
				Type:          ActionTypeDelete,
				ModelID:       existing.ModelID,
				ExistingPrice: existing,
			})
		}
	}

	return actions
}

func (svc *ChannelService) SaveChannelModelPrices(
	ctx context.Context,
	channelID int,
	inputs []SaveChannelModelPriceInput,
) ([]*ent.ChannelModelPrice, error) {
	now := time.Now()

	db := svc.entFromContext(ctx)

	prices, err := db.ChannelModelPrice.Query().
		Where(
			channelmodelprice.ChannelID(channelID),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing channel model prices: %w", err)
	}

	actions := calculatePriceChanges(prices, inputs)

	var results []*ent.ChannelModelPrice

	err = svc.RunInTransaction(ctx, func(ctx context.Context) error {
		db := svc.entFromContext(ctx)

		for _, action := range actions {
			var (
				entity *ent.ChannelModelPrice
				refID  string
				err    error
			)

			switch action.Type {
			case ActionTypeSkip:
				results = append(results, action.ExistingPrice)
				continue

			case ActionTypeDelete:
				// Archive old versions
				_, err = db.ChannelModelPriceVersion.Update().
					Where(
						channelmodelpriceversion.ChannelModelPriceIDEQ(action.ExistingPrice.ID),
						channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
					).
					SetStatus(channelmodelpriceversion.StatusArchived).
					SetEffectiveEndAt(now).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to archive channel model price versions for delete: %w", err)
				}

				err = db.ChannelModelPrice.DeleteOne(action.ExistingPrice).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to delete channel model price: %w", err)
				}

				continue

			case ActionTypeCreate:
				refID = generateReferenceID()

				entity, err = db.ChannelModelPrice.Create().
					SetChannelID(channelID).
					SetModelID(action.ModelID).
					SetPrice(action.Price).
					SetRefreanceID(refID).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create channel model price: %w", err)
				}

			case ActionTypeUpdate:
				entity = action.ExistingPrice
				// Archive old versions
				_, err = db.ChannelModelPriceVersion.Update().
					Where(
						channelmodelpriceversion.ChannelModelPriceIDEQ(entity.ID),
						channelmodelpriceversion.StatusEQ(channelmodelpriceversion.StatusActive),
					).
					SetStatus(channelmodelpriceversion.StatusArchived).
					SetEffectiveEndAt(now).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to archive old channel model price versions: %w", err)
				}

				refID = generateReferenceID()

				entity, err = db.ChannelModelPrice.UpdateOneID(entity.ID).
					SetPrice(action.Price).
					SetRefreanceID(refID).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to update channel model price: %w", err)
				}
			}

			// 3. Create new version
			_, err = db.ChannelModelPriceVersion.Create().
				SetChannelID(channelID).
				SetModelID(action.ModelID).
				SetChannelModelPriceID(entity.ID).
				SetPrice(action.Price).
				SetStatus(channelmodelpriceversion.StatusActive).
				SetEffectiveStartAt(now).
				SetRefreanceID(refID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create channel model price version: %w", err)
			}

			results = append(results, entity)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Refresh cached model prices for enabled channel
	if ch := svc.GetEnabledChannel(channelID); ch != nil {
		svc.preloadModelPrices(ctx, ch)

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "refreshed cached model prices after save",
				log.Int("channel_id", channelID),
				log.Int("count", len(ch.cachedModelPrices)),
			)
		}
	}

	return results, nil
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func generateReferenceID() string {
	b := make([]byte, 8)
	for i := range b {
		//nolint:gosec // not a security issue.
		b[i] = letters[rand.IntN(len(letters))]
	}

	return string(b)
}
