package biz

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

// SQLite accepts ON CONFLICT DO UPDATE without a conflict target, PostgreSQL does not,
// so the migration claim must be exercised against a real PostgreSQL server.
func TestQuotaRoutingMigration_Postgres(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PG_DSN not set; skipping real-Postgres quota routing migration")
	}

	t.Run("absent legacy settings write the marker", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixtureWithClient(t, newPostgresQuotaRoutingMigrationClient(t, dsn))

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireNoSystemKey(SystemKeyQuotaRoutingSettings)
		fixture.requireMarker()
	})

	t.Run("legacy settings and allowed channels are migrated once", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixtureWithClient(t, newPostgresQuotaRoutingMigrationClient(t, dsn))
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, DePrioritize: true, AllowedChannelIDs: []int{1}})
		fixture.createChannel(1, objects.ChannelSettings{})

		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		require.NoError(t, fixture.migrator.Migrate(context.Background()))
		fixture.requireMode(objects.QuotaRoutingModeBackpressure)
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})

	t.Run("concurrent startup migration is idempotent", func(t *testing.T) {
		fixture := newQuotaRoutingMigrationFixtureWithClient(t, newPostgresQuotaRoutingMigrationClient(t, dsn))
		fixture.setLegacy(legacyQuotaEnforcementSettings{Enabled: true, AllowedChannelIDs: []int{1}})
		fixture.createChannel(1, objects.ChannelSettings{})

		errs := make(chan error, 4)
		var wg sync.WaitGroup
		for range cap(errs) {
			migrator := &quotaRoutingMigrator{system: fixture.system, channels: fixture.channels}
			wg.Go(func() {
				errs <- migrator.Migrate(context.Background())
			})
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		fixture.requireMode(objects.QuotaRoutingModeRemoveOnExhausted)
		fixture.requireChannelMode(1, objects.QuotaRoutingModeIgnoreQuota)
		fixture.requireMarker()
	})
}

// newPostgresQuotaRoutingMigrationClient isolates each test in a throwaway schema so
// singleton system keys and channel IDs never collide with existing data.
func newPostgresQuotaRoutingMigrationClient(t *testing.T, dsn string) *ent.Client {
	t.Helper()

	config, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)

	adminDB := stdlib.OpenDB(*config.Copy())
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := "quota_routing_" + uuid.NewString()[:8]
	_, err = adminDB.ExecContext(context.Background(), fmt.Sprintf("CREATE SCHEMA %q", schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA %q CASCADE", schema))
	})

	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	require.NoError(t, db.PingContext(context.Background()))

	client := enttest.NewClient(t, enttest.WithOptions(ent.Driver(entsql.OpenDB(dialect.Postgres, db))))
	t.Cleanup(func() { _ = client.Close() })

	return client
}
