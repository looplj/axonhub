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

	// Create processing records with this node's ID (should be canceled)
	thisNodeID := svc.nodeID
	var processingIDs []int
	for i := 0; i < 2; i++ {
		req, err := client.Request.Create().
			SetProjectID(proj.ID).
			SetTraceID(tr.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(request.StatusProcessing).
			SetNodeID(thisNodeID).
			SetStream(false).
			Save(ctx)
		require.NoError(t, err)
		processingIDs = append(processingIDs, req.ID)
	}

	// Create a completed record with this node's ID (should remain unchanged)
	completedReq, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusCompleted).
		SetNodeID(thisNodeID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Call the method
	err = svc.ClearProcessingRequestsOnShutdown(ctx)
	require.NoError(t, err)

	// Verify processing records are now canceled
	for _, id := range processingIDs {
		req, err := client.Request.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, request.StatusCanceled, req.Status, "processing record should be canceled")
	}

	// Verify completed record is unchanged
	req, err := client.Request.Get(ctx, completedReq.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCompleted, req.Status, "completed record should be unchanged")
}

func TestRequestService_ClearProcessingRequestsOnShutdown_MultiNode(t *testing.T) {
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

	// Create processing record for DIFFERENT node (should NOT be canceled)
	otherNodeReq, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetNodeID("other-node-123").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create processing record for THIS node (should be canceled)
	thisNodeReq, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetNodeID(svc.nodeID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Call the method
	err = svc.ClearProcessingRequestsOnShutdown(ctx)
	require.NoError(t, err)

	// Verify other node's request is still processing
	req, err := client.Request.Get(ctx, otherNodeReq.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusProcessing, req.Status, "other node's request should still be processing")

	// Verify this node's request is canceled
	req, err = client.Request.Get(ctx, thisNodeReq.ID)
	require.NoError(t, err)
	require.Equal(t, request.StatusCanceled, req.Status, "this node's request should be canceled")
}

func TestRequestService_ClearProcessingRequestsOnShutdown_NoProcessingRequests(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

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

	// Create a parent request
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetNodeID(svc.nodeID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create processing executions with this node's ID
	thisNodeID := svc.nodeID
	var processingIDs []int
	for i := 0; i < 2; i++ {
		exec, err := client.RequestExecution.Create().
			SetProjectID(proj.ID).
			SetRequestID(req.ID).
			SetModelID("gpt-4").
			SetFormat("openai/chat_completions").
			SetRequestBody([]byte(`{"model":"gpt-4"}`)).
			SetStatus(requestexecution.StatusProcessing).
			SetNodeID(thisNodeID).
			SetStream(false).
			Save(ctx)
		require.NoError(t, err)
		processingIDs = append(processingIDs, exec.ID)
	}

	// Create a completed execution
	completedExec, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusCompleted).
		SetNodeID(thisNodeID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Call the method
	err = svc.ClearProcessingExecutionsOnShutdown(ctx)
	require.NoError(t, err)

	// Verify processing executions are now canceled
	for _, id := range processingIDs {
		exec, err := client.RequestExecution.Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, requestexecution.StatusCanceled, exec.Status, "processing execution should be canceled")
	}

	// Verify completed execution is unchanged
	exec, err := client.RequestExecution.Get(ctx, completedExec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCompleted, exec.Status, "completed execution should be unchanged")
}

func TestRequestService_ClearProcessingExecutionsOnShutdown_MultiNode(t *testing.T) {
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

	// Create a parent request
	req, err := client.Request.Create().
		SetProjectID(proj.ID).
		SetTraceID(tr.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(request.StatusProcessing).
		SetNodeID(svc.nodeID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create execution for DIFFERENT node
	otherNodeExec, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusProcessing).
		SetNodeID("other-node-456").
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Create execution for THIS node
	thisNodeExec, err := client.RequestExecution.Create().
		SetProjectID(proj.ID).
		SetRequestID(req.ID).
		SetModelID("gpt-4").
		SetFormat("openai/chat_completions").
		SetRequestBody([]byte(`{"model":"gpt-4"}`)).
		SetStatus(requestexecution.StatusProcessing).
		SetNodeID(svc.nodeID).
		SetStream(false).
		Save(ctx)
	require.NoError(t, err)

	// Call the method
	err = svc.ClearProcessingExecutionsOnShutdown(ctx)
	require.NoError(t, err)

	// Verify other node's execution is still processing
	exec, err := client.RequestExecution.Get(ctx, otherNodeExec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusProcessing, exec.Status, "other node's execution should still be processing")

	// Verify this node's execution is canceled
	exec, err = client.RequestExecution.Get(ctx, thisNodeExec.ID)
	require.NoError(t, err)
	require.Equal(t, requestexecution.StatusCanceled, exec.Status, "this node's execution should be canceled")
}

func TestRequestService_ClearProcessingExecutionsOnShutdown_NoProcessingExecutions(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	err := svc.ClearProcessingExecutionsOnShutdown(ctx)
	require.NoError(t, err)
}
