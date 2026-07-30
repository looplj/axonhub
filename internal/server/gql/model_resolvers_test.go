package gql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestQueryModels_ProjectAPIKeyManagerCanReadModels(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:query-models-project-access?mode=memory&_fk=1")
	defer client.Close()

	setupCtx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("query-models-project").SaveX(setupCtx)
	roleEntity := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"write_api_keys"}).
		SaveX(setupCtx)
	userEntity := client.User.Create().
		SetEmail("query-models-user@example.com").
		SetPassword("password").
		SetStatus(user.StatusActivated).
		SaveX(setupCtx)
	client.UserProject.Create().SetUserID(userEntity.ID).SetProjectID(project.ID).SaveX(setupCtx)
	client.UserRole.Create().SetUserID(userEntity.ID).SetRoleID(roleEntity.ID).SaveX(setupCtx)
	client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("query-models-channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "key"}).
		SetSupportedModels([]string{"project-model"}).
		SetDefaultTestModel("project-model").
		SetStatus(channel.StatusEnabled).
		SaveX(setupCtx)

	userWithEdges := client.User.Query().Where(user.IDEQ(userEntity.ID)).WithRoles().WithProjectUsers().OnlyX(setupCtx)
	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.NewUserContext(ctx, userWithEdges.ID)
	ctx = contexts.WithUser(ctx, userWithEdges)
	ctx = contexts.WithProjectID(ctx, project.ID)

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})
	channelService := biz.NewChannelServiceForTest(client)
	modelService := biz.NewModelService(biz.ModelServiceParams{
		ChannelService: channelService,
		SystemService:  systemService,
		Ent:            client,
	})
	resolver := &queryResolver{&Resolver{
		client:         client,
		systemService:  systemService,
		channelService: channelService,
		modelService:   modelService,
	}}

	models, err := resolver.QueryModels(ctx, QueryModelsInput{})
	require.NoError(t, err)
	require.Contains(t, models, &biz.ModelIdentityWithStatus{ID: "project-model", Status: channel.StatusEnabled})
	channels, err := resolver.AllChannelSummarys(ctx, nil)
	require.NoError(t, err)
	require.Len(t, channels, 1)

	ctxWithoutProject := ent.NewContext(context.Background(), client)
	ctxWithoutProject = authz.NewUserContext(ctxWithoutProject, userWithEdges.ID)
	ctxWithoutProject = contexts.WithUser(ctxWithoutProject, userWithEdges)
	_, err = resolver.QueryModels(ctxWithoutProject, QueryModelsInput{})
	require.Error(t, err)
}
