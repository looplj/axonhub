// 职责: 在建立实时连接前完成模型授权、渠道选择、配额与价格快照并逐响应记账。
// 关联文件:
//
//	internal/server/api/realtime.go
//	internal/server/biz/realtime_billing.go
//	internal/server/orchestrator/candidates.go
//	llm/realtime/adapter.go
//
// 注意事项:
//
//	会话固定模型、渠道与价格；不记录音视频或对话内容，不进入一次性请求 Transformer。

// Package orchestrator coordinates provider selection and request execution.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/metrics"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
	"github.com/looplj/axonhub/internal/server/biz"
	rt "github.com/looplj/axonhub/internal/server/realtime"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	protocol "github.com/looplj/axonhub/llm/realtime"
	openairt "github.com/looplj/axonhub/llm/transformer/openai/realtime"
)

type RealtimeService struct {
	Transport           rt.Dialer
	Auth                *biz.AuthService
	Selector            CandidateSelector
	Channels            *biz.ChannelService
	Quota               *biz.QuotaService
	Usage               *biz.UsageLogService
	Requests            *biz.RequestService
	System              *biz.SystemService
	LoadBalancers       map[string]*LoadBalancer
	QuotaStatusProvider ProviderQuotaStatusProvider
	modelCircuitBreaker *biz.ModelCircuitBreaker
}

type RealtimeServiceParams struct {
	fx.In

	Transport             rt.Dialer
	Auth                  *biz.AuthService
	Selector              *DefaultSelector
	Channels              *biz.ChannelService
	Quota                 *biz.QuotaService
	Usage                 *biz.UsageLogService
	Requests              *biz.RequestService
	System                *biz.SystemService          `optional:"true"`
	ChannelLimiterManager *ChannelLimiterManager      `optional:"true"`
	QuotaStatusProvider   ProviderQuotaStatusProvider `optional:"true"`
}

type realtimeAdapter interface {
	Client([]byte) ([]byte, error)
	Server([]byte) ([]byte, error)
}

type RealtimeTarget struct {
	SessionID               string
	UserAgent               string
	HandshakeTimeout        time.Duration
	ClientEventsPerSecond   int
	APIFormat               llm.APIFormat
	SafetyIdentifier        string
	Channel                 *biz.Channel
	Model, ClientModel, URL string
	RoutingModel            string
	Price                   *biz.RealtimePriceSnapshot
	APIKey                  *ent.APIKey
	Native                  bool
	Translation             bool
	CircuitBreakerEnabled   bool
	CircuitBreakerProbe     bool
}

type realtimeCandidateRoute struct {
	entry    biz.ChannelModelEntry
	format   string
	url      string
	price    *biz.RealtimePriceSnapshot
	endpoint objects.ChannelEndpoint
}

// Prepare deliberately completes authorization and model mapping before dialing or upgrading.
func (s *RealtimeService) Prepare(ctx context.Context, model string, native bool) (*RealtimeTarget, error) {
	return s.prepare(ctx, model, native, false)
}

func (s *RealtimeService) PrepareTranslation(ctx context.Context, model string) (*RealtimeTarget, error) {
	return s.prepare(ctx, model, false, true)
}

func (s *RealtimeService) prepare(ctx context.Context, model string, native, translation bool) (*RealtimeTarget, error) {
	key, ok := contexts.GetAPIKey(ctx)
	if !ok || key == nil {
		return nil, errors.New("realtime requires an authenticated API key")
	}
	if model == "" || len(model) > 256 {
		return nil, biz.ErrInvalidModel
	}
	if err := authz.RequireScope(ctx, scopes.ScopeWriteRequests); err != nil {
		return nil, err
	}
	if err := authz.RequireScope(ctx, scopes.ScopeReadChannels); err != nil {
		return nil, err
	}
	profile := key.GetActiveProfile()
	if profile != nil && len(profile.ModelIDs) > 0 && !slices.Contains(profile.ModelIDs, model) {
		return nil, biz.ErrInvalidModel
	}
	if err := s.Usage.EnsureRealtimeUsageReconciled(ctx, key.ID); err != nil {
		return nil, err
	}
	if err := s.checkQuota(ctx, key); err != nil {
		return nil, err
	}
	mapped := NewModelMapper().MapModel(ctx, key, model)
	routes := make(map[*ChannelModelsCandidate][]realtimeCandidateRoute)
	eligible := func(candidate *ChannelModelsCandidate) bool {
		candidateRoutes := s.realtimeCandidateRoutes(ctx, candidate, native, translation)
		routes[candidate] = candidateRoutes
		return len(candidateRoutes) > 0
	}
	candidates, routingPolicy, err := s.selectCandidates(ctx, key, mapped, eligible)
	if err != nil {
		return nil, err
	}
	probeBlocked := false
	for _, candidate := range candidates {
		for _, route := range routes[candidate] {
			probe := false
			circuitBreakerEnabled := routingPolicy.LoadBalancerStrategy == biz.LoadBalancerStrategyCircuitBreaker && s.modelCircuitBreaker != nil
			if circuitBreakerEnabled {
				stats := s.modelCircuitBreaker.GetModelCircuitBreakerStats(ctx, candidate.Channel.ID, mapped)
				if stats.State == biz.StateOpen {
					if !s.modelCircuitBreaker.TryBeginProbe(ctx, candidate.Channel.ID, mapped) {
						probeBlocked = true
						continue
					}
					probe = true
				}
			}
			if loadBalancer := s.LoadBalancers[routingPolicy.LoadBalancerStrategy]; loadBalancer != nil {
				loadBalancer.TrackSelection(candidate)
			}
			clientEventsPerSecond := 0
			if candidate.Channel.Settings != nil && candidate.Channel.Settings.Realtime != nil && candidate.Channel.Settings.Realtime.ClientEventsPerSecond != nil {
				clientEventsPerSecond = int(*candidate.Channel.Settings.Realtime.ClientEventsPerSecond)
			}
			identifier := sha256.Sum256([]byte("axonhub-realtime-v1:" + key.Key + ":" + strconv.Itoa(key.UserID) + ":" + strconv.Itoa(key.ProjectID)))
			return &RealtimeTarget{
				HandshakeTimeout:      realtimeHandshakeTimeout(route.endpoint),
				ClientEventsPerSecond: clientEventsPerSecond,
				Channel:               candidate.Channel,
				Model:                 route.entry.ActualModel,
				ClientModel:           model,
				RoutingModel:          mapped,
				URL:                   route.url,
				Price:                 route.price,
				APIKey:                key,
				Native:                native,
				Translation:           translation,
				APIFormat:             llm.APIFormat(route.format),
				SafetyIdentifier:      hex.EncodeToString(identifier[:]),
				CircuitBreakerEnabled: circuitBreakerEnabled,
				CircuitBreakerProbe:   probe,
			}, nil
		}
	}
	if probeBlocked {
		return nil, ErrRealtimeCircuitBreakerOpen
	}
	return nil, biz.ErrInvalidModel
}

var ErrRealtimeCircuitBreakerOpen = errors.New("all eligible realtime channels are circuit open")

func (s *RealtimeService) ReleaseRealtimeProbe(target *RealtimeTarget) {
	if s.modelCircuitBreaker == nil || target == nil || !target.CircuitBreakerProbe || target.Channel == nil {
		return
	}
	s.modelCircuitBreaker.EndProbe(target.Channel.ID, target.RoutingModel)
	target.CircuitBreakerProbe = false
}

func (s *RealtimeService) RecordRealtimeSuccess(ctx context.Context, target *RealtimeTarget) {
	s.RecordRealtimeSuccessAt(ctx, target, time.Now(), nil, 0)
}

func (s *RealtimeService) RecordRealtimeSuccessAt(ctx context.Context, target *RealtimeTarget, started time.Time, firstOutput *time.Time, completionTokens int64) {
	if target == nil || target.Channel == nil {
		return
	}
	if s.Channels != nil {
		perf := newRealtimePerformanceRecord(target, started, firstOutput, completionTokens)
		perf.MarkSuccess()
		if perf.FirstTokenTime == nil {
			end := perf.EndTime
			perf.FirstTokenTime = &end
		}
		s.Channels.RecordRealtimePerformance(ctx, perf)
	}
	if s.modelCircuitBreaker == nil || !target.CircuitBreakerEnabled {
		return
	}
	s.ReleaseRealtimeProbe(target)
	s.modelCircuitBreaker.RecordSuccess(ctx, target.Channel.ID, target.RoutingModel)
}

func (s *RealtimeService) RecordRealtimeFailure(ctx context.Context, target *RealtimeTarget) {
	s.RecordRealtimeFailureAt(ctx, target, time.Now())
}

func (s *RealtimeService) RecordRealtimeFailureAt(ctx context.Context, target *RealtimeTarget, started time.Time) {
	if target == nil || target.Channel == nil {
		return
	}
	if s.Channels != nil {
		perf := newRealtimePerformanceRecord(target, started, nil, 0)
		perf.MarkFailed(0)
		s.Channels.RecordRealtimePerformance(ctx, perf)
	}
	if s.modelCircuitBreaker == nil || !target.CircuitBreakerEnabled {
		return
	}
	wasProbe := target.CircuitBreakerProbe
	s.ReleaseRealtimeProbe(target)
	s.modelCircuitBreaker.RecordError(ctx, target.Channel.ID, target.RoutingModel, wasProbe)
}

func newRealtimePerformanceRecord(target *RealtimeTarget, started time.Time, firstOutput *time.Time, completionTokens int64) *biz.PerformanceRecord {
	if started.IsZero() {
		started = time.Now()
	}
	perf := &biz.PerformanceRecord{
		ChannelID: target.Channel.ID,
		StartTime: started,
		Stream:    true,
	}
	if firstOutput != nil && !firstOutput.IsZero() {
		ts := *firstOutput
		if ts.Before(started) {
			ts = started
		}
		perf.FirstTokenTime = &ts
	}
	if completionTokens > 0 {
		perf.CompletionTokens = completionTokens
	}
	return perf
}

func (s *RealtimeService) realtimeCandidateRoutes(ctx context.Context, candidate *ChannelModelsCandidate, native, translation bool) []realtimeCandidateRoute {
	if candidate == nil || candidate.Channel == nil {
		return nil
	}
	var routes []realtimeCandidateRoute
	for i, entry := range candidate.Models {
		format := candidate.APIFormat
		if i < len(candidate.modelAPIFormats) {
			format = candidate.modelAPIFormats[i]
		}
		if native && format != llm.APIFormatQwenRealtimeWebSocket.String() {
			continue
		}
		var endpointURL func(string, string, string) (string, error)
		switch {
		case translation:
			if format != llm.APIFormatOpenAIRealtimeWebSocket.String() || candidate.Channel.Type != channel.TypeOpenai || entry.ActualModel != openairt.TranslationModel {
				continue
			}
			endpointURL = openairt.TranslationEndpointURL
		case format == llm.APIFormatOpenAIRealtimeWebSocket.String() && candidate.Channel.Type == channel.TypeOpenai:
			if _, err := openairt.NewAdapter(entry.ActualModel); err != nil {
				continue
			}
			endpointURL = openairt.EndpointURL
		case format == llm.APIFormatQwenRealtimeWebSocket.String() && candidate.Channel.Type == channel.TypeBailian:
			if _, err := protocol.NewAdapter(entry.ActualModel, native); err != nil {
				continue
			}
			endpointURL = protocol.EndpointURL
		default:
			continue
		}
		for _, endpoint := range candidate.Channel.ResolveEndpoints() {
			if endpoint.APIFormat != format {
				continue
			}
			url, err := endpointURL(endpoint.BaseURL, endpoint.Path, entry.ActualModel)
			if err != nil {
				log.Warn(ctx, "realtime candidate configuration rejected", log.Int("channel_id", candidate.Channel.ID))
				continue
			}
			quotePrice := s.Usage.RealtimePrice
			if translation {
				quotePrice = s.Usage.RealtimeTranslationPrice
			}
			price, err := quotePrice(ctx, candidate.Channel.ID, entry.ActualModel)
			if err != nil {
				log.Warn(ctx, "realtime candidate configuration rejected", log.Int("channel_id", candidate.Channel.ID))
				continue
			}
			if price != nil && !translation && format == llm.APIFormatOpenAIRealtimeWebSocket.String() {
				if err := objects.ValidateOpenAIRealtimePrice(price.Price); err != nil {
					log.Warn(ctx, "realtime candidate configuration rejected", log.Int("channel_id", candidate.Channel.ID))
					continue
				}
				price.RequireOpenAIImageCacheRates = true
			}
			routes = append(routes, realtimeCandidateRoute{entry: entry, format: format, url: url, price: price, endpoint: endpoint})
		}
	}
	return routes
}

func realtimeHandshakeTimeout(endpoint objects.ChannelEndpoint) time.Duration {
	if endpoint.HandshakeTimeoutMs == nil {
		return 0
	}
	return time.Duration(*endpoint.HandshakeTimeoutMs) * time.Millisecond
}

type realtimeCandidateEligibilitySelector struct {
	wrapped  CandidateSelector
	eligible func(*ChannelModelsCandidate) bool
}

func (s realtimeCandidateEligibilitySelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}
	filtered := make([]*ChannelModelsCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if s.eligible(candidate) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func (s *RealtimeService) selectCandidates(ctx context.Context, key *ent.APIKey, model string, eligible func(*ChannelModelsCandidate) bool) ([]*ChannelModelsCandidate, EffectiveRoutingPolicy, error) {
	selector := s.Selector
	profile := key.GetActiveProfile()
	if project := key.Edges.Project; project != nil {
		if p := project.GetActiveProfile(); p != nil {
			if len(p.ChannelIDs) > 0 {
				selector = WithSelectedChannelsSelector(selector, p.ChannelIDs)
			}
			if len(p.ChannelTags) > 0 {
				selector = WithChannelTagsFilterSelector(selector, p.ChannelTags, p.ChannelTagsMatchMode)
			}
		}
	}
	if profile != nil {
		if len(profile.ChannelIDs) > 0 {
			selector = WithSelectedChannelsSelector(selector, profile.ChannelIDs)
		}
		if len(profile.ChannelTags) > 0 {
			selector = WithChannelTagsFilterSelector(selector, profile.ChannelTags, profile.ChannelTagsMatchMode)
		}
	}
	selector = WithStreamPolicySelector(selector)
	if eligible != nil {
		selector = realtimeCandidateEligibilitySelector{wrapped: selector, eligible: eligible}
	}
	if s.System != nil && len(s.LoadBalancers) > 0 {
		retryPolicy := s.System.RetryPolicyOrDefault(ctx)
		routingPolicy := EffectiveRoutingPolicy{}
		loadBalanced := WithRoutingPolicyLoadBalancedSelector(
			selector,
			s.LoadBalancers,
			retryPolicy,
			s.Requests,
			key,
			&routingPolicy,
			NewQuotaRoutingGate(s.QuotaStatusProvider, s.System.QuotaRoutingSettingsOrDefault(ctx)),
		)
		candidates, err := loadBalanced.SelectAll(ctx, &llm.Request{Model: model, RequestType: llm.RequestTypeRealtime, APIFormat: llm.APIFormatOpenAIRealtime, Stream: lo.ToPtr(true)})
		return candidates, routingPolicy, err
	}

	candidates, err := selector.Select(ctx, &llm.Request{Model: model, RequestType: llm.RequestTypeRealtime, APIFormat: llm.APIFormatOpenAIRealtime, Stream: lo.ToPtr(true)})
	return candidates, EffectiveRoutingPolicy{}, err
}

func (s *RealtimeService) checkQuota(ctx context.Context, key *ent.APIKey, includeRequests ...bool) error {
	p := key.GetActiveProfile()
	if p == nil || p.Quota == nil {
		return nil
	}
	quota := *p.Quota
	if len(includeRequests) > 0 && !includeRequests[0] {
		quota.Requests = nil
		if quota.TotalTokens == nil && quota.Cost == nil {
			return nil
		}
	}
	result, err := s.Quota.CheckAPIKeyQuota(ctx, key.ID, &quota)
	if err != nil {
		return errors.New("realtime quota check unavailable")
	}
	if !result.Allowed {
		return rt.ErrQuotaExceeded
	}
	return nil
}

// Connect configures the PCM contract before a downstream WebSocket is accepted.
func (s *RealtimeService) Connect(ctx context.Context, target *RealtimeTarget, config rt.Config) (*websocket.Conn, *RealtimeConversation, [][]byte, error) {
	key, err := target.Channel.RealtimeAPIKey(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	var proxy *httpclient.ProxyConfig
	if target.Channel.Settings != nil {
		proxy = target.Channel.Settings.Proxy
	}
	safety := ""
	if target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		safety = target.SafetyIdentifier
	}
	upstream, err := s.Transport.Connect(ctx, target.URL, key, proxy, config.HandshakeTimeout, safety)
	if err != nil {
		return nil, nil, nil, err
	}
	transferred := false
	defer func() {
		if !transferred {
			_ = upstream.CloseNow()
		}
	}()
	conversation, initial, err := s.configure(ctx, upstream, target, config)
	if err != nil {
		return nil, nil, nil, err
	}
	conversation.credential = key
	transferred = true
	return upstream, conversation, initial, nil
}

func (s *RealtimeService) configure(ctx context.Context, upstream *websocket.Conn, target *RealtimeTarget, config rt.Config) (*RealtimeConversation, [][]byte, error) {
	var adapter realtimeAdapter
	var err error
	if target.Translation {
		adapter = &openairt.Translation{}
	} else if target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		var native *openairt.Adapter
		native, err = openairt.NewAdapter(target.Model)
		if err == nil {
			native.ClientModel = target.ClientModel
			adapter = native
		}
	} else {
		adapter, err = protocol.NewAdapter(target.Model, target.Native)
	}
	if err != nil {
		return nil, nil, err
	}
	state := rt.NewState()
	_ = state.Connecting()
	c := &RealtimeConversation{service: s, target: target, adapter: adapter, state: state, ctx: ctx, started: time.Now(), responses: map[string]*realtimeResponse{}}
	upstream.SetReadLimit(rt.MaxMessageBytes)
	configCtx, cancelConfig := context.WithTimeout(ctx, config.HandshakeTimeout)
	defer cancelConfig()
	kind, created, err := upstream.Read(configCtx)
	if err != nil || kind != websocket.MessageText || gjson.GetBytes(created, "type").String() != "session.created" {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(configCtx.Err(), context.DeadlineExceeded) {
			return nil, nil, context.DeadlineExceeded
		}
		if timeout, ok := errors.AsType[net.Error](err); ok && timeout.Timeout() {
			return nil, nil, context.DeadlineExceeded
		}
		return nil, nil, errors.New("upstream did not create a realtime session")
	}
	if err := state.ObserveServer("session.created"); err != nil {
		return nil, nil, err
	}
	if target.Translation {
		first, err := adapter.Server(created)
		if err != nil {
			return nil, nil, err
		}
		c.translation = &translationAccounting{}
		state.AwaitClientConfiguration()
		return c, [][]byte{first}, nil
	}
	if target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		first, err := adapter.Server(created)
		if err != nil {
			return nil, nil, err
		}
		if gjson.GetBytes(created, "session.type").String() == "realtime" && gjson.GetBytes(created, "session.audio.input.format.type").String() != "" && gjson.GetBytes(created, "session.audio.output.format.type").String() != "" {
			state.ConfirmDefaultConfiguration()
		} else {
			state.AwaitClientConfiguration()
		}
		return c, [][]byte{first}, nil
	}
	event := []byte(`{"type":"session.update","session":{"type":"realtime","output_modalities":["audio"],"audio":{"input":{"format":{"type":"audio/pcm","rate":24000},"turn_detection":null},"output":{"format":{"type":"audio/pcm","rate":24000}}}}}`)
	if target.Native {
		event, _ = json.Marshal(map[string]any{"type": "session.update", "session": map[string]any{"modalities": []string{"text", "audio"}, "turn_detection": nil, "audio": map[string]any{"input": map[string]any{"format": map[string]any{"type": "pcm", "sample_rate": protocol.NativeInputRate(target.Model)}}, "output": map[string]any{"format": map[string]any{"type": "pcm", "sample_rate": 24000}}}}})
	}
	wire, err := adapter.Client(event)
	if err != nil {
		return nil, nil, err
	}
	if err := upstream.Write(configCtx, websocket.MessageText, wire); err != nil {
		return nil, nil, errors.New("upstream configuration write failed")
	}
	_, updated, err := upstream.Read(configCtx)
	if err != nil || gjson.GetBytes(updated, "type").String() != "session.updated" {
		return nil, nil, errors.New("upstream realtime configuration was not acknowledged")
	}
	if err := state.ObserveServer("session.updated"); err != nil {
		return nil, nil, err
	}
	first, err := adapter.Server(created)
	if err != nil {
		return nil, nil, err
	}
	second, err := adapter.Server(updated)
	if err != nil {
		return nil, nil, err
	}
	// Only gateway-facing model aliases are exposed to GA clients.
	first = c.aliasModel(first)
	second = c.aliasModel(second)
	var initialEvent map[string]json.RawMessage
	if err := json.Unmarshal(second, &initialEvent); err != nil {
		return nil, nil, err
	}
	initialEvent["type"] = json.RawMessage(`"session.created"`)
	if id := gjson.GetBytes(first, "event_id").String(); id != "" {
		initialEvent["event_id"], _ = json.Marshal(id)
	}
	confirmedCreated, err := json.Marshal(initialEvent)
	if err != nil {
		return nil, nil, err
	}
	return c, [][]byte{confirmedCreated}, nil
}

type realtimeResponse struct {
	createEventID     string
	reservationOrder  int
	request           *ent.Request
	done              bool
	auditFinalized    bool
	auditStatus       request.Status
	auditSummary      map[string]any
	started           time.Time
	generationStarted time.Time
	firstOutput       *int64
	firstOutputAt     *time.Time
	hadFunctionCall   bool
	observedUsage     *llm.Usage
}

func (r *realtimeResponse) performanceStart() time.Time {
	if !r.generationStarted.IsZero() {
		return r.generationStarted
	}
	return r.started
}

type realtimeAudioCommit struct {
	eventID          string
	reserved         bool
	reservationOrder int
}

type RealtimeConversation struct {
	translation                   *translationAccounting
	pendingConfigurationID        string
	pendingClearID                string
	preClearCreates               int
	reservationOrder              int
	clearingInput                 *realtimeResponse
	auditsCreated                 int
	pendingInput                  *realtimeResponse
	committedInputs               []*realtimeResponse
	pendingCommits                []realtimeAudioCommit
	pendingResponses              []*realtimeResponse
	service                       *RealtimeService
	target                        *RealtimeTarget
	adapter                       realtimeAdapter
	state                         *rt.State
	ctx                           context.Context
	started                       time.Time
	mu                            sync.Mutex
	responses                     map[string]*realtimeResponse
	credential                    string
	audioBytes                    int64
	audioInBytes                  atomic.Int64
	audioOutBytes                 int64
	firstAudioLatency             *int64
	turns                         int
	auditIDs                      []int
	closeStats                    *realtimeCloseStats
	circuitBreakerFailureRecorded bool
	checkMu                       sync.Mutex
	lastCheck                     time.Time
	lastCheckErr                  error
	latestKey                     *ent.APIKey
}

func (c *RealtimeConversation) Check(ctx context.Context) error {
	if c.state.ConfigurationExpired(15 * time.Second) {
		return errors.New("realtime configuration acknowledgement timed out")
	}
	if adapter, ok := c.adapter.(*openairt.Translation); ok {
		c.mu.Lock()
		expired := adapter.Expired()
		c.mu.Unlock()
		if expired {
			return errors.New("translation configuration acknowledgement timed out")
		}
	}
	currentChannel := c.service.Channels.GetEnabledChannel(c.target.Channel.ID)
	if currentChannel == nil {
		return errors.New("realtime channel disabled")
	}
	c.checkMu.Lock()
	defer c.checkMu.Unlock()
	if time.Since(c.lastCheck) < time.Second {
		return c.lastCheckErr
	}
	c.lastCheck = time.Now()
	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	defer checkCancel()
	current, err := c.service.Auth.AuthenticateAPIKey(checkCtx, c.target.APIKey.Key)
	if err != nil {
		c.lastCheckErr = errors.New("realtime API key is no longer authorized")
		return c.lastCheckErr
	}
	authorizedCtx := contexts.WithAPIKey(contexts.ForkRequest(checkCtx), current)
	if err := authz.RequireScope(authorizedCtx, scopes.ScopeWriteRequests); err != nil {
		c.lastCheckErr = errors.New("realtime request scope revoked")
		return c.lastCheckErr
	}
	if err := authz.RequireScope(authorizedCtx, scopes.ScopeReadChannels); err != nil {
		c.lastCheckErr = errors.New("realtime model scope revoked")
		return c.lastCheckErr
	}
	if p := current.GetActiveProfile(); p != nil && len(p.ModelIDs) > 0 && !slices.Contains(p.ModelIDs, c.target.ClientModel) {
		c.lastCheckErr = errors.New("realtime model permission revoked")
		return c.lastCheckErr
	}
	if !realtimeProfilesAllowChannel(current, currentChannel) {
		c.lastCheckErr = errors.New("realtime channel permission revoked")
		return c.lastCheckErr
	}
	c.latestKey = current
	if err := c.service.Usage.EnsureRealtimeUsageReconciled(checkCtx, current.ID); err != nil {
		c.lastCheckErr = err
		return err
	}
	if c.translation != nil {
		c.lastCheckErr = c.checkTranslationQuota(checkCtx, current)
	} else {
		c.lastCheckErr = c.service.checkQuota(checkCtx, current, false)
	}
	return c.lastCheckErr
}

func realtimeProfilesAllowChannel(key *ent.APIKey, ch *biz.Channel) bool {
	if p := key.GetActiveProfile(); p != nil {
		if len(p.ChannelIDs) > 0 && !slices.Contains(p.ChannelIDs, ch.ID) {
			return false
		}
		if !p.MatchChannelTags(ch.Tags) {
			return false
		}
	}
	if key.Edges.Project != nil {
		if p := key.Edges.Project.GetActiveProfile(); p != nil {
			if len(p.ChannelIDs) > 0 && !slices.Contains(p.ChannelIDs, ch.ID) {
				return false
			}
			if len(p.ChannelTags) > 0 && !objects.MatchChannelTags(p.ChannelTags, p.ChannelTagsMatchMode, ch.Tags) {
				return false
			}
		}
	}
	return true
}

func (c *RealtimeConversation) Client(data []byte) ([]byte, error) {
	if err := c.Check(c.ctx); err != nil {
		return nil, err
	}
	if c.translation != nil {
		return c.translationClient(data)
	}
	kind := gjson.GetBytes(data, "type").String()
	if kind == "response.cancel" {
		c.mu.Lock()
		active := false
		id := gjson.GetBytes(data, "response_id").String()
		for responseID, response := range c.responses {
			if !response.done && (id == "" || id == responseID) {
				active = true
				break
			}
		}
		// A create already sent upstream may start before the cancel reaches it.
		active = active || id == "" && len(c.pendingResponses) > 0
		c.mu.Unlock()
		if !active {
			return nil, rt.ErrResponseCancelNotActive
		}
	}
	if kind != "session.update" && kind != "response.cancel" && !c.state.Ready() {
		return nil, rt.ErrUpdatePending
	}
	var eventID string
	if kind == "session.update" || kind == "response.create" || kind == "input_audio_buffer.clear" || kind == "input_audio_buffer.commit" {
		eventID = gjson.GetBytes(data, "event_id").String()
		if eventID == "" {
			eventID = "evt_axonhub_" + uuid.NewString()
			var err error
			data, err = sjson.SetBytes(data, "event_id", eventID)
			if err != nil {
				return nil, errors.New("could not assign realtime event ID")
			}
		}
	}
	if kind == "session.update" || kind == "input_audio_buffer.clear" || kind == "input_audio_buffer.commit" {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasPendingCreate(eventID) || eventID == c.pendingClearID || eventID == c.pendingConfigurationID || c.pendingCommitIndex(eventID) >= 0 {
			return nil, errors.New("realtime event ID is already pending")
		}
		if kind == "input_audio_buffer.commit" && (len(c.committedInputs) >= 8 || len(c.pendingCommits) >= 8) {
			return nil, errors.New("too many outstanding realtime audio commits")
		}
		if kind == "input_audio_buffer.clear" && c.pendingClearID != "" {
			return nil, errors.New("audio buffer clear is pending")
		}
		if kind == "session.update" {
			if err := c.state.Update(); err != nil {
				return nil, err
			}
		}
	}
	wire, err := c.adapter.Client(data)
	if err != nil {
		if kind == "session.update" {
			c.state.ConfigurationRejected()
		}
		return nil, err
	}
	switch kind {
	case "session.update":
		c.pendingConfigurationID = eventID
	case "input_audio_buffer.commit":
		commit := realtimeAudioCommit{eventID: eventID, reserved: c.pendingInput != nil}
		if c.pendingInput != nil {
			c.pendingInput.reservationOrder = c.nextReservationOrder()
			commit.reservationOrder = c.pendingInput.reservationOrder
			c.committedInputs = append(c.committedInputs, c.pendingInput)
			c.pendingInput = nil
		} else {
			commit.reservationOrder = c.nextReservationOrder()
		}
		c.pendingCommits = append(c.pendingCommits, commit)
		c.audioBytes = 0
	case "input_audio_buffer.clear":
		c.pendingClearID = eventID
		c.preClearCreates = len(c.pendingResponses)
		c.clearingInput = c.pendingInput
		if c.clearingInput != nil {
			c.clearingInput.reservationOrder = c.nextReservationOrder()
		}
		c.pendingInput = nil
		c.audioBytes = 0
	}
	if kind == "response.create" || kind == "input_audio_buffer.append" {
		c.checkMu.Lock()
		quotaKey := c.latestKey
		if quotaKey == nil {
			quotaKey = c.target.APIKey
		}
		c.checkMu.Unlock()
		c.mu.Lock()
		if eventID != "" && (eventID == c.pendingConfigurationID || eventID == c.pendingClearID || c.hasPendingCreate(eventID) || c.pendingCommitIndex(eventID) >= 0) {
			c.mu.Unlock()
			return nil, errors.New("realtime event ID is already pending")
		}
		if len(c.responses)+len(c.pendingResponses) >= 512 || len(c.pendingResponses) >= 8 {
			c.mu.Unlock()
			return nil, rt.ErrAccountingUnavailable
		}
		if err := c.reserveClientInput(kind, eventID, quotaKey); err != nil {
			c.mu.Unlock()
			return nil, err
		}
		c.mu.Unlock()
	}
	if kind == "input_audio_buffer.append" {
		c.mu.Lock()
		c.audioBytes += int64(len(gjson.GetBytes(data, "audio").String()))
		over := c.audioBytes > 24<<20
		c.mu.Unlock()
		if over {
			return nil, rt.ErrBackpressure
		}
	}
	return wire, nil
}

// reserveClientInput runs with c.mu held. A manual generation consumes committed
// audio, leaving any uncommitted buffer reserved for a later turn.
func (c *RealtimeConversation) reserveClientInput(kind, eventID string, key *ent.APIKey) error {
	if kind == "input_audio_buffer.append" && c.pendingInput != nil {
		return nil
	}
	if kind == "response.create" && len(c.committedInputs) > 0 {
		reservation := c.committedInputs[0]
		c.committedInputs = slices.Delete(c.committedInputs, 0, 1)
		reservation.createEventID = eventID
		reservation.generationStarted = time.Now()
		c.pendingResponses = append(c.pendingResponses, reservation)
		return nil
	}
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()
	if err := c.service.checkQuota(ctx, key); err != nil {
		return err
	}
	reservation, err := c.createResponse("pending")
	if err != nil {
		return rt.ErrAccountingUnavailable
	}
	if kind == "response.create" {
		reservation.createEventID = eventID
		reservation.reservationOrder = c.nextReservationOrder()
		reservation.generationStarted = reservation.started
		c.pendingResponses = append(c.pendingResponses, reservation)
	} else {
		c.pendingInput = reservation
	}
	return nil
}

func (c *RealtimeConversation) Server(data []byte) ([]byte, error) {
	kind := gjson.GetBytes(data, "type").String()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.translation != nil {
		converted, err := c.adapter.Server(data)
		if err != nil {
			return nil, err
		}
		c.observeAudioOutput(kind, converted)
		if kind == "session.closed" {
			c.translation.closed = true
		}
		return applyRealtimeErrorPolicy(converted)
	}
	if err := c.prepareServerEvent(kind, data); err != nil {
		if errors.Is(err, errDuplicateResponseDone) {
			return nil, nil
		}
		return nil, err
	}
	id := gjson.GetBytes(data, "response.id").String()
	if kind == "response.created" {
		if err := c.admitResponse(id); err != nil {
			return nil, err
		}
		c.turns++
	}
	c.observeResponseOutput(kind, data)
	if kind == "response.done" {
		if err := c.finishResponse(data, id); err != nil {
			return nil, err
		}
	}
	converted, err := c.adapter.Server(data)
	if err != nil {
		return nil, err
	}
	c.observeAudioOutput(gjson.GetBytes(converted, "type").String(), converted)
	if c.target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		converted, err = applyRealtimeErrorPolicy(converted)
		if err != nil {
			return nil, err
		}
	}
	return c.aliasModel(converted), nil
}

func (c *RealtimeConversation) prepareServerEvent(kind string, data []byte) error {
	if kind == "error" {
		if err := c.rejectPendingEvent(gjson.GetBytes(data, "error.event_id").String()); err != nil {
			return err
		}
	}
	if kind == "session.updated" {
		c.pendingConfigurationID = ""
	}
	if kind == "response.done" {
		if previous := c.responses[gjson.GetBytes(data, "response.id").String()]; previous != nil && previous.done {
			return errDuplicateResponseDone
		}
	}
	responseQueued := c.clearingInput != nil || c.preClearCreates > 0 || len(c.committedInputs) > 0 ||
		(c.pendingClearID == "" && (len(c.pendingResponses) > 0 || c.pendingInput != nil))
	if err := c.state.ObserveServer(kind, responseQueued); err != nil {
		return err
	}
	if kind == "input_audio_buffer.cleared" {
		return c.confirmAudioClear()
	}
	if kind == "input_audio_buffer.committed" {
		c.confirmAudioCommit()
	}
	return nil
}

func (c *RealtimeConversation) confirmAudioCommit() {
	if c.pendingClearID != "" && c.clearingInput != nil {
		preClearCommit := slices.IndexFunc(c.pendingCommits, func(commit realtimeAudioCommit) bool {
			return commit.reservationOrder < c.clearingInput.reservationOrder
		})
		if preClearCommit >= 0 {
			c.pendingCommits = slices.Delete(c.pendingCommits, preClearCommit, preClearCommit+1)
			return
		}
		index := slices.IndexFunc(c.committedInputs, func(input *realtimeResponse) bool {
			return input.reservationOrder > c.clearingInput.reservationOrder
		})
		if index < 0 {
			index = len(c.committedInputs)
		}
		c.committedInputs = slices.Insert(c.committedInputs, index, c.clearingInput)
		c.clearingInput = nil
		return
	}
	if len(c.pendingCommits) > 0 {
		// A delayed manual acknowledgement does not clear post-commit bytes.
		c.pendingCommits = slices.Delete(c.pendingCommits, 0, 1)
		return
	}
	if c.pendingInput != nil {
		c.pendingInput.reservationOrder = c.nextReservationOrder()
		c.committedInputs = append(c.committedInputs, c.pendingInput)
		c.pendingInput = nil
		c.audioBytes = 0
	}
}

func realtimePCMBytes(audio string) int64 {
	return int64(base64.StdEncoding.DecodedLen(len(audio)) - (len(audio) - len(strings.TrimRight(audio, "="))))
}

func (c *RealtimeConversation) observeAudioOutput(kind string, data []byte) {
	switch kind {
	case "response.audio.delta", "response.output_audio.delta", "session.output_audio.delta":
		if count := realtimePCMBytes(gjson.GetBytes(data, "delta").String()); count > 0 {
			c.audioOutBytes += count
			if c.firstAudioLatency == nil {
				c.firstAudioLatency = lo.ToPtr(time.Since(c.started).Milliseconds())
			}
		}
	}
}

func (c *RealtimeConversation) observeResponseOutput(kind string, data []byte) {
	if kind == "response.audio.delta" || kind == "response.text.delta" || kind == "response.output_audio.delta" || kind == "response.output_text.delta" || kind == "response.function_call_arguments.delta" {
		responseID := gjson.GetBytes(data, "response_id").String()
		if response := c.responses[responseID]; response != nil && response.firstOutput == nil {
			now := time.Now()
			response.firstOutput = lo.ToPtr(now.Sub(response.started).Milliseconds())
			response.firstOutputAt = &now
		}
	}
	if !isFunctionCallEvent(kind, data) {
		return
	}
	if response := c.responses[gjson.GetBytes(data, "response_id").String()]; response != nil {
		response.hadFunctionCall = true
	}
}

func isFunctionCallEvent(kind string, data []byte) bool {
	switch kind {
	case "response.function_call_arguments.delta", "response.function_call_arguments.done":
		return true
	case "response.output_item.added", "response.output_item.done":
		return gjson.GetBytes(data, "item.type").String() == "function_call"
	default:
		return false
	}
}

func (c *RealtimeConversation) finishResponse(data []byte, id string) error {
	parser := protocol.ParseUsage
	if c.target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		parser = openairt.ParseUsage
	}
	responseID, usage, err := parser(data)
	response := c.responses[id]
	if response == nil {
		return errors.New("response.done without response.created")
	}
	if response.done {
		return nil
	}
	if err != nil || usage == nil || responseID != id {
		if realtimeResponseStatus(data) == request.StatusCanceled && gjson.GetBytes(data, "response.usage").Type == gjson.Null {
			// A cancellation can be terminal without a provider usage statement.
			// Keep it auditable, without inventing a charge or ending the session.
			return c.finishCanceledResponseWithoutUsage(response, id)
		}
		return errors.New("realtime response usage missing or invalid")
	}
	response.observedUsage = usage
	for _, item := range gjson.GetBytes(data, "response.output").Array() {
		if item.Get("type").String() == "function_call" {
			response.hadFunctionCall = true
		}
	}
	if c.target.Price != nil {
		if err := applyRealtimeBillingPolicy(c.target, usage, response.hadFunctionCall); err != nil {
			return err
		}
	}
	// Persist confirmed usage even when the downstream has gone away.
	billingCtx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 10*time.Second)
	defer cancel()
	projectID, _ := contexts.GetProjectID(c.ctx)
	_, err = c.service.Usage.CreateRealtimeUsage(billingCtx, biz.CreateUsageLogParams{RequestID: response.request.ID, ProjectID: projectID, ChannelID: c.target.Channel.ID, ActualModelID: c.target.Model, Usage: usage, Source: usagelog.SourceAPI, Format: string(llm.APIFormatOpenAIRealtime), APIKeyID: lo.ToPtr(c.target.APIKey.ID)}, c.target.Price)
	if err != nil {
		return errors.New("realtime usage persistence failed")
	}
	response.done = true
	metrics.Metrics.RealtimeUsage(billingCtx, usage, response.firstOutput)
	status := realtimeResponseStatus(data)
	if status == request.StatusCompleted && c.credential != "" {
		c.service.Channels.RecordRealtimeCredentialSuccess(c.target.Channel.ID, c.credential)
	}
	summary := c.withPriceAudit(map[string]any{"response_id": id, "usage": usage, "realtime": true, "usage_confirmed": true, "upstream_model": c.target.Model, "upstream_format": c.target.APIFormat})
	if status == request.StatusFailed {
		summary["error_code"] = "upstream_rejected"
		summary["error"] = "The upstream Realtime response failed."
	}
	response.auditStatus = status
	response.auditSummary = summary
	err = c.service.Requests.UpdateRealtimeAudit(billingCtx, response.request, status, id, summary, &biz.LatencyMetrics{LatencyMs: lo.ToPtr(time.Since(response.started).Milliseconds()), FirstTokenLatencyMs: response.firstOutput}, nil)
	if err != nil {
		return errors.New("realtime audit update failed")
	}
	response.auditFinalized = true
	switch status {
	case request.StatusCompleted:
		c.service.RecordRealtimeSuccessAt(billingCtx, c.target, response.performanceStart(), response.firstOutputAt, usage.CompletionTokens)
		c.circuitBreakerFailureRecorded = false
	case request.StatusFailed:
		c.service.RecordRealtimeFailureAt(billingCtx, c.target, response.performanceStart())
		c.circuitBreakerFailureRecorded = true
	}
	return nil
}

func (c *RealtimeConversation) finishCanceledResponseWithoutUsage(response *realtimeResponse, id string) error {
	response.done = true
	response.auditStatus = request.StatusCanceled
	response.auditSummary = c.withPriceAudit(map[string]any{
		"response_id": id, "realtime": true, "usage_confirmed": false,
		"upstream_model": c.target.Model, "upstream_format": c.target.APIFormat,
	})
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 5*time.Second)
	defer cancel()
	latency := &biz.LatencyMetrics{LatencyMs: lo.ToPtr(time.Since(response.started).Milliseconds()), FirstTokenLatencyMs: response.firstOutput}
	if err := c.service.Requests.UpdateRealtimeAudit(ctx, response.request, response.auditStatus, id, response.auditSummary, latency, nil); err != nil {
		return errors.New("realtime audit update failed")
	}
	response.auditFinalized = true
	return nil
}

func realtimeResponseStatus(data []byte) request.Status {
	switch gjson.GetBytes(data, "response.status").String() {
	case "cancelled", "canceled":
		return request.StatusCanceled
	case "failed", "incomplete":
		return request.StatusFailed
	default:
		return request.StatusCompleted
	}
}

var errDuplicateResponseDone = errors.New("duplicate response.done")

// ServerFollowup completes the GA lifecycle for a fully formed Qwen item.
func (c *RealtimeConversation) ServerFollowup(data []byte) ([]byte, error) {
	if c.target.Native || c.target.APIFormat != llm.APIFormatQwenRealtimeWebSocket || gjson.GetBytes(data, "type").String() != "conversation.item.added" {
		return nil, nil
	}
	done, err := sjson.SetBytes(data, "type", "conversation.item.done")
	if err != nil {
		return nil, err
	}
	return sjson.SetBytes(done, "event_id", "evt_axonhub_"+uuid.NewString())
}

// confirmAudioClear finalizes the audio reserved before the clear was sent.
func (c *RealtimeConversation) confirmAudioClear() error {
	if pending := c.clearingInput; pending != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 5*time.Second)
		defer cancel()
		summary := map[string]any{"realtime": true, "input_discarded_before_response": true, "usage_confirmed": false}
		if err := c.service.Requests.UpdateRealtimeAudit(ctx, pending.request, request.StatusCanceled, "", summary, nil, nil); err != nil {
			return errors.New("realtime discarded-input audit failed")
		}
	}
	c.clearingInput = nil
	c.pendingClearID = ""
	c.preClearCreates = 0
	return nil
}

// hasPendingCreate is called with c.mu held.
func (c *RealtimeConversation) hasPendingCreate(eventID string) bool {
	return eventID != "" && slices.ContainsFunc(c.pendingResponses, func(r *realtimeResponse) bool {
		return r.createEventID == eventID
	})
}

func (c *RealtimeConversation) pendingCommitIndex(eventID string) int {
	return slices.IndexFunc(c.pendingCommits, func(commit realtimeAudioCommit) bool {
		return commit.eventID == eventID
	})
}

// rejectPendingEvent finalizes only the reservation named by the provider.
// The caller holds c.mu to serialize this with response admission.
func (c *RealtimeConversation) rejectPendingEvent(eventID string) error {
	if eventID == "" {
		return nil
	}
	if index := c.pendingCommitIndex(eventID); index >= 0 {
		if !c.pendingCommits[index].reserved {
			c.pendingCommits = slices.Delete(c.pendingCommits, index, index+1)
			return nil
		}
		// Upstream buffer boundaries are uncertain after a rejected commit.
		// Keep both reservations for Close to reconcile.
		return errors.New("upstream rejected realtime audio buffer commit")
	}
	if eventID == c.pendingClearID {
		// The adapter may already have reset its PCM buffer. Stop forwarding and
		// let Close preserve both reservations for reconciliation.
		return errors.New("upstream rejected realtime audio buffer clear")
	}
	if eventID == c.pendingConfigurationID {
		c.state.ConfigurationRejected()
		c.pendingConfigurationID = ""
		return nil
	}
	index := slices.IndexFunc(c.pendingResponses, func(r *realtimeResponse) bool {
		return r.createEventID == eventID
	})
	if index < 0 {
		return nil
	}
	pending := c.pendingResponses[index]
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 5*time.Second)
	defer cancel()
	summary := map[string]any{"realtime": true, "usage_confirmed": false, "response_create_rejected": true, "error_code": "upstream_rejected"}
	if err := c.service.Requests.UpdateRealtimeAudit(ctx, pending.request, request.StatusFailed, "", summary, nil, nil); err != nil {
		return errors.New("realtime rejected-response audit failed")
	}
	if index < c.preClearCreates {
		c.preClearCreates--
	}
	c.pendingResponses = slices.Delete(c.pendingResponses, index, index+1)
	return nil
}

// nextReservationOrder is called with c.mu held at the create, commit or clear boundary.
func (c *RealtimeConversation) nextReservationOrder() int {
	c.reservationOrder++
	return c.reservationOrder
}

// admitResponse consumes an existing reservation or checks quota before an automatic turn.
// The caller holds c.mu so each reservation is consumed exactly once.
func (c *RealtimeConversation) admitResponse(id string) error {
	if _, ok := c.responses[id]; ok {
		return errors.New("duplicate response id")
	}
	if len(c.responses) >= 512 {
		return errors.New("realtime response limit reached")
	}
	var r *realtimeResponse
	explicitReady := len(c.pendingResponses) > 0 && (c.pendingClearID == "" || c.preClearCreates > 0)
	if explicitReady && (len(c.committedInputs) == 0 || c.pendingResponses[0].reservationOrder <= c.committedInputs[0].reservationOrder) {
		if c.preClearCreates > 0 {
			c.preClearCreates--
		}
		r = c.pendingResponses[0]
		c.pendingResponses = slices.Delete(c.pendingResponses, 0, 1)
	} else if len(c.committedInputs) > 0 {
		r = c.committedInputs[0]
		c.committedInputs = slices.Delete(c.committedInputs, 0, 1)
	} else if c.clearingInput != nil {
		r = c.clearingInput
		c.clearingInput = nil
	} else if c.pendingClearID == "" && c.pendingInput != nil {
		r = c.pendingInput
		c.pendingInput = nil
	} else {
		c.checkMu.Lock()
		quotaKey := c.latestKey
		if quotaKey == nil {
			quotaKey = c.target.APIKey
		}
		c.checkMu.Unlock()
		quotaCtx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
		quotaErr := c.service.checkQuota(quotaCtx, quotaKey)
		cancel()
		if quotaErr != nil {
			return quotaErr
		}
		var err error
		r, err = c.createResponse(id)
		if err != nil {
			return err
		}
	}
	if r.generationStarted.IsZero() {
		r.generationStarted = time.Now()
	}
	c.responses[id] = r
	return nil
}

func (c *RealtimeConversation) withPriceAudit(fields map[string]any) map[string]any {
	if price := c.target.Price; price != nil {
		fields["price_reference_id"] = price.ReferenceID
		fields["price_snapshot"] = price.Price
		fields["price_quoted_at"] = price.QuotedAt
	}
	return fields
}

func (c *RealtimeConversation) createResponse(id string) (*realtimeResponse, error) {
	if c.auditsCreated >= 512 {
		return nil, errors.New("realtime audit limit reached")
	}
	if id == "" || len(id) > 256 {
		return nil, errors.New("invalid upstream response id")
	}
	metadata, _ := json.Marshal(c.withPriceAudit(map[string]any{"model": c.target.ClientModel, "realtime": true, "native_qwen": c.target.Native, "channel_id": c.target.Channel.ID, "upstream_model": c.target.Model, "upstream_format": c.target.APIFormat}))
	req, err := c.service.Requests.CreateRealtimeRequest(c.ctx, c.target.ClientModel, c.target.Channel.ID, c.target.SessionID, metadata, c.target.UserAgent)
	if err != nil {
		return nil, errors.New("realtime audit create failed")
	}
	req.ChannelID = c.target.Channel.ID
	c.auditsCreated++
	c.auditIDs = append(c.auditIDs, req.ID)
	return &realtimeResponse{request: req, started: time.Now()}, nil
}

func (c *RealtimeConversation) aliasModel(data []byte) []byte {
	if c.target.Native || c.target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket || data == nil {
		return data
	}
	var event map[string]any
	if json.Unmarshal(data, &event) != nil {
		return data
	}
	for _, field := range []string{"session", "response"} {
		if body, ok := event[field].(map[string]any); ok {
			if _, exists := body["model"]; exists {
				body["model"] = c.target.ClientModel
			}
		}
	}
	result, err := json.Marshal(event)
	if err != nil {
		return nil
	}
	return result
}

type realtimeCloseStats struct {
	DurationMs  int64
	Code        int
	Reason      string
	Interrupted bool
}

// RecordSessionEnd is called after the socket workers have joined.
func (c *RealtimeConversation) RecordSessionEnd(result error) {
	code, reason := rt.CloseStatus(result)
	c.closeStats = &realtimeCloseStats{time.Since(c.started).Milliseconds(), code, reason, rt.IsAbnormal(result)}
	if reason == "upstream_unavailable" && !c.circuitBreakerFailureRecorded {
		c.service.RecordRealtimeFailureAt(context.WithoutCancel(c.ctx), c.target, c.started)
	}
}

func (c *RealtimeConversation) Close() {
	c.state.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 10*time.Second)
	defer cancel()
	if c.translation != nil {
		c.closeTranslation(ctx)
		return
	}
	pending := append([]*realtimeResponse{}, c.pendingResponses...)
	pending = append(pending, c.committedInputs...)
	if c.clearingInput != nil {
		pending = append(pending, c.clearingInput)
	}
	if c.pendingInput != nil {
		pending = append(pending, c.pendingInput)
	}
	for _, r := range pending {
		if ctx.Err() != nil {
			break
		}
		status := c.unfinishedAuditStatus()
		if err := c.service.Requests.UpdateRealtimeAudit(ctx, r.request, status, "", c.withPriceAudit(map[string]any{"realtime": true, "usage_confirmed": false, "error": "session ended before input or generation accounting was confirmed", "error_code": c.closeAuditErrorCode(), "upstream_model": c.target.Model}), nil, nil); err != nil {
			log.Warn(context.WithoutCancel(c.ctx), "realtime pending audit finalization failed", log.Int("request_id", r.request.ID))
		}
	}
	c.pendingResponses = nil
	c.committedInputs = nil
	c.pendingCommits = nil
	c.pendingInput = nil
	c.clearingInput = nil
	c.pendingClearID = ""
	c.preClearCreates = 0
	for id, r := range c.responses {
		if r.auditFinalized {
			continue
		}
		if ctx.Err() != nil {
			log.Warn(context.WithoutCancel(c.ctx), "realtime audit drain deadline exceeded")
			break
		}
		status := c.unfinishedAuditStatus()
		summary := c.withPriceAudit(map[string]any{"response_id": id, "realtime": true, "usage_confirmed": false, "error": "session ended before confirmed response usage", "error_code": c.closeAuditErrorCode(), "upstream_model": c.target.Model, "upstream_format": c.target.APIFormat})
		if r.observedUsage != nil {
			summary["observed_usage"] = r.observedUsage
		}
		if r.done {
			status = r.auditStatus
			summary = r.auditSummary
		}
		if err := c.service.Requests.UpdateRealtimeAudit(ctx, r.request, status, id, summary, &biz.LatencyMetrics{LatencyMs: lo.ToPtr(time.Since(r.started).Milliseconds()), FirstTokenLatencyMs: r.firstOutput}, nil); err != nil {
			log.Warn(context.WithoutCancel(c.ctx), "realtime audit finalization failed", log.Int("request_id", r.request.ID))
			continue
		}
		r.auditFinalized = true
	}
}

func (c *RealtimeConversation) unfinishedAuditStatus() request.Status {
	if c.closeStats != nil && c.closeStats.Code == int(websocket.StatusNormalClosure) && c.closeAuditErrorCode() == "client_closed" {
		return request.StatusCanceled
	}
	return request.StatusFailed
}

func (c *RealtimeConversation) closeAuditErrorCode() string {
	if c.closeStats != nil {
		switch c.closeStats.Reason {
		case "internal_error", "upstream_unavailable", "backpressure":
			return c.closeStats.Reason
		}
	}
	return "client_closed"
}

// PersistSessionMetadata runs after the reservation has been released.
func (c *RealtimeConversation) PersistSessionMetadata() {
	stats := c.closeStats
	if stats == nil || len(c.auditIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 5*time.Second)
	defer cancel()
	metadata := map[string]any{
		"interrupted": stats.Interrupted, "close_code": stats.Code, "close_reason": stats.Reason, "duration_ms": stats.DurationMs,
		"turns": c.turns, "audio_in_bytes": c.audioInBytes.Load(), "audio_out_bytes": c.audioOutBytes,
		"first_audio_latency_ms": c.firstAudioLatency,
	}
	if err := c.service.Requests.UpdateRealtimeSessionMetadata(ctx, c.auditIDs, metadata, stats.DurationMs); err != nil {
		log.Warn(ctx, "realtime session metadata update failed")
	}
}

func NewRealtimeService(params RealtimeServiceParams) *RealtimeService {
	var loadBalancers map[string]*LoadBalancer
	var modelCircuitBreaker *biz.ModelCircuitBreaker
	if params.System != nil {
		modelCircuitBreaker = biz.NewModelCircuitBreaker()
		loadBalancers = newRealtimeLoadBalancers(params.Channels, params.ChannelLimiterManager, modelCircuitBreaker)
	}
	return &RealtimeService{
		Transport:           params.Transport,
		Auth:                params.Auth,
		Selector:            params.Selector,
		Channels:            params.Channels,
		Quota:               params.Quota,
		Usage:               params.Usage,
		Requests:            params.Requests,
		System:              params.System,
		LoadBalancers:       loadBalancers,
		QuotaStatusProvider: params.QuotaStatusProvider,
		modelCircuitBreaker: modelCircuitBreaker,
	}
}

func newRealtimeLoadBalancers(channels *biz.ChannelService, limiterManager *ChannelLimiterManager, modelCircuitBreaker *biz.ModelCircuitBreaker) map[string]*LoadBalancer {
	selectionTracker := NewChannelRequestTracker()
	rateLimitStrategy := NewRateLimitAwareStrategy(selectionTracker, limiterManager)

	return map[string]*LoadBalancer{
		biz.LoadBalancerStrategyAdaptive: NewLoadBalancer(channels, channels,
			NewErrorAwareStrategy(),
			NewWeightRoundRobinStrategy(),
			NewLatencyAwareStrategy(),
			rateLimitStrategy,
		),
		biz.LoadBalancerStrategyFailover: NewLoadBalancer(nil, channels,
			NewWeightStrategy(),
			NewRandomStrategy(),
			rateLimitStrategy,
		),
		biz.LoadBalancerStrategyCircuitBreaker: NewLoadBalancer(nil, channels,
			NewWeightStrategy(),
			NewModelAwareCircuitBreakerStrategy(modelCircuitBreaker),
			rateLimitStrategy,
		),
		biz.LoadBalancerStrategyRoundRobin: NewLoadBalancer(channels, channels,
			NewRoundRobinStrategy(),
			rateLimitStrategy,
		).WithoutWeightTieBreaker().WithRoundRobinHealthFilter(NewRoundRobinHealthStrategy()),
	}
}

func (c *RealtimeConversation) WithContext(ctx context.Context) { c.ctx = ctx }

// applyRealtimeBillingPolicy never guesses the split between free transcripts and billable tool arguments.
func applyRealtimeBillingPolicy(target *RealtimeTarget, usage *llm.Usage, hasFunctionCall bool) error {
	if target.APIFormat != llm.APIFormatQwenRealtimeWebSocket || !protocol.FreeAudioTranscript(target.Model) || usage.Realtime == nil || usage.Realtime.OutputAudioTokens == 0 {
		return nil
	}
	if hasFunctionCall && usage.Realtime.OutputTextTokens > 0 {
		return errors.New("mixed Qwen speech and tool text usage requires reconciliation")
	}
	usage.Realtime.UnbilledOutputTextTokens = usage.Realtime.OutputTextTokens
	return nil
}
