package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/role"
)

// V1_0_0_Beta6 implements DataMigrator for version 1.0.0-beta6 migration.
type V1_0_0_Beta6 struct{}

// NewV1_0_0_Beta6 creates the v1.0.0-beta6 data migrator.
func NewV1_0_0_Beta6() DataMigrator {
	return &V1_0_0_Beta6{}
}

// Version returns the migration version.
func (v *V1_0_0_Beta6) Version() string {
	return "v1.0.0-beta6"
}

// Migrate normalizes legacy system role project IDs to the system sentinel.
func (v *V1_0_0_Beta6) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")
	_, err := client.Role.Update().
		Where(role.LevelEQ(role.LevelSystem), role.ProjectIDIsNil()).
		SetProjectID(0).
		Save(ctx)

	return err
}
