package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/system"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestQuotaRoutingMigration(t *testing.T) {
	t.Run("fresh migration uses system bypass and preserves settings", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, DePrioritize: true, AllowedChannelIDs: []int{1, 2}})
		fixture.createChannel(1, objects.ChannelSettings{ModelMappings: []objects.ModelMapping{{From: "alias", To: "model"}}})
		fixture.createChannel(2, objects.ChannelSettings{})

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireMode(objects.QuotaRoutingModeBackpressure)
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireChannelMode(2, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireChannelMapping(1)
		fixture.requireMarker()
	})

	t.Run("rerun is idempotent", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, ExhaustedOnly: true, AllowedChannelIDs: []int{1}})
		fixture.createChannel(1, objects.ChannelSettings{})

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireMode(objects.QuotaRoutingModeRemoveOnExhausted)
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
	})

	t.Run("concurrent startup migration is idempotent", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, AllowedChannelIDs: []int{1}})
		fixture.createChannel(1, objects.ChannelSettings{})

		migrators := []*quotaRoutingMigrator{
			{system: fixture.system, channels: fixture.channels},
			{system: fixture.system, channels: fixture.channels},
		}
		errs := make(chan error, len(migrators))
		var wg sync.WaitGroup
		for _, migrator := range migrators {
			wg.Go(func() {
				errs <- migrator.Migrate(context.Background())
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})

	t.Run("missing channels are skipped", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, ExhaustedOnly: true, AllowedChannelIDs: []int{1, 99}})
		fixture.createChannel(1, objects.ChannelSettings{})

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})

	t.Run("nil channel settings are migrated", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, AllowedChannelIDs: []int{1}})
		fixture.createChannelWithoutSettings(1)

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})

	t.Run("existing new settings do not suppress channel migration", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, DePrioritize: true, AllowedChannelIDs: []int{1}})
		fixture.setRoutingSettings(QuotaRoutingSettings{DefaultMode: objects.QuotaRoutingModeIgnoreQuota})
		fixture.createChannel(1, objects.ChannelSettings{})

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireMode(objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})

	t.Run("empty allowed channels only writes mode and marker", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: false})

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireMode(objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})

	t.Run("absent legacy settings leave defaults untouched", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireNoSystemKey(SystemKeyQuotaRoutingSettings)
		fixture.requireMarker()
	})

	t.Run("channel failure rolls back all migration writes", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixture(t)
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, DePrioritize: true, AllowedChannelIDs: []int{1, 2}})
		fixture.createChannel(1, objects.ChannelSettings{ModelMappings: []objects.ModelMapping{{From: "alias", To: "model"}}})
		fixture.createChannel(2, objects.ChannelSettings{})
		fixture.migrator.beforeChannelUpdate = func(id int) error {
			if id == 2 {
				return fmt.Errorf("injected channel update failure")
			}
			return nil
		}

		require.Error(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireNoSystemKey(SystemKeyQuotaRoutingMigrationDone)
		fixture.requireNoSystemKey(SystemKeyQuotaRoutingSettings)
		fixture.requireChannelMode(1, "")
		fixture.requireChannelMapping(1)

		fixture.migrator.beforeChannelUpdate = nil
		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireMode(objects.QuotaRoutingModeBackpressure)
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireChannelMode(2, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})
}

type quotaRoutingMigrationFixture struct {
	t        *testing.T
	client   *ent.Client
	ctx      context.Context
	system   *SystemService
	channels *ChannelService
	migrator *quotaRoutingMigrator
}

func newQuotaRoutingMigrationFixture(t *testing.T) *quotaRoutingMigrationFixture {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:quota-routing-migration-"+t.Name()+"?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	systemService := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: xcache.Config{Mode: xcache.ModeMemory}})
	channelService := NewChannelServiceForTest(client)
	return &quotaRoutingMigrationFixture{
		t:        t,
		client:   client,
		ctx:      ctx,
		system:   systemService,
		channels: channelService,
		migrator: &quotaRoutingMigrator{system: systemService, channels: channelService},
	}
}

func (f *quotaRoutingMigrationFixture) setLegacy(settings legacyQuotaEnforcementSettings) {
	f.t.Helper()
	value, err := json.Marshal(settings)
	require.NoError(f.t, err)
	_, err = f.client.System.Create().SetKey(SystemKeyQuotaEnforcementSettings).SetValue(string(value)).Save(f.ctx)
	require.NoError(f.t, err)
}

func (f *quotaRoutingMigrationFixture) setRoutingSettings(settings QuotaRoutingSettings) {
	f.t.Helper()
	require.NoError(f.t, f.system.SetQuotaRoutingSettings(f.ctx, settings))
}

func (f *quotaRoutingMigrationFixture) createChannel(id int, settings objects.ChannelSettings) {
	f.t.Helper()
	_, err := f.client.Channel.Create().
		SetName(fmt.Sprintf("channel-%d", id)).
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key"}}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetSettings(&settings).
		Save(f.ctx)
	require.NoError(f.t, err)
}

func (f *quotaRoutingMigrationFixture) createChannelWithoutSettings(id int) {
	f.t.Helper()
	_, err := f.client.Channel.Create().
		SetName(fmt.Sprintf("channel-%d", id)).
		SetType(channel.TypeOpenai).
		SetBaseURL("https://api.openai.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key"}}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SetSettings(nil).
		Save(f.ctx)
	require.NoError(f.t, err)
}

func (f *quotaRoutingMigrationFixture) requireMode(want objects.QuotaRoutingMode) {
	f.t.Helper()
	value, err := f.client.System.Query().Where(system.KeyEQ(SystemKeyQuotaRoutingSettings)).Only(f.ctx)
	require.NoError(f.t, err)
	var settings QuotaRoutingSettings
	require.NoError(f.t, json.Unmarshal([]byte(value.Value), &settings))
	require.Equal(f.t, want, settings.DefaultMode)
}

func (f *quotaRoutingMigrationFixture) requireMarker() {
	f.t.Helper()
	value, err := f.client.System.Query().Where(system.KeyEQ(SystemKeyQuotaRoutingMigrationDone)).Only(f.ctx)
	require.NoError(f.t, err)
	require.Equal(f.t, "true", value.Value)
}

func (f *quotaRoutingMigrationFixture) requireNoSystemKey(key string) {
	f.t.Helper()
	_, err := f.client.System.Query().Where(system.KeyEQ(key)).Only(f.ctx)
	require.Error(f.t, err)
	require.True(f.t, ent.IsNotFound(err))
}

func (f *quotaRoutingMigrationFixture) requireChannelMode(id int, want objects.QuotaRoutingMode) {
	f.t.Helper()
	value, err := f.client.Channel.Query().Where(channel.IDEQ(id)).Only(f.ctx)
	require.NoError(f.t, err)
	require.Equal(f.t, want, value.Settings.QuotaRoutingMode)
}

func (f *quotaRoutingMigrationFixture) requireChannelMapping(id int) {
	f.t.Helper()
	value, err := f.client.Channel.Query().Where(channel.IDEQ(id)).Only(f.ctx)
	require.NoError(f.t, err)
	require.Equal(f.t, []objects.ModelMapping{{From: "alias", To: "model"}}, value.Settings.ModelMappings)
}
