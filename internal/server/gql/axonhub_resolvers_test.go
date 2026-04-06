package gql

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func setupTestQueryResolver(t *testing.T) (*queryResolver, context.Context, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	resolver := &queryResolver{&Resolver{client: client}}

	return resolver, ctx, client
}

func TestQueryResolver_AllChannelSummarys_ProjectProfileUsesIntersection(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	idOnlyChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("ID Only").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-1"}).
		SetSupportedModels([]string{"id-only-model"}).
		SetDefaultTestModel("id-only-model").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	matchingChannel, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Matching").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-2"}).
		SetSupportedModels([]string{"matching-model"}).
		SetDefaultTestModel("matching-model").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"allowed"}).
		Save(ctx)
	require.NoError(t, err)

	projectEntity, err := client.Project.Create().
		SetName("Project A").
		SetDescription("test project").
		SetProfiles(&objects.ProjectProfiles{
			ActiveProfile: "production",
			Profiles: []objects.ProjectProfile{
				{
					Name:        "production",
					ChannelIDs:  []int{idOnlyChannel.ID, matchingChannel.ID},
					ChannelTags: []string{"allowed"},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	projectCtx := contexts.WithProjectID(ctx, projectEntity.ID)

	channels, err := resolver.AllChannelSummarys(projectCtx, nil)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	require.Equal(t, matchingChannel.ID, channels[0].ID)
}

func TestQueryResolver_AllChannelTags_ProjectProfileFiltersVisibleTags(t *testing.T) {
	resolver, ctx, client := setupTestQueryResolver(t)
	defer client.Close()

	_, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Visible Channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-visible"}).
		SetSupportedModels([]string{"visible-model"}).
		SetDefaultTestModel("visible-model").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"shared", "visible"}).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Hidden Channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "key-hidden"}).
		SetSupportedModels([]string{"hidden-model"}).
		SetDefaultTestModel("hidden-model").
		SetStatus(channel.StatusEnabled).
		SetTags([]string{"shared", "hidden"}).
		Save(ctx)
	require.NoError(t, err)

	projectEntity, err := client.Project.Create().
		SetName("Project B").
		SetDescription("test project").
		SetProfiles(&objects.ProjectProfiles{
			ActiveProfile: "production",
			Profiles: []objects.ProjectProfile{
				{
					Name:        "production",
					ChannelTags: []string{"visible"},
				},
			},
		}).
		Save(ctx)
	require.NoError(t, err)

	projectCtx := contexts.WithProjectID(ctx, projectEntity.ID)

	tags, err := resolver.AllChannelTags(projectCtx)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"shared", "visible"}, lo.Uniq(tags))
}
