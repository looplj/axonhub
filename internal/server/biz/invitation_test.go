package biz

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
)

func setupInvitationService(t *testing.T) (*InvitationService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:invitation?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	service := &InvitationService{
		AbstractService:     &AbstractService{db: client},
		permissionValidator: NewPermissionValidator(),
	}

	return service, client, ctx
}

func createInvitationRole(t *testing.T, client *ent.Client, ctx context.Context, projectID int) *ent.Role {
	t.Helper()

	projectRole, err := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(projectID).
		SetScopes([]string{"write_requests"}).
		Save(ctx)
	require.NoError(t, err)
	return projectRole
}

func TestInvitationService_SingleUseInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("single-use-project").Save(ctx)
	require.NoError(t, err)
	projectRole := createInvitationRole(t, client, ctx, project.ID)

	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, projectRole.ID, nil, 1)
	require.NoError(t, err)

	registered, err := service.RegisterInvitation(ctx, created.Token, "first@example.com", "password", "First", "Member")
	require.NoError(t, err)
	require.Equal(t, "first@example.com", registered.Email)

	_, err = service.RegisterInvitation(ctx, created.Token, "second@example.com", "password", "Second", "Member")
	require.Error(t, err)

	exists, err := client.UserProject.Query().Where(
		userproject.UserIDEQ(registered.ID),
		userproject.ProjectIDEQ(project.ID),
	).Exist(ctx)
	require.NoError(t, err)
	require.True(t, exists)
	hasRole, err := registered.QueryRoles().Where(role.IDEQ(projectRole.ID)).Exist(ctx)
	require.NoError(t, err)
	require.True(t, hasRole)

	userInfo := ConvertUserToUserInfo(ctx, registered)
	require.Len(t, userInfo.Projects, 1)
	require.Contains(t, userInfo.Projects[0].EffectiveScopes, "write_requests")
}

func TestInvitationService_UnlimitedInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("unlimited-project").Save(ctx)
	require.NoError(t, err)
	projectRole := createInvitationRole(t, client, ctx, project.ID)
	neverExpires := 0
	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, projectRole.ID, &neverExpires, 0)
	require.NoError(t, err)

	first, err := service.RegisterInvitation(ctx, created.Token, "first@example.com", "password", "First", "Member")
	require.NoError(t, err)
	second, err := service.RegisterInvitation(ctx, created.Token, "second@example.com", "password", "Second", "Member")
	require.NoError(t, err)

	info, err := service.GetInvitation(ctx, created.Token)
	require.NoError(t, err)
	require.Equal(t, 0, info.MaxUses)
	require.Equal(t, 2, info.UsedCount)
	require.NotEqual(t, first.ID, second.ID)
}

func TestInvitationService_RejectsUnmigratedLegacyInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	project, err := client.Project.Create().SetName("legacy-project").Save(ctx)
	require.NoError(t, err)
	token := "legacy-token"
	_, err = client.Invitation.Create().
		SetTokenHash(hashInvitationToken(token)).
		SetProjectID(project.ID).
		Save(ctx)
	require.NoError(t, err)

	_, err = service.RegisterInvitation(ctx, token, "legacy@example.com", "password", "Legacy", "Member")
	require.ErrorContains(t, err, "invitation role is required")
}

func TestInvitationService_RejectsRoleTheInviterCannotGrant(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	project, err := client.Project.Create().SetName("restricted-project").Save(ctx)
	require.NoError(t, err)
	projectRole := createInvitationRole(t, client, ctx, project.ID)
	inviter, err := client.User.Create().SetEmail("inviter@example.com").SetPassword("password").Save(ctx)
	require.NoError(t, err)
	_, err = client.UserProject.Create().SetUserID(inviter.ID).SetProjectID(project.ID).SetScopes([]string{"write_users"}).Save(ctx)
	require.NoError(t, err)
	inviter, err = client.User.Query().Where(user.IDEQ(inviter.ID)).WithProjectUsers().WithRoles().Only(ctx)
	require.NoError(t, err)

	_, err = service.CreateInvitation(contexts.WithUser(ctx, inviter), project.ID, projectRole.ID, nil, 1)
	require.ErrorContains(t, err, "permission denied")
}

func TestInvitationService_RejectsDeletedInvitationRole(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("deleted-role-project").Save(ctx)
	require.NoError(t, err)
	projectRole := createInvitationRole(t, client, ctx, project.ID)
	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, projectRole.ID, nil, 1)
	require.NoError(t, err)
	require.NoError(t, client.Role.DeleteOneID(projectRole.ID).Exec(ctx))

	_, err = service.RegisterInvitation(ctx, created.Token, "member@example.com", "password", "Member", "User")
	require.ErrorContains(t, err, "invitation role is no longer available")
}

func TestInvitationService_ExpiredInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("expired-project").Save(ctx)
	require.NoError(t, err)
	projectRole := createInvitationRole(t, client, ctx, project.ID)
	oneHour := 1
	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, projectRole.ID, &oneHour, 1)
	require.NoError(t, err)

	invitation, err := client.Invitation.Query().Only(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Invitation.UpdateOneID(invitation.ID).SetExpiresAt(time.Now().Add(-time.Hour)).Exec(ctx))

	_, err = service.GetInvitation(ctx, created.Token)
	require.Error(t, err)
	_, err = service.RegisterInvitation(ctx, created.Token, "member@example.com", "password", "Member", "User")
	require.Error(t, err)
}

func TestGenerateInvitationToken(t *testing.T) {
	token, err := generateInvitationToken()
	require.NoError(t, err)
	require.Len(t, token, 8)

	bytes, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	require.Len(t, bytes, 6)
}
