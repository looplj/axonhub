package ent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPromptProjectRelationship(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:prompt-project?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("project").SaveX(ctx)
	prompt := client.Prompt.Create().
		SetProjectID(project.ID).
		SetName("prompt").
		SetRole("system").
		SetContent("content").
		SetSettings(objects.PromptSettings{}).
		SaveX(ctx)

	prompts, err := project.QueryPrompts().All(ctx)
	require.NoError(t, err)
	require.Len(t, prompts, 1)
	require.Equal(t, []int{prompt.ID}, []int{prompts[0].ID})

	linkedProject, err := prompt.QueryProject().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, project.ID, linkedProject.ID)
}

func TestSystemRoleUsesDefaultProjectID(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:system-role?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	role, err := client.Role.Create().SetName("admin").SetScopes([]string{}).Save(ctx)
	require.NoError(t, err)
	require.NotNil(t, role.ProjectID)
	require.Zero(t, *role.ProjectID)

	_, err = client.Role.Create().SetName("admin").SetScopes([]string{}).Save(ctx)
	require.Error(t, err)
	require.True(t, ent.IsConstraintError(err))
}
