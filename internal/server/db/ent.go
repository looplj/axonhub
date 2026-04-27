package db

import (
	"context"
	"database/sql"
	"fmt"
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

// NewEntClient creates an Ent client. When read_replica.read_dsn is configured,
// SELECT/WITH queries are automatically routed to the replica; all writes go to master.
// Transactions always run on master. If read_dsn is empty, all queries go to master.
func NewEntClient(cfg Config) *ent.Client {
	var opts []ent.Option
	if cfg.Debug {
		opts = append(opts, ent.Debug())
	}

	dbDialect, masterDB, err := openDB(cfg.Dialect, cfg.DSN,
		cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime)
	if err != nil {
		panic(err)
	}

	var drv dialect.Driver
	if cfg.ReadReplica.DSN != "" {
		readDialect, replicaDB, err := openDB(cfg.Dialect, cfg.ReadReplica.DSN,
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

	return client
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
