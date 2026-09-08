package backup

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

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

func TestQuotaRoutingSettings_RestoreWarnsAfterMigrationMarker(t *testing.T) {
	client, service, ctx := setupBackupTest(t)
	defer client.Close()

	_, err := client.System.Create().
		SetKey(biz.SystemKeyQuotaRoutingMigrationDone).
		SetValue("true").
		Save(ctx)
	require.NoError(t, err)
	before, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)

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
			Value: `{"enabled":true,"dePrioritize":true,"allowedChannelIDs":[7]}`,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, service.Restore(ctx, data, RestoreOptions{IncludeSystemConfigs: true}))

	select {
	case msg := <-warning:
		require.Contains(t, msg, "will not re-migrate")
	default:
		t.Fatal("expected quota migration warning")
	}
	after, err := client.Channel.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
