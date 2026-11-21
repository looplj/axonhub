// 职责: 验证实时入口安全边界、对账管理员权限与优雅关闭。
// 关联文件:
//
//	internal/server/api/realtime.go

package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	rt "github.com/looplj/axonhub/internal/server/realtime"
)

func TestRealtimeReconcileBoundaryRejectsNonOwner(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		router := gin.New()
		h := &RealtimeHandlers{}
		router.POST("/admin/realtime/reconcile", func(c *gin.Context) {
			if authenticated {
				c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), &ent.User{ID: 42, IsOwner: false}))
			}
			h.ReconcileAudit(c)
		})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/realtime/reconcile", nil))
		require.Equal(t, http.StatusForbidden, w.Code, "unprivileged requests must stop before accessing accounting")
	}
}

func TestRealtimeBoundaryRejectsCredentialsInQueryAndOrigin(t *testing.T) {
	for _, tc := range []struct {
		url, origin string
		status      int
	}{{"/v1/realtime?model=voice&api_key=secret", "", 400}, {"/v1/realtime?model=voice", "https://evil.test", 403}, {"/v1/realtime?model=voice&host=127.0.0.1", "", 400}} {
		router := gin.New()
		router.GET("/v1/realtime", RealtimeBoundary(), func(c *gin.Context) { c.Status(204) })
		req := httptest.NewRequest(http.MethodGet, tc.url, nil)
		req.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		require.Equal(t, tc.status, w.Code)
		require.NotContains(t, w.Body.String(), "secret")
	}
}

func TestRealtimeShutdownDrainsBeforeServicesStop(t *testing.T) {
	var h *RealtimeHandlers
	app := fx.New(fx.NopLogger, fx.Supply(&orchestrator.RealtimeService{}, xcache.Config{}), fx.Provide(NewRealtimeHandlers), fx.Populate(&h))
	require.NoError(t, app.Err())
	require.NoError(t, app.Start(t.Context()))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release, err := h.trackSession(cancel)
	require.NoError(t, err)
	finished := make(chan struct{})
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("shutdown worker panicked: %v", recovered)
			}
			close(finished)
		}()
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		release()
	}()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	start := time.Now()
	require.NoError(t, app.Stop(stopCtx))
	require.GreaterOrEqual(t, time.Since(start), 20*time.Millisecond)
	<-finished
	_, err = h.trackSession(cancel)
	require.Error(t, err)
}

func TestRealtimeRedisInitializationIsCancelableWithoutSessionLock(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			accepted <- struct{}{}
			go func() { defer conn.Close(); _, _ = io.Copy(io.Discard, conn) }()
		}
	}()

	h := &RealtimeHandlers{cache: xcache.Config{Redis: xredis.Config{Addr: listener.Addr().String()}}, sessions: map[uint64]context.CancelFunc{}, drained: make(chan struct{})}
	close(h.drained)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	initialized := make(chan error, 1)
	go func() {
		_, initErr := h.limiter(ctx, rt.Config{})
		initialized <- initErr
	}()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("Redis initialization did not reach the stalled test server")
	}

	trackDone := make(chan error, 1)
	go func() {
		untrack, trackErr := h.trackSession(func() {})
		if trackErr == nil {
			untrack()
		}
		trackDone <- trackErr
	}()
	select {
	case trackErr := <-trackDone:
		require.NoError(t, trackErr, "session tracking must not wait for Redis network I/O")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Redis initialization held the session-tracking lock")
	}
	select {
	case initErr := <-initialized:
		require.Error(t, initErr, "canceling the request must stop Redis initialization")
	case <-time.After(time.Second):
		t.Fatal("Redis initialization ignored request cancellation")
	}
}
