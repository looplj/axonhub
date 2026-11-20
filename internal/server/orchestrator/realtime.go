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
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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
	Transport rt.Dialer
	Auth      *biz.AuthService
	Selector  *DefaultSelector
	Channels  *biz.ChannelService
	Quota     *biz.QuotaService
	Usage     *biz.UsageLogService
	Requests  *biz.RequestService
}

type RealtimeServiceParams struct {
	fx.In

	Transport rt.Dialer
	Auth      *biz.AuthService
	Selector  *DefaultSelector
	Channels  *biz.ChannelService
	Quota     *biz.QuotaService
	Usage     *biz.UsageLogService
	Requests  *biz.RequestService
}

type realtimeAdapter interface {
	Client([]byte) ([]byte, error)
	Server([]byte) ([]byte, error)
}

type RealtimeTarget struct {
	SessionID               string
	HandshakeTimeout        time.Duration
	APIFormat               llm.APIFormat
	SafetyIdentifier        string
	Channel                 *biz.Channel
	Model, ClientModel, URL string
	Price                   *biz.RealtimePriceSnapshot
	APIKey                  *ent.APIKey
	Native                  bool
}

// Prepare deliberately completes authorization and model mapping before dialing or upgrading.
func (s *RealtimeService) Prepare(ctx context.Context, model string, native bool) (*RealtimeTarget, error) {
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
	var selector CandidateSelector = s.Selector
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
	candidates, err := selector.Select(ctx, &llm.Request{Model: mapped, RequestType: llm.RequestTypeRealtime, APIFormat: llm.APIFormatOpenAIRealtime, Stream: lo.ToPtr(true)})
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		ch := candidate.Channel
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
			case format == llm.APIFormatOpenAIRealtimeWebSocket.String() && ch.Type == channel.TypeOpenai:
				if _, err := openairt.NewAdapter(entry.ActualModel); err != nil {
					continue
				}
				endpointURL = openairt.EndpointURL
			case format == llm.APIFormatQwenRealtimeWebSocket.String() && ch.Type == channel.TypeBailian:
				if _, err := protocol.NewAdapter(entry.ActualModel, native); err != nil {
					continue
				}
				endpointURL = protocol.EndpointURL
			default:
				continue
			}
			for _, ep := range ch.ResolveEndpoints() {
				if ep.APIFormat != format {
					continue
				}
				target, err := endpointURL(ep.BaseURL, ep.Path, entry.ActualModel)
				if err != nil {
					log.Warn(ctx, "realtime candidate configuration rejected", log.Int("channel_id", ch.ID))
					continue
				}
				price, err := s.Usage.RealtimePrice(ctx, ch.ID, entry.ActualModel)
				if err != nil {
					log.Warn(ctx, "realtime candidate configuration rejected", log.Int("channel_id", ch.ID))
					continue
				}
				if format == llm.APIFormatOpenAIRealtimeWebSocket.String() {
					if err := objects.ValidateOpenAIRealtimePrice(price.Price); err != nil {
						log.Warn(ctx, "realtime candidate configuration rejected", log.Int("channel_id", ch.ID))
						continue
					}
				}
				identifier := sha256.Sum256([]byte("axonhub-realtime-v1:" + key.Key + ":" + strconv.Itoa(key.UserID) + ":" + strconv.Itoa(key.ProjectID)))
				timeout := time.Duration(0)
				if ep.HandshakeTimeoutMs != nil {
					timeout = time.Duration(*ep.HandshakeTimeoutMs) * time.Millisecond
				}
				return &RealtimeTarget{HandshakeTimeout: timeout, Channel: ch, Model: entry.ActualModel, ClientModel: model, URL: target, Price: price, APIKey: key, Native: native, APIFormat: llm.APIFormat(format), SafetyIdentifier: hex.EncodeToString(identifier[:])}, nil
			}
		}
	}
	return nil, biz.ErrInvalidModel
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
	conversation, initial, err := s.configure(ctx, upstream, target, config)
	if err != nil {
		upstream.Close()
		return nil, nil, nil, err
	}
	conversation.credential = key
	return upstream, conversation, initial, nil
}

func (s *RealtimeService) configure(ctx context.Context, upstream *websocket.Conn, target *RealtimeTarget, config rt.Config) (*RealtimeConversation, [][]byte, error) {
	var adapter realtimeAdapter
	var err error
	if target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		adapter, err = openairt.NewAdapter(target.Model)
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
	deadline := time.Now().Add(config.HandshakeTimeout)
	if parentDeadline, ok := ctx.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	stopCancel := context.AfterFunc(ctx, func() { _ = upstream.Close() })
	defer stopCancel()
	_ = upstream.SetReadDeadline(deadline)
	kind, created, err := upstream.ReadMessage()
	if err != nil || kind != websocket.TextMessage || gjson.GetBytes(created, "type").String() != "session.created" {
		return nil, nil, errors.New("upstream did not create a realtime session")
	}
	if err := state.ObserveServer("session.created"); err != nil {
		return nil, nil, err
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
		_ = upstream.SetReadDeadline(time.Time{})
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
	_ = upstream.SetWriteDeadline(deadline)
	if err := upstream.WriteMessage(websocket.TextMessage, wire); err != nil {
		return nil, nil, errors.New("upstream configuration write failed")
	}
	_, updated, err := upstream.ReadMessage()
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
	_ = upstream.SetReadDeadline(time.Time{})
	_ = upstream.SetWriteDeadline(time.Time{})
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
	createEventID   string
	request         *ent.Request
	done            bool
	auditFinalized  bool
	auditStatus     request.Status
	auditSummary    map[string]any
	started         time.Time
	firstOutput     *int64
	hadFunctionCall bool
	observedUsage   *llm.Usage
}
type RealtimeConversation struct {
	pendingConfigurationID string
	pendingClearID         string
	clearingInput          *realtimeResponse
	auditsCreated          int
	pendingInput           *realtimeResponse
	pendingResponses       []*realtimeResponse
	service                *RealtimeService
	target                 *RealtimeTarget
	adapter                realtimeAdapter
	state                  *rt.State
	ctx                    context.Context
	started                time.Time
	mu                     sync.Mutex
	responses              map[string]*realtimeResponse
	credential             string
	audioBytes             int64
	checkMu                sync.Mutex
	lastCheck              time.Time
	lastCheckErr           error
	latestKey              *ent.APIKey
}

func (c *RealtimeConversation) Check(ctx context.Context) error {
	if c.state.ConfigurationExpired(15 * time.Second) {
		return errors.New("realtime configuration acknowledgement timed out")
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
	c.lastCheckErr = c.service.checkQuota(checkCtx, current, false)
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
	kind := gjson.GetBytes(data, "type").String()
	if kind != "session.update" && !c.state.Ready() {
		return nil, errors.New("session configuration pending")
	}
	var eventID string
	if kind == "session.update" || kind == "response.create" || kind == "input_audio_buffer.clear" {
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
	if kind == "session.update" || kind == "input_audio_buffer.clear" {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.hasPendingCreate(eventID) || eventID == c.pendingClearID {
			return nil, errors.New("realtime event ID is already pending")
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
	case "input_audio_buffer.clear":
		c.pendingClearID = eventID
		c.clearingInput = c.pendingInput
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
		if eventID != "" && (eventID == c.pendingConfigurationID || eventID == c.pendingClearID || c.hasPendingCreate(eventID)) {
			c.mu.Unlock()
			return nil, errors.New("realtime event ID is already pending")
		}
		if len(c.responses)+len(c.pendingResponses) >= 512 || len(c.pendingResponses) >= 8 {
			c.mu.Unlock()
			return nil, rt.ErrAccountingUnavailable
		}
		if c.pendingInput == nil {
			quotaCtx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.service.checkQuota(quotaCtx, quotaKey)
			cancel()
			if err != nil {
				c.mu.Unlock()
				return nil, err
			}
			pending, err := c.createResponse("pending")
			if err != nil {
				c.mu.Unlock()
				return nil, rt.ErrAccountingUnavailable
			}
			c.pendingInput = pending
		}
		if kind == "response.create" {
			c.pendingInput.createEventID = eventID
			c.pendingResponses = append(c.pendingResponses, c.pendingInput)
			c.pendingInput = nil
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
	if kind == "input_audio_buffer.commit" {
		c.mu.Lock()
		c.audioBytes = 0
		c.mu.Unlock()
	}
	return wire, nil
}

func (c *RealtimeConversation) Server(data []byte) ([]byte, error) {
	kind := gjson.GetBytes(data, "type").String()
	c.mu.Lock()
	defer c.mu.Unlock()
	if kind == "error" {
		if err := c.rejectPendingEvent(gjson.GetBytes(data, "error.event_id").String()); err != nil {
			return nil, err
		}
	}
	if kind == "session.updated" {
		c.pendingConfigurationID = ""
	}
	if kind == "response.done" {
		if previous := c.responses[gjson.GetBytes(data, "response.id").String()]; previous != nil && previous.done {
			return nil, nil
		}
	}
	if err := c.state.ObserveServer(kind, len(c.pendingResponses) > 0 || c.pendingInput != nil); err != nil {
		return nil, err
	}
	if kind == "input_audio_buffer.cleared" {
		if err := c.confirmAudioClear(); err != nil {
			return nil, err
		}
	}
	if kind == "input_audio_buffer.committed" {
		c.audioBytes = 0
	}
	id := gjson.GetBytes(data, "response.id").String()
	if kind == "response.created" {
		if err := c.admitResponse(id); err != nil {
			return nil, err
		}
	}
	if kind == "response.audio.delta" || kind == "response.text.delta" || kind == "response.output_audio.delta" || kind == "response.output_text.delta" {
		responseID := gjson.GetBytes(data, "response_id").String()
		if r := c.responses[responseID]; r != nil && r.firstOutput == nil {
			r.firstOutput = lo.ToPtr(time.Since(r.started).Milliseconds())
		}
	}
	if kind == "response.function_call_arguments.delta" || kind == "response.function_call_arguments.done" || ((kind == "response.output_item.added" || kind == "response.output_item.done") && gjson.GetBytes(data, "item.type").String() == "function_call") {
		if response := c.responses[gjson.GetBytes(data, "response_id").String()]; response != nil {
			response.hadFunctionCall = true
		}
	}
	if kind == "response.done" {
		parseUsage := protocol.ParseUsage
		if c.target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
			parseUsage = openairt.ParseUsage
		}
		responseID, usage, err := parseUsage(data)
		r := c.responses[id]
		if r == nil {
			return nil, errors.New("response.done without response.created")
		}
		if r.done {
			return nil, nil
		}
		if err != nil || usage == nil || responseID != id {
			return nil, errors.New("realtime response usage missing or invalid")
		}
		r.observedUsage = usage
		for _, item := range gjson.GetBytes(data, "response.output").Array() {
			if item.Get("type").String() == "function_call" {
				r.hadFunctionCall = true
			}
		}
		if err := applyRealtimeBillingPolicy(c.target, usage, r.hadFunctionCall); err != nil {
			return nil, err
		}
		// Persist confirmed usage even when the downstream has gone away.
		billingCtx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 10*time.Second)
		defer cancel()
		projectID, _ := contexts.GetProjectID(c.ctx)
		_, err = c.service.Usage.CreateRealtimeUsage(billingCtx, biz.CreateUsageLogParams{RequestID: r.request.ID, ProjectID: projectID, ChannelID: c.target.Channel.ID, ActualModelID: c.target.Model, Usage: usage, Source: usagelog.SourceAPI, Format: string(llm.APIFormatOpenAIRealtime), APIKeyID: lo.ToPtr(c.target.APIKey.ID)}, c.target.Price)
		if err != nil {
			return nil, errors.New("realtime usage persistence failed")
		}
		r.done = true
		metrics.Metrics.RealtimeUsage(billingCtx, usage, r.firstOutput)
		status := request.StatusCompleted
		switch gjson.GetBytes(data, "response.status").String() {
		case "cancelled", "canceled":
			status = request.StatusCanceled
		case "failed", "incomplete":
			status = request.StatusFailed
		}
		if status == request.StatusCompleted && c.credential != "" {
			c.service.Channels.RecordRealtimeCredentialSuccess(c.target.Channel.ID, c.credential)
		}
		summary := map[string]any{"response_id": id, "usage": usage, "realtime": true, "usage_confirmed": true, "upstream_model": c.target.Model, "upstream_format": c.target.APIFormat, "price_reference_id": c.target.Price.ReferenceID, "price_snapshot": c.target.Price.Price, "price_quoted_at": c.target.Price.QuotedAt}
		r.auditStatus = status
		r.auditSummary = summary
		err = c.service.Requests.UpdateRealtimeAudit(billingCtx, r.request, status, id, summary, &biz.LatencyMetrics{LatencyMs: lo.ToPtr(time.Since(r.started).Milliseconds()), FirstTokenLatencyMs: r.firstOutput}, nil)
		if err != nil {
			return nil, errors.New("realtime audit update failed")
		}
		r.auditFinalized = true
	}
	converted, err := c.adapter.Server(data)
	if err != nil {
		return nil, err
	}
	if c.target.APIFormat == llm.APIFormatOpenAIRealtimeWebSocket {
		converted, err = applyRealtimeErrorPolicy(converted)
		if err != nil {
			return nil, err
		}
	}
	return c.aliasModel(converted), nil
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
	return nil
}

// hasPendingCreate is called with c.mu held.
func (c *RealtimeConversation) hasPendingCreate(eventID string) bool {
	return eventID != "" && slices.ContainsFunc(c.pendingResponses, func(r *realtimeResponse) bool {
		return r.createEventID == eventID
	})
}

// rejectPendingEvent finalizes only the reservation named by the provider.
// The caller holds c.mu to serialize this with response admission.
func (c *RealtimeConversation) rejectPendingEvent(eventID string) error {
	if eventID == "" {
		return nil
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
	summary := map[string]any{"realtime": true, "usage_confirmed": false, "response_create_rejected": true}
	if err := c.service.Requests.UpdateRealtimeAudit(ctx, pending.request, request.StatusFailed, "", summary, nil, nil); err != nil {
		return errors.New("realtime rejected-response audit failed")
	}
	c.pendingResponses = slices.Delete(c.pendingResponses, index, index+1)
	return nil
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
	if len(c.pendingResponses) > 0 {
		r = c.pendingResponses[0]
		c.pendingResponses = c.pendingResponses[1:]
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
	c.responses[id] = r
	return nil
}

func (c *RealtimeConversation) createResponse(id string) (*realtimeResponse, error) {
	if c.auditsCreated >= 512 {
		return nil, errors.New("realtime audit limit reached")
	}
	if id == "" || len(id) > 256 {
		return nil, errors.New("invalid upstream response id")
	}
	metadata, _ := json.Marshal(map[string]any{"model": c.target.ClientModel, "realtime": true, "native_qwen": c.target.Native, "channel_id": c.target.Channel.ID, "upstream_model": c.target.Model, "upstream_format": c.target.APIFormat, "price_reference_id": c.target.Price.ReferenceID, "price_snapshot": c.target.Price.Price, "price_quoted_at": c.target.Price.QuotedAt})
	req, err := c.service.Requests.CreateRealtimeRequest(c.ctx, c.target.ClientModel, c.target.Channel.ID, c.target.SessionID, metadata)
	if err != nil {
		return nil, errors.New("realtime audit create failed")
	}
	req.ChannelID = c.target.Channel.ID
	c.auditsCreated++
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

func (c *RealtimeConversation) Close() {
	c.state.Close()
	c.mu.Lock()
	defer c.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.ctx), 10*time.Second)
	defer cancel()
	pending := append([]*realtimeResponse{}, c.pendingResponses...)
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
		if err := c.service.Requests.UpdateRealtimeAudit(ctx, r.request, request.StatusFailed, "", map[string]any{"realtime": true, "usage_confirmed": false, "error": "session ended before input or generation accounting was confirmed", "upstream_model": c.target.Model, "price_reference_id": c.target.Price.ReferenceID, "price_snapshot": c.target.Price.Price}, nil, nil); err != nil {
			log.Warn(context.WithoutCancel(c.ctx), "realtime pending audit finalization failed", log.Int("request_id", r.request.ID))
		}
	}
	c.pendingResponses = nil
	c.pendingInput = nil
	c.clearingInput = nil
	c.pendingClearID = ""
	for id, r := range c.responses {
		if r.auditFinalized {
			continue
		}
		if ctx.Err() != nil {
			log.Warn(context.WithoutCancel(c.ctx), "realtime audit drain deadline exceeded")
			break
		}
		status := request.StatusFailed
		summary := map[string]any{"realtime": true, "usage_confirmed": false, "error": "session ended before confirmed response usage", "upstream_model": c.target.Model, "upstream_format": c.target.APIFormat, "price_reference_id": c.target.Price.ReferenceID, "price_snapshot": c.target.Price.Price, "price_quoted_at": c.target.Price.QuotedAt}
		if r.observedUsage != nil {
			summary["observed_usage"] = r.observedUsage
		}
		if r.done {
			status = r.auditStatus
			summary = r.auditSummary
		}
		if err := c.service.Requests.UpdateRealtimeAudit(ctx, r.request, status, id, summary, &biz.LatencyMetrics{LatencyMs: lo.ToPtr(time.Since(r.started).Milliseconds())}, nil); err != nil {
			log.Warn(context.WithoutCancel(c.ctx), "realtime audit finalization failed", log.Int("request_id", r.request.ID))
			continue
		}
		r.auditFinalized = true
	}
}

func NewRealtimeService(params RealtimeServiceParams) *RealtimeService {
	return &RealtimeService{Transport: params.Transport, Auth: params.Auth, Selector: params.Selector, Channels: params.Channels, Quota: params.Quota, Usage: params.Usage, Requests: params.Requests}
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
