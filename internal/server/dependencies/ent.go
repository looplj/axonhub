package dependencies

import (
	"context"

	"entgo.io/ent/dialect"

	_ "github.com/mattn/go-sqlite3"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/migrate"
	_ "github.com/looplj/axonhub/internal/ent/runtime"
)

func NewEntClient() *ent.Client {
	client, err := ent.Open(dialect.SQLite, "file:axonhub.db?cache=shared&_fk=1&journal_mode=WAL", ent.Debug())
	if err != nil {
		panic(err)
	}

	if err := client.Schema.Create(
		context.Background(),
		migrate.WithGlobalUniqueID(false),
		migrate.WithForeignKeys(false),
	); err != nil {
		panic(err)
	}
	return client
}
