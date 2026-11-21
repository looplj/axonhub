// 职责: 验证备份恢复不能绕过端点协议隔离或渠道实时配置校验。
// 关联文件:
//
//	internal/server/backup/restore.go
//	internal/server/biz/channel_realtime.go

package backup

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
)

func TestRestoreMigratesLegacyRealtimeEndpointsAndRejectsInvalidSettings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		endpoint  []objects.ChannelEndpoint
		config    *objects.ChannelRealtimeConfig
		wantError bool
		migrated  bool
	}{
		{name: "legacy-realtime-endpoint", endpoint: []objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}, {APIFormat: "openai/chat_completions", Path: "/chat/completions"}}, migrated: true},
		{name: "invalid-legacy-endpoint", endpoint: []objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://127.0.0.1", Path: "/v1/realtime"}}, wantError: true},
		{name: "untrusted-host", config: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://127.0.0.1", Path: "/v1/realtime"}, wantError: true},
		{name: "provider-mismatch", config: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "qwen/realtime-ws", BaseURL: "wss://workspace.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}, wantError: true},
		{name: "valid-channel-realtime", config: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, service, ctx := setupBackupTest(t)
			defer db.Close()
			data, err := json.Marshal(BackupData{Version: BackupVersion, Channels: []*BackupChannel{{Name: tc.name, Type: channel.TypeOpenai, BaseURL: "https://api.openai.com/v1", Status: channel.StatusEnabled, SupportedModels: []string{"voice"}, DefaultTestModel: "voice", Settings: &objects.ChannelSettings{Realtime: tc.config}, Endpoints: tc.endpoint, Credentials: objects.ChannelCredentials{APIKey: "synthetic-restore"}}}})
			require.NoError(t, err)
			err = service.Restore(ctx, bytes.NewReader(data), RestoreOptions{IncludeChannels: true, ChannelConflictStrategy: ConflictStrategyOverwrite})
			if tc.wantError {
				require.Error(t, err)
				require.Zero(t, db.Channel.Query().CountX(ctx), "failed restore must be atomic")
				return
			}
			require.NoError(t, err)
			saved := db.Channel.Query().OnlyX(ctx)
			if tc.migrated {
				require.True(t, saved.Settings.Realtime.Enabled)
				require.Equal(t, tc.endpoint[0].APIFormat, saved.Settings.Realtime.Protocol)
				require.Equal(t, tc.endpoint[1:], saved.Endpoints)
				require.NoError(t, service.Restore(ctx, bytes.NewReader(data), RestoreOptions{IncludeChannels: true, ChannelConflictStrategy: ConflictStrategyOverwrite}))
				again := db.Channel.GetX(ctx, saved.ID)
				require.Equal(t, saved.Settings, again.Settings)
				require.Equal(t, saved.Endpoints, again.Endpoints)
				return
			}
			require.Equal(t, tc.config, saved.Settings.Realtime)
			require.Empty(t, saved.Endpoints)
		})
	}
}

func TestRestoreSkipsConflictingProviderBeforeRealtimeValidation(t *testing.T) {
	db, service, ctx := setupBackupTest(t)
	defer db.Close()
	local := db.Channel.Create().SetName("same").SetType(channel.TypeBailian).SetBaseURL("https://example.test").SetCredentials(objects.ChannelCredentials{APIKey: "synthetic"}).SetSupportedModels([]string{"voice"}).SetDefaultTestModel("voice").SaveX(ctx)
	data, err := json.Marshal(BackupData{Version: BackupVersion, Channels: []*BackupChannel{{Name: "same", Type: channel.TypeOpenai, Credentials: objects.ChannelCredentials{APIKey: "synthetic"}, Settings: &objects.ChannelSettings{Realtime: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}}}})
	require.NoError(t, err)
	require.NoError(t, service.Restore(ctx, bytes.NewReader(data), RestoreOptions{IncludeChannels: true, ChannelConflictStrategy: ConflictStrategySkip}))
	saved := db.Channel.GetX(ctx, local.ID)
	require.Equal(t, local.Type, saved.Type)
	require.Equal(t, local.Settings, saved.Settings)
}
