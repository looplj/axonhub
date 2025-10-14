package enttest

import (
	"database/sql"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	_ "github.com/looplj/axonhub/internal/pkg/sqlite"
)

func NewEntClient(t TestingT, driverName, dataSourceName string) *ent.Client {
	sqlDB, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		panic(err)
	}

	return NewClient(t,
		WithOptions(
			ent.Driver(entsql.OpenDB(driverName, sqlDB)),
		),
		WithMigrateOptions(
			migrate.WithGlobalUniqueID(false),
			migrate.WithForeignKeys(false),
		),
	)
}
