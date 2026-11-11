package biz

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
)

func TestPluginScopedUserAndRoleManagement(t *testing.T) {
	roles, users, db := setupTestRoleService(t)
	defer db.Close()
	setupTestUserAPIKeyService(t, users, db)
	setup := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	owner := createOwnerUser(t, setup, db)
	target := db.User.Create().SetEmail("target@example.com").SetPassword("password").SetStatus(user.StatusActivated).SaveX(setup)
	project := db.Project.Create().SetName("project").SaveX(setup)
	limitedRole := db.Role.Create().SetName("limited").SetLevel(role.LevelProject).SetProjectID(project.ID).SetScopes([]string{"write_requests"}).SaveX(setup)
	highRole := db.Role.Create().SetName("high").SetLevel(role.LevelSystem).SetScopes([]string{"write_settings"}).SaveX(setup)
	parent := contexts.WithUser(authz.NewUserContext(ent.NewContext(t.Context(), db), owner.ID), owner)
	parent = contexts.WithProjectID(parent, project.ID)
	grants := []string{"read_users", "write_users", "read_roles", "write_roles", "read_projects", "write_requests"}
	ctx, err := authz.DelegatePlugin(parent, 1, grants)
	require.NoError(t, err)
	_, err = users.UpdateUser(ctx, target.ID, ent.UpdateUserInput{FirstName: lo.ToPtr("updated")})
	require.NoError(t, err)
	_, err = users.UpdateUser(ctx, owner.ID, ent.UpdateUserInput{FirstName: lo.ToPtr("forbidden")})
	require.Error(t, err)
	_, err = users.UpdateUser(ctx, target.ID, ent.UpdateUserInput{AppendScopes: []string{"write_settings"}})
	require.Error(t, err)
	require.Empty(t, db.User.GetX(setup, target.ID).Scopes)
	_, err = users.UpdateUser(ctx, target.ID, ent.UpdateUserInput{AddRoleIDs: []int{highRole.ID}})
	require.Error(t, err)
	_, err = users.AddUserToProject(ctx, target.ID, project.ID, nil, nil, []int{limitedRole.ID})
	require.NoError(t, err)
	_, err = users.UpdateProjectUser(ctx, target.ID, project.ID, nil, []string{"write_requests"}, nil, nil)
	require.NoError(t, err)
	for _, value := range []bool{true, false} {
		_, err = users.UpdateUser(ctx, target.ID, ent.UpdateUserInput{IsOwner: lo.ToPtr(value)})
		require.Error(t, err)
		_, err = users.UpdateProjectUser(ctx, target.ID, project.ID, lo.ToPtr(value), nil, nil, nil)
		require.Error(t, err)
		_, err = users.AddUserToProject(ctx, owner.ID, project.ID, lo.ToPtr(value), nil, nil)
		require.Error(t, err)
	}
	require.False(t, db.User.GetX(setup, target.ID).IsOwner)
	_, err = roles.UpdateRole(ctx, limitedRole.ID, ent.UpdateRoleInput{Name: lo.ToPtr("renamed")})
	require.NoError(t, err)
	_, err = roles.UpdateRole(ctx, limitedRole.ID, ent.UpdateRoleInput{Scopes: []string{"write_settings"}})
	require.Error(t, err)

	personal := db.APIKey.Create().SetUserID(target.ID).SetProjectID(project.ID).SetName("personal").SetKey("personal-key").SetType(apikey.TypePersonal).SaveX(setup)
	other := db.APIKey.Create().SetUserID(target.ID).SetProjectID(project.ID).SetName("regular").SetKey("regular-key").SetType(apikey.TypeUser).SaveX(setup)
	rollback := errors.New("rollback")
	err = users.RunInTransaction(ctx, func(txCtx context.Context) error {
		_, err := users.UpdateUserStatus(txCtx, target.ID, user.StatusDeactivated)
		if err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.Equal(t, apikey.StatusEnabled, db.APIKey.GetX(setup, personal.ID).Status)
	require.Equal(t, user.StatusActivated, db.User.GetX(setup, target.ID).Status)
	_, err = users.UpdateUserStatus(ctx, target.ID, user.StatusDeactivated)
	require.NoError(t, err)
	require.Equal(t, apikey.StatusDisabled, db.APIKey.GetX(setup, personal.ID).Status)
	require.Equal(t, apikey.StatusEnabled, db.APIKey.GetX(setup, other.ID).Status)

	for _, bulk := range []bool{false, true} {
		r := db.Role.Create().SetName(lo.Ternary(bulk, "deletable-bulk", "deletable-single")).SetLevel(role.LevelProject).SetProjectID(project.ID).SaveX(setup)
		invite := db.Invitation.Create().SetProjectID(project.ID).SetRoleID(r.ID).SetTokenHash(r.Name + lo.Ternary(bulk, "bulk", "single")).SaveX(setup)
		readOnly, delegateErr := authz.DelegatePlugin(parent, 3, []string{"read_roles"})
		require.NoError(t, delegateErr)
		if bulk {
			err = roles.BulkDeleteRoles(readOnly, []int{r.ID})
		} else {
			err = roles.DeleteRole(readOnly, r.ID)
		}
		require.Error(t, err)
		require.NotNil(t, db.Invitation.GetX(setup, invite.ID), "failed deletion must roll back host cleanup")
		if bulk {
			err = roles.BulkDeleteRoles(ctx, []int{r.ID})
		} else {
			err = roles.DeleteRole(ctx, r.ID)
		}
		require.NoError(t, err)
		_, err = db.Invitation.Get(setup, invite.ID)
		require.True(t, ent.IsNotFound(err))
	}
	revoked, err := authz.DelegatePlugin(parent, 2, nil)
	require.NoError(t, err)
	_, err = users.UpdateUser(revoked, target.ID, ent.UpdateUserInput{FirstName: lo.ToPtr("denied")})
	require.Error(t, err)
	require.Equal(t, "updated", db.User.GetX(setup, target.ID).FirstName)
	_, err = users.UpdateOwnProfile(ctx, ent.UpdateUserInput{FirstName: lo.ToPtr("denied")})
	require.Error(t, err)
}
