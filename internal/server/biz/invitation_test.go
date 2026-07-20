package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/userproject"
)

func setupInvitationService(t *testing.T) (*InvitationService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:invitation?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	service := &InvitationService{AbstractService: &AbstractService{db: client}}

	return service, client, ctx
}

func TestInvitationService_SingleUseInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("single-use-project").Save(ctx)
	require.NoError(t, err)

	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, nil, 1)
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
}

func TestInvitationService_UnlimitedInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("unlimited-project").Save(ctx)
	require.NoError(t, err)
	neverExpires := 0
	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, &neverExpires, 0)
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

func TestInvitationService_ExpiredInvitation(t *testing.T) {
	service, client, ctx := setupInvitationService(t)
	defer client.Close()

	owner, err := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	project, err := client.Project.Create().SetName("expired-project").Save(ctx)
	require.NoError(t, err)
	oneHour := 1
	created, err := service.CreateInvitation(contexts.WithUser(ctx, owner), project.ID, &oneHour, 1)
	require.NoError(t, err)

	invitation, err := client.Invitation.Query().Only(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Invitation.UpdateOneID(invitation.ID).SetExpiresAt(time.Now().Add(-time.Hour)).Exec(ctx))

	_, err = service.GetInvitation(ctx, created.Token)
	require.Error(t, err)
	_, err = service.RegisterInvitation(ctx, created.Token, "member@example.com", "password", "Member", "User")
	require.Error(t, err)
}
