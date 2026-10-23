package biz

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

// FrozenModelChannel reports one channel that currently freezes the given
// model, together with the active cooldown entries for that model.
type FrozenModelChannel struct {
	Channel   *ent.Channel        `json:"channel"`
	Cooldowns []objects.ModelCooldown `json:"cooldowns"`
}

// QueryFrozenModelChannels lists every enabled channel that currently has an
// active cooldown for the requested model. Used by the models UI to surface
// "this model is frozen on channel X" next to the model itself.
func (svc *ModelService) QueryFrozenModelChannels(ctx context.Context, modelID string) ([]*FrozenModelChannel, error) {
	if modelID == "" {
		return []*FrozenModelChannel{}, nil
	}

	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*FrozenModelChannel, 0)
	for _, ch := range channels {
		cooldowns := make([]objects.ModelCooldown, 0)
		for _, mc := range ch.ModelCooldowns {
			if mc.Model != modelID || !mc.IsActive() {
				continue
			}
			cooldowns = append(cooldowns, mc)
		}
		if len(cooldowns) == 0 {
			continue
		}
		result = append(result, &FrozenModelChannel{Channel: ch, Cooldowns: cooldowns})
	}

	return result, nil
}
