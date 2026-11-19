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

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/stretchr/testify/require"
)

func TestRestoreRejectsRealtimeEndpointsAndInvalidChannelRealtime(t *testing.T) {
	for _, tc := range []struct {
		name      string
		endpoint  []objects.ChannelEndpoint
		config    *objects.ChannelRealtimeConfig
		wantError bool
	}{
		{name: "legacy-realtime-endpoint", endpoint: []objects.ChannelEndpoint{{APIFormat: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}}, wantError: true},
		{name: "untrusted-host", config: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://127.0.0.1", Path: "/v1/realtime"}, wantError: true},
		{name: "provider-mismatch", config: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "qwen/realtime-ws", BaseURL: "wss://workspace.cn-beijing.maas.aliyuncs.com", Path: "/api-ws/v1/realtime"}, wantError: true},
		{name: "valid-channel-realtime", config: &objects.ChannelRealtimeConfig{Enabled: true, Protocol: "openai/realtime-ws", BaseURL: "wss://api.openai.com", Path: "/v1/realtime"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, service, ctx := setupBackupTest(t)
			defer db.Close()
			data, err := json.Marshal(BackupData{Version: BackupVersion, Channels: []*BackupChannel{{Channel: ent.Channel{Name: tc.name, Type: channel.TypeOpenai, BaseURL: "https://api.openai.com/v1", Status: channel.StatusEnabled, SupportedModels: []string{"voice"}, DefaultTestModel: "voice", Settings: &objects.ChannelSettings{Realtime: tc.config}, Endpoints: tc.endpoint}, Credentials: objects.ChannelCredentials{APIKey: "synthetic-restore"}}}})
			require.NoError(t, err)
			err = service.Restore(ctx, bytes.NewReader(data), RestoreOptions{IncludeChannels: true, ChannelConflictStrategy: ConflictStrategyOverwrite})
			if tc.wantError {
				require.Error(t, err)
				require.Zero(t, db.Channel.Query().CountX(ctx), "failed restore must be atomic")
				return
			}
			require.NoError(t, err)
			saved := db.Channel.Query().OnlyX(ctx)
			require.Equal(t, tc.config, saved.Settings.Realtime)
			require.Empty(t, saved.Endpoints)
		})
	}
}
