package biz

import (
	"context"
	"testing"

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

	// Create minimal services needed for RequestService
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

	requestService := NewRequestService(client, systemService, usageLogService, dataStorageService)

	return requestService, client, ctx
}

func TestRequestService_ClearProcessingRequestsOnShutdown(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	// Create test project
	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create test trace
	tr, err := client.Trace.Create().
		SetTraceID("test-trace").
		SetProjectID(proj.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create processing requests
	req1, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	req2, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create a non-processing request (should not be affected)
	req3, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusCompleted).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Call the method
	err = svc.ClearProcessingRequestsOnShutdown(ctx)
	require.NoError(t, err)

	// Verify processing requests are now canceled
	updatedReq1, err := client.Request.Get(ctx, req1.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCanceled, updatedReq1.Status)

	updatedReq2, err := client.Request.Get(ctx, req2.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCanceled, updatedReq2.Status)

	// Verify non-processing request is unchanged
	updatedReq3, err := client.Request.Get(ctx, req3.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCompleted, updatedReq3.Status)

	// Count processing requests - should be 0
	processingCount, err := client.Request.Query().
		Where(request.StatusEQ(request.StatusProcessing)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, processingCount)
}

func TestRequestService_ClearProcessingRequestsOnShutdown_NoProcessingRequests(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	// Call the method when there are no processing requests
	err := svc.ClearProcessingRequestsOnShutdown(ctx)
	require.NoError(t, err)
}

func TestRequestService_ClearProcessingExecutionsOnShutdown(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	// Create test project
	proj, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create test trace
	tr, err := client.Trace.Create().
		SetTraceID("test-trace").
		SetProjectID(proj.ID).
		Save(ctx)
	require.NoError(t, err)

	// Create a request for the executions
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create processing executions
	exec1, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusProcessing).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	exec2, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusProcessing).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create a non-processing execution (should not be affected)
	exec3, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Call the method
	err = svc.ClearProcessingExecutionsOnShutdown(ctx)
	require.NoError(t, err)

	// Verify processing executions are now canceled
	updatedExec1, err := client.RequestExecution.Get(ctx, exec1.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCanceled, updatedExec1.Status)

	updatedExec2, err := client.RequestExecution.Get(ctx, exec2.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCanceled, updatedExec2.Status)

	// Verify non-processing execution is unchanged
	updatedExec3, err := client.RequestExecution.Get(ctx, exec3.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCompleted, updatedExec3.Status)

	// Count processing executions - should be 0
	processingCount, err := client.RequestExecution.Query().
		Where(requestexecution.StatusEQ(requestexecution.StatusProcessing)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, processingCount)
}

func TestRequestService_ClearProcessingExecutionsOnShutdown_NoProcessingExecutions(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	// Call the method when there are no processing executions
	err := svc.ClearProcessingExecutionsOnShutdown(ctx)
	require.NoError(t, err)
}
