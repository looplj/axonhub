package datamigrate

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
)

// V1_0_0_Beta9 implements DataMigrator for version 1.0.0-beta9 migration.
type V1_0_0_Beta9 struct{}

// NewV1_0_0_Beta9 creates the v1.0.0-beta9 data migrator.
func NewV1_0_0_Beta9() *V1_0_0_Beta9 {
	return &V1_0_0_Beta9{}
}

// Version returns the migration version.
func (v *V1_0_0_Beta9) Version() string {
	return "v1.0.0-beta9"
}

// Migrate performs the v1.0.0-beta9 data cleanup:
//
//  1. Purges the stale settings.providerQuota key from channels.
//     The old OpenCode Go quota checker stored workspaceId and a live auth cookie
//     under settings.providerQuota.opencodeGo. Quota now uses the official usage
//     API keyed by the channel's own API key, so the key is never read again and
//     must not linger in the database (it may hold a live session credential).
//
//  2. Strips the monotonic-clock suffix from channel updated_at values.
//     Setting a model price used to write time.Now() (with its "m=" monotonic
//     reading and local timezone) into channels.updated_at. The SQLite driver
//     serializes such values with time.Time.String(), persisting e.g.
//     "2026-08-18 00:13:11.396794292 +0800 CST m=+0.000011483" instead of the
//     canonical UTC format produced by the updated_at mixin (xtime.UTCNow).
//     When UpdateChannel later edits type/baseURL it snapshots updated_at and
//     appends WHERE updated_at = <snapshot>; the driver drops the "m=+..."
//     suffix on scan, so the re-serialized value never matches the stored
//     string and every update fails with "channel was updated concurrently".
//     This rewrites affected rows to the same string the driver produces after
//     a round-trip, making the optimistic-lock comparison consistent again.
func (v *V1_0_0_Beta9) Migrate(ctx context.Context, client *ent.Client) (retErr error) {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")

	// Use the dialect.Driver.Exec interface rather than asserting a concrete
	// *entsql.Driver: under read-replica configuration the ent client wraps the
	// driver in a routerDriver (internal/server/db), which still implements
	// dialect.Driver.Exec (routing writes to the master).
	dialectName := client.Driver().Dialect()

	if retErr = v.purgeProviderQuota(ctx, client, dialectName); retErr != nil {
		return retErr
	}
	return v.stripMonotonicUpdatedAt(ctx, client, dialectName)
}

// purgeProviderQuota removes the obsolete settings.providerQuota key.
func (v *V1_0_0_Beta9) purgeProviderQuota(ctx context.Context, client *ent.Client, dialectName string) error {
	var stmt string
	switch dialectName {
	case dialect.Postgres:
		stmt = `UPDATE channels SET settings = settings #- '{providerQuota}' WHERE settings ? 'providerQuota'`
	case dialect.SQLite:
		// json_type (not json_extract) distinguishes a present JSON-null value
		// from an absent key, so "providerQuota": null is purged as well.
		stmt = `UPDATE channels SET settings = json_remove(settings, '$.providerQuota') WHERE json_type(settings, '$.providerQuota') IS NOT NULL`
	default:
		log.Info(ctx, "Unsupported dialect, skipping stale providerQuota cleanup",
			log.String("dialect", dialectName))
		return nil
	}

	return v.execWithAffectedLog(ctx, client, stmt,
		"failed to purge settings.providerQuota",
		"Purged stale settings.providerQuota from channels")
}

// stripMonotonicUpdatedAt removes the " m=+..." monotonic suffix from
// channels.updated_at values written by the old channel_price.go time.Now().
func (v *V1_0_0_Beta9) stripMonotonicUpdatedAt(ctx context.Context, client *ent.Client, dialectName string) error {
	var stmt string
	switch dialectName {
	case dialect.Postgres:
		stmt = `UPDATE channels SET updated_at = split_part(updated_at, ' m=', 1) WHERE updated_at LIKE '% m=%'`
	case dialect.SQLite:
		stmt = `UPDATE channels SET updated_at = substr(updated_at, 1, instr(updated_at, ' m=') - 1) WHERE updated_at LIKE '% m=%'`
	default:
		// MySQL/MariaDB and other dialects are not in scope for this cleanup;
		// the "m=" suffix only ever originates from the default driver format.
		log.Info(ctx, "Unsupported dialect, skipping channel updated_at monotonic cleanup",
			log.String("dialect", dialectName))
		return nil
	}

	return v.execWithAffectedLog(ctx, client, stmt,
		"failed to strip monotonic suffix from channels.updated_at",
		"Stripped monotonic suffix from channels.updated_at")
}

// execWithAffectedLog runs a raw UPDATE and logs the number of affected rows.
func (v *V1_0_0_Beta9) execWithAffectedLog(ctx context.Context, client *ent.Client, stmt, errMsg, logMsg string) error {
	var result sql.Result
	if err := client.Driver().Exec(ctx, stmt, []any{}, &result); err != nil {
		return fmt.Errorf("%s: %w", errMsg, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		log.Warn(ctx, "failed to read affected rows", log.Cause(err))
	} else {
		log.Info(ctx, logMsg, log.Int64("affected", affected))
	}

	return nil
}