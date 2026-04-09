package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/tracing"
)

func setupTestThreadMiddleware(t *testing.T) (*gin.Engine, *ent.Client, *biz.ThreadService) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{},
		Ent:         client,
	})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		Client:        client,
		SystemService: systemService,
		CacheConfig:   xcache.Config{},
		Executor:      executors.NewPoolScheduleExecutor(),
	})
	channelService := biz.NewChannelServiceForTest(client)
	usageLogService := biz.NewUsageLogService(client, systemService, channelService)
	traceService := biz.NewTraceService(biz.TraceServiceParams{
		RequestService: biz.NewRequestService(client, systemService, usageLogService, dataStorageService),
		Ent:            client,
	})

	threadService := biz.NewThreadService(client, traceService)

	router := gin.New()

	return router, client, threadService
}

func TestWithThreadID_Success(t *testing.T) {
	router, client, threadService := setupTestThreadMiddleware(t)
	defer client.Close()

	ctx := authz.WithTestBypass(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Setup middleware and test endpoint
	router.Use(func(c *gin.Context) {
		// Add privacy context
		ctx := authz.WithTestBypass(c.Request.Context())
		// Add ent client to context
		ctx = ent.NewContext(ctx, client)
		// Add project ID to context
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithThread(tracing.Config{}, threadService))
	router.GET("/test", func(c *gin.Context) {
		thread, ok := contexts.GetThread(c.Request.Context())
		if !ok {
			c.JSON(400, gin.H{"error": "thread not found"})
			return
		}

		c.JSON(200, gin.H{"thread_id": thread.ThreadID, "id": thread.ID})
	})

	// Test with AH-Thread-Id header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Ah-Thread-Id", "thread-test-123")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify thread was created and stored in context
	thread, err := threadService.GetThreadByID(ctx, "thread-test-123", testProject.ID)
	require.NoError(t, err)
	require.Equal(t, "thread-test-123", thread.ThreadID)
}

func TestWithThreadID_NoHeader(t *testing.T) {
	router, client, threadService := setupTestThreadMiddleware(t)
	defer client.Close()

	router.Use(WithThread(tracing.Config{}, threadService))
	router.GET("/test", func(c *gin.Context) {
		_, ok := contexts.GetThread(c.Request.Context())
		c.JSON(200, gin.H{"has_thread": ok})
	})

	// Test without AH-Thread-Id header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestWithThreadID_NoProjectID(t *testing.T) {
	router, client, threadService := setupTestThreadMiddleware(t)
	defer client.Close()

	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ent.NewContext(c.Request.Context(), client))
		c.Next()
	})
	router.Use(WithThread(tracing.Config{}, threadService))
	router.GET("/test", func(c *gin.Context) {
		_, ok := contexts.GetThread(c.Request.Context())
		c.JSON(200, gin.H{"has_thread": ok})
	})

	// Test with AH-Thread-Id header but no project ID in context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Ah-Thread-Id", "thread-test-123")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should skip thread creation and continue
	require.Equal(t, http.StatusOK, w.Code)
}

func TestWithThreadID_Idempotent(t *testing.T) {
	router, client, threadService := setupTestThreadMiddleware(t)
	defer client.Close()

	ctx := authz.WithTestBypass(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	router.Use(func(c *gin.Context) {
		ctx := authz.WithTestBypass(c.Request.Context())
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithThread(tracing.Config{}, threadService))
	router.GET("/test", func(c *gin.Context) {
		thread, ok := contexts.GetThread(c.Request.Context())
		if !ok {
			c.JSON(400, gin.H{"error": "thread not found"})
			return
		}

		c.JSON(200, gin.H{"thread_id": thread.ThreadID, "id": thread.ID})
	})

	threadID := "thread-idempotent-123"

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("Ah-Thread-Id", threadID)

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	thread1, err := threadService.GetThreadByID(ctx, threadID, testProject.ID)
	require.NoError(t, err)

	// Second request with same thread ID
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Ah-Thread-Id", threadID)

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	thread2, err := threadService.GetThreadByID(ctx, threadID, testProject.ID)
	require.NoError(t, err)

	// Should return the same thread
	require.Equal(t, thread1.ID, thread2.ID)
}
