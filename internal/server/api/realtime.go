// 职责: 提供严格鉴权后的 OpenAI GA 和千问原生实时 WebSocket 入口。
// 关联文件:
//
//	internal/server/routes.go
//	internal/server/orchestrator/realtime.go
//	internal/server/realtime/session.go
//	internal/pkg/xredis/client.go
//
// 注意事项:
//
//	首期仅支持带 Authorization 的服务端客户端，拒绝 Origin 和查询凭证。
//	Redis 按首次实时调用延迟初始化；无实时端点使用时不建立额外连接或后台任务。
package api

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/pkg/xredis"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/lifecycle"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	rt "github.com/looplj/axonhub/internal/server/realtime"
)

type RealtimeHandlers struct {
	service     *orchestrator.RealtimeService
	cache       xcache.Config
	mu          sync.Mutex
	admission   *rt.Admission
	redis       redis.UniversalClient
	sessions    map[uint64]context.CancelFunc
	nextSession uint64
	drained     chan struct{}
	stopping    bool
}

func NewRealtimeHandlers(service *orchestrator.RealtimeService, cache xcache.Config, lc fx.Lifecycle) *RealtimeHandlers {
	drained := make(chan struct{})
	close(drained)
	h := &RealtimeHandlers{service: service, cache: cache, sessions: map[uint64]context.CancelFunc{}, drained: drained}
	lc.Append(fx.Hook{OnStop: func(ctx context.Context) error {
		h.mu.Lock()
		h.stopping = true
		cancels := make([]context.CancelFunc, 0, len(h.sessions))
		for _, cancel := range h.sessions {
			cancels = append(cancels, cancel)
		}
		drained := h.drained
		client := h.redis
		h.mu.Unlock()
		for _, cancel := range cancels {
			cancel()
		}
		select {
		case <-drained:
		case <-ctx.Done():
			return ctx.Err()
		}
		if client != nil {
			return client.Close()
		}
		return nil
	}})
	return h
}

// RealtimeBoundary is intentionally separate from the ordinary API middleware stack.
func RealtimeBoundary() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") != "" {
			JSONError(c, http.StatusForbidden, errors.New("Realtime currently supports server clients using Authorization headers"))
			c.Abort()
			return
		}
		for key := range c.Request.URL.Query() {
			if key != "model" {
				JSONError(c, http.StatusBadRequest, errors.New("Realtime only accepts the model query parameter"))
				c.Abort()
				return
			}
		}
		if len(c.Request.URL.Query()["model"]) != 1 || c.Query("model") == "" {
			JSONError(c, http.StatusBadRequest, errors.New("exactly one model is required"))
			c.Abort()
			return
		}
		c.Next()
	}
}
func (h *RealtimeHandlers) limiter(config rt.Config) (*rt.Admission, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.admission != nil {
		return h.admission, nil
	}
	redisCfg := h.cache.Redis
	if redisCfg.URL != "" || redisCfg.Addr != "" || len(redisCfg.Addrs) > 0 {
		client, err := xredis.NewClient(redisCfg)
		if err != nil {
			return nil, errors.New("Realtime shared admission is unavailable")
		}
		h.redis = client
	}
	h.admission = rt.NewAdmission(h.redis, config.Limits, config.MaxDuration+config.HandshakeTimeout+time.Minute+10*time.Second)
	return h.admission, nil
}
func (h *RealtimeHandlers) Handler(config rt.Config, native bool) gin.HandlerFunc {
	config = config.Defaults()
	return func(c *gin.Context) {
		if !websocket.IsWebSocketUpgrade(c.Request) {
			JSONError(c, http.StatusBadRequest, errors.New("websocket upgrade required"))
			return
		}
		ctx, cancel := context.WithCancel(c.Request.Context())
		defer cancel()
		untrack, err := h.trackSession(cancel)
		if err != nil {
			JSONError(c, http.StatusServiceUnavailable, err)
			return
		}
		defer untrack()
		stop := context.AfterFunc(lifecycle.FromContext(ctx).Context(), cancel)
		defer stop()
		metrics.Metrics.RealtimeEvent(ctx, "connection_attempt")
		target, err := h.service.Prepare(ctx, c.Query("model"), native)
		if err != nil {
			metrics.Metrics.RealtimeEvent(ctx, "authorization_or_quota_rejected")
			status := http.StatusForbidden
			if errors.Is(err, biz.ErrInvalidModel) {
				status = http.StatusNotFound
			}
			message := "Realtime model unavailable, permission denied, quota exceeded, or required prices not configured"
			if errors.Is(err, biz.ErrRealtimeUsageUnreconciled) {
				status = http.StatusConflict
				message = "Realtime usage is pending reconciliation; additional realtime sessions are blocked"
			}
			JSONError(c, status, errors.New(message))
			return
		}
		limiter, err := h.limiter(config)
		if err != nil {
			JSONError(c, http.StatusServiceUnavailable, err)
			return
		}
		project, _ := contexts.GetProjectID(ctx)
		target.SessionID = "rt-session-" + uuid.NewString()
		release, err := limiter.Acquire(ctx, target.APIKey.ID, project, target.Channel.ID, target.SessionID)
		if err != nil {
			metrics.Metrics.RealtimeEvent(ctx, "admission_rejected")
			JSONError(c, http.StatusTooManyRequests, errors.New("Realtime admission denied"))
			return
		}
		defer release()
		accountingCtx, accountingCancel := context.WithTimeout(ctx, 5*time.Second)
		err = h.service.Usage.EnsureRealtimeUsageReconciled(accountingCtx, target.APIKey.ID, limiter.IsActive)
		accountingCancel()
		if err != nil {
			JSONError(c, http.StatusConflict, errors.New("Realtime accounting is pending reconciliation or session ownership is unavailable"))
			return
		}
		connectionConfig := config
		if target.HandshakeTimeout > 0 {
			connectionConfig.HandshakeTimeout = target.HandshakeTimeout
		}
		dialCtx, dialCancel := context.WithTimeout(ctx, connectionConfig.HandshakeTimeout)
		upstream, conversation, initial, err := h.service.Connect(dialCtx, target, connectionConfig)
		dialCancel()
		if err != nil {
			metrics.Metrics.RealtimeEvent(ctx, "upstream_handshake_failed")
			status := 0
			var handshake *rt.HandshakeError
			if errors.As(err, &handshake) {
				status = handshake.Status
			}
			log.Warn(ctx, "realtime upstream establishment failed", log.Int("channel_id", target.Channel.ID), log.Int("upstream_status", status))
			JSONError(c, http.StatusBadGateway, errors.New("Realtime upstream handshake or configuration failed"))
			return
		}
		defer upstream.Close()
		defer conversation.Close()
		// Conversation accounting must inherit the session, not the short dialing deadline.
		conversation.WithContext(ctx)
		controller := http.NewResponseController(c.Writer)
		_ = controller.SetReadDeadline(time.Time{})
		_ = controller.SetWriteDeadline(time.Time{})
		downstream, err := (&websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, HandshakeTimeout: config.HandshakeTimeout}).Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer downstream.Close()
		_ = downstream.SetWriteDeadline(time.Now().Add(5 * time.Second))
		for _, event := range initial {
			if event != nil {
				if err := downstream.WriteMessage(websocket.TextMessage, event); err != nil {
					return
				}
			}
		}
		metrics.Metrics.RealtimeEvent(ctx, "connection_established")
		metrics.Metrics.RealtimeActive(ctx, 1)
		defer metrics.Metrics.RealtimeActive(context.WithoutCancel(ctx), -1)
		started := time.Now()
		err = rt.Run(ctx, downstream, upstream, rt.Options{IdleTimeout: config.IdleTimeout, MaxDuration: config.MaxDuration}, rt.Hooks{Client: conversation.Client, Server: conversation.Server, Check: conversation.Check})
		metrics.Metrics.RealtimeDuration(context.WithoutCancel(ctx), time.Since(started).Seconds())
		if err != nil {
			metrics.Metrics.RealtimeEvent(context.WithoutCancel(ctx), "session_ended")
			if rt.IsAbnormal(err) {
				metrics.Metrics.RealtimeEvent(context.WithoutCancel(ctx), "session_abnormal")
			}
			var end *rt.SessionError
			if errors.As(err, &end) {
				metrics.Metrics.RealtimeEvent(context.WithoutCancel(ctx), end.Side+"_disconnected")
			}
			if errors.Is(err, rt.ErrBackpressure) {
				metrics.Metrics.RealtimeEvent(context.WithoutCancel(ctx), "backpressure")
			}
		}
		log.Info(ctx, "realtime session closed", log.Int("channel_id", target.Channel.ID), log.Int("api_key_id", target.APIKey.ID), log.Int64("duration_ms", time.Since(started).Milliseconds()), log.Bool("interrupted", rt.IsAbnormal(err)))
	}
}

// trackSession keeps the shutdown barrier open until connections and billing have drained.
func (h *RealtimeHandlers) trackSession(cancel context.CancelFunc) (func(), error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopping {
		return nil, errors.New("Realtime gateway is shutting down")
	}
	if len(h.sessions) == 0 {
		h.drained = make(chan struct{})
	}
	h.nextSession++
	id := h.nextSession
	h.sessions[id] = cancel
	var once sync.Once
	return func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			delete(h.sessions, id)
			if len(h.sessions) == 0 {
				close(h.drained)
			}
		})
	}, nil
}
