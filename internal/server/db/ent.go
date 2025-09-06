package db

import (
	"context"
	"database/sql"
	"fmt"

	"entgo.io/ent/dialect"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	_ "github.com/looplj/axonhub/internal/ent/runtime"
	_ "github.com/looplj/axonhub/internal/pkg/sqlite"
)

func NewEntClient(cfg Config) *ent.Client {
	var opts []ent.Option
	if cfg.Debug {
		opts = append(opts, ent.Debug())
	}

	var (
		sqlDB     *sql.DB
		dbDialect string
		err       error
	)

	switch cfg.Dialect {
	case "postgres", "pgx", "postgresdb", "pg", "postgresql":
		sqlDB, err = sql.Open("pgx", cfg.DSN)
		if err != nil {
			panic(err)
		}

		dbDialect = dialect.Postgres
	case "sqlite3", "sqlite":
		sqlDB, err = sql.Open("sqlite3", cfg.DSN)
		if err != nil {
			panic(err)
		}

		dbDialect = dialect.SQLite
	case "mysql", "tidb":
		sqlDB, err = sql.Open("mysql", cfg.DSN)
		if err != nil {
			panic(err)
		}

		dbDialect = dialect.MySQL
	default:
		panic(fmt.Errorf("invalid dialect: %s", cfg.Dialect))
	}
	drv := entsql.OpenDB(dbDialect, sqlDB)
	opts = append(opts, ent.Driver(drv))
	client := ent.NewClient(opts...)

	err = client.Schema.Create(
		context.Background(),
		migrate.WithGlobalUniqueID(false),
		migrate.WithForeignKeys(false),
	)
	if err != nil {
		panic(err)
	}
	return client
}
