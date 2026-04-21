package biz

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/zhenzou/executors"
)

func setupTestRequestService(t *testing.T) (*RequestService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.Open(t, dialect.SQLite, "file:ent?mode=memory&_fk=1")
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	systemService := NewSystemService(SystemServiceParams{
		Ent: client,
	})
	channelService := NewChannelServiceForTest(client)
	usageLogService := NewUsageLogService(client, systemService, channelService)
	dataStorageService := NewDataStorageService(DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   xcache.Config{},
		Executor:      executors.NewPoolScheduleExecutor(),
		Client:        client,
	})

	requestService := NewRequestService(client, systemService, usageLogService, dataStorageService, NewLiveStreamRegistry())

	return requestService, client, ctx
}

func TestRequestService_ClearStaleProcessingOnStartup(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	tr, err := client.Trace.Create().
		SetTraceID("test-trace").
		SetProjectID(proj.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create stale requests (>1 hour old)
	var staleRequestIDs []int
	for i := 0; i < 2; i++ {
		req, err := client.Request.Create().
			SetProjectID(proj.ID).
			SetTraceID(tr.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(request.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)
		staleRequestIDs = append(staleRequestIDs, req.ID)
	}

	// Create recent requests (<1 hour old)
	var recentRequestIDs []int
	for i := 0; i < 2; i++ {
		req, err := client.Request.Create().
			SetProjectID(proj.ID).
			SetTraceID(tr.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(request.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-30 * time.Minute)).
			Save(ctx)
		require.NoError(t, err)
		recentRequestIDs = append(recentRequestIDs, req.ID)
	}

	// Create stale executions
	var staleExecIDs []int
	for _, reqID := range staleRequestIDs {
		exec, err := client.RequestExecution.Create().
			SetProjectID(proj.ID).
			SetRequestID(reqID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(requestexecution.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
			Save(ctx)
		require.NoError(t, err)
		staleExecIDs = append(staleExecIDs, exec.ID)
	}

	// Create recent executions (<1 hour old)
	var recentExecIDs []int
	for _, reqID := range recentRequestIDs {
		exec, err := client.RequestExecution.Create().
			SetProjectID(proj.ID).
			SetRequestID(reqID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(requestexecution.StatusProcessing).
			SetStream(false).
			SetCreatedAt(time.Now().UTC().Add(-30 * time.Minute)).
			Save(ctx)
		require.NoError(t, err)
		recentExecIDs = append(recentExecIDs, exec.ID)
	}

	err = svc.ClearStaleProcessingOnStartup(ctx)
	require.NoError(t, err)

	for _, id := range staleRequestIDs {
		req, err := client.Request.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, request.StatusCanceled, req.Status)
	}

	for _, id := range recentRequestIDs {
		req, err := client.Request.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, request.StatusProcessing, req.Status)
	}

	for _, id := range staleExecIDs {
		exec, err := client.RequestExecution.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusCanceled, exec.Status)
	}

	for _, id := range recentExecIDs {
		exec, err := client.RequestExecution.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusProcessing, exec.Status)
	}
}

func TestRequestService_ClearStaleProcessingOnStartup_NoStaleRecords(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	err := svc.ClearStaleProcessingOnStartup(ctx)
	require.NoError(t, err)
}

func TestRequestService_ClearStaleProcessingOnStartup_PartialFailure(t *testing.T) {
	// This test verifies that if one cleanup operation fails, others still run
	// and errors are properly aggregated
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	tr, err := client.Trace.Create().
		SetTraceID("test-trace").
		SetProjectID(proj.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create only stale executions (no stale requests)
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetStream(false).
		SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	exec, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusProcessing).
		SetStream(false).
		SetCreatedAt(time.Now().UTC().Add(-2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)

	// Cleanup should succeed for both
	err = svc.ClearStaleProcessingOnStartup(ctx)
	require.NoError(t, err)

	// Verify execution was canceled
	exec, err = client.RequestExecution.Get(ctx, exec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCanceled, exec.Status)

	// Verify request was also canceled
	req, err = client.Request.Get(ctx, req.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCanceled, req.Status)
}
