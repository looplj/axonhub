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
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"

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

type realtimeSetupTimings struct {
	Started      time.Time
	AdmissionMs  int64
	AccountingMs int64
	DialMs       int64
	UpgradeMs    int64
	TotalMs      int64
}

type realtimeSetupOutcome struct {
	Upstream      *websocket.Conn
	Conversation  *orchestrator.RealtimeConversation
	Initial       [][]byte
	AccountingErr error
	DialErr       error
}

type realtimeSetupPart struct {
	upstream     *websocket.Conn
	conversation *orchestrator.RealtimeConversation
	initial      [][]byte
	err          error
	elapsedMs    int64
}

func logRealtimeSetupWorkerPanic(ctx context.Context, worker string, recovered any) {
	defer func() { _ = recover() }()
	log.Error(ctx, "realtime setup worker panicked", log.String("worker", worker), log.String("panic_type", fmt.Sprintf("%T", recovered)))
}

func cleanupRealtimeSetupResult(part realtimeSetupPart) {
	if part.conversation != nil {
		func() {
			defer func() { _ = recover() }()
			part.conversation.Close()
		}()
	}
	if part.upstream != nil {
		func() {
			defer func() { _ = recover() }()
			_ = part.upstream.Close()
		}()
	}
}

func runRealtimeSetup(
	ctx context.Context,
	handshakeTimeout time.Duration,
	accounting func(context.Context) error,
	dial func(context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error),
) (realtimeSetupOutcome, realtimeSetupTimings) {
	started := time.Now()
	accountingCtx, cancelAccounting := context.WithTimeout(ctx, 5*time.Second)
	dialCtx, cancelDial := context.WithTimeout(ctx, handshakeTimeout)
	defer cancelAccounting()
	defer cancelDial()

	accountingResults := make(chan realtimeSetupPart, 1)
	accountingReturned := make(chan struct{})
	dialResults := make(chan realtimeSetupPart, 1)
	go func() {
		partStarted := time.Now()
		part := realtimeSetupPart{}
		defer func() {
			recovered := recover()
			close(accountingReturned)
			if recovered != nil {
				logRealtimeSetupWorkerPanic(ctx, "accounting", recovered)
				part.err = errors.New("realtime accounting setup failed")
			}
			part.elapsedMs = time.Since(partStarted).Milliseconds()
			accountingResults <- part
		}()
		part.err = accounting(accountingCtx)
	}()
	go func() {
		partStarted := time.Now()
		part := realtimeSetupPart{}
		defer func() {
			if recovered := recover(); recovered != nil {
				logRealtimeSetupWorkerPanic(ctx, "dial", recovered)
				part.err = errors.New("realtime upstream handshake failed")
				cleanupRealtimeSetupResult(part)
			}
			part.elapsedMs = time.Since(partStarted).Milliseconds()
			dialResults <- part
		}()
		part.upstream, part.conversation, part.initial, part.err = dial(dialCtx)
	}()

	var outcome realtimeSetupOutcome
	timings := realtimeSetupTimings{Started: started}
	accountingDone, dialDone := false, false
	for !accountingDone || !dialDone {
		select {
		case part := <-accountingResults:
			accountingDone = true
			outcome.AccountingErr = part.err
			timings.AccountingMs = part.elapsedMs
			if part.err != nil && !dialDone {
				cancelDial()
			}
		case part := <-dialResults:
			dialDone = true
			outcome.Upstream = part.upstream
			outcome.Conversation = part.conversation
			outcome.Initial = part.initial
			outcome.DialErr = part.err
			timings.DialMs = part.elapsedMs
			if part.err != nil && !accountingDone {
				select {
				case <-accountingReturned:
					// Preserve an accounting failure that completed before dial failed,
					// even when its result has not yet been selected from the channel.
				default:
					cancelAccounting()
				}
			}
		}
	}
	timings.TotalMs = time.Since(started).Milliseconds()

	if outcome.AccountingErr != nil || outcome.DialErr != nil {
		cleanupRealtimeSetupResult(realtimeSetupPart{conversation: outcome.Conversation, upstream: outcome.Upstream})
		if outcome.Upstream != nil {
			outcome.Upstream = nil
		}
		outcome.Conversation = nil
		outcome.Initial = nil
	}
	return outcome, timings
}

func realtimeHandshakeError(err error) (int, string, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout, "upstream_handshake_timeout", "Realtime upstream handshake timeout"
	}
	if handshake, ok := errors.AsType[*rt.HandshakeError](err); ok && handshake.Status > 0 {
		return http.StatusBadGateway, "upstream_handshake_failed", "Realtime upstream handshake was rejected"
	}
	return http.StatusBadGateway, "upstream_handshake_failed", "Realtime upstream handshake failed"
}

func writeRealtimeHandshakeError(c *gin.Context, err error) (int, string, string) {
	status, code, message := realtimeHandshakeError(err)
	c.JSON(status, gin.H{"error": gin.H{
		"type": "upstream_error", "code": code, "error_code": code, "message": message,
	}})
	return status, code, message
}

func (h *RealtimeHandlers) logSetupTimings(ctx context.Context, target *orchestrator.RealtimeTarget, timings realtimeSetupTimings) {
	if target == nil || target.Channel == nil {
		return
	}
	timings.TotalMs = time.Since(timings.Started).Milliseconds()
	fields := []zap.Field{
		log.Int("channel_id", target.Channel.ID),
		log.String("upstream_model", target.Model),
		log.Bool("native_qwen", target.Native),
		log.Int64("admission_ms", timings.AdmissionMs),
		log.Int64("accounting_ms", timings.AccountingMs),
		log.Int64("dial_ms", timings.DialMs),
		log.Int64("upgrade_ms", timings.UpgradeMs),
		log.Int64("total_ms", timings.TotalMs),
	}
	if timings.TotalMs >= 1000 {
		log.Info(ctx, "realtime connection setup timings", fields...)
		return
	}
	log.Debug(ctx, "realtime connection setup timings", fields...)
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
	return h.handler(config, func(ctx context.Context, model string) (*orchestrator.RealtimeTarget, error) {
		return h.service.Prepare(ctx, model, native)
	})
}

func (h *RealtimeHandlers) TranslationHandler(config rt.Config) gin.HandlerFunc {
	return h.handler(config, h.service.PrepareTranslation)
}

func (h *RealtimeHandlers) handler(config rt.Config, prepare func(context.Context, string) (*orchestrator.RealtimeTarget, error)) gin.HandlerFunc {
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
		target, err := prepare(ctx, c.Query("model"))
		if err != nil {
			metrics.Metrics.RealtimeEvent(ctx, "authorization_or_quota_rejected")
			status := http.StatusForbidden
			if errors.Is(err, biz.ErrInvalidModel) {
				status = http.StatusNotFound
			} else if errors.Is(err, orchestrator.ErrRealtimeCircuitBreakerOpen) {
				status = http.StatusServiceUnavailable
			}
			message := "Realtime model unavailable, permission denied, quota exceeded, or invalid price configuration"
			if errors.Is(err, orchestrator.ErrRealtimeCircuitBreakerOpen) {
				message = err.Error()
			}
			JSONError(c, status, errors.New(message))
			return
		}
		defer h.service.ReleaseRealtimeProbe(target)
		setupTimings := realtimeSetupTimings{Started: time.Now()}
		attemptStarted := setupTimings.Started
		admissionStarted := time.Now()
		limiter, err := h.limiter(ctx, config)
		if err != nil {
			setupTimings.AdmissionMs = time.Since(admissionStarted).Milliseconds()
			h.logSetupTimings(ctx, target, setupTimings)
			JSONError(c, http.StatusServiceUnavailable, err)
			return
		}
		project, _ := contexts.GetProjectID(ctx)
		target.SessionID = "rt-session-" + uuid.NewString()
		target.UserAgent = realtimeClientLabel(c.Request)
		release, err := limiter.AcquireForReconnect(ctx, target.APIKey.ID, project, target.Channel.ID, target.SessionID)
		setupTimings.AdmissionMs = time.Since(admissionStarted).Milliseconds()
		if err != nil {
			metrics.Metrics.RealtimeEvent(ctx, "admission_rejected")
			if !errors.Is(err, rt.ErrAdmission) {
				h.logSetupTimings(ctx, target, setupTimings)
				JSONError(c, http.StatusServiceUnavailable, errors.New("Realtime admission unavailable"))
				return
			}
			message := err.Error()
			h.logSetupTimings(ctx, target, setupTimings)
			h.recordConnectionFailure(ctx, target, "admission_denied", message, attemptStarted)
			log.Info(ctx, "realtime admission rejected", log.String("error_code", "admission_denied"), log.Int("channel_id", target.Channel.ID))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{
				"type": "rate_limit_exceeded", "code": "rate_limit_exceeded", "message": message,
			}})
			return
		}
		defer release()
		connectionConfig := config
		if target.HandshakeTimeout > 0 {
			connectionConfig.HandshakeTimeout = target.HandshakeTimeout
		}
		setup, setupDurations := runRealtimeSetup(ctx, connectionConfig.HandshakeTimeout,
			func(accountingCtx context.Context) error {
				return h.service.Usage.EnsureRealtimeUsageReconciled(accountingCtx, target.APIKey.ID)
			},
			func(dialCtx context.Context) (*websocket.Conn, *orchestrator.RealtimeConversation, [][]byte, error) {
				return h.service.Connect(dialCtx, target, connectionConfig)
			},
		)
		setupTimings.AccountingMs = setupDurations.AccountingMs
		setupTimings.DialMs = setupDurations.DialMs
		if setup.AccountingErr != nil && !errors.Is(setup.AccountingErr, context.Canceled) {
			h.logSetupTimings(ctx, target, setupTimings)
			release()
			JSONError(c, http.StatusServiceUnavailable, errors.New("Realtime accounting storage is unavailable"))
			return
		}
		if setup.DialErr != nil {
			_, errorCode, message := realtimeHandshakeError(setup.DialErr)
			h.logSetupTimings(ctx, target, setupTimings)
			metrics.Metrics.RealtimeEvent(ctx, "upstream_handshake_failed")
			if errorCode == "upstream_handshake_timeout" {
				metrics.Metrics.RealtimeEvent(ctx, errorCode)
			}
			upstreamStatus := 0
			if handshake, ok := errors.AsType[*rt.HandshakeError](setup.DialErr); ok {
				upstreamStatus = handshake.Status
			}
			if upstreamStatus > 0 {
				evaluateRealtimeAPIKeyFailure(ctx, h.service.Channels, target, upstreamStatus, message)
			}
			if !errors.Is(setup.DialErr, context.Canceled) {
				h.service.RecordRealtimeFailure(ctx, target)
			}
			release()
			h.recordConnectionFailure(ctx, target, errorCode, message, attemptStarted)
			log.Warn(ctx, "realtime upstream establishment failed",
				log.String("error_code", errorCode), log.Int("channel_id", target.Channel.ID),
				log.Int("upstream_status", upstreamStatus), log.Int64("elapsed_ms", setupTimings.DialMs))
			writeRealtimeHandshakeError(c, setup.DialErr)
			return
		}
		if setup.AccountingErr != nil {
			h.logSetupTimings(ctx, target, setupTimings)
			release()
			JSONError(c, http.StatusServiceUnavailable, errors.New("Realtime accounting storage is unavailable"))
			return
		}
		upstream, conversation, initial := setup.Upstream, setup.Conversation, setup.Initial
		defer conversation.PersistSessionMetadata()
		defer conversation.Close()
		// Connection admission ends after both sockets/workers stop, before audit
		// finalization, which may wait on storage. Keep the outer release for dial failures.
		defer func() { _ = upstream.Close(); release() }()
		controller := http.NewResponseController(c.Writer)
		_ = controller.SetReadDeadline(time.Time{})
		_ = controller.SetWriteDeadline(time.Time{})
		upgradeStarted := time.Now()
		downstream, err := (&websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, HandshakeTimeout: config.HandshakeTimeout}).Upgrade(c.Writer, c.Request, nil)
		setupTimings.UpgradeMs = time.Since(upgradeStarted).Milliseconds()
		if err != nil {
			h.logSetupTimings(ctx, target, setupTimings)
			return
		}
		defer downstream.Close()
		_ = downstream.SetWriteDeadline(time.Now().Add(5 * time.Second))
		for _, event := range initial {
			if event != nil {
				if err := downstream.WriteMessage(websocket.TextMessage, event); err != nil {
					h.logSetupTimings(ctx, target, setupTimings)
					return
				}
			}
		}
		h.logSetupTimings(ctx, target, setupTimings)
		metrics.Metrics.RealtimeEvent(ctx, "connection_established")
		metrics.Metrics.RealtimeActive(ctx, 1)
		defer metrics.Metrics.RealtimeActive(context.WithoutCancel(ctx), -1)
		started := time.Now()
		err = rt.Run(ctx, downstream, upstream, rt.Options{IdleTimeout: config.IdleTimeout, MaxDuration: config.MaxDuration, ClientEventsPerSecond: target.ClientEventsPerSecond}, rt.Hooks{BindContext: conversation.WithContext, Client: conversation.Client, Server: conversation.Server, ServerFollowup: conversation.ServerFollowup, UpstreamWritten: conversation.UpstreamWritten, ServerDone: conversation.ServerDone, Check: conversation.Check})
		conversation.RecordSessionEnd(err)
		// Translation bills at session end. Persist its usage before admitting the
		// next session on this lease so it sees the previous session's cost.
		if target.Translation {
			conversation.Close()
		}
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

func (h *RealtimeHandlers) recordConnectionFailure(ctx context.Context, target *orchestrator.RealtimeTarget, code, message string, started time.Time) {
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	metadata := map[string]any{
		"model": target.ClientModel,
		"realtime": map[string]any{
			"interrupted": false, "close_code": nil, "close_reason": code, "turns": 0,
			"audio_in_bytes": int64(0), "audio_out_bytes": int64(0),
			"first_audio_latency_ms": nil,
		},
		"channel_id":      target.Channel.ID,
		"upstream_model":  target.Model,
		"upstream_format": target.APIFormat,
		"native_qwen":     target.Native,
	}
	if err := h.service.Requests.RecordRealtimeConnectionFailure(
		auditCtx, target.ClientModel, target.Channel.ID, target.SessionID,
		metadata, code, message, time.Since(started).Milliseconds(), target.UserAgent,
	); err != nil {
		log.Warn(auditCtx, "realtime connection failure audit unavailable", log.String("error_code", code))
	}
}

type realtimeAPIKeyFailureEvaluator interface {
	EvaluateAPIKeyRulesForFailure(context.Context, int, string, int, string) bool
}

func evaluateRealtimeAPIKeyFailure(ctx context.Context, channels realtimeAPIKeyFailureEvaluator, target *orchestrator.RealtimeTarget, status int, message string) {
	if target == nil || target.Channel == nil || channels == nil || status == 0 {
		return
	}
	key, ok := contexts.GetChannelAPIKey(ctx)
	if !ok || key == "" {
		return
	}
	channels.EvaluateAPIKeyRulesForFailure(ctx, target.Channel.ID, key, status, message)
}

func realtimeClientLabel(r *http.Request) string {
	label := strings.TrimSpace(r.Header.Get("X-Client-Name") + " " + r.Header.Get("X-Client-Version"))
	if label == "" {
		label = r.UserAgent()
	}
	const maxRunes = 256
	runes := []rune(label)
	if len(runes) > maxRunes {
		label = string(runes[:maxRunes])
	}
	return label
}
