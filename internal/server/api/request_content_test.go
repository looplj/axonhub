package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/zhenzou/executors"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/datastorage"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestRequestContentHandlers_DownloadRequestContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	cacheConfig := xcache.Config{
		Mode: xcache.ModeMemory,
		Memory: xcache.MemoryConfig{
			Expiration:      5 * time.Minute,
			CleanupInterval: 10 * time.Minute,
		},
	}

	executor := executors.NewPoolScheduleExecutor(executors.WithMaxConcurrent(1))
	t.Cleanup(func() { _ = executor.Shutdown(context.Background()) })

	systemService := biz.NewSystemService(biz.SystemServiceParams{CacheConfig: cacheConfig})
	dataStorageService := biz.NewDataStorageService(biz.DataStorageServiceParams{
		SystemService: systemService,
		CacheConfig:   cacheConfig,
		Executor:      executor,
		Client:        client,
	})

	h := NewRequestContentHandlers(RequestContentHandlersParams{
		DataStorageService: dataStorageService,
	})

	ctx := ent.NewContext(context.Background(), client)
	ctx = authz.WithTestBypass(ctx)

	project, err := client.Project.Create().SetName("p1").SetDescription("d").Save(ctx)
	require.NoError(t, err)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := ent.NewContext(c.Request.Context(), client)
		ctx = contexts.WithUser(ctx, &ent.User{ID: 1, IsOwner: true})
		ctx = contexts.WithProjectID(ctx, project.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	router.GET("/admin/requests/:request_id/content", h.DownloadRequestContent)

	contentDir := t.TempDir()
	ds, err := client.DataStorage.Create().
		SetName("content-fs").
		SetDescription("content").
		SetPrimary(false).
		SetType(datastorage.TypeFs).
		SetSettings(&objects.DataStorageSettings{Directory: &contentDir}).
		SetStatus(datastorage.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	reqRow, err := client.Request.Create().
		SetProjectID(project.ID).
		SetModelID("m1").
		SetFormat("openai/video").
		SetSource("api").
		SetStatus("completed").
		SetStream(false).
		SetClientIP("").
		SetRequestHeaders(objects.JSONRawMessage(`{}`)).
		SetRequestBody(objects.JSONRawMessage(`{}`)).
		Save(ctx)
	require.NoError(t, err)

	key := fmt.Sprintf("/%d/requests/%d/video/video.mp4", project.ID, reqRow.ID)
	fullPath := filepath.Join(contentDir, filepath.FromSlash(key))
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0o755))
	require.NoError(t, os.WriteFile(fullPath, []byte("video-content"), 0o644))

	_, err = client.Request.UpdateOneID(reqRow.ID).
		SetContentSaved(true).
		SetContentStorageID(ds.ID).
		SetContentStorageKey(key).
		Save(ctx)
	require.NoError(t, err)

	requestIDStr := fmt.Sprintf("%d", reqRow.ID)

	t.Run("downloads content", func(t *testing.T) {
		path := fmt.Sprintf(
			"/admin/requests/%s/content",
			url.PathEscape(requestIDStr),
		)

		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("unexpected status=%d body=%s", w.Code, w.Body.String())
		}

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "video-content", w.Body.String())
		require.Contains(t, w.Header().Get("Content-Disposition"), "video.mp4")
	})

	t.Run("returns 404 for mismatched project", func(t *testing.T) {
		router2 := gin.New()
		router2.Use(func(c *gin.Context) {
			ctx := ent.NewContext(c.Request.Context(), client)
			ctx = contexts.WithUser(ctx, &ent.User{ID: 1, IsOwner: true})
			ctx = contexts.WithProjectID(ctx, project.ID+999)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
		router2.GET("/admin/requests/:request_id/content", h.DownloadRequestContent)

		path := fmt.Sprintf("/admin/requests/%s/content", url.PathEscape(requestIDStr))
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router2.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("returns 404 when not saved", func(t *testing.T) {
		unsaved, err := client.Request.Create().
			SetProjectID(project.ID).
			SetModelID("m2").
			SetFormat("openai/video").
			SetSource("api").
			SetStatus("completed").
			SetStream(false).
			SetClientIP("").
			SetRequestHeaders(objects.JSONRawMessage(`{}`)).
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetContentSaved(false).
			Save(ctx)
		require.NoError(t, err)

		unsavedIDStr := fmt.Sprintf("%d", unsaved.ID)
		path := fmt.Sprintf(
			"/admin/requests/%s/content",
			url.PathEscape(unsavedIDStr),
		)

		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("escapes directory traversal on fs", func(t *testing.T) {
		traversalKey := "/../../etc/passwd"
		evil, err := client.Request.Create().
			SetProjectID(project.ID).
			SetModelID("m3").
			SetFormat("openai/video").
			SetSource("api").
			SetStatus("completed").
			SetStream(false).
			SetClientIP("").
			SetRequestHeaders(objects.JSONRawMessage(`{}`)).
			SetRequestBody(objects.JSONRawMessage(`{}`)).
			SetContentSaved(true).
			SetContentStorageID(ds.ID).
			SetContentStorageKey(traversalKey).
			Save(ctx)
		require.NoError(t, err)

		evilIDStr := fmt.Sprintf("%d", evil.ID)
		path := fmt.Sprintf(
			"/admin/requests/%s/content",
			url.PathEscape(evilIDStr),
		)

		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code)
	})
}
