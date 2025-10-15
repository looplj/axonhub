package datamigrate

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// V0_3_0 creates a default project if it doesn't exist.
func V0_3_0(ctx context.Context, client *ent.Client) error {
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

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
	description := "Default project"
	input := ent.CreateProjectInput{
		Slug:        "default",
		Name:        "Default",
		Description: &description,
	}

	ctx = contexts.WithUser(ctx, owner)
	ctx = ent.NewContext(ctx, client)

	_, err = projectService.CreateProject(ctx, input)
	if err != nil {
		return err
	}

	return nil
}
