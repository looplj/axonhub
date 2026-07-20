package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/role"
)

// V0_9_39 implements DataMigrator for version 0.9.39 migration.
type V0_9_39 struct{}

// NewV0_9_39 creates the v0.9.39 data migrator.
func NewV0_9_39() DataMigrator {
	return &V0_9_39{}
}

// Version returns the migration version.
func (v *V0_9_39) Version() string {
	return "v0.9.39"
}

// Migrate normalizes legacy system role project IDs to the system sentinel.
func (v *V0_9_39) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")
	_, err := client.Role.Update().
		Where(role.LevelEQ(role.LevelSystem), role.ProjectIDIsNil()).
		SetProjectID(0).
		Save(ctx)

	return err
}
