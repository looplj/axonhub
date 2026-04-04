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

// testClearProcessingEntity defines how to test clearing processing status for an entity type
type testClearProcessingEntity struct {
	name string
	// createProcessing creates processing records and returns their IDs
	createProcessing func(t *testing.T, ctx context.Context, client *ent.Client, projID int, traceID int) []int
	// createNonProcessing creates a non-processing record and returns its ID
	createNonProcessing func(t *testing.T, ctx context.Context, client *ent.Client, projID int, traceID int) int
	// getStatus gets the status of a record by ID
	getStatus func(t *testing.T, ctx context.Context, client *ent.Client, id int) string
	// countProcessing counts processing records
	countProcessing func(ctx context.Context, client *ent.Client) (int, error)
	// callShutdownMethod calls the shutdown method being tested
	callShutdownMethod func(ctx context.Context, svc *RequestService) error
}

func testClearProcessingOnShutdown(t *testing.T, tc testClearProcessingEntity) {
	t.Helper()

	t.Run(tc.name, func(t *testing.T) {

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

		// Create processing records
		processingIDs := tc.createProcessing(t, ctx, client, proj.ID, tr.ID)

		// Create a non-processing record
		nonProcessingID := tc.createNonProcessing(t, ctx, client, proj.ID, tr.ID)

		// Call the method
		err = tc.callShutdownMethod(ctx, svc)
		require.NoError(t, err)

		// Verify processing records are now canceled
		for _, id := range processingIDs {
			status := tc.getStatus(t, ctx, client, id)
			require.Equal(t, "canceled", status, "processing record should be canceled")
		}

		// Verify non-processing record is unchanged
		nonProcessingStatus := tc.getStatus(t, ctx, client, nonProcessingID)
		require.Equal(t, "completed", nonProcessingStatus, "non-processing record should be unchanged")

		// Count processing records - should be 0
		processingCount, err := tc.countProcessing(ctx, client)
		require.NoError(t, err)
		require.Equal(t, 0, processingCount, "no processing records should remain after shutdown")
	})
}

func TestRequestService_ClearProcessingRequestsOnShutdown(t *testing.T) {
	tc := testClearProcessingEntity{
		name: "requests",
		createProcessing: func(t *testing.T, ctx context.Context, client *ent.Client, projID int, traceID int) []int {
			t.Helper()
			var ids []int
			for i := 0; i < 2; i++ {
				req, err := client.Request.Create().
					SetProjectID(projID).
					SetTraceID(traceID).
					SetModelID("gpt-4").
					SetFormat("openai/chat_completions").
					SetRequestBody([]byte(`{"model":"gpt-4"}`)).
					SetStatus(request.StatusProcessing).
					SetStream(false).
					Save(ctx)
				require.NoError(t, err)
				ids = append(ids, req.ID)
			}
			return ids
		},
		createNonProcessing: func(t *testing.T, ctx context.Context, client *ent.Client, projID int, traceID int) int {
			t.Helper()
			req, err := client.Request.Create().
				SetProjectID(projID).
				SetTraceID(traceID).
				SetModelID("gpt-4").
				SetFormat("openai/chat_completions").
				SetRequestBody([]byte(`{"model":"gpt-4"}`)).
				SetStatus(request.StatusCompleted).
				SetStream(false).
				Save(ctx)
			require.NoError(t, err)
			return req.ID
		},
		getStatus: func(t *testing.T, ctx context.Context, client *ent.Client, id int) string {
			t.Helper()
			req, err := client.Request.Get(ctx, id)
			require.NoError(t, err)
			return string(req.Status)
		},
		countProcessing: func(ctx context.Context, client *ent.Client) (int, error) {
			return client.Request.Query().
				Where(request.StatusEQ(request.StatusProcessing)).
				Count(ctx)
		},
		callShutdownMethod: func(ctx context.Context, svc *RequestService) error {
			return svc.ClearProcessingRequestsOnShutdown(ctx)
		},
	}
	testClearProcessingOnShutdown(t, tc)
}

func TestRequestService_ClearProcessingRequestsOnShutdown_NoProcessingRequests(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	err := svc.ClearProcessingRequestsOnShutdown(ctx)
	require.NoError(t, err)
}

func TestRequestService_ClearProcessingExecutionsOnShutdown(t *testing.T) {
	tc := testClearProcessingEntity{
		name: "executions",
		createProcessing: func(t *testing.T, ctx context.Context, client *ent.Client, projID int, traceID int) []int {
			t.Helper()
			// First create a request for the executions
			req, err := client.Request.Create().
				SetProjectID(projID).
				SetTraceID(traceID).
				SetModelID("gpt-4").
				SetFormat("openai/chat_completions").
				SetRequestBody([]byte(`{"model":"gpt-4"}`)).
				SetStatus(request.StatusProcessing).
				SetStream(false).
				Save(ctx)
			require.NoError(t, err)

			var ids []int
			for i := 0; i < 2; i++ {
				exec, err := client.RequestExecution.Create().
					SetProjectID(projID).
					SetRequestID(req.ID).
					SetModelID("gpt-4").
					SetFormat("openai/chat_completions").
					SetRequestBody([]byte(`{"model":"gpt-4"}`)).
					SetStatus(requestexecution.StatusProcessing).
					SetStream(false).
					Save(ctx)
				require.NoError(t, err)
				ids = append(ids, exec.ID)
			}
			return ids
		},
		createNonProcessing: func(t *testing.T, ctx context.Context, client *ent.Client, projID int, traceID int) int {
			t.Helper()
			// First create a request for the execution
			req, err := client.Request.Create().
				SetProjectID(projID).
				SetTraceID(traceID).
				SetModelID("gpt-4").
				SetFormat("openai/chat_completions").
				SetRequestBody([]byte(`{"model":"gpt-4"}`)).
				SetStatus(request.StatusProcessing).
				SetStream(false).
				Save(ctx)
			require.NoError(t, err)

			exec, err := client.RequestExecution.Create().
				SetProjectID(projID).
				SetRequestID(req.ID).
				SetModelID("gpt-4").
				SetFormat("openai/chat_completions").
				SetRequestBody([]byte(`{"model":"gpt-4"}`)).
				SetStatus(requestexecution.StatusCompleted).
				SetStream(false).
				Save(ctx)
			require.NoError(t, err)
			return exec.ID
		},
		getStatus: func(t *testing.T, ctx context.Context, client *ent.Client, id int) string {
			t.Helper()
			exec, err := client.RequestExecution.Get(ctx, id)
			require.NoError(t, err)
			return string(exec.Status)
		},
		countProcessing: func(ctx context.Context, client *ent.Client) (int, error) {
			return client.RequestExecution.Query().
				Where(requestexecution.StatusEQ(requestexecution.StatusProcessing)).
				Count(ctx)
		},
		callShutdownMethod: func(ctx context.Context, svc *RequestService) error {
			return svc.ClearProcessingExecutionsOnShutdown(ctx)
		},
	}
	testClearProcessingOnShutdown(t, tc)
}

func TestRequestService_ClearProcessingExecutionsOnShutdown_NoProcessingExecutions(t *testing.T) {
	svc, client, ctx := setupTestRequestService(t)
	defer client.Close()

	err := svc.ClearProcessingExecutionsOnShutdown(ctx)
	require.NoError(t, err)
}
