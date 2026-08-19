package scopes_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestProjectOwnerCanReadMemberPersonalKeyRequests(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:project-owner-personal-key-requests?mode=memory&_fk=1")
	defer client.Close()

	setupCtx := privacy.DecisionContext(context.Background(), privacy.Allow)
	project := client.Project.Create().SetName("project").SaveX(setupCtx)
	owner := client.User.Create().SetEmail("owner@example.com").SetPassword("password").SaveX(setupCtx)
	creator := client.User.Create().SetEmail("creator@example.com").SetPassword("password").SaveX(setupCtx)
	member := client.User.Create().SetEmail("member@example.com").SetPassword("password").SaveX(setupCtx)
	client.User.Create().SetEmail("outside@example.com").SetPassword("password").SaveX(setupCtx)

	client.UserProject.Create().SetUserID(owner.ID).SetProjectID(project.ID).SetIsOwner(true).SaveX(setupCtx)
	client.UserProject.Create().SetUserID(creator.ID).SetProjectID(project.ID).SetScopes([]string{string(scopes.ScopeReadRequests)}).SaveX(setupCtx)
	client.UserProject.Create().SetUserID(member.ID).SetProjectID(project.ID).SetScopes([]string{string(scopes.ScopeReadRequests)}).SaveX(setupCtx)

	personalKey := client.APIKey.Create().
		SetName("creator personal key").
		SetKey("personal-key").
		SetType(apikey.TypePersonal).
		SetUserID(creator.ID).
		SetProjectID(project.ID).
		SaveX(setupCtx)
	personalRequest := client.Request.Create().
		SetAPIKeyID(personalKey.ID).
		SetProjectID(project.ID).
		SetModelID("test-model").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SaveX(setupCtx)
	client.UsageLog.Create().
		SetRequestID(personalRequest.ID).
		SetAPIKeyID(personalKey.ID).
		SetProjectID(project.ID).
		SetModelID("test-model").
		SaveX(setupCtx)

	loadUser := func(id int) *ent.User {
		return client.User.Query().Where(user.IDEQ(id)).WithProjectUsers().OnlyX(setupCtx)
	}
	queryCount := func(user *ent.User) (int, int, error) {
		ctx := ent.NewContext(context.Background(), client)
		ctx = contexts.WithProjectID(contexts.WithUser(ctx, user), project.ID)
		requestCount, err := client.Request.Query().Count(ctx)
		if err != nil {
			return 0, 0, err
		}
		usageLogCount, err := client.UsageLog.Query().Count(ctx)
		return requestCount, usageLogCount, err
	}
	projectOwner := loadUser(owner.ID)
	ownerCtx := ent.NewContext(context.Background(), client)
	ownerCtx = contexts.WithProjectID(contexts.WithUser(ownerCtx, projectOwner), project.ID)

	memberCount, err := client.User.Query().Count(ownerCtx)
	require.NoError(t, err)
	require.Equal(t, 3, memberCount)

	personalKeyCount, err := client.APIKey.Query().Count(ownerCtx)
	require.NoError(t, err)
	require.Equal(t, 1, personalKeyCount)
	loadedCreator, err := personalKey.QueryUser().Only(ownerCtx)
	require.NoError(t, err)
	require.Equal(t, creator.ID, loadedCreator.ID)

	memberCtx := ent.NewContext(context.Background(), client)
	memberCtx = contexts.WithProjectID(contexts.WithUser(memberCtx, loadUser(member.ID)), project.ID)
	_, err = client.User.Query().Count(memberCtx)
	require.Error(t, err)

	tests := []struct {
		name              string
		user              *ent.User
		wantRequestCount  int
		wantUsageLogCount int
	}{
		{name: "project owner", user: projectOwner, wantRequestCount: 1, wantUsageLogCount: 1},
		{name: "personal key creator", user: loadUser(creator.ID), wantRequestCount: 1, wantUsageLogCount: 1},
		{name: "regular member", user: loadUser(member.ID), wantRequestCount: 0, wantUsageLogCount: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount, usageLogCount, err := queryCount(tt.user)
			require.NoError(t, err)
			require.Equal(t, tt.wantRequestCount, requestCount)
			require.Equal(t, tt.wantUsageLogCount, usageLogCount)
		})
	}
}
