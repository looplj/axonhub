// 职责: 验证实时渠道配置迁移的字段保留、重复执行、已有配置优先与失败回滚。
// 关联文件:
//
//	internal/server/biz/channel_realtime_migration.go
//	internal/server/biz/channel_realtime.go

package biz

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/objects"
)

func realtimeMigrationFixture(t *testing.T) (*ent.Client, context.Context) {
	t.Helper()
	db := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { _ = db.Close() })
	return db, authz.WithTestBypass(ent.NewContext(context.Background(), db))
}

func seedLegacyRealtimeChannel(t *testing.T, db *ent.Client, ctx context.Context, kind channel.Type, ep objects.ChannelEndpoint, settings *objects.ChannelSettings) *ent.Channel {
	t.Helper()
	create := db.Channel.Create().SetName(fmt.Sprintf("legacy-%d", db.Channel.Query().CountX(ctx))).SetType(kind).SetBaseURL("https://untouched-http.example/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SetSupportedModels([]string{"voice"}).SetDefaultTestModel("voice").SetEndpoints([]objects.ChannelEndpoint{{APIFormat: "openai/chat_completions", Path: "/chat/completions"}, ep})
	if settings != nil {
		create.SetSettings(settings)
	}
	return create.SaveX(ctx)
}

func TestMigrateChannelRealtimeMovesAndRepeats(t *testing.T) {
	for _, tc := range []struct {
		kind                 channel.Type
		protocol, host, path string
	}{{channel.TypeOpenai, "openai/realtime-ws", "wss://api.openai.com", "/v1/realtime"}, {channel.TypeBailian, "qwen/realtime-ws", "wss://llm-migration.cn-beijing.maas.aliyuncs.com", "/api-ws/v1/realtime"}} {
		t.Run(string(tc.kind), func(t *testing.T) {
			db, ctx := realtimeMigrationFixture(t)
			timeout := int64(9876)
			cfg := &objects.ChannelSettings{ExtraModelPrefix: "keep", ModelMappings: []objects.ModelMapping{{From: "voice", To: "upstream"}}}
			row := seedLegacyRealtimeChannel(t, db, ctx, tc.kind, objects.ChannelEndpoint{APIFormat: tc.protocol, BaseURL: tc.host, Path: tc.path, Transport: "websocket", HandshakeTimeoutMs: &timeout}, cfg)
			require.NoError(t, MigrateChannelRealtime(context.Background(), db))
			moved := db.Channel.GetX(ctx, row.ID)
			require.Len(t, moved.Endpoints, 1)
			require.Equal(t, "openai/chat_completions", moved.Endpoints[0].APIFormat)
			require.Equal(t, "https://untouched-http.example/v1", moved.BaseURL)
			require.Equal(t, row.Credentials, moved.Credentials)
			require.Equal(t, cfg.ModelMappings, moved.Settings.ModelMappings)
			require.Equal(t, "keep", moved.Settings.ExtraModelPrefix)
			require.Equal(t, &objects.ChannelRealtimeConfig{Enabled: true, Protocol: tc.protocol, BaseURL: tc.host, Path: tc.path, HandshakeTimeoutMs: &timeout}, moved.Settings.Realtime)
			require.NoError(t, MigrateChannelRealtime(context.Background(), db))
			twice := db.Channel.GetX(ctx, row.ID)
			require.Equal(t, moved.Settings, twice.Settings)
			require.Equal(t, moved.Endpoints, twice.Endpoints)
			require.Equal(t, moved.UpdatedAt, twice.UpdatedAt)
		})
	}
}

func TestMigrateChannelRealtimePreservesExplicitSetting(t *testing.T) {
	db, ctx := realtimeMigrationFixture(t)
	existing := &objects.ChannelRealtimeConfig{Enabled: false, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}
	row := seedLegacyRealtimeChannel(t, db, ctx, channel.TypeOpenai, objects.ChannelEndpoint{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}, &objects.ChannelSettings{Realtime: existing})
	require.NoError(t, MigrateChannelRealtime(context.Background(), db))
	moved := db.Channel.GetX(ctx, row.ID)
	require.Equal(t, existing, moved.Settings.Realtime)
	require.Len(t, moved.Endpoints, 1)
	require.Nil(t, (&Channel{Channel: moved}).RealtimeEndpoint(), "migration must not reactivate disabled Realtime")
}

func TestMigrateChannelRealtimeIgnoresStaleEndpointsWhenSettingExists(t *testing.T) {
	db, ctx := realtimeMigrationFixture(t)
	existing := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}
	row := db.Channel.Create().
		SetName("authoritative-realtime-setting").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://untouched-http.example/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).
		SetSupportedModels([]string{"voice"}).
		SetDefaultTestModel("voice").
		SetEndpoints([]objects.ChannelEndpoint{
			{APIFormat: "openai/chat_completions", Path: "/chat/completions"},
			{APIFormat: "openai/realtime-ws", BaseURL: "wss://stale.example", Path: "/wrong"},
			{APIFormat: "qwen/realtime-ws", BaseURL: "wss://stale.cn-beijing.maas.aliyuncs.com", Path: "/wrong"},
		}).
		SetSettings(&objects.ChannelSettings{Realtime: existing}).
		SaveX(ctx)

	require.NoError(t, MigrateChannelRealtime(context.Background(), db))
	migrated := db.Channel.GetX(ctx, row.ID)
	require.Equal(t, existing, migrated.Settings.Realtime)
	require.Equal(t, []objects.ChannelEndpoint{{APIFormat: "openai/chat_completions", Path: "/chat/completions"}}, migrated.Endpoints)
}

func TestMigrateChannelRealtimeIncludesSoftDeletedChannels(t *testing.T) {
	db, ctx := realtimeMigrationFixture(t)
	row := seedLegacyRealtimeChannel(t, db, ctx, channel.TypeOpenai, objects.ChannelEndpoint{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}, &objects.ChannelSettings{ExtraModelPrefix: "preserved"})
	row = db.Channel.UpdateOne(row).SetStatus(channel.StatusDisabled).SaveX(ctx)
	require.NoError(t, db.Channel.DeleteOne(row).Exec(ctx))
	includeDeleted := schematype.SkipSoftDelete(ctx)
	deleted := db.Channel.GetX(includeDeleted, row.ID)
	require.Positive(t, deleted.DeletedAt)
	require.NoError(t, MigrateChannelRealtime(context.Background(), db))
	moved := db.Channel.GetX(includeDeleted, row.ID)
	require.Len(t, moved.Endpoints, 1, "historical realtime endpoint must be removed even when the channel is soft-deleted")
	require.Equal(t, "openai/chat_completions", moved.Endpoints[0].APIFormat)
	require.NotNil(t, moved.Settings.Realtime)
	require.Equal(t, "openai/realtime-ws", moved.Settings.Realtime.Protocol)
	require.Equal(t, deleted.DeletedAt, moved.DeletedAt)
	require.Equal(t, deleted.Status, moved.Status)
	require.Equal(t, "preserved", moved.Settings.ExtraModelPrefix)
	require.NoError(t, MigrateChannelRealtime(context.Background(), db))
	again := db.Channel.GetX(includeDeleted, row.ID)
	require.Equal(t, moved.Endpoints, again.Endpoints)
	require.Equal(t, moved.Settings, again.Settings)
	require.Equal(t, moved.UpdatedAt, again.UpdatedAt)
	require.Equal(t, moved.DeletedAt, again.DeletedAt)
	require.Equal(t, moved.Status, again.Status)
}

func TestMigrateChannelRealtimeLeavesInvalidProviderInactive(t *testing.T) {
	db, ctx := realtimeMigrationFixture(t)
	good := seedLegacyRealtimeChannel(t, db, ctx, channel.TypeOpenai, objects.ChannelEndpoint{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}, nil)
	bad := seedLegacyRealtimeChannel(t, db, ctx, channel.TypeOpenai, objects.ChannelEndpoint{APIFormat: "qwen/realtime-ws", BaseURL: "wss://llm-invalid.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}, nil)
	require.NoError(t, MigrateChannelRealtime(context.Background(), db))
	migrated := db.Channel.GetX(ctx, good.ID)
	require.True(t, migrated.Settings.Realtime.Enabled)
	require.Len(t, migrated.Endpoints, 1)
	unchanged := db.Channel.GetX(ctx, bad.ID)
	require.Equal(t, bad.Endpoints, unchanged.Endpoints)
	require.Equal(t, bad.Settings, unchanged.Settings)
	require.Nil(t, (&Channel{Channel: unchanged}).RealtimeEndpoint())
	require.NoError(t, MigrateChannelRealtime(context.Background(), db))
	require.Equal(t, migrated.UpdatedAt, db.Channel.GetX(ctx, good.ID).UpdatedAt)
	require.Equal(t, bad.UpdatedAt, db.Channel.GetX(ctx, bad.ID).UpdatedAt)
}

func TestChannelRealtimeRuntimeDescriptionDoesNotReadLegacyEndpoints(t *testing.T) {
	ch := &Channel{Channel: &ent.Channel{Type: channel.TypeOpenai, Endpoints: []objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}}}
	require.Nil(t, ch.RealtimeEndpoint())
	ch.Settings = &objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws"}}
	ep := ch.RealtimeEndpoint()
	require.NotNil(t, ep)
	require.Equal(t, "wss://api.openai.com", ep.BaseURL)
	require.Equal(t, "/v1/realtime", ep.Path)
	ch.Settings.Realtime.BaseURL = "wss://127.0.0.1"
	require.Nil(t, ch.RealtimeEndpoint(), "invalid configured hosts never become dialable targets")
}
