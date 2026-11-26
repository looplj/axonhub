// 职责: 验证复制渠道仅继承渠道级实时配置且保留显式设置覆盖。
// 关联文件:
//
//	internal/server/biz/channel_duplicate.go
//	internal/objects/channel_realtime.go

package biz

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestDuplicateChannelInheritsOnlyRealtimeWhenSettingsAbsent(t *testing.T) {
	svc, db := setupTestChannelService(t)
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	cfg := &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime", HandshakeTimeoutMs: lo.ToPtr(int64(3456)), ClientEventsPerSecond: lo.ToPtr(int64(300))}
	source := db.Channel.Create().SetName("source-rt").SetType(channel.TypeOpenai).SetBaseURL("https://api.openai.com/v1").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-source"}).SetSupportedModels([]string{"voice"}).SetDefaultTestModel("voice").SetSettings(&objects.ChannelSettings{Realtime: cfg, PassThroughBody: lo.ToPtr(true)}).SaveX(ctx)
	input := ent.CreateChannelInput{Type: channel.TypeOpenai, Name: "copy-rt", BaseURL: lo.ToPtr(source.BaseURL), Credentials: objects.ChannelCredentials{APIKey: "synthetic-copy"}, SupportedModels: source.SupportedModels, DefaultTestModel: source.DefaultTestModel}
	copied, err := svc.DuplicateChannel(ctx, source.ID, input)
	require.NoError(t, err)
	require.NotNil(t, copied.Settings)
	require.Equal(t, cfg, copied.Settings.Realtime)
	require.Nil(t, copied.Settings.PassThroughBody, "general settings must retain the historical no-inheritance behavior")
	require.Empty(t, copied.Endpoints)
	*copied.Settings.Realtime.HandshakeTimeoutMs = 4567
	*copied.Settings.Realtime.ClientEventsPerSecond = 400
	unchanged := db.Channel.GetX(ctx, source.ID)
	require.Equal(t, int64(3456), *unchanged.Settings.Realtime.HandshakeTimeoutMs)
	require.Equal(t, int64(300), *unchanged.Settings.Realtime.ClientEventsPerSecond)
	input.Name = "copy-explicit-disabled"
	input.Settings = &objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: false}}
	disabled, err := svc.DuplicateChannel(ctx, source.ID, input)
	require.NoError(t, err)
	require.False(t, disabled.Settings.Realtime.Enabled, "explicit caller configuration must override inheritance")
}
