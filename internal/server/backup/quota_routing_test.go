package backup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/system"
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

func TestQuotaRoutingSettings_RestoreNewSettingsDoesNotApplyLegacyMapping(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	data, err := json.Marshal(BackupData{
		Version: BackupVersion,
		SystemConfigs: []*BackupSystemConfig{
			{Key: biz.SystemKeyQuotaRoutingSettings, Value: `{"defaultMode":"backpressure"}`},
			{Key: biz.SystemKeyQuotaEnforcementSettings, Value: `{"enabled":true,"allowedChannelIDs":[7]}`},
		},
		Channels: []*BackupChannel{{
			Channel: ent.Channel{
				ID:              7,
				Name:            "new-settings-channel",
				Type:            channel.TypeOpenai,
				BaseURL:         "https://api.openai.com",
				Status:          channel.StatusEnabled,
				SupportedModels: []string{"gpt-4"},
				Settings:        &objects.ChannelSettings{QuotaRoutingMode: objects.QuotaRoutingModeRemoveOnExhausted},
			},
			Credentials: objects.ChannelCredentials{APIKeys: []string{"key"}},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Restore(ctx, data, RestoreOptions{IncludeSystemConfigs: true, IncludeChannels: true}))

	restored, err := client.Channel.Query().Where(channel.NameEQ("new-settings-channel")).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, restored.Settings.QuotaRoutingMode)
	marker, err := client.System.Query().Where(system.KeyEQ(biz.SystemKeyQuotaRoutingMigrationDone)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "true", marker.Value)
}

func TestQuotaRoutingSettings_RestoreSystemOnlyDoesNotRemapExistingChannels(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	_, err := client.Channel.Create().
		SetName("existing-channel").
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"existing-key"}}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetSettings(&objects.ChannelSettings{QuotaRoutingMode: objects.QuotaRoutingModeRemoveOnExhausted}).
		Save(ctx)
	require.NoError(t, err)

	data, err := json.Marshal(BackupData{
		Version: BackupVersion,
		SystemConfigs: []*BackupSystemConfig{{
			Key:   biz.SystemKeyQuotaEnforcementSettings,
			Value: `{"enabled":true,"allowedChannelIDs":[1]}`,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Restore(ctx, data, RestoreOptions{IncludeSystemConfigs: true}))

	existing, err := client.Channel.Query().Where(channel.NameEQ("existing-channel")).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, existing.Settings.QuotaRoutingMode)
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, service.systemService.QuotaRoutingSettingsOrDefault(ctx).DefaultMode)
	marker, err := client.System.Query().Where(system.KeyEQ(biz.SystemKeyQuotaRoutingMigrationDone)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "true", marker.Value)
}
