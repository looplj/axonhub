package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// DataMigrator is an interface for data migration operations.
type DataMigrator interface {
	Migrate(ctx context.Context, client *ent.Client) error
}
