// 职责: 验证实时入口安全边界、对账管理员权限与优雅关闭。
// 关联文件:
//
//	internal/server/api/realtime.go

package api

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	rt "github.com/looplj/axonhub/internal/server/realtime"
)

type realtimeAPIKeyFailureRecorder struct {
	channelID int
	apiKey    string
	status    int
	message   string
}

func (r *realtimeAPIKeyFailureRecorder) EvaluateAPIKeyRulesForFailure(_ context.Context, channelID int, apiKey string, status int, message string) bool {
	r.channelID = channelID
	r.apiKey = apiKey
	r.status = status
	r.message = message
	return true
}

func TestRealtimeHandshakeFailureAppliesAutoDisableToSelectedChannelKey(t *testing.T) {
	ctx := contexts.EnsureContainer(context.Background())
	contexts.WithChannelAPIKey(ctx, "selected-provider-key")
	recorder := &realtimeAPIKeyFailureRecorder{}
	target := &orchestrator.RealtimeTarget{Channel: &biz.Channel{Channel: &ent.Channel{ID: 17}}}
	handshake := &rt.HandshakeError{Status: http.StatusUnauthorized, FailureReason: "invalid_api_key: provider rejected the credential"}

	evaluateRealtimeAPIKeyFailure(ctx, recorder, target, http.StatusUnauthorized, realtimeAPIKeyFailureMessage(handshake, "upstream handshake rejected"))

	require.Equal(t, 17, recorder.channelID)
	require.Equal(t, "selected-provider-key", recorder.apiKey)
	require.Equal(t, http.StatusUnauthorized, recorder.status)
	require.Equal(t, "invalid_api_key: provider rejected the credential", recorder.message)
}

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

func TestRealtimeAccountingAndDialRunInParallel(t *testing.T) {
	var started sync.WaitGroup
	started.Add(2)
	delay := 350 * time.Millisecond
	accounting := func(ctx context.Context) error {
		started.Done()
		started.Wait()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}
	dial := func(ctx context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
		started.Done()
		started.Wait()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		case <-timer.C:
			return nil, nil, nil, nil
		}
	}

	result, timing := runRealtimeSetup(context.Background(), time.Second, accounting, dial)
	require.NoError(t, result.AccountingErr)
	require.NoError(t, result.DialErr)
	require.GreaterOrEqual(t, timing.AccountingMs, delay.Milliseconds()-25)
	require.GreaterOrEqual(t, timing.DialMs, delay.Milliseconds()-25)
	require.Less(t, timing.TotalMs, timing.AccountingMs+timing.DialMs-150)
}

func TestRealtimeAccountingFailureClosesSuccessfulDial(t *testing.T) {
	upstream, provider := apiRealtimeSocketPair(t)
	accountingErr := errors.New("synthetic accounting failure")
	result, _ := runRealtimeSetup(context.Background(), time.Second,
		func(context.Context) error { return accountingErr },
		func(context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
			time.Sleep(25 * time.Millisecond)
			return upstream, nil, nil, nil
		},
	)
	require.ErrorIs(t, result.AccountingErr, accountingErr)
	require.NoError(t, result.DialErr)
	require.Nil(t, result.Upstream, "an upstream connection cannot escape a failed accounting check")
	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, err := provider.Read(readCtx)
	require.Error(t, err, "successful dial must be closed when accounting fails")
}

func TestRealtimeSetupWorkerPanicsDoNotLeaveWaitersBlocked(t *testing.T) {
	t.Run("accounting", func(t *testing.T) {
		result, _ := runRealtimeSetup(context.Background(), time.Second,
			func(context.Context) error { panic("synthetic accounting panic detail") },
			func(context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
				return nil, nil, nil, nil
			},
		)
		require.ErrorContains(t, result.AccountingErr, "realtime accounting setup failed")
	})
	t.Run("dial", func(t *testing.T) {
		result, _ := runRealtimeSetup(context.Background(), time.Second,
			func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			func(context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
				panic("synthetic dial panic detail")
			},
		)
		require.ErrorContains(t, result.AccountingErr, "context canceled")
		require.ErrorContains(t, result.DialErr, "realtime upstream handshake failed")
	})
}

func TestRealtimeDialFailureMarksAccountingCancellation(t *testing.T) {
	dialErr := errors.New("synthetic upstream dial failure")
	result, _ := runRealtimeSetup(context.Background(), time.Second,
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		func(context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
			return nil, nil, nil, dialErr
		},
	)
	require.ErrorIs(t, result.DialErr, dialErr)
	require.ErrorIs(t, result.AccountingErr, context.Canceled)
}

func TestRealtimeDialFailureDoesNotMaskCompletedAccountingError(t *testing.T) {
	accountingErr := errors.New("synthetic independent accounting failure")
	accountingReturned := make(chan struct{})
	result, _ := runRealtimeSetup(context.Background(), time.Second,
		func(context.Context) error {
			close(accountingReturned)
			return accountingErr
		},
		func(context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
			<-accountingReturned
			return nil, nil, nil, errors.New("synthetic dial failure")
		},
	)
	require.ErrorIs(t, result.AccountingErr, accountingErr)
	require.ErrorContains(t, result.DialErr, "synthetic dial failure")
}

func apiRealtimeSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err == nil {
			accepted <- conn
		}
	}))
	t.Cleanup(server.Close)
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(dialCtx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	peer := <-accepted
	t.Cleanup(func() { _ = client.CloseNow(); _ = peer.CloseNow() })
	return client, peer
}

func TestRealtimeHandshakeErrorsDistinguishTimeoutAndRejection(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		router := gin.New()
		router.GET("/realtime", func(c *gin.Context) {
			writeRealtimeHandshakeError(c, context.DeadlineExceeded)
		})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/realtime", nil))
		status, code, message := response.Code, "upstream_handshake_timeout", "Realtime upstream handshake timeout"
		require.Equal(t, http.StatusGatewayTimeout, status)
		require.Contains(t, response.Body.String(), code)
		require.Contains(t, response.Body.String(), "error_code")
		require.Contains(t, response.Body.String(), message)
		require.Contains(t, strings.ToLower(message), "timeout")
	})
	t.Run("upstream rejection", func(t *testing.T) {
		router := gin.New()
		router.GET("/realtime", func(c *gin.Context) {
			writeRealtimeHandshakeError(c, &rt.HandshakeError{Status: http.StatusUnauthorized})
		})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/realtime", nil))
		status, code, message := response.Code, "upstream_handshake_failed", "Realtime upstream handshake was rejected"
		require.Equal(t, http.StatusBadGateway, status)
		require.Contains(t, response.Body.String(), code)
		require.Contains(t, response.Body.String(), message)
		require.Contains(t, strings.ToLower(message), "rejected")
	})
}

func TestRealtimeSetupTimingLogContainsStagesAndUsesSlowThreshold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "realtime.log")
	log.SetGlobalConfig(log.Config{
		Level:    log.DebugLevel,
		Output:   "file",
		Encoding: "json",
		File:     log.FileConfig{Path: path},
	})
	t.Cleanup(func() { log.SetGlobalConfig(log.Config{}) })

	h := &RealtimeHandlers{}
	target := &orchestrator.RealtimeTarget{
		Channel: &biz.Channel{Channel: &ent.Channel{ID: 17}},
		Model:   "qwen3.8-omni-flash-realtime",
		Native:  true,
	}
	fast := realtimeSetupTimings{
		Started: time.Now().Add(-100 * time.Millisecond), AdmissionMs: 10,
		AccountingMs: 200, DialMs: 210, UpgradeMs: 2,
	}
	h.logSetupTimings(context.Background(), target, fast)
	slow := fast
	slow.Started = time.Now().Add(-1100 * time.Millisecond)
	h.logSetupTimings(context.Background(), target, slow)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, lines[0], `"level":"DEBUG"`)
	require.Contains(t, lines[1], `"level":"INFO"`)
	for _, field := range []string{"admission_ms", "accounting_ms", "dial_ms", "upgrade_ms", "total_ms", "channel_id", "upstream_model", "native_qwen"} {
		require.Contains(t, string(data), field)
	}
	require.NotContains(t, string(data), "Authorization")
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
	var serverWorkers sync.WaitGroup
	serverWorkers.Add(1)
	startRealtimeTestWorker(t, "Redis test listener", func() {
		defer serverWorkers.Done()
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			select {
			case accepted <- struct{}{}:
			default:
			}
			serverWorkers.Add(1)
			startRealtimeTestWorker(t, "Redis test connection drainer", func() {
				defer serverWorkers.Done()
				defer conn.Close()
				_, _ = io.Copy(io.Discard, conn)
			})
		}
	})
	defer func() {
		_ = listener.Close()
		serverWorkers.Wait()
	}()

	h := &RealtimeHandlers{cache: xcache.Config{Redis: xredis.Config{Addr: listener.Addr().String()}}, sessions: map[uint64]context.CancelFunc{}, drained: make(chan struct{})}
	close(h.drained)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	initialized := make(chan error, 1)
	startRealtimeTestWorker(t, "Redis admission initializer", func() {
		_, initErr := h.limiter(ctx, rt.Config{})
		initialized <- initErr
	})
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("Redis initialization did not reach the stalled test server")
	}

	trackDone := make(chan error, 1)
	startRealtimeTestWorker(t, "Realtime session tracker", func() {
		untrack, trackErr := h.trackSession(func() {})
		if trackErr == nil {
			untrack()
		}
		trackDone <- trackErr
	})
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

func startRealtimeTestWorker(t *testing.T, name string, worker func()) {
	t.Helper()
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("%s panicked: %v", name, recovered)
			}
		}()
		worker()
	}()
}
