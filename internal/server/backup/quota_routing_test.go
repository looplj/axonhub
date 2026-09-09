package backup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestQuotaRoutingSettings_BackupKeys(t *testing.T) {
	require.Contains(t, systemConfigBackupKeys, biz.SystemKeyQuotaEnforcementSettings)
	require.Contains(t, systemConfigBackupKeys, biz.SystemKeyQuotaRoutingSettings)
}

func TestQuotaRoutingSettings_RestoreLegacyMappingWithoutMigrationMarker(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()
	warning := make(chan string, 1)
	log.GetGlobalLogger().AddHook(log.HookFunc(func(_ context.Context, msg string, fields ...log.Field) []log.Field {
		if strings.Contains(msg, "will not re-migrate") {
			warning <- msg
		}
		return fields
	}))

	data, err := json.Marshal(BackupData{
		Version: BackupVersion,
		SystemConfigs: []*BackupSystemConfig{{
			Key:   biz.SystemKeyQuotaEnforcementSettings,
			Value: `{"enabled":true,"exhaustedOnly":true,"allowedChannelIDs":[7]}`,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Restore(ctx, data, RestoreOptions{IncludeSystemConfigs: true}))

	settings := service.systemService.QuotaRoutingSettingsOrDefault(ctx)
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, settings.DefaultMode)
	select {
	case msg := <-warning:
		t.Fatalf("unexpected quota migration warning: %s", msg)
	default:
	}
}

func TestQuotaRoutingSettings_RestoreLegacyMappingAfterMigrationMarker(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	_, err := client.System.Create().
		SetKey(biz.SystemKeyQuotaRoutingMigrationDone).
		SetValue("true").
		Save(ctx)
	require.NoError(t, err)
	data, err := json.Marshal(BackupData{
		Version: BackupVersion,
		SystemConfigs: []*BackupSystemConfig{{
			Key:   biz.SystemKeyQuotaEnforcementSettings,
			Value: `{"enabled":true,"dePrioritize":true,"allowedChannelIDs":[7]}`,
		}},
		Channels: []*BackupChannel{{
			Channel: ent.Channel{
				ID:              7,
				Name:            "restored-channel",
				Type:            channel.TypeOpenai,
				BaseURL:         "https://api.openai.com",
				Status:          channel.StatusEnabled,
				SupportedModels: []string{"gpt-4"},
				Settings:        &objects.ChannelSettings{},
			},
			Credentials: objects.ChannelCredentials{APIKeys: []string{"key"}},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Restore(ctx, data, RestoreOptions{IncludeSystemConfigs: true, IncludeChannels: true}))

	restored, err := client.Channel.Query().Where(channel.NameEQ("restored-channel")).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeIgnoreQuota, restored.Settings.QuotaRoutingMode)
}
