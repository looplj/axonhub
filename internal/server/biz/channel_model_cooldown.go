package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

// HasActiveModelCooldown reports whether the request model is currently in a
// model-scoped cooldown on this channel. The channel itself stays enabled and
// keeps serving its other models.
func (ch *Channel) HasActiveModelCooldown(requestModel string, now time.Time) bool {
	if ch == nil || requestModel == "" {
		return false
	}

	for _, mc := range ch.ModelCooldowns {
		if mc.Model != requestModel {
			continue
		}
		if mc.ExpiresAt == nil || mc.ExpiresAt.After(now) {
			return true
		}
	}

	return false
}

// markChannelModelCooldown puts one (channel, model) pair into a temporary
// cooldown. The channel stays enabled; candidate selection skips the model on
// this channel until the cooldown expires or an operator clears it.
func (svc *ChannelService) markChannelModelCooldown(ctx context.Context, channelID int, requestModel string, count, cooldownMinutes int) bool {
	if requestModel == "" {
		return false
	}

	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	ch, err := svc.db.Channel.Get(ctx, channelID)
	if err != nil {
		log.Error(ctx, "Failed to load channel for model cooldown",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
		return false
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(cooldownMinutes) * time.Minute)
	reason := fmt.Sprintf(
		"Client cancellation cooldown: %d consecutive cancellations, auto-recover after %d minutes",
		max(count, 1),
		cooldownMinutes,
	)

	// Repeated cancellations must not extend an already running cooldown: keep
	// the earliest expiry instead of resetting it on every threshold hit.
	for _, mc := range ch.ModelCooldowns {
		if mc.Model == requestModel && mc.IsActive() {
			log.Debug(ctx, "Channel model is already in cancellation cooldown, skipping",
				log.Int("channel_id", channelID),
				log.String("model", requestModel),
			)
			return true
		}
	}

	// A disabled channel has no candidate traffic to cool down; re-enabling it
	// already restores everything.
	if ch.Status != channel.StatusEnabled {
		return false
	}

	cooldowns := append(lo.Filter(ch.ModelCooldowns, func(mc objects.ModelCooldown, _ int) bool {
		return !mc.IsExpired()
	}), objects.ModelCooldown{
		Model:     requestModel,
		Reason:    reason,
		CreatedAt: now,
		ExpiresAt: &expiresAt,
	})

	if _, err := svc.db.Channel.UpdateOneID(channelID).SetModelCooldowns(cooldowns).Save(ctx); err != nil {
		log.Error(ctx, "Failed to persist model cooldown",
			log.Int("channel_id", channelID),
			log.String("model", requestModel),
			log.Cause(err),
		)
		return false
	}

	log.Warn(ctx, "Channel model entered cancellation cooldown",
		log.Int("channel_id", channelID),
		log.String("model", requestModel),
		log.Int("threshold", max(count, 1)),
		log.Int("cooldown_minutes", cooldownMinutes),
	)

	// Reload synchronously so candidate selection drops the model immediately.
	if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
		log.Warn(ctx, "Failed to synchronously reload channels after model cooldown",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
	}
	svc.asyncReloadChannels()

	return true
}

// ClearChannelModelCooldown removes one model-scoped cooldown from a channel
// (manual unfreeze) and returns the updated channel.
func (svc *ChannelService) ClearChannelModelCooldown(ctx context.Context, channelID int, requestModel string) (*ent.Channel, error) {
	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	svc.modelCooldownsLock.Lock()
	defer svc.modelCooldownsLock.Unlock()

	entClient := svc.entFromContext(ctx)
	ch, err := entClient.Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	remaining := lo.Filter(ch.ModelCooldowns, func(mc objects.ModelCooldown, _ int) bool {
		return mc.Model != requestModel
	})
	if len(remaining) == len(ch.ModelCooldowns) {
		return ch, nil
	}

	updated, err := entClient.Channel.UpdateOneID(channelID).SetModelCooldowns(remaining).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update channel: %w", err)
	}

	log.Info(ctx, "Cleared channel model cooldown",
		log.Int("channel_id", channelID),
		log.String("model", requestModel),
	)

	if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
		log.Warn(ctx, "Failed to synchronously reload channels after clearing model cooldown",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
	}
	svc.asyncReloadChannels()

	return updated, nil
}

// FreezeChannelModel manually freezes a (channel, model) pair. durationMinutes
// <= 0 freezes the model until an operator unfreezes it; a positive value gives
// the freeze an expiry, and an already running freeze is moved to the requested
// expiry instead of stacking.
func (svc *ChannelService) FreezeChannelModel(ctx context.Context, channelID int, requestModel string, durationMinutes int) (*ent.Channel, error) {
	if requestModel == "" {
		return nil, fmt.Errorf("model is required")
	}

	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	svc.modelCooldownsLock.Lock()
	defer svc.modelCooldownsLock.Unlock()

	entClient := svc.entFromContext(ctx)
	ch, err := entClient.Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	var expiresAt *time.Time
	if durationMinutes > 0 {
		expires := time.Now().Add(time.Duration(durationMinutes) * time.Minute)
		expiresAt = &expires
	}

	remaining := make([]objects.ModelCooldown, 0, len(ch.ModelCooldowns)+1)
	replaced := false
	for _, mc := range ch.ModelCooldowns {
		if mc.Model != requestModel || mc.IsExpired() {
			remaining = append(remaining, mc)
			continue
		}
		// Move an already running freeze to the requested expiry (or forever).
		remaining = append(remaining, objects.ModelCooldown{
			Model:     mc.Model,
			Reason:    "Manually frozen by user",
			CreatedAt: mc.CreatedAt,
			ExpiresAt: expiresAt,
		})
		replaced = true
	}
	if !replaced {
		remaining = append(remaining, objects.ModelCooldown{
			Model:     requestModel,
			Reason:    "Manually frozen by user",
			CreatedAt: time.Now(),
			ExpiresAt: expiresAt,
		})
	}

	updated, err := entClient.Channel.UpdateOneID(channelID).SetModelCooldowns(remaining).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update channel: %w", err)
	}

	log.Info(ctx, "Channel model frozen manually",
		log.Int("channel_id", channelID),
		log.String("model", requestModel),
		log.Int("duration_minutes", durationMinutes),
	)

	if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
		log.Warn(ctx, "Failed to synchronously reload channels after manual model freeze",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
	}
	svc.asyncReloadChannels()

	return updated, nil
}

// cleanupChannelExpiredModelCooldowns drops elapsed model cooldown entries from
// one channel. Returns whether anything was removed.
func (svc *ChannelService) cleanupChannelExpiredModelCooldowns(ctx context.Context, channelID int) (bool, int, error) {
	entClient := svc.entFromContext(ctx)
	ch, err := entClient.Channel.Get(ctx, channelID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get channel: %w", err)
	}

	remaining := lo.Filter(ch.ModelCooldowns, func(mc objects.ModelCooldown, _ int) bool {
		return !mc.IsExpired()
	})
	removed := len(ch.ModelCooldowns) - len(remaining)
	if removed == 0 {
		return false, 0, nil
	}

	if _, err := entClient.Channel.UpdateOneID(ch.ID).SetModelCooldowns(remaining).Save(ctx); err != nil {
		return false, 0, fmt.Errorf("failed to update channel: %w", err)
	}

	return true, removed, nil
}
