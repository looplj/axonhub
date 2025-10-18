package datamigrate

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userrole"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// V0_3_0 creates a default project if it doesn't exist.
func V0_3_0(ctx context.Context, client *ent.Client) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	err := createDefaultProject(ctx, client)
	if err != nil {
		return err
	}

	// Update user role created_at and updated_at for the old data.
	ts := time.Now()

	row, err := client.UserRole.Update().
		Where(userrole.CreatedAtIsNil()).
		SetCreatedAt(ts).
		SetUpdatedAt(ts).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "updated user role created_at and updated_at for the old data", log.Int("row", row))

	// Update role project_id for the old data.
	row, err = client.Role.Update().
		Where(role.ProjectIDIsNil()).
		SetProjectID(0).
		Save(ctx)
	if err != nil {
		return err
	}

	log.Info(ctx, "updated role project_id for the old data", log.Int("row", row))

	return nil
}

func createDefaultProject(ctx context.Context, client *ent.Client) error {
	// Check if a project already exists
	exists, err := client.Project.Query().Limit(1).Exist(ctx)
	if err != nil {
		return err
	}

	if exists {
		log.Info(ctx, "existed project found, skip migration")
		return nil
	}

	// Find the owner user
	owner, err := client.User.Query().Where(user.IsOwner(true)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// No owner user exists yet, skip project creation
			// Project will be created when the system is initialized
			log.Info(ctx, "no owner user found, skip project creation")
			return nil
		}

		return err
	}

	// Use the ProjectService to create the default project
	// This will automatically create the three default roles (admin, developer, viewer)
	projectService := biz.NewProjectService(biz.ProjectServiceParams{})
	input := ent.CreateProjectInput{
		Name:        "Default",
		Description: lo.ToPtr("Default project"),
	}

	ctx = contexts.WithUser(ctx, owner)
	ctx = ent.NewContext(ctx, client)

	_, err = projectService.CreateProject(ctx, input)
	if err != nil {
		return err
	}

	return nil
}
