package datamigrate

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// V0_3_0 implements DataMigrator for version 0.3.0 migration.
type V0_3_0 struct{}

// NewV0_3_0 creates a new V0_3_0 data migrator.
func NewV0_3_0() DataMigrator {
	return &V0_3_0{}
}

// Version returns the version of this migrator.
func (v *V0_3_0) Version() string {
	return "v0.3.0"
}

// Migrate performs the version 0.3.0 data migration.
func (v *V0_3_0) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = authz.WithSystemBypass(context.Background(), "database-migrate")
	// Check if a project already exists
	_, err = client.Project.Query().Limit(1).First(ctx)
	if err == nil {
		log.Info(ctx, "existed project found, skip migration")
		return nil
	}

	ctx, tx, err := client.OpenTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	proj, owner, err := v.createDefaultProject(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// No owner user exists yet, skip project creation
			// Project will be created when the system is initialized
			log.Info(ctx, "no owner user found, skip migration")
			// Rollback the transaction before returning
			return tx.Rollback()
		}

		return err
	}

	// Update user role created_at and updated_at for the old data.
	ts := time.Now()

	row, err := ent.FromContext(ctx).UserRole.Update().
		Where(userrole.CreatedAtIsNil()).
		SetCreatedAt(ts).
		SetUpdatedAt(ts).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "updated user role created_at and updated_at for the old data", log.Int("row", row))

	// Update role project_id for the old data.
	row, err = ent.FromContext(ctx).Role.Update().
		Where(role.ProjectIDIsNil()).
		SetProjectID(0).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "updated role project_id for the old data", log.Int("row", row))

	// Assign all users (except owner) to the Default project
	err = v.assignUsersToProject(ctx, proj, owner)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (v *V0_3_0) createDefaultProject(ctx context.Context) (*ent.Project, *ent.User, error) {
	// Find the owner user
	client := ent.FromContext(ctx)

	owner, err := client.User.Query().Where(user.IsOwner(true)).First(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Use the ProjectService to create the default project
	// This will automatically create the three default roles (admin, developer, viewer)
	projectService := biz.NewProjectService(biz.ProjectServiceParams{})
	input := ent.CreateProjectInput{
		Name:        "Default",
		Description: lo.ToPtr("Default project"),
	}

	ctx = contexts.WithUser(ctx, owner)

	proj, err := projectService.CreateProject(ctx, input)
	if err != nil {
		return nil, nil, err
	}

	return proj, owner, nil
}

func (v *V0_3_0) assignUsersToProject(ctx context.Context, proj *ent.Project, owner *ent.User) error {
	client := ent.FromContext(ctx)

	// Get all users except the owner
	users, err := client.User.Query().All(ctx)
	if err != nil {
		return err
	}

	// Assign each user to the default project
	for _, u := range users {
		// Owner is already in the project.
		if u.ID == owner.ID {
			continue
		}
		// Assume the user is not in the project.
		_, err := client.UserProject.Create().
			SetUserID(u.ID).
			SetProjectID(proj.ID).
			SetIsOwner(false).
			SetScopes([]string{}).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				log.Info(ctx, "user already in project", log.Int("user_id", u.ID), log.Int("project_id", proj.ID))
				continue
			}

			return err
		}

		log.Info(ctx, "assigned user to default project", log.Int("user_id", u.ID), log.Int("project_id", proj.ID))
	}

	return nil
}
