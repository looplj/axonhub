package datamigrate

import (
	"context"
	"fmt"
	"slices"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/schema/schematype"
	"github.com/looplj/axonhub/internal/ent/userrole"
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
func (v *V1_0_0_Beta6) Migrate(ctx context.Context, client *ent.Client) (err error) {
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
	roles, err := txClient.Role.Query().
		Where(
			role.LevelEQ(role.LevelSystem),
			role.Or(role.ProjectIDIsNil(), role.ProjectIDEQ(0)),
		).
		Order(ent.Asc(role.FieldID)).
		All(ctx)
	if err != nil {
		return err
	}

	rolesByName := make(map[string][]*ent.Role)
	for _, systemRole := range roles {
		rolesByName[systemRole.Name] = append(rolesByName[systemRole.Name], systemRole)
	}

	type mergePlan struct {
		canonical  *ent.Role
		duplicates []*ent.Role
	}

	plans := make([]mergePlan, 0, len(rolesByName))
	for name, roleGroup := range rolesByName {
		var (
			canonical *ent.Role
			hasLegacy bool
		)

		for _, systemRole := range roleGroup {
			if systemRole.ProjectID == nil {
				hasLegacy = true
			} else if *systemRole.ProjectID == 0 {
				canonical = systemRole
			}
		}
		if !hasLegacy {
			continue
		}
		if canonical == nil {
			canonical = roleGroup[0]
		}

		for _, systemRole := range roleGroup {
			if !sameScopes(canonical.Scopes, systemRole.Scopes) {
				return fmt.Errorf("cannot merge duplicate system roles named %q with different scopes", name)
			}
		}

		plan := mergePlan{canonical: canonical}
		for _, systemRole := range roleGroup {
			if systemRole.ID != canonical.ID {
				plan.duplicates = append(plan.duplicates, systemRole)
			}
		}
		plans = append(plans, plan)
	}

	for _, plan := range plans {
		for _, duplicate := range plan.duplicates {
			assignments, err := txClient.UserRole.Query().
				Where(userrole.RoleID(duplicate.ID)).
				All(ctx)
			if err != nil {
				return err
			}
			for _, assignment := range assignments {
				err = txClient.UserRole.Create().
					SetUserID(assignment.UserID).
					SetRoleID(plan.canonical.ID).
					Exec(ctx)
				if err != nil && !ent.IsConstraintError(err) {
					return err
				}
			}
			if _, err = txClient.UserRole.Delete().Where(userrole.RoleID(duplicate.ID)).Exec(ctx); err != nil {
				return err
			}
			if err = txClient.Role.DeleteOneID(duplicate.ID).Exec(schematype.SkipSoftDelete(ctx)); err != nil {
				return err
			}
		}
		if plan.canonical.ProjectID == nil {
			if err = txClient.Role.UpdateOneID(plan.canonical.ID).SetProjectID(0).Exec(ctx); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func sameScopes(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a = append([]string(nil), a...)
	b = append([]string(nil), b...)
	slices.Sort(a)
	slices.Sort(b)

	return slices.Equal(a, b)
}
