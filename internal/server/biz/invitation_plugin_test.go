package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/user"
)

func TestPluginProjectInvitation(t *testing.T) {
	svc, db, setup := setupInvitationService(t)
	defer db.Close()
	project := db.Project.Create().SetName("plugin-invitation").SaveX(setup)
	other := db.Project.Create().SetName("other-project").SaveX(setup)
	member := db.User.Create().SetEmail("plugin-inviter@example.com").SetPassword("password").SaveX(setup)
	db.UserProject.Create().SetUserID(member.ID).SetProjectID(project.ID).SetScopes([]string{"write_users", "write_requests"}).SaveX(setup)
	member = db.User.Query().Where(user.IDEQ(member.ID)).WithRoles().WithProjectUsers().OnlyX(setup)
	role := createInvitationRole(t, db, setup, project.ID)
	parent := contexts.WithUser(authz.NewUserContext(ent.NewContext(context.Background(), db), member.ID), member)
	ctx, err := authz.DelegatePlugin(parent, 1, []string{"write_users", "write_requests"})
	require.NoError(t, err)
	created, err := svc.CreateInvitation(ctx, project.ID, role.ID, nil, 1)
	require.NoError(t, err)
	require.NotEmpty(t, created.Token)
	require.Error(t, svc.canInvite(ctx, other.ID))

	noRoleGrant, err := authz.DelegatePlugin(parent, 2, []string{"write_users"})
	require.NoError(t, err)
	_, err = svc.CreateInvitation(noRoleGrant, project.ID, role.ID, nil, 1)
	require.Error(t, err, "cannot grant a role beyond installation grants")
	revoked, err := authz.DelegatePlugin(parent, 3, nil)
	require.NoError(t, err)
	_, err = svc.CreateInvitation(revoked, project.ID, role.ID, nil, 1)
	require.Error(t, err)
	require.Equal(t, 1, db.Invitation.Query().CountX(setup))

	owner := &ent.User{ID: member.ID, IsOwner: true}
	ownerCtx := contexts.WithUser(authz.NewUserContext(context.Background(), owner.ID), owner)
	ownerPlugin, err := authz.DelegatePlugin(ownerCtx, 4, nil)
	require.NoError(t, err)
	require.NoError(t, svc.canInvite(ownerCtx, project.ID))
	require.Error(t, svc.canInvite(ownerPlugin, project.ID))
}
