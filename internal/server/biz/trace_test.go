package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
)

func setupTestTraceService(t *testing.T) (*TraceService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	traceService := NewTraceService()

	return traceService, client
}

func TestTraceService_GetOrCreateTrace(t *testing.T) {
	traceService, client := setupTestTraceService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-test-123"

	// Test creating a new trace without thread
	trace1, err := traceService.GetOrCreateTrace(ctx, testProject.ID, traceID, nil)
	require.NoError(t, err)
	require.NotNil(t, trace1)
	require.Equal(t, traceID, trace1.TraceID)
	require.Equal(t, testProject.ID, trace1.ProjectID)

	// Test getting existing trace (should return the same trace)
	trace2, err := traceService.GetOrCreateTrace(ctx, testProject.ID, traceID, nil)
	require.NoError(t, err)
	require.NotNil(t, trace2)
	require.Equal(t, trace1.ID, trace2.ID)
	require.Equal(t, traceID, trace2.TraceID)
	require.Equal(t, testProject.ID, trace2.ProjectID)

	// Test creating a trace with different traceID
	differentTraceID := "trace-test-456"
	trace3, err := traceService.GetOrCreateTrace(ctx, testProject.ID, differentTraceID, nil)
	require.NoError(t, err)
	require.NotNil(t, trace3)
	require.NotEqual(t, trace1.ID, trace3.ID)
	require.Equal(t, differentTraceID, trace3.TraceID)
}

func TestTraceService_GetOrCreateTrace_WithThread(t *testing.T) {
	traceService, client := setupTestTraceService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create a thread
	testThread, err := client.Thread.Create().
		SetThreadID("thread-123").
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-with-thread-123"

	// Test creating a trace with thread
	trace, err := traceService.GetOrCreateTrace(ctx, testProject.ID, traceID, &testThread.ID)
	require.NoError(t, err)
	require.NotNil(t, trace)
	require.Equal(t, traceID, trace.TraceID)
	require.Equal(t, testProject.ID, trace.ProjectID)
	require.Equal(t, testThread.ID, trace.ThreadID)
}

func TestTraceService_GetOrCreateTrace_DifferentProjects(t *testing.T) {
	traceService, client := setupTestTraceService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create two test projects
	project1, err := client.Project.Create().
		SetName("project-1").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	project2, err := client.Project.Create().
		SetName("project-2").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Use different trace IDs for different projects (trace_id is globally unique)
	traceID1 := "trace-project1-123"
	traceID2 := "trace-project2-456"

	// Create trace in project 1
	trace1, err := traceService.GetOrCreateTrace(ctx, project1.ID, traceID1, nil)
	require.NoError(t, err)
	require.Equal(t, project1.ID, trace1.ProjectID)
	require.Equal(t, traceID1, trace1.TraceID)

	// Create trace in project 2 with different traceID
	trace2, err := traceService.GetOrCreateTrace(ctx, project2.ID, traceID2, nil)
	require.NoError(t, err)
	require.Equal(t, project2.ID, trace2.ProjectID)
	require.Equal(t, traceID2, trace2.TraceID)
	require.NotEqual(t, trace1.ID, trace2.ID)
}

func TestTraceService_GetTraceByID(t *testing.T) {
	traceService, client := setupTestTraceService(t)
	defer client.Close()

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	traceID := "trace-get-test-123"

	// Create a trace first
	createdTrace, err := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Test getting the trace
	retrievedTrace, err := traceService.GetTraceByID(ctx, traceID, testProject.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedTrace)
	require.Equal(t, createdTrace.ID, retrievedTrace.ID)
	require.Equal(t, traceID, retrievedTrace.TraceID)

	// Test getting non-existent trace
	_, err = traceService.GetTraceByID(ctx, "non-existent", testProject.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get trace")
}

func TestTraceService_GetOrCreateTrace_NoClient(t *testing.T) {
	traceService := NewTraceService()
	ctx := context.Background()

	// Test without ent client in context
	_, err := traceService.GetOrCreateTrace(ctx, 1, "trace-123", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ent client not found in context")
}
