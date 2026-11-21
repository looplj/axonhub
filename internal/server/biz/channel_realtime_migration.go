// 职责: 将历史 Realtime 端点原子搬迁到渠道设置，保留 HTTP 端点及其他渠道数据。
// 关联文件:
//
//	internal/server/biz/channel_realtime.go
//	internal/ent/migrate/datamigrate/migrator.go
//	internal/objects/channel.go
//	internal/objects/channel_realtime.go
//	internal/ent/schema/schematype/soft_delete.go
//
// 注意事项: 包括软删除渠道但不恢复其状态；无历史实时端点时不写入，重复执行不改变配置或更新时间。

package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

func MigrateChannelRealtime(ctx context.Context, db *ent.Client) error {
	_, err := authz.RunAsHost(ctx, db, "channel-realtime-migration", func(host context.Context) (struct{}, error) {
		service := &AbstractService{db: db}
		for attempt := range 3 {
			err := service.RunInTransaction(host, func(txctx context.Context) error {
				return migrateChannelRealtimeRows(txctx, service.entFromContext(txctx))
			})
			if err == nil {
				return struct{}{}, nil
			}
			message := strings.ToLower(err.Error())
			retryable := errors.Is(err, ErrChannelUpdatedConcurrently) || strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") || strings.Contains(message, "could not serialize") || strings.Contains(message, "serialization failure") || strings.Contains(message, "deadlock detected")
			if !retryable || attempt == 2 {
				return struct{}{}, err
			}
			timer := time.NewTimer(time.Duration(attempt+1) * 20 * time.Millisecond)
			select {
			case <-host.Done():
				timer.Stop()
				return struct{}{}, host.Err()
			case <-timer.C:
			}
		}
		return struct{}{}, errors.New("Realtime migration retries exhausted")
	})
	return err
}

func migrateChannelRealtimeRows(ctx context.Context, db *ent.Client) error {
	ctx = schematype.SkipSoftDelete(ctx)
	rows, err := db.Channel.Query().Order(ent.Asc(channel.FieldID)).All(ctx)
	if err != nil {
		return fmt.Errorf("load channels for Realtime migration: %w", err)
	}
	for _, row := range rows {
		settings, ordinary, changed, err := ConvertLegacyChannelRealtime(row.Type, row.Settings, row.Endpoints)
		if err != nil {
			log.Warn(ctx, "legacy Realtime configuration left inactive", log.Int("channel_id", row.ID), log.Cause(err))
			continue
		}
		if !changed {
			continue
		}
		_, err = db.Channel.UpdateOneID(row.ID).Where(channel.UpdatedAtEQ(row.UpdatedAt)).SetSettings(settings).SetEndpoints(ordinary).Save(ctx)
		if ent.IsNotFound(err) {
			return ErrChannelUpdatedConcurrently
		}
		if err != nil {
			return fmt.Errorf("save channel %d Realtime migration: %w", row.ID, err)
		}
	}
	return nil
}

// ConvertLegacyChannelRealtime moves legacy endpoint config without mutating its inputs.
func ConvertLegacyChannelRealtime(kind channel.Type, current *objects.ChannelSettings, endpoints []objects.ChannelEndpoint) (*objects.ChannelSettings, []objects.ChannelEndpoint, bool, error) {
	var legacy []objects.ChannelEndpoint
	for _, ep := range endpoints {
		if objects.IsRealtimeAPIFormat(ep.APIFormat) {
			legacy = append(legacy, ep)
		}
	}
	if len(legacy) == 0 {
		return current, endpoints, false, nil
	}
	settings := objects.ChannelSettings{}
	if current != nil {
		settings = *current
	}
	// A saved setting, including a deliberate disable, takes precedence.
	if settings.Realtime == nil {
		migrated, err := realtimeConfigFromLegacy(kind, legacy)
		if err != nil {
			return nil, nil, false, err
		}
		settings.Realtime = migrated
	}
	if err := ValidateChannelRealtime(kind, settings.Realtime); err != nil {
		return nil, nil, false, err
	}
	ordinary := make([]objects.ChannelEndpoint, 0, len(endpoints)-len(legacy))
	for _, ep := range endpoints {
		if !objects.IsRealtimeAPIFormat(ep.APIFormat) {
			ordinary = append(ordinary, ep)
		}
	}
	return &settings, ordinary, true, nil
}

func realtimeConfigFromLegacy(kind channel.Type, legacy []objects.ChannelEndpoint) (*objects.ChannelRealtimeConfig, error) {
	if len(legacy) != 1 {
		return nil, errors.New("exactly one legacy Realtime endpoint is required")
	}
	ep := legacy[0]
	cfg := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: ep.APIFormat, BaseURL: ep.BaseURL, Path: ep.Path, HandshakeTimeoutMs: ep.HandshakeTimeoutMs}
	if err := ValidateChannelRealtime(kind, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
