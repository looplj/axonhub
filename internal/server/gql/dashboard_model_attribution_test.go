package gql

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/objects"
)

// seedAliasTraffic records one request that was made through a model alias: the requests
// row holds the client-requested alias, while the execution and its usage log hold the
// channel model actually executed after model mapping. Dashboard model statistics must
// always attribute to the real model, never to the requested alias.
func seedAliasTraffic(t *testing.T, client *ent.Client, ctx context.Context) int {
	t.Helper()

	p, err := client.Project.Create().SetName("p").SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()

	// The catalog row exists only for the real model; the alias deliberately has none.
	client.Model.Create().
		SetDeveloper("openai").
		SetModelID("gpt-4o-real").
		SetName("GPT-4o").
		SetGroup("gpt").
		SetIcon("icon").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SaveX(ctx)

	req, err := client.Request.Create().
		SetProjectID(p.ID).
		SetAPIKeyID(1).
		SetModelID("alias-fast").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetCreatedAt(now.Add(-2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(1).
		SetProjectID(p.ID).
		SetChannelID(1).
		SetModelID("gpt-4o-real").
		SetCompletionTokens(2000).
		SetCreatedAt(now.Add(-2 * time.Hour)).
		SaveX(ctx)

	client.RequestExecution.Create().
		SetProjectID(p.ID).
		SetRequestID(req.ID).
		SetChannelID(1).
		SetModelID("gpt-4o-real").
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetStream(true).
		SetMetricsLatencyMs(2400).
		SetMetricsFirstTokenLatencyMs(400).
		SetCreatedAt(now.Add(-2 * time.Hour)).
		SetUpdatedAt(now).
		SaveX(ctx)

	return p.ID
}

func TestFastestModels_AttributeToExecutedModel(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	seedAliasTraffic(t, client, ctx)

	limit := 5
	models, err := resolver.FastestModels(ctx, FastestChannelsInput{
		TimeWindow: relativeWindowLast24Hours,
		Limit:      &limit,
	})
	require.NoError(t, err)

	require.Len(t, models, 1)
	assert.Equal(t, "gpt-4o-real", models[0].ModelID)
	assert.Equal(t, "GPT-4o", models[0].ModelName)
}

func TestModelPerformanceStats_AttributeToExecutedModel(t *testing.T) {
	resolver, ctx, client := setupRecentPerformanceResolver(t)
	defer client.Close()

	seedAliasTraffic(t, client, ctx)

	stats, err := resolver.ModelPerformanceStats(ctx, ptrTimeWindow(relativeWindowLast24Hours), nil, nil)
	require.NoError(t, err)

	require.NotEmpty(t, stats)
	for _, stat := range stats {
		assert.Equal(t, "gpt-4o-real", stat.ModelID)
		assert.NotEqual(t, "alias-fast", stat.ModelID)
	}
}
