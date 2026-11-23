package gql

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/client"
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
		ModelCache: biz.NewModelCacheService(biz.ModelCacheServiceParams{
			Ent: client,
		}),
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

	pluginCtx, err := authz.DelegatePlugin(ctx, 1, []string{"write_api_keys"})
	require.NoError(t, err)
	pluginModels, err := resolver.QueryModels(pluginCtx, QueryModelsInput{})
	require.NoError(t, err)
	require.Equal(t, models, pluginModels)
	pluginChannels, err := resolver.AllChannelSummarys(pluginCtx, nil)
	require.NoError(t, err)
	require.Len(t, pluginChannels, 1)
	require.Equal(t, channels[0].ID, pluginChannels[0].ID)
	revoked, err := authz.DelegatePlugin(ctx, 2, nil)
	require.NoError(t, err)
	_, err = resolver.QueryModels(revoked, QueryModelsInput{})
	require.Error(t, err)
	_, err = resolver.AllChannelSummarys(revoked, nil)
	require.Error(t, err)

	ctxWithoutProject := ent.NewContext(context.Background(), client)
	ctxWithoutProject = authz.NewUserContext(ctxWithoutProject, userWithEdges.ID)
	ctxWithoutProject = contexts.WithUser(ctxWithoutProject, userWithEdges)
	_, err = resolver.QueryModels(ctxWithoutProject, QueryModelsInput{})
	require.Error(t, err)
}

func TestCreateModelReferencePricesGraphQL(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = db.Close() })
	srv := NewGraphqlHandlers(Dependencies{Ent: db, ModelService: biz.NewModelService(biz.ModelServiceParams{Ent: db})})
	api := client.New(srv.Graphql, func(r *client.Request) { r.HTTP = r.HTTP.WithContext(authz.WithTestBypass(r.HTTP.Context())) })
	const create = `mutation Create($input: CreateModelInput!, $edits: [ManualPriceEdit!]) {
		createModel(input:$input, editedPrices:$edits) { missingReferencePrices modelCard { cost { input output } } }
	}`
	input := map[string]any{
		"developer": "dev", "name": "Manual", "modelID": "manual", "icon": "dev", "group": "dev",
		"modelCard": map[string]any{"cost": map[string]float64{"input": 3, "output": 0}},
		"settings":  map[string]any{"associations": []any{}},
	}
	var response struct {
		CreateModel struct {
			MissingReferencePrices []string
			ModelCard              struct {
				Cost struct{ Input, Output float64 }
			}
		}
	}
	edits := []map[string]any{{"field": "cacheRead", "missing": true}, {"field": "cacheWrite", "missing": true}}
	require.NoError(t, api.Post(create, &response, client.Var("input", input), client.Var("edits", edits)))
	require.ElementsMatch(t, []string{"cacheRead", "cacheWrite"}, response.CreateModel.MissingReferencePrices)
	require.Equal(t, 3.0, response.CreateModel.ModelCard.Cost.Input)
	require.Zero(t, response.CreateModel.ModelCard.Cost.Output)
	stored := db.Model.Query().OnlyX(authz.WithTestBypass(t.Context()))
	require.Nil(t, stored.ModelCard.ReferencePrice("cacheWrite"))
	require.Equal(t, 0.0, *stored.ModelCard.ReferencePrice("output"))
	input["modelID"], input["name"] = "legacy", "Legacy"
	require.NoError(t, api.Post(create, &response, client.Var("input", input)))
	require.Empty(t, response.CreateModel.MissingReferencePrices)
	input["modelID"], input["name"] = "invalid", "Invalid"
	err := api.Post(create, &response, client.Var("input", input), client.Var("edits", []map[string]any{{"field": "typo", "missing": true}}))
	require.Error(t, err)
	require.Equal(t, 2, db.Model.Query().CountX(authz.WithTestBypass(t.Context())))
}

func TestCatalogCheckSitePermissionsAndValidation(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = db.Close() })
	system := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: xcache.Config{Mode: xcache.ModeMemory}, Ent: db})
	srv := NewGraphqlHandlers(Dependencies{Ent: db, ModelService: biz.NewModelService(biz.ModelServiceParams{Ent: db, SystemService: system})})
	const status = `query { catalogCheckStatus { failed } }`
	const save = `mutation SetTime($zone:String!,$time:String!) { saveCatalogCheckSettings(input:{enabled:true,providers:[],ignorePrefix:false,time:$time,timezone:$zone}) { failed } }`
	for _, tc := range []struct {
		name                    string
		siteScope, projectScope string
		query, zone, time, code string
	}{
		{"project channel grant", "", "write_channels", status, "", "", "FORBIDDEN"},
		{"site settings reader", "read_settings", "", status, "", "", ""},
		{"project settings reader", "", "read_settings", status, "", "", "FORBIDDEN"},
		{"site channel grant", "write_channels", "", status, "", "", ""},
		{"channel editor cannot configure schedule", "write_channels", "", save, "UTC", "08:30", "FORBIDDEN"},
		{"project settings grant", "", "write_settings", save, "UTC", "08:30", "FORBIDDEN"},
		{"invalid time", "write_settings", "", save, "UTC", "24:60", "VALIDATION_FAILED"},
		{"invalid timezone", "write_settings", "", save, "Invalid/Timezone", "08:30", "VALIDATION_FAILED"},
		{"site settings grant", "write_settings", "", save, "UTC", "08:30", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := &ent.User{ID: 1}
			if tc.siteScope != "" {
				u.Scopes = []string{tc.siteScope}
			}
			if tc.projectScope != "" {
				u.Edges.ProjectUsers = []*ent.UserProject{{ProjectID: 100, Scopes: []string{tc.projectScope}}}
			}
			api := client.New(srv.Graphql, func(r *client.Request) {
				ctx := authz.NewUserContext(r.HTTP.Context(), u.ID)
				r.HTTP = r.HTTP.WithContext(contexts.WithProjectID(contexts.WithUser(ctx, u), 100))
			})
			var options []client.Option
			if tc.query == save {
				options = []client.Option{client.Var("zone", tc.zone), client.Var("time", tc.time)}
			}
			var response map[string]any
			err := api.Post(tc.query, &response, options...)
			if tc.code == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.code)
			}
		})
	}
}
