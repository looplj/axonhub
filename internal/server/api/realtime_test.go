// 职责: 验证实时入口在连接升级之前拒绝 URL 凭证与浏览器 Origin。
// 关联文件:
//
//	internal/server/api/realtime.go
package api

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
			if recover() != nil {
				close(finished)
			}
		}()
		<-ctx.Done()
		time.Sleep(20 * time.Millisecond)
		release()
		close(finished)
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
