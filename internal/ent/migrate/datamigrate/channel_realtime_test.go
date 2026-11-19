// 职责: 验证启动迁移会将历史实时端点搬到渠道设置且重复运行不改变数据。
// 关联文件:
//
//	internal/ent/migrate/datamigrate/migrator.go
//	internal/server/biz/channel_realtime_migration.go
package datamigrate_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/build"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/stretchr/testify/require"
)

func TestStartupChannelRealtimeMigrationAndIdempotence(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { _ = db.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	db.System.Create().SetKey(biz.SystemKeyInitialized).SetValue("true").SaveX(ctx)
	db.System.Create().SetKey(biz.SystemKeyVersion).SetValue(build.Version).SaveX(ctx)
	row := db.Channel.Create().SetName("legacy-rt").SetType(channel.TypeOpenai).SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SetSupportedModels([]string{"gpt-realtime-2.1"}).SetDefaultTestModel("gpt-realtime-2.1").SetSettings(&objects.ChannelSettings{ExtraModelPrefix: "preserve"}).SetEndpoints([]objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime", Transport: "websocket"}, {APIFormat: "openai/chat_completions", Path: "/chat/completions"}}).SaveX(ctx)
	require.NoError(t, datamigrate.NewMigratorWithoutRegistrations(db).Run(context.Background()))
	migrated := db.Channel.GetX(ctx, row.ID)
	require.Len(t, migrated.Endpoints, 1, "startup migration must run even when database version equals build version")
	require.Equal(t, "openai/chat_completions", migrated.Endpoints[0].APIFormat)
	require.Equal(t, "preserve", migrated.Settings.ExtraModelPrefix)
	raw, err := json.Marshal(migrated.Settings)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(raw, &settings))
	cfg, ok := settings["realtime"].(map[string]any)
	require.True(t, ok, "Realtime is stored in channel settings")
	require.Equal(t, "openai/realtime-ws", cfg["protocol"])
	require.Equal(t, true, cfg["enabled"])
	require.NoError(t, datamigrate.NewMigratorWithoutRegistrations(db).Run(context.Background()))
	again := db.Channel.GetX(ctx, row.ID)
	require.Equal(t, migrated.UpdatedAt, again.UpdatedAt, "repeat migration must not issue channel writes")
	require.Equal(t, migrated.Settings, again.Settings)
	require.Equal(t, migrated.Endpoints, again.Endpoints)
}
