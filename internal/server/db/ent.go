package db

import (
	"context"
	"database/sql"

	"entgo.io/ent/dialect"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	_ "github.com/looplj/axonhub/internal/ent/runtime"
)

func NewEntClient(cfg Config) *ent.Client {
	var opts []ent.Option
	if cfg.Debug {
		opts = append(opts, ent.Debug())
	}

	var client *ent.Client

	switch cfg.Dialect {
	case "postgres":
		db, err := sql.Open("pgx", cfg.DSN)
		if err != nil {
			panic(err)
		}

		drv := entsql.OpenDB(dialect.Postgres, db)

		opts = append(opts, ent.Driver(drv))
		client = ent.NewClient(opts...)
	default:
		var err error

		client, err = ent.Open(
			cfg.Dialect,
			cfg.DSN,
			opts...,
		)
		if err != nil {
			panic(err)
		}
	}

	err := client.Schema.Create(
		context.Background(),
		migrate.WithGlobalUniqueID(false),
		migrate.WithForeignKeys(false),
	)
	if err != nil {
		panic(err)
	}

	return client
}
