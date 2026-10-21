package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

// PurgeChannelFromModelAssociations removes every reference to the given channel
// ids from model association rules, both on individual models and in the
// developer-level settings.
//
// Deleting a channel without this leaves orphaned rules behind: the rule keeps a
// channelId that no longer resolves, so the association silently still exists
// while its channel is gone. Purging instead of merely hiding keeps the stored
// state truthful, and the association dialog stops listing a channel that no
// longer exists.
//
// A rule that names the channel directly (channel_model / channel_regex) is
// dropped whole once its channel disappears.
func (svc *ModelService) PurgeChannelFromModelAssociations(ctx context.Context, channelIDs []int) error {
	if len(channelIDs) == 0 {
		return nil
	}

	targets := make(map[int]struct{}, len(channelIDs))
	for _, id := range channelIDs {
		targets[id] = struct{}{}
	}

	if err := svc.purgeChannelFromModelSettings(ctx, targets); err != nil {
		return err
	}

	if err := svc.purgeChannelFromDeveloperAssociations(ctx, targets); err != nil {
		return err
	}

	log.Info(ctx, "purged deleted channels from model associations",
		log.Int("channels", len(channelIDs)),
	)

	return nil
}

// purgeChannelFromModelSettings rewrites each model whose association list named
// a deleted channel. Models without associations are skipped before any decode
// so the cleanup stays cheap on large catalogs.
func (svc *ModelService) purgeChannelFromModelSettings(ctx context.Context, targets map[int]struct{}) error {
	models, err := svc.entFromContext(ctx).Model.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to query models for association cleanup: %w", err)
	}

	changed := 0

	for _, m := range models {
		if m.Settings == nil || len(m.Settings.Associations) == 0 {
			continue
		}

		if !associationsReferenceChannels(m.Settings.Associations, targets) {
			continue
		}

		settings := *m.Settings
		settings.Associations = pruneChannelAssociations(m.Settings.Associations, targets)

		if _, err := svc.entFromContext(ctx).Model.UpdateOneID(m.ID).SetSettings(&settings).Save(ctx); err != nil {
			return fmt.Errorf("failed to clean associations of model %d: %w", m.ID, err)
		}

		changed++
	}

	if changed > 0 {
		log.Info(ctx, "purged deleted channels from per-model associations", log.Int("models", changed))
	}

	return nil
}

// purgeChannelFromDeveloperAssociations applies the same pruning to the developer
// level association sets stored in the system model settings.
func (svc *ModelService) purgeChannelFromDeveloperAssociations(ctx context.Context, targets map[int]struct{}) error {
	if svc.systemService == nil {
		return nil
	}

	settings, err := svc.systemService.ModelSettings(ctx)
	if err != nil {
		return fmt.Errorf("failed to read model settings for association cleanup: %w", err)
	}

	if settings == nil || len(settings.DeveloperSettings) == 0 {
		return nil
	}

	changed := false

	for _, developerSetting := range settings.DeveloperSettings {
		if developerSetting == nil || len(developerSetting.Associations) == 0 {
			continue
		}

		if !associationsReferenceChannels(developerSetting.Associations, targets) {
			continue
		}

		developerSetting.Associations = pruneChannelAssociations(developerSetting.Associations, targets)
		changed = true
	}

	if !changed {
		return nil
	}

	if err := svc.systemService.SetModelSettings(ctx, *settings); err != nil {
		return fmt.Errorf("failed to clean developer associations: %w", err)
	}

	return nil
}

// associationsReferenceChannels reports whether any rule names one of the given
// channels, which lets the caller skip untouched rows entirely.
func associationsReferenceChannels(associations []*objects.ModelAssociation, targets map[int]struct{}) bool {
	for _, assoc := range associations {
		if assoc == nil {
			continue
		}

		if assoc.ChannelModel != nil {
			if _, ok := targets[assoc.ChannelModel.ChannelID]; ok {
				return true
			}
		}

		if assoc.ChannelRegex != nil {
			if _, ok := targets[assoc.ChannelRegex.ChannelID]; ok {
				return true
			}
		}
	}

	return false
}

// pruneChannelAssociations drops every rule bound to a deleted channel. The input
// slice is never mutated: surviving rules are shallow-copied so the caller's
// snapshot still describes the pre-delete state.
func pruneChannelAssociations(
	associations []*objects.ModelAssociation,
	targets map[int]struct{},
) []*objects.ModelAssociation {
	if len(associations) == 0 {
		return associations
	}

	pruned := make([]*objects.ModelAssociation, 0, len(associations))

	for _, assoc := range associations {
		if assoc == nil {
			continue
		}

		if associationTargetsChannel(assoc, targets) {
			continue
		}

		cloned := *assoc
		pruned = append(pruned, &cloned)
	}

	return pruned
}

// associationTargetsChannel reports whether the rule names a deleted channel.
// Tag-scoped rules are matched by tag rather than id, so a deleted channel does
// not leave them dangling.
func associationTargetsChannel(assoc *objects.ModelAssociation, targets map[int]struct{}) bool {
	if assoc.ChannelModel != nil {
		if _, ok := targets[assoc.ChannelModel.ChannelID]; ok {
			return true
		}
	}

	if assoc.ChannelRegex != nil {
		if _, ok := targets[assoc.ChannelRegex.ChannelID]; ok {
			return true
		}
	}

	return false
}
