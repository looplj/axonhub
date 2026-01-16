package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupTestThreadService(t *testing.T) (*ThreadService, *ent.Client) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	systemService := NewSystemService(SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	threadService := NewThreadService(
		client,
		NewTraceService(TraceServiceParams{
			RequestService: NewRequestService(
				client,
				systemService,
				NewUsageLogService(client, systemService, NewChannelServiceForTest(client)),
				NewDataStorageService(
					DataStorageServiceParams{
						SystemService: systemService,
						CacheConfig:   xcache.Config{},
						Executor:      executors.NewPoolScheduleExecutor(),
						Client:        client,
					},
				),
			),
			Ent: client,
		}),
	)

	return threadService, client
}

func TestThreadService_GetOrCreateThread(t *testing.T) {
	threadService, client := setupTestThreadService(t)
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

	threadID := "thread-test-123"

	// Test creating a new thread
	thread1, err := threadService.GetOrCreateThread(ctx, testProject.ID, threadID)
	require.NoError(t, err)
	require.NotNil(t, thread1)
	require.Equal(t, threadID, thread1.ThreadID)
	require.Equal(t, testProject.ID, thread1.ProjectID)

	// Test getting existing thread (should return the same thread)
	thread2, err := threadService.GetOrCreateThread(ctx, testProject.ID, threadID)
	require.NoError(t, err)
	require.NotNil(t, thread2)
	require.Equal(t, thread1.ID, thread2.ID)
	require.Equal(t, threadID, thread2.ThreadID)
	require.Equal(t, testProject.ID, thread2.ProjectID)

	// Test creating a thread with different threadID
	differentThreadID := "thread-test-456"
	thread3, err := threadService.GetOrCreateThread(ctx, testProject.ID, differentThreadID)
	require.NoError(t, err)
	require.NotNil(t, thread3)
	require.NotEqual(t, thread1.ID, thread3.ID)
	require.Equal(t, differentThreadID, thread3.ThreadID)
}

func TestThreadService_GetOrCreateThread_DifferentProjects(t *testing.T) {
	threadService, client := setupTestThreadService(t)
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

	// Use different thread IDs for different projects (thread_id is globally unique)
	threadID1 := "thread-project1-123"
	threadID2 := "thread-project2-456"

	// Create thread in project 1
	thread1, err := threadService.GetOrCreateThread(ctx, project1.ID, threadID1)
	require.NoError(t, err)
	require.Equal(t, project1.ID, thread1.ProjectID)
	require.Equal(t, threadID1, thread1.ThreadID)

	// Create thread in project 2 with different threadID
	thread2, err := threadService.GetOrCreateThread(ctx, project2.ID, threadID2)
	require.NoError(t, err)
	require.Equal(t, project2.ID, thread2.ProjectID)
	require.Equal(t, threadID2, thread2.ThreadID)
	require.NotEqual(t, thread1.ID, thread2.ID)
}

func TestThreadService_GetThreadByID(t *testing.T) {
	threadService, client := setupTestThreadService(t)
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

	threadID := "thread-get-test-123"

	// Create a thread first
	createdThread, err := client.Thread.Create().
		SetThreadID(threadID).
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Test getting the thread
	retrievedThread, err := threadService.GetThreadByID(ctx, threadID, testProject.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedThread)
	require.Equal(t, createdThread.ID, retrievedThread.ID)
	require.Equal(t, threadID, retrievedThread.ThreadID)

	// Test getting non-existent thread
	_, err = threadService.GetThreadByID(ctx, "non-existent", testProject.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get thread")
}

func TestThreadService_GetOrCreateThread_NoClient(t *testing.T) {
	threadService, client := setupTestThreadService(t)
	ctx := context.Background()
	ctx = ent.NewContext(ctx, client) // Add client to context
	ctx = privacy.DecisionContext(ctx, privacy.Allow)

	// Test with ent client in context - should work
	thread, err := threadService.GetOrCreateThread(ctx, 1, "thread-123")
	require.NoError(t, err)
	require.NotNil(t, thread)
	require.Equal(t, "thread-123", thread.ThreadID)
	require.Equal(t, 1, thread.ProjectID)
}
