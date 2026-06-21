package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	"github.com/looplj/axonhub/internal/ent/migrate/datamigrate"
	"github.com/looplj/axonhub/internal/ent/migrate/schemahook"
	_ "github.com/looplj/axonhub/internal/ent/runtime"
	_ "github.com/looplj/axonhub/internal/pkg/sqlite"
)

const defaultSQLiteBusyTimeoutMs = 5000

// NewEntClient creates an Ent client. When read_replica.read_dsn is configured,
// SELECT/WITH queries are automatically routed to the replica; all writes go to master.
// Transactions always run on master. If read_dsn is empty, all queries go to master.
func NewEntClient(cfg Config) *ent.Client {
	var opts []ent.Option
	if cfg.Debug {
		opts = append(opts, ent.Debug())
	}

	masterDSN := ensureSQLiteDSN(cfg.Dialect, cfg.DSN, cfg.DisableSQLiteAutoWAL)
	dbDialect, masterDB, err := openDB(cfg.Dialect, masterDSN,
		cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime)
	if err != nil {
		panic(err)
	}

	var drv dialect.Driver
	if cfg.ReadReplica.DSN != "" {
		replicaDSN := ensureSQLiteDSN(cfg.Dialect, cfg.ReadReplica.DSN, cfg.DisableSQLiteAutoWAL)
		readDialect, replicaDB, err := openDB(cfg.Dialect, replicaDSN,
			cfg.ReadReplica.MaxOpenConns, cfg.ReadReplica.MaxIdleConns,
			cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime)
		if err != nil {
			panic(err)
		}
		if readDialect != dbDialect {
			panic(fmt.Errorf("read replica dialect mismatch: got %s, want %s", readDialect, dbDialect))
		}
		masterDriver := entsql.OpenDB(dbDialect, masterDB)
		replicaDriver := entsql.OpenDB(dbDialect, replicaDB)
		drv = newRouterDriver(masterDriver, replicaDriver)
	} else {
		drv = entsql.OpenDB(dbDialect, masterDB)
	}

	opts = append(opts, ent.Driver(drv))
	client := ent.NewClient(opts...)

	if !cfg.DisableAutoMigration {
		// Pre-migration guard: the new api_keys_by_project_name unique index can't
		// be created on a deployment that already holds duplicate live
		// (project_id, name) api_keys (possible because name uniqueness was only
		// app-level and non-atomic before this — and absent entirely before #1292).
		// Auto-migration would otherwise panic with an opaque DDL error. Fail fast
		// instead with an actionable message and let the operator resolve the
		// duplicates; never mutate the keys themselves. No-op on a fresh DB.
		if err := assertNoDuplicateAPIKeyNames(context.Background(), masterDB, dbDialect); err != nil {
			panic(err)
		}

		err = client.Schema.Create(
			context.Background(),
			migrate.WithGlobalUniqueID(false),
			migrate.WithForeignKeys(false),
			migrate.WithDropIndex(true),
			migrate.WithDropColumn(true),
			schema.WithHooks(schemahook.V0_3_0),
		)
		if err != nil {
			panic(err)
		}

		migrator := datamigrate.NewMigrator(client)
		if err := migrator.Run(context.Background()); err != nil {
			panic(err)
		}
	}

	return client
}

// ensureSQLiteDSN appends SQLite PRAGMA DSN parameters for modernc.org/sqlite when absent.
// Users can override any pragma by setting it explicitly in the DSN.
func ensureSQLiteDSN(dialectName, dsn string, disableWAL bool) string {
	switch dialectName {
	case "sqlite3", "sqlite":
		if !disableWAL && !strings.Contains(dsn, "journal_mode") {
			dsn = appendSQLiteDSNParam(dsn, "_pragma=journal_mode(WAL)")
		}
		if !strings.Contains(dsn, "busy_timeout") {
			dsn = appendSQLiteDSNParam(dsn, fmt.Sprintf("_pragma=busy_timeout(%d)", defaultSQLiteBusyTimeoutMs))
		}
	}
	return dsn
}

func appendSQLiteDSNParam(dsn, param string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + param
	}

	return dsn + "?" + param
}

// openDB opens a sql.DB for the given dialect and DSN, applies pool settings,
// and returns the ent dialect string along with the DB handle.
func openDB(dialectName, dsn string, maxOpen, maxIdle int, maxLifetime, maxIdleTime time.Duration) (string, *sql.DB, error) {
	ed, err := entDialect(dialectName)
	if err != nil {
		return "", nil, err
	}

	drvName, err := driverName(dialectName)
	if err != nil {
		return "", nil, err
	}

	sqlDB, err := sql.Open(drvName, dsn)
	if err != nil {
		return "", nil, err
	}

	if maxOpen > 0 {
		sqlDB.SetMaxOpenConns(maxOpen)
	}
	if maxIdle > 0 {
		sqlDB.SetMaxIdleConns(maxIdle)
	}
	if maxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(maxLifetime)
	}
	if maxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(maxIdleTime)
	}

	return ed, sqlDB, nil
}

func driverName(dialectName string) (string, error) {
	switch dialectName {
	case "postgres", "pgx", "postgresdb", "pg", "postgresql":
		return "pgx", nil
	case "sqlite3", "sqlite":
		return "sqlite3", nil
	case "mysql", "tidb":
		return "mysql", nil
	default:
		return "", fmt.Errorf("invalid dialect: %s", dialectName)
	}
}

func entDialect(dialectName string) (string, error) {
	switch dialectName {
	case "postgres", "pgx", "postgresdb", "pg", "postgresql":
		return dialect.Postgres, nil
	case "sqlite3", "sqlite":
		return dialect.SQLite, nil
	case "mysql", "tidb":
		return dialect.MySQL, nil
	default:
		return "", fmt.Errorf("invalid dialect: %s", dialectName)
	}
}

// assertNoDuplicateAPIKeyNames fails fast (returning a non-nil error) when the
// api_keys table already holds more than one live row sharing the same
// (project_id, name). It must run before the api_keys_by_project_name unique
// index is created, turning what would otherwise be an opaque "CREATE UNIQUE
// INDEX failed" panic into an actionable message that lists the conflicting
// keys. It deliberately does NOT mutate any data — the operator decides which
// keys to rename or disable, since each is a real, in-use credential. It is a
// no-op when the api_keys table doesn't exist yet (fresh database).
func assertNoDuplicateAPIKeyNames(ctx context.Context, db *sql.DB, dbDialect string) error {
	exists, err := apiKeysTableExists(ctx, db, dbDialect)
	if err != nil {
		return fmt.Errorf("check api_keys table existence: %w", err)
	}
	if !exists {
		return nil
	}

	rows, err := db.QueryContext(ctx, `SELECT project_id, name
FROM api_keys
WHERE deleted_at = 0
GROUP BY project_id, name
HAVING COUNT(*) > 1
ORDER BY project_id, name`)
	if err != nil {
		return fmt.Errorf("scan api_keys for duplicate names: %w", err)
	}
	defer rows.Close()

	type dupGroup struct {
		projectID int
		name      string
	}

	var groups []dupGroup

	for rows.Next() {
		var g dupGroup
		if err := rows.Scan(&g.projectID, &g.name); err != nil {
			return fmt.Errorf("scan duplicate api_keys row: %w", err)
		}

		groups = append(groups, g)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate duplicate api_keys rows: %w", err)
	}

	if len(groups) == 0 {
		return nil
	}

	ph1, ph2 := "?", "?"
	if dbDialect == dialect.Postgres {
		ph1, ph2 = "$1", "$2"
	}

	idQuery := fmt.Sprintf(
		"SELECT id FROM api_keys WHERE deleted_at = 0 AND project_id = %s AND name = %s ORDER BY id",
		ph1, ph2,
	)

	var b strings.Builder

	b.WriteString("cannot create unique index api_keys_by_project_name: duplicate api key names found (resolve, then restart):\n")

	for _, g := range groups {
		ids, err := queryAPIKeyIDs(ctx, db, idQuery, g.projectID, g.name)
		if err != nil {
			return err
		}

		idStrs := make([]string, len(ids))
		for i, id := range ids {
			idStrs[i] = strconv.Itoa(id)
		}

		fmt.Fprintf(&b, "  - project_id=%d name=%q ids=[%s]\n", g.projectID, g.name, strings.Join(idStrs, ", "))
	}

	b.WriteString("Rename or disable the extra keys (e.g. via the dashboard), then restart.")

	return errors.New(b.String())
}

// queryAPIKeyIDs returns the ids of the live api_keys matching (project_id, name),
// used to enumerate the conflicting keys in the duplicate-name error message.
func queryAPIKeyIDs(ctx context.Context, db *sql.DB, query string, projectID int, name string) ([]int, error) {
	rows, err := db.QueryContext(ctx, query, projectID, name)
	if err != nil {
		return nil, fmt.Errorf("query duplicate api_key ids: %w", err)
	}
	defer rows.Close()

	var ids []int

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan api_key id: %w", err)
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api_key ids: %w", err)
	}

	return ids, nil
}

// apiKeysTableExists reports whether the api_keys table is present, so the
// dedupe step can be skipped on a fresh database where Schema.Create will create
// the table (and the unique index) from scratch.
func apiKeysTableExists(ctx context.Context, db *sql.DB, dbDialect string) (bool, error) {
	var query string

	switch dbDialect {
	case dialect.SQLite:
		query = "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'api_keys'"
	case dialect.MySQL:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'api_keys'"
	case dialect.Postgres:
		query = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'api_keys'"
	default:
		return false, fmt.Errorf("unsupported dialect: %s", dbDialect)
	}

	var count int
	if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}
