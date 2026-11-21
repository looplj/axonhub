// 职责: 提供严格鉴权后的 OpenAI GA 和千问原生实时 WebSocket 入口。
// 关联文件:
//
//	internal/server/routes.go
//	internal/server/orchestrator/realtime.go
//	internal/server/realtime/session.go
//	internal/server/realtime/admission.go
//	internal/pkg/xredis/client.go
//
// 注意事项:
//
//	首期仅支持带 Authorization 的服务端客户端，拒绝 Origin 和查询凭证。
//	Redis 按首次实时调用延迟初始化；无实时端点使用时不建立额外连接或后台任务。
//	传输退出后先释放连接租约，再收尾审计；握手等待交接不提高并发上限。

// Package api exposes authenticated HTTP and WebSocket API handlers.
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
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/lifecycle"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	rt "github.com/looplj/axonhub/internal/server/realtime"
)

type RealtimeHandlers struct {
	service      *orchestrator.RealtimeService
	cache        xcache.Config
	mu           sync.Mutex
	admission    *rt.Admission
	redis        redis.UniversalClient
	redisInit    chan struct{}
	redisInitErr error
	sessions     map[uint64]context.CancelFunc
	nextSession  uint64
	drained      chan struct{}
	stopping     bool
}

// ReconcileAudit is an owner-only acknowledgement, not a usage or balance edit.
func (h *RealtimeHandlers) ReconcileAudit(c *gin.Context) {
	if !scopes.IsDirectOwner(c.Request.Context()) {
		JSONError(c, http.StatusForbidden, errors.New("direct owner required"))
		return
	}
	var input struct {
		APIKeyID   int    `json:"apiKeyID" binding:"required,gt=0"`
		RequestIDs []int  `json:"requestIDs"`
		Note       string `json:"note" binding:"required,max=1024"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || len(input.RequestIDs) > 500 {
		JSONError(c, http.StatusBadRequest, errors.New("invalid reconciliation input"))
		return
	}
	count, err := h.service.Usage.ReconcileRealtimeAudits(c.Request.Context(), input.APIKeyID, input.RequestIDs, input.Note)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("realtime audit reconciliation failed"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"reviewedCount": count, "usageOrBalanceChanged": false})
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
		redisInit := h.redisInit
		h.mu.Unlock()
		for _, cancel := range cancels {
			cancel()
		}
		select {
		case <-drained:
		case <-ctx.Done():
			return ctx.Err()
		}
		if redisInit != nil {
			select {
			case <-redisInit:
			case <-ctx.Done():
				return ctx.Err()
			}
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

func (h *RealtimeHandlers) limiter(ctx context.Context, config rt.Config) (*rt.Admission, error) {
	for {
		h.mu.Lock()
		if h.admission != nil {
			admission := h.admission
			h.mu.Unlock()
			return admission, nil
		}
		if h.stopping {
			h.mu.Unlock()
			return nil, errors.New("Realtime admission is shutting down")
		}
		if initializing := h.redisInit; initializing != nil {
			h.mu.Unlock()
			select {
			case <-initializing:
				h.mu.Lock()
				admission, initErr := h.admission, h.redisInitErr
				h.mu.Unlock()
				if admission != nil {
					return admission, nil
				}
				if initErr != nil {
					return nil, errors.New("Realtime shared admission is unavailable")
				}
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		initializing := make(chan struct{})
		h.redisInit = initializing
		redisCfg := h.cache.Redis
		h.mu.Unlock()

		var client redis.UniversalClient
		var err error
		if redisCfg.URL != "" || redisCfg.Addr != "" || len(redisCfg.Addrs) > 0 {
			initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			client, err = xredis.NewClientContext(initCtx, redisCfg)
			cancel()
		}

		h.mu.Lock()
		if err == nil && !h.stopping {
			h.redis = client
			h.admission = rt.NewAdmission(client, config.Limits, config.MaxDuration+config.HandshakeTimeout+time.Minute+10*time.Second)
			client = nil
		} else if err == nil {
			err = errors.New("Realtime admission is shutting down")
		}
		h.redisInitErr = err
		h.redisInit = nil
		close(initializing)
		admission := h.admission
		h.mu.Unlock()
		if client != nil {
			_ = client.Close()
		}
		if err != nil {
			return nil, errors.New("Realtime shared admission is unavailable")
		}
		return admission, nil
	}
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
			JSONError(c, status, errors.New(message))
			return
		}
		limiter, err := h.limiter(ctx, config)
		if err != nil {
			JSONError(c, http.StatusServiceUnavailable, err)
			return
		}
		project, _ := contexts.GetProjectID(ctx)
		target.SessionID = "rt-session-" + uuid.NewString()
		release, err := limiter.AcquireForReconnect(ctx, target.APIKey.ID, project, target.Channel.ID, target.SessionID)
		if err != nil {
			metrics.Metrics.RealtimeEvent(ctx, "admission_rejected")
			JSONError(c, http.StatusTooManyRequests, errors.New("Realtime admission denied"))
			return
		}
		defer release()
		accountingCtx, accountingCancel := context.WithTimeout(ctx, 5*time.Second)
		err = h.service.Usage.EnsureRealtimeUsageReconciled(accountingCtx, target.APIKey.ID)
		accountingCancel()
		if err != nil {
			JSONError(c, http.StatusServiceUnavailable, errors.New("Realtime accounting storage is unavailable"))
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
			if handshake, ok := errors.AsType[*rt.HandshakeError](err); ok {
				status = handshake.Status
			}
			log.Warn(ctx, "realtime upstream establishment failed", log.Int("channel_id", target.Channel.ID), log.Int("upstream_status", status))
			JSONError(c, http.StatusBadGateway, errors.New("Realtime upstream handshake or configuration failed"))
			return
		}
		defer conversation.Close()
		// Connection admission ends after both sockets/workers stop, before audit
		// finalization, which may wait on storage. Keep the outer release for dial failures.
		defer func() { _ = upstream.Close(); release() }()
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
		err = rt.Run(ctx, downstream, upstream, rt.Options{IdleTimeout: config.IdleTimeout, MaxDuration: config.MaxDuration}, rt.Hooks{BindContext: conversation.WithContext, Client: conversation.Client, Server: conversation.Server, ServerFollowup: conversation.ServerFollowup, Check: conversation.Check})
		release() // Run has closed both peers and joined every worker.
		metrics.Metrics.RealtimeDuration(context.WithoutCancel(ctx), time.Since(started).Seconds())
		if err != nil {
			metrics.Metrics.RealtimeEvent(context.WithoutCancel(ctx), "session_ended")
			if rt.IsAbnormal(err) {
				metrics.Metrics.RealtimeEvent(context.WithoutCancel(ctx), "session_abnormal")
			}
			if end, ok := errors.AsType[*rt.SessionError](err); ok {
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
