package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/privacy"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/tracing"
)

func setupTestTraceMiddleware(t *testing.T) (*gin.Engine, *ent.Client, *biz.TraceService) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")

	systemService := biz.NewSystemService(biz.SystemServiceParams{
		CacheConfig: xcache.Config{},
	})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		Client:        client,
		SystemService: systemService,
		CacheConfig:   xcache.Config{},
		Executor:      executors.NewPoolScheduleExecutor(),
	})
	usageLogService := biz.NewUsageLogService(systemService)
	traceService := biz.NewTraceService(biz.NewRequestService(systemService, usageLogService, dataStorageService))

	router := gin.New()

	return router, client, traceService
}

func TestWithTrace_ClaudeCodeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := tracing.Config{
		TraceHeader:            "AH-Trace-Id",
		ClaudeCodeTraceEnabled: false,
	}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	ctx := privacy.DecisionContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), privacy.Allow)
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	router.Use(func(c *gin.Context) {
		ctx := privacy.DecisionContext(c.Request.Context(), privacy.Allow)
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithTrace(config, traceService))

	var (
		traceHeader  string
		capturedBody []byte
		expectedBody []byte
	)

	router.POST("/anthropic/v1/messages", func(c *gin.Context) {
		traceHeader = c.GetHeader(config.TraceHeader)

		genericReq, err := httpclient.ReadHTTPRequest(c.Request)
		require.NoError(t, err)

		capturedBody = genericReq.Body

		var payload struct {
			Metadata struct {
				UserID string `json:"user_id"`
			} `json:"metadata"`
		}
		require.NoError(t, json.Unmarshal(capturedBody, &payload))
		c.Status(http.StatusOK)
	})

	payload := map[string]any{
		"metadata": map[string]any{
			"user_id": "user_123",
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	expectedBody = body

	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, traceHeader)
	require.JSONEq(t, string(expectedBody), string(capturedBody))
}

func TestWithTrace_ClaudeCodeSetsTraceHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	traceHeaderName := "X-Trace-Id"
	config := tracing.Config{
		TraceHeader:            traceHeaderName,
		ClaudeCodeTraceEnabled: true,
	}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	ctx := privacy.DecisionContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), privacy.Allow)
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	router.Use(func(c *gin.Context) {
		ctx := privacy.DecisionContext(c.Request.Context(), privacy.Allow)
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithTrace(config, traceService))

	var (
		capturedUserID string
		capturedBody   []byte
		expectedBody   []byte
	)

	router.POST("/anthropic/v1/messages", func(c *gin.Context) {
		genericReq, err := httpclient.ReadHTTPRequest(c.Request)
		require.NoError(t, err)

		capturedBody = genericReq.Body

		var payload struct {
			Metadata struct {
				UserID string `json:"user_id"`
			} `json:"metadata"`
		}
		require.NoError(t, json.Unmarshal(capturedBody, &payload))
		capturedUserID = payload.Metadata.UserID

		trace, ok := contexts.GetTrace(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, "xxx", trace.TraceID)

		c.Status(http.StatusOK)
	})

	userID := "user_xxx_account__session_xxx"

	requestPayload := map[string]any{
		"metadata": map[string]any{
			"user_id": userID,
		},
	}
	body, err := json.Marshal(requestPayload)
	require.NoError(t, err)

	expectedBody = body

	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, userID, capturedUserID)
	require.JSONEq(t, string(expectedBody), string(capturedBody))
}

func TestWithTrace_ClaudeCodePreservesExistingTraceHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config := tracing.Config{
		TraceHeader:            "AH-Trace-Id",
		ClaudeCodeTraceEnabled: true,
	}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	ctx := privacy.DecisionContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), privacy.Allow)
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	router.Use(func(c *gin.Context) {
		ctx := privacy.DecisionContext(c.Request.Context(), privacy.Allow)
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithTrace(config, traceService))

	const existingTraceID = "existing-trace"

	var (
		capturedTraceID string
		capturedUserID  string
		capturedBody    []byte
		expectedBody    []byte
	)

	router.POST("/anthropic/v1/messages", func(c *gin.Context) {
		capturedTraceID = c.GetHeader(config.TraceHeader)

		genericReq, err := httpclient.ReadHTTPRequest(c.Request)
		require.NoError(t, err)

		capturedBody = genericReq.Body

		var payload struct {
			Metadata struct {
				UserID string `json:"user_id"`
			} `json:"metadata"`
		}
		require.NoError(t, json.Unmarshal(capturedBody, &payload))
		capturedUserID = payload.Metadata.UserID

		c.Status(http.StatusOK)
	})

	body, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"user_id": "user_123",
		},
	})
	require.NoError(t, err)

	expectedBody = body

	req := httptest.NewRequest(http.MethodPost, "/anthropic/v1/messages", bytes.NewReader(body))
	req.Header.Set("Ah-Trace-Id", existingTraceID)

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, existingTraceID, capturedTraceID)
	require.Equal(t, "user_123", capturedUserID)
	require.JSONEq(t, string(expectedBody), string(capturedBody))
}

func TestWithTraceID_Success(t *testing.T) {
	config := tracing.Config{}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	ctx := privacy.DecisionContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), privacy.Allow)
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Setup middleware and test endpoint
	router.Use(func(c *gin.Context) {
		ctx := privacy.DecisionContext(c.Request.Context(), privacy.Allow)
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithTrace(config, traceService))
	router.GET("/test", func(c *gin.Context) {
		trace, ok := contexts.GetTrace(c.Request.Context())
		if !ok {
			c.JSON(400, gin.H{"error": "trace not found"})
			return
		}

		c.JSON(200, gin.H{"trace_id": trace.TraceID, "id": trace.ID})
	})

	// Test with AH-Trace-Id header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Ah-Trace-Id", "trace-test-123")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify trace was created and stored in context
	trace, err := traceService.GetTraceByID(ctx, "trace-test-123", testProject.ID)
	require.NoError(t, err)
	require.Equal(t, "trace-test-123", trace.TraceID)
}

func TestWithTraceID_WithThread(t *testing.T) {
	config := tracing.Config{}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	ctx := privacy.DecisionContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), privacy.Allow)
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	// Create a test thread
	testThread, err := client.Thread.Create().
		SetThreadID("thread-123").
		SetProjectID(testProject.ID).
		Save(ctx)
	require.NoError(t, err)

	// Setup middleware and test endpoint
	router.Use(func(c *gin.Context) {
		ctx := privacy.DecisionContext(c.Request.Context(), privacy.Allow)
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		ctx = contexts.WithThread(ctx, testThread)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithTrace(config, traceService))
	router.GET("/test", func(c *gin.Context) {
		trace, ok := contexts.GetTrace(c.Request.Context())
		if !ok {
			c.JSON(400, gin.H{"error": "trace not found"})
			return
		}

		c.JSON(200, gin.H{"trace_id": trace.TraceID, "thread_id": trace.ThreadID})
	})

	// Test with AH-Trace-Id header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Ah-Trace-Id", "trace-with-thread-123")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify trace was created with thread relationship
	trace, err := traceService.GetTraceByID(ctx, "trace-with-thread-123", testProject.ID)
	require.NoError(t, err)
	require.Equal(t, "trace-with-thread-123", trace.TraceID)
	require.Equal(t, testThread.ID, trace.ThreadID)
}

func TestWithTraceID_NoHeader(t *testing.T) {
	config := tracing.Config{}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	router.Use(WithTrace(config, traceService))
	router.GET("/test", func(c *gin.Context) {
		_, ok := contexts.GetTrace(c.Request.Context())
		c.JSON(200, gin.H{"has_trace": ok})
	})

	// Test without AH-Trace-Id header
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestWithTraceID_NoProjectID(t *testing.T) {
	config := tracing.Config{}

	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	router.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(ent.NewContext(c.Request.Context(), client))
		c.Next()
	})
	router.Use(WithTrace(config, traceService))
	router.GET("/test", func(c *gin.Context) {
		_, ok := contexts.GetTrace(c.Request.Context())
		c.JSON(200, gin.H{"has_trace": ok})
	})

	// Test with AH-Trace-Id header but no project ID in context
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Ah-Trace-Id", "trace-test-123")

	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should skip trace creation and continue
	require.Equal(t, http.StatusOK, w.Code)
}

func TestWithTraceID_Idempotent(t *testing.T) {
	router, client, traceService := setupTestTraceMiddleware(t)
	defer client.Close()

	ctx := privacy.DecisionContext(httptest.NewRequest(http.MethodGet, "/", nil).Context(), privacy.Allow)
	ctx = ent.NewContext(ctx, client)

	// Create a test project
	testProject, err := client.Project.Create().
		SetName("test-project").
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	router.Use(func(c *gin.Context) {
		ctx := privacy.DecisionContext(c.Request.Context(), privacy.Allow)
		ctx = ent.NewContext(ctx, client)
		ctx = contexts.WithProjectID(ctx, testProject.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(WithTrace(tracing.Config{}, traceService))
	router.GET("/test", func(c *gin.Context) {
		trace, ok := contexts.GetTrace(c.Request.Context())
		if !ok {
			c.JSON(400, gin.H{"error": "trace not found"})
			return
		}

		c.JSON(200, gin.H{"trace_id": trace.TraceID, "id": trace.ID})
	})

	traceID := "trace-idempotent-123"

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("Ah-Trace-Id", traceID)

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	trace1, err := traceService.GetTraceByID(ctx, traceID, testProject.ID)
	require.NoError(t, err)

	// Second request with same trace ID
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Ah-Trace-Id", traceID)

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	trace2, err := traceService.GetTraceByID(ctx, traceID, testProject.ID)
	require.NoError(t, err)

	// Should return the same trace
	require.Equal(t, trace1.ID, trace2.ID)
}
