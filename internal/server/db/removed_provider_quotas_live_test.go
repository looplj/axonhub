package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

// Live-server tests for cleanupRemovedProviderQuotaTypes. The SQLite coverage
// in removed_provider_quotas_test.go uses a hand-built table with a text
// provider_type; the upgrade hazard this purge exists for is MySQL/TiDB-only
// (a real ENUM column rewritten in place), so the purge must be proven against
// a real server. On MySQL/TiDB the table is created exactly like an
// installation being upgraded: the enum still contains the removed value
// wafer. The PostgreSQL test exercises the postgres cleanup query end to
// end ($1 placeholders and the ANY(current_schemas(false)) existence check,
// the table living in a throwaway schema off the search path).
//
// The tests are opt-in and skip when the DSN env vars are unset (e.g. in CI):
//
//	AXONHUB_TEST_MYSQL_DSN="root:root@tcp(localhost:3306)/axonhub?parseTime=true" \
//	AXONHUB_TEST_TIDB_DSN="root@tcp(localhost:4000)/axonhub?parseTime=true" \
//	AXONHUB_TEST_PG_DSN="postgres://postgres:postgres@localhost:55432/axonhub?sslmode=disable" \
//	  go test ./internal/server/db/ -run TestCleanupRemovedProviderQuotaTypes -v -count=1
//
// The MySQL/TiDB connection must be allowed to CREATE/DROP DATABASE
// (the test works in a throwaway database so the DSN's database is untouched).

var quotaPurgeDialects = []struct {
	name    string
	dialect string
	envVar  string
}{
	{name: "mysql", dialect: "mysql", envVar: "AXONHUB_TEST_MYSQL_DSN"},
	{name: "tidb", dialect: "tidb", envVar: "AXONHUB_TEST_TIDB_DSN"},
}

// legacyQuotaPurgeTableDDLMySQL creates provider_quota_status the way an
// installation being upgraded has it: the provider_type enum still contains
// the removed value wafer alongside every current value. Kept portable across
// MySQL and TiDB (no InnoDB-only options, plain AUTO_INCREMENT + ENUM).
const legacyQuotaPurgeTableDDLMySQL = `CREATE TABLE provider_quota_status (
	id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	deleted_at INT NOT NULL DEFAULT 0,
	provider_type ENUM('claudecode','codex','antigravity','xai_subscription','github_copilot','nanogpt','cline','synthetic','neuralwatt','apertis','opencode_go','kimi_code','minimax','zhipu','zai','charm_hyper','zenmux','commandcode','ollama','wafer') NOT NULL,
	status ENUM('available','warning','exhausted','unknown') NOT NULL,
	quota_data JSON NOT NULL,
	next_reset_at DATETIME NULL,
	ready TINYINT(1) NOT NULL DEFAULT 1,
	next_check_at DATETIME NOT NULL,
	account_key VARCHAR(255) NOT NULL DEFAULT '',
	channel_id INT NOT NULL,
	UNIQUE KEY providerquotastatus_channel_id (channel_id)
)`

// TestCleanupRemovedProviderQuotaTypes_MySQLTiDB proves the purge deletes
// both live and soft-deleted rows of a removed provider against a real
// ENUM column, leaving rows of surviving providers untouched.
func TestCleanupRemovedProviderQuotaTypes_MySQLTiDB(t *testing.T) {
	ran := 0

	for _, d := range quotaPurgeDialects {
		dsn := os.Getenv(d.envVar)
		if dsn == "" {
			t.Logf("skipping %s: set %s to run", d.name, d.envVar)
			continue
		}

		ran++

		t.Run(d.name, func(t *testing.T) {
			runQuotaPurgeMySQL(t, d.dialect, dsn)
		})
	}

	if ran == 0 {
		t.Skip("set AXONHUB_TEST_MYSQL_DSN and/or AXONHUB_TEST_TIDB_DSN to run the live MySQL/TiDB purge test")
	}
}

func runQuotaPurgeMySQL(t *testing.T, dialectName, dsn string) {
	t.Helper()
	ctx := context.Background()

	cfg, err := mysql.ParseDSN(dsn)
	require.NoError(t, err)

	admin, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	require.NoError(t, admin.PingContext(ctx))
	t.Cleanup(func() { _ = admin.Close() })

	// Isolate the test in a throwaway database so the DSN's database is
	// never touched; tableExists's information_schema check then resolves
	// against this database via DATABASE() exactly like production.
	dbName := "qc_" + uuid.NewString()[:8]
	_, err = admin.Exec("CREATE DATABASE IF NOT EXISTS " + dbName)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + dbName) })

	cfg.DBName = dbName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(legacyQuotaPurgeTableDDLMySQL)
	require.NoError(t, err)

	insertQuotaPurgeRows(t, db, `
		(1001, 'wafer', 'exhausted', '{}', NULL, 0, NOW(), '', 0),
		(1002, 'wafer', 'available', '{}', NULL, 1, NOW(), '', 1798761600),
		(1003, 'synthetic', 'available', '{}', NULL, 1, NOW(), '', 0)`)

	assertQuotaPurgeResult(t, db, dialectName)
}

// insertQuotaPurgeRows inserts a live wafer row, a soft-deleted wafer row
// (nonzero deleted_at), and a live row of a still-supported provider.
func insertQuotaPurgeRows(t *testing.T, db *sql.DB, rows string) {
	t.Helper()

	_, err := db.Exec(`INSERT INTO provider_quota_status
		(channel_id, provider_type, status, quota_data, next_reset_at, ready, next_check_at, account_key, deleted_at)
		VALUES ` + rows)
	require.NoError(t, err)
}

// assertQuotaPurgeResult runs the cleanup and asserts that both wafer rows
// are gone, the supported provider's row survives, and a second run is a
// clean no-op.
func assertQuotaPurgeResult(t *testing.T, db *sql.DB, dialectName string) {
	t.Helper()
	ctx := context.Background()

	require.NoError(t, cleanupRemovedProviderQuotaTypes(ctx, db, dialectName))

	if got := countQuotaRows(t, db, "wafer", false); got != 0 {
		t.Fatalf("live wafer rows remain: %d", got)
	}
	if got := countQuotaRows(t, db, "wafer", true); got != 0 {
		t.Fatalf("soft-deleted wafer rows remain: %d", got)
	}
	if got := countQuotaRows(t, db, "synthetic", false); got != 1 {
		t.Fatalf("synthetic rows = %d, want 1", got)
	}

	// Idempotent: a second run does nothing and returns no error.
	require.NoError(t, cleanupRemovedProviderQuotaTypes(ctx, db, dialectName))
}

// legacyQuotaPurgeTableDDLPostgres matches the production PostgreSQL shape:
// Ent stores enum fields as varchar there, so the purge's $1 placeholder
// delete is what has to work.
const legacyQuotaPurgeTableDDLPostgres = `CREATE TABLE provider_quota_status (
	id BIGSERIAL PRIMARY KEY,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deleted_at INTEGER NOT NULL DEFAULT 0,
	provider_type VARCHAR(255) NOT NULL,
	status VARCHAR(255) NOT NULL,
	quota_data JSONB NOT NULL,
	next_reset_at TIMESTAMPTZ,
	ready BOOLEAN NOT NULL DEFAULT true,
	next_check_at TIMESTAMPTZ NOT NULL,
	account_key VARCHAR(255) NOT NULL DEFAULT '',
	channel_id INTEGER NOT NULL,
	UNIQUE (channel_id)
)`

// TestCleanupRemovedProviderQuotaTypes_Postgres proves the postgres cleanup
// query (ANY(current_schemas(false)) existence check plus $1 placeholder
// delete) purges live and soft-deleted rows of a removed provider. The table
// lives in a throwaway schema on the search path — not the default public
// schema — so the existence check's current_schemas(false) scoping is
// genuinely exercised.
func TestCleanupRemovedProviderQuotaTypes_Postgres(t *testing.T) {
	dsn := os.Getenv("AXONHUB_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("AXONHUB_TEST_PG_DSN not set; skipping real-Postgres quota purge")
	}
	runQuotaPurgePostgres(t, dsn)
}

func runQuotaPurgePostgres(t *testing.T, dsn string) {
	t.Helper()
	ctx := context.Background()

	config, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)

	adminDB := stdlib.OpenDB(*config.Copy())
	require.NoError(t, adminDB.PingContext(ctx))
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := "quota_purge_" + uuid.NewString()[:8]
	_, err = adminDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %q", schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema))
	})

	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	require.NoError(t, db.PingContext(ctx))
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(legacyQuotaPurgeTableDDLPostgres)
	require.NoError(t, err)

	insertQuotaPurgeRows(t, db, `
		(2001, 'wafer', 'exhausted', '{}', NULL, false, now(), '', 0),
		(2002, 'wafer', 'available', '{}', NULL, true, now(), '', 1798761600),
		(2003, 'synthetic', 'available', '{}', NULL, true, now(), '', 0)`)

	require.NoError(t, cleanupRemovedProviderQuotaTypes(ctx, db, "postgres"))

	if got := countQuotaPurgeRowsPG(t, db, "wafer", false); got != 0 {
		t.Fatalf("live wafer rows remain: %d", got)
	}
	if got := countQuotaPurgeRowsPG(t, db, "wafer", true); got != 0 {
		t.Fatalf("soft-deleted wafer rows remain: %d", got)
	}
	if got := countQuotaPurgeRowsPG(t, db, "synthetic", false); got != 1 {
		t.Fatalf("synthetic rows = %d, want 1", got)
	}

	// Idempotent: a second run does nothing and returns no error.
	require.NoError(t, cleanupRemovedProviderQuotaTypes(ctx, db, "postgres"))
}

func countQuotaPurgeRowsPG(t *testing.T, db *sql.DB, providerType string, softDeleted bool) int {
	t.Helper()

	deletedClause := "deleted_at = 0"
	if softDeleted {
		deletedClause = "deleted_at > 0"
	}

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM provider_quota_status
		 WHERE provider_type = $1 AND `+deletedClause, providerType).Scan(&count)
	require.NoError(t, err)
	return count
}
