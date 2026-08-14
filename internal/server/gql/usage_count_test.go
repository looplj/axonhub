package gql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUsageRequestCountsExcludeVisionDelegation(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)
	projectEntity, err := client.Project.Create().
		SetName("p").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC().Add(-time.Second)
	requestEntity, err := client.Request.Create().
		SetProjectID(projectEntity.ID).
		SetAPIKeyID(1).
		SetModelID("text-model").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	visionExecution, err := client.RequestExecution.Create().
		SetProjectID(projectEntity.ID).
		SetRequestID(requestEntity.ID).
		SetModelID("vision-model").
		SetPurpose(requestexecution.PurposeVisionDelegation).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetStatus(requestexecution.StatusCompleted).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	primaryExecution, err := client.RequestExecution.Create().
		SetProjectID(projectEntity.ID).
		SetRequestID(requestEntity.ID).
		SetModelID("text-model").
		SetPurpose(requestexecution.PurposePrimary).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetStatus(requestexecution.StatusCompleted).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(requestEntity.ID).
		SetRequestExecutionID(visionExecution.ID).
		SetAPIKeyID(1).
		SetProjectID(projectEntity.ID).
		SetModelID("vision-model").
		SetPromptTokens(8).
		SetCompletionTokens(12).
		SetTotalTokens(20).
		SetTotalCost(2).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(requestEntity.ID).
		SetRequestExecutionID(primaryExecution.ID).
		SetAPIKeyID(1).
		SetProjectID(projectEntity.ID).
		SetModelID("text-model").
		SetPromptTokens(4).
		SetCompletionTokens(6).
		SetTotalTokens(10).
		SetTotalCost(1).
		SetCreatedAt(now).
		Save(ctx)
	require.NoError(t, err)

	resolver := &queryResolver{&Resolver{
		client:        client,
		systemService: biz.NewSystemService(biz.SystemServiceParams{Ent: client}),
	}}

	overview, err := resolver.AnalyticsOverview(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 1, overview.TotalRequests)
	require.Equal(t, 30, overview.TotalTokens)
	require.InDelta(t, 3, overview.TotalCost, 0.0001)

	daily, err := resolver.AnalyticsDailyStats(ctx, &AnalyticsFilter{
		StartTime: pointerTo(now.Format("2006-01-02")),
		EndTime:   pointerTo(now.Format("2006-01-02")),
	})
	require.NoError(t, err)
	require.Len(t, daily, 1)
	require.Equal(t, 1, daily[0].RequestCount)
	require.Equal(t, 30, daily[0].TotalTokens)
	require.InDelta(t, 3, daily[0].Cost, 0.0001)

	requestStats, err := resolver.RequestStats(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, requestStats.RequestsToday)

	dashboardDaily, err := resolver.DailyRequestStats(ctx)
	require.NoError(t, err)
	today := dashboardDaily[len(dashboardDaily)-1]
	require.Equal(t, 1, today.Count)
	require.Equal(t, 30, today.Tokens)
	require.InDelta(t, 3, today.Cost, 0.0001)

	modelStats, err := resolver.AnalyticsDimensionStats(ctx, nil, "model")
	require.NoError(t, err)
	require.Len(t, modelStats, 2)
	statsByModel := make(map[string]*AnalyticsDimensionStat, len(modelStats))
	for _, stats := range modelStats {
		statsByModel[stats.ID] = stats
	}
	require.Equal(t, 1, statsByModel["text-model"].RequestCount)
	require.Equal(t, 10, statsByModel["text-model"].TotalTokens)
	require.Equal(t, 0, statsByModel["vision-model"].RequestCount)
	require.Equal(t, 20, statsByModel["vision-model"].TotalTokens)
}

func pointerTo[T any](value T) *T {
	return &value
}
