package datamigrate

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/scopes"
)

// V1_0_0_Beta8 updates the default project Developer role scopes.
type V1_0_0_Beta8 struct{}

// NewV1_0_0_Beta8 creates the v1.0.0-beta8 data migrator.
func NewV1_0_0_Beta8() DataMigrator {
	return &V1_0_0_Beta8{}
}

// Version returns the migration version.
func (v *V1_0_0_Beta8) Version() string {
	return "v1.0.0-beta8"
}

// Migrate removes prompt permissions and adds request read access to unchanged Developer presets.
func (v *V1_0_0_Beta8) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")
	ctx, tx, err := client.OpenTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	txClient := ent.FromContext(ctx)
	roles, err := txClient.Role.Query().Where(
		role.LevelEQ(role.LevelProject),
		role.NameEQ("Developer"),
	).All(ctx)
	if err != nil {
		return fmt.Errorf("query project Developer roles: %w", err)
	}

	legacyScopes := []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadPrompts),
		string(scopes.ScopeWritePrompts),
		string(scopes.ScopeWriteRequests),
	}
	defaultScopes := []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadRequests),
		string(scopes.ScopeWriteRequests),
	}

	for _, developerRole := range roles {
		if !sameScopes(developerRole.Scopes, legacyScopes) {
			continue
		}

		if err := txClient.Role.UpdateOneID(developerRole.ID).SetScopes(defaultScopes).Exec(ctx); err != nil {
			return fmt.Errorf("update Developer role %d scopes: %w", developerRole.ID, err)
		}
	}

	return tx.Commit()
}
