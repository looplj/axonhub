package datamigrate

import (
	"context"
	"fmt"
	"time"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/invitation"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/scopes"
)

// V1_0_0_Beta7 implements DataMigrator for version 1.0.0-beta7 migration.
type V1_0_0_Beta7 struct{}

// NewV1_0_0_Beta7 creates the v1.0.0-beta7 data migrator.
func NewV1_0_0_Beta7() DataMigrator {
	return &V1_0_0_Beta7{}
}

// Version returns the migration version.
func (v *V1_0_0_Beta7) Version() string {
	return "v1.0.0-beta7"
}

// Migrate binds legacy invitations to the project's default Developer role.
func (v *V1_0_0_Beta7) Migrate(ctx context.Context, client *ent.Client) (err error) {
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

	legacyInvitations, err := txClient.Invitation.Query().Where(
		invitation.RoleIDIsNil(),
		invitation.DeletedAtEQ(0),
	).All(ctx)
	if err != nil {
		return err
	}
	activeInvitations := legacyInvitations[:0]
	now := time.Now()
	for _, legacyInvitation := range legacyInvitations {
		if legacyInvitation.ExpiresAt != nil && !legacyInvitation.ExpiresAt.After(now) {
			continue
		}
		if legacyInvitation.MaxUses > 0 && legacyInvitation.UsedCount >= legacyInvitation.MaxUses {
			continue
		}
		activeInvitations = append(activeInvitations, legacyInvitation)
	}
	legacyInvitations = activeInvitations

	for _, legacyInvitation := range legacyInvitations {
		// Remove a stale project role only when this project needs invitation backfill.
		softDeletedDevelopers, err := txClient.Role.Query().Where(
			role.LevelEQ(role.LevelProject),
			role.ProjectIDEQ(legacyInvitation.ProjectID),
			role.NameEQ("Developer"),
		).All(schematype.SkipSoftDelete(ctx))
		if err != nil {
			return fmt.Errorf("query soft-deleted Developer roles for project %d: %w", legacyInvitation.ProjectID, err)
		}
		for _, dr := range softDeletedDevelopers {
			if dr.DeletedAt == 0 {
				continue
			}
			if _, err := txClient.UserRole.Delete().Where(userrole.RoleID(dr.ID)).Exec(ctx); err != nil {
				return fmt.Errorf("clear stale UserRole rows for Developer role %d: %w", dr.ID, err)
			}
			if err := txClient.Role.DeleteOneID(dr.ID).Exec(schematype.SkipSoftDelete(ctx)); err != nil {
				return fmt.Errorf("permanently delete soft-deleted Developer role %d: %w", dr.ID, err)
			}
		}

		developerRole, err := txClient.Role.Query().Where(
			role.LevelEQ(role.LevelProject),
			role.ProjectIDEQ(legacyInvitation.ProjectID),
			role.NameEQ("Developer"),
		).Only(ctx)
		if ent.IsNotFound(err) {
			developerRole, err = txClient.Role.Create().
				SetName("Developer").
				SetLevel(role.LevelProject).
				SetProjectID(legacyInvitation.ProjectID).
				SetScopes([]string{
					string(scopes.ScopeReadAPIKeys),
					string(scopes.ScopeWriteAPIKeys),
					string(scopes.ScopeReadRequests),
					string(scopes.ScopeWriteRequests),
				}).
				Save(ctx)
		}
		if err != nil {
			return fmt.Errorf("find Developer role for legacy invitation %d: %w", legacyInvitation.ID, err)
		}
		if err := txClient.Invitation.UpdateOneID(legacyInvitation.ID).SetRoleID(developerRole.ID).Exec(ctx); err != nil {
			return fmt.Errorf("assign Developer role to legacy invitation %d: %w", legacyInvitation.ID, err)
		}
	}

	return tx.Commit()
}
