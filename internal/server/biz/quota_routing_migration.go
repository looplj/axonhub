package biz

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

const quotaRoutingMigrationReason = "quota-routing-migration"

type quotaRoutingMigrator struct {
	system   *SystemService
	channels *ChannelService

	beforeChannelUpdate func(id int) error
}

func (m *quotaRoutingMigrator) Migrate(ctx context.Context) error {
	_, err := authz.RunWithSystemBypass(ctx, quotaRoutingMigrationReason, func(bypassCtx context.Context) (struct{}, error) {
		return struct{}{}, m.system.RunInTransaction(bypassCtx, m.migrateInTransaction)
	})
	return err
}

func (m *quotaRoutingMigrator) migrateInTransaction(ctx context.Context) error {
	if _, err := m.system.getSystemValue(ctx, SystemKeyQuotaRoutingMigrationDone); err == nil {
		return nil
	} else if !ent.IsNotFound(err) {
		return fmt.Errorf("failed to check quota routing migration marker: %w", err)
	}

	legacyValue, err := m.system.getSystemValue(ctx, legacyQuotaEnforcementSettingsKey)
	if err != nil {
		if ent.IsNotFound(err) {
			return m.system.setSystemValue(ctx, SystemKeyQuotaRoutingMigrationDone, "true")
		}
		return fmt.Errorf("failed to read legacy quota enforcement settings: %w", err)
	}

	var legacy legacyQuotaEnforcementSettings
	if err := json.Unmarshal([]byte(legacyValue), &legacy); err != nil {
		return fmt.Errorf("failed to decode legacy quota enforcement settings: %w", err)
	}

	mode := quotaRoutingModeFromLegacy(legacy)
	if _, err := m.system.getSystemValue(ctx, SystemKeyQuotaRoutingSettings); err != nil {
		if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to check quota routing settings: %w", err)
		}
		if err := m.system.SetQuotaRoutingSettings(ctx, QuotaRoutingSettings{DefaultMode: mode}); err != nil {
			return fmt.Errorf("failed to migrate quota routing settings: %w", err)
		}
	}

	migratedIDs := make([]int, 0, len(legacy.AllowedChannelIDs))
	for _, id := range legacy.AllowedChannelIDs {
		ch, err := m.channels.entFromContext(ctx).Channel.Query().Where(channel.IDEQ(id)).Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				log.Warn(ctx, "quota routing migration skipped missing channel", log.Int("channel_id", id))
				continue
			}
			return fmt.Errorf("failed to load channel %d during quota routing migration: %w", id, err)
		}

		settings := *ch.Settings
		settings.QuotaRoutingMode = objects.QuotaRoutingModeIgnoreQuota
		if m.beforeChannelUpdate != nil {
			if err := m.beforeChannelUpdate(id); err != nil {
				return err
			}
		}
		if _, err := m.channels.UpdateChannel(ctx, id, &ent.UpdateChannelInput{Settings: &settings}); err != nil {
			return fmt.Errorf("failed to migrate channel %d: %w", id, err)
		}
		migratedIDs = append(migratedIDs, id)
	}

	log.Info(ctx, "migrated quota routing settings", log.Any("channel_ids", migratedIDs), log.String("default_mode", string(mode)))
	if err := m.system.setSystemValue(ctx, SystemKeyQuotaRoutingMigrationDone, "true"); err != nil {
		return fmt.Errorf("failed to write quota routing migration marker: %w", err)
	}
	return nil
}

func quotaRoutingModeFromLegacy(legacy legacyQuotaEnforcementSettings) objects.QuotaRoutingMode {
	if !legacy.Enabled {
		return objects.QuotaRoutingModeRemoveOnExhausted
	}
	if legacy.ExhaustedOnly || legacy.Mode == "EXHAUSTED_ONLY" || legacy.Mode == "exhausted_only" {
		return objects.QuotaRoutingModeRemoveOnExhausted
	}
	if legacy.DePrioritize || legacy.Mode == "DE_PRIORITIZE" || legacy.Mode == "de_prioritize" {
		return objects.QuotaRoutingModeBackpressure
	}
	return objects.QuotaRoutingModeRemoveOnExhausted
}
