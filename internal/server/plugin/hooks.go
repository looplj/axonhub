package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

const (
	hookBudget         = 10 * time.Second
	hookBorrowWait     = 100 * time.Millisecond
	hookInputLimit     = 2 << 20
	onRequestExport    = "onRequest"
	onAttemptExport    = "onAttempt"
	hookOverloadedCode = "plugin_overloaded"
)

// SetHookTimeoutForTest overrides the 10s request-hook budget for this manager.
// Production callers leave it unset.
func (m *Manager) SetHookTimeoutForTest(timeout time.Duration) {
	if m == nil {
		return
	}
	m.hookTimeout = timeout
}

func (m *Manager) hookBudget() time.Duration {
	if m != nil && m.hookTimeout > 0 {
		return m.hookTimeout
	}
	return hookBudget
}

// Middleware captures one generation for both stages and all retries.
// The request middleware owns the generation until pipeline.Process returns.
func (m *Manager) Middleware(ctx context.Context) (orchestrator.RequestHooks, error) {
	if m == nil {
		return orchestrator.RequestHooks{}, nil
	}
	gen, err := acquireHookGeneration(ctx, m.current)
	if err != nil {
		return orchestrator.RequestHooks{}, err
	}
	if gen == nil {
		return orchestrator.RequestHooks{}, nil
	}
	session := &requestHookSession{
		manager:          m,
		gen:              gen,
		states:           map[int]json.RawMessage{},
		requestOverrides: map[string]int{},
	}
	contexts.SetRequestHookState(ctx, session)
	request := &requestHookMiddleware{session: session}
	return orchestrator.RequestHooks{
		Request: request,
		Attempt: pipeline.OnRawRequest("plugin-attempt-hooks", request.OnAttemptRawRequest),
	}, nil
}

// acquireHookGeneration retries if refresh retired the captured generation.
// A closed generation that is still current indicates shutdown; return an error
// rather than silently executing a request without its configured hooks.
func acquireHookGeneration(ctx context.Context, current func() *Generation) (*Generation, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		gen := current()
		if !generationHasRequestHooks(gen) {
			return nil, nil
		}
		if err := gen.Acquire(); err != nil {
			if errors.Is(err, ErrGenerationClosed) && current() != gen {
				continue
			}
			return nil, fmt.Errorf("acquire plugin request hooks: %w", err)
		}
		return gen, nil
	}
}

// PassThroughApplied records plugins whose non-model edits this attempt discarded
// because the channel replayed the client body. A nil session does nothing.
func (m *Manager) PassThroughApplied(ctx context.Context, channelID int) {
	if m == nil {
		return
	}
	session, _ := contexts.GetRequestHookState(ctx).(*requestHookSession)
	if session == nil || session.manager != m {
		return
	}
	requestID, _ := contexts.GetRequestID(ctx)
	for _, item := range session.passThroughTargets() {
		log.Warn(ctx, "plugin request rewrite overridden by pass-through",
			log.String("plugin_id", item.pluginID),
			log.Int("channel_id", channelID),
			log.String("request_id", requestID),
		)
		m.addHookCounter(item.installationID, false)
	}
}

func (h *requestHookMiddleware) OnProcessEnd() {
	if h == nil || h.session == nil {
		return
	}
	h.session.release()
}

type requestHookMiddleware struct {
	pipeline.DummyMiddleware

	session *requestHookSession
}

var _ pipeline.OutboundLlmRequestMiddleware = (*requestHookMiddleware)(nil)

func (h *requestHookMiddleware) Name() string { return "plugin-request-hooks" }

func (h *requestHookMiddleware) OnInboundLlmRequest(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if h == nil || h.session == nil || request == nil {
		return request, nil
	}
	contexts.SetRequestHookState(ctx, h.session)
	h.session.captureEntry(ctx, request)
	return h.session.runRequestHooks(ctx, request)
}

// OnOutboundLlmRequest captures attempt metadata and resets pass-through counting.
// It does not execute JavaScript; onAttempt runs later on the upstream JSON body.
func (h *requestHookMiddleware) OnOutboundLlmRequest(ctx context.Context, request *llm.Request, format llm.APIFormat) (*llm.Request, error) {
	if h == nil || h.session == nil || request == nil {
		return request, nil
	}
	contexts.SetRequestHookState(ctx, h.session)
	h.session.beginAttempt()
	channel, _ := contexts.GetAttemptChannel(ctx)
	attempt, err := json.Marshal(attemptContextJSON{
		Channel:   attemptChannelJSON{ID: channel.ID, Name: channel.Name, Type: channel.Type},
		Model:     request.Model,
		APIFormat: string(format),
	})
	if err != nil {
		return request, nil //nolint:nilerr // Hook metadata serialization failures leave the request unchanged.
	}
	h.session.attemptJSON = attempt
	return request, nil
}

type requestHookSession struct {
	manager          *Manager
	gen              *Generation
	states           map[int]json.RawMessage
	requestJSON      json.RawMessage
	attemptJSON      json.RawMessage
	requestOverrides map[string]int
	counted          map[string]struct{}
	released         bool
}

type hookOverride struct {
	pluginID       string
	installationID int
}

func (s *requestHookSession) captureEntry(ctx context.Context, request *llm.Request) {
	if s == nil || len(s.requestJSON) > 0 {
		return
	}
	meta := requestContextJSON{Model: request.Model, APIFormat: string(request.APIFormat), RequestType: string(request.RequestType)}
	if meta.RequestType == "" {
		meta.RequestType = string(llm.RequestTypeChat)
	}
	if projectID, ok := contexts.GetProjectID(ctx); ok {
		meta.ProjectID = &projectID
	}
	if apiKey, ok := contexts.GetAPIKey(ctx); ok && apiKey != nil {
		meta.APIKeyID = &apiKey.ID
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		raw = []byte(`{"projectId":null,"apiKeyId":null,"model":"","apiFormat":""}`)
	}
	s.requestJSON = raw
}

func (s *requestHookSession) beginAttempt() {
	if s == nil {
		return
	}
	s.counted = map[string]struct{}{}
}

func (s *requestHookSession) release() {
	if s == nil {
		return
	}
	if s.released {
		return
	}
	s.released = true
	s.states = nil
	s.requestOverrides = nil
	if s.gen != nil {
		s.gen.Release()
		s.gen = nil
	}
}

func (s *requestHookSession) runRequestHooks(ctx context.Context, request *llm.Request) (*llm.Request, error) {
	if s == nil || s.released || s.gen == nil || s.manager == nil {
		return request, nil
	}
	for _, item := range s.gen.Plugins() {
		if !hookPlugin(item, onRequestExport) || !item.Manifest.Hooks.Request.Matches(request.RequestType) {
			continue
		}
		next, err := s.callRequestHook(ctx, item, request)
		if err != nil {
			return nil, err
		}
		request = next
	}
	return request, nil
}

func (s *requestHookSession) callRequestHook(ctx context.Context, item *LoadedPlugin, request *llm.Request) (*llm.Request, error) {
	raw, err := hookRequestJSON(request)
	if err != nil {
		s.manager.noteCallFailure(item.ID)
		return request, nil //nolint:nilerr // Hook input serialization failures are counted but must not block the request.
	}
	if len(raw) > hookInputLimit {
		s.manager.noteCallFailure(item.ID)
		return request, nil
	}
	args, err := json.Marshal([]json.RawMessage{raw})
	if err != nil {
		s.manager.noteCallFailure(item.ID)
		return request, nil //nolint:nilerr // Hook input serialization failures are counted but must not block the request.
	}
	state := s.states[item.ID]
	var decoded llm.Request
	var protected []string
	result, err := s.manager.Invoke(ctx, InvokeRequest{
		InstallationID: item.ID,
		Export:         onRequestExport,
		Args:           args,
		State:          state,
		Config:         item.Config,
		Timeout:        s.manager.hookBudget(),
		Generation:     s.gen,
		BorrowWait:     hookBorrowWait,
		ValidateReturn: func(value json.RawMessage) error {
			output, err := applyHookReturn(raw, value)
			if err != nil {
				return err
			}
			if len(output) == 0 {
				return nil
			}
			if err := json.Unmarshal(output, &decoded); err != nil {
				return err
			}
			protected = protectedRequestFields(request, &decoded)
			if len(protected) > 0 {
				return errProtectedRewrite
			}
			return nil
		},
		HostRequest: s.requestJSON,
		AllowReject: true,
	})
	if errors.Is(err, errProtectedRewrite) {
		s.manager.noteProtectedDrop(ctx, item, onRequestExport, string(request.APIFormat), protected)
		return request, nil
	}
	if err != nil {
		return s.failCall(ctx, item, request, onRequestExport, err)
	}
	if len(bytes.TrimSpace(result.Value)) == 0 {
		s.commitState(item.ID, result.State)
		return request, nil
	}
	decoded.Stream = request.Stream
	before, beforeErr := comparablePublic(request)
	replacePublicRequest(request, decoded)
	after, afterErr := comparablePublic(request)
	if beforeErr == nil && afterErr == nil && !bytes.Equal(before, after) {
		s.noteRequestOverride(item)
	}
	s.commitState(item.ID, result.State)
	return request, nil
}

func (s *requestHookSession) failCall(ctx context.Context, item *LoadedPlugin, request *llm.Request, export string, err error) (*llm.Request, error) {
	if errors.Is(err, ErrVMUnavailable) {
		s.manager.addHookCounter(item.ID, true)
		requestID, _ := contexts.GetRequestID(ctx)
		log.Warn(ctx, "plugin hook overloaded",
			log.String("plugin_id", item.PluginID),
			log.String("hook", export),
			log.String("request_id", requestID),
		)
		response := pluginOverloadedResponse(item.PluginID, requestID)
		if export == onAttemptExport {
			return nil, orchestrator.NewPluginOverloadedError(response)
		}
		return nil, response
	}
	if errors.Is(err, ErrCircuitOpen) || errors.Is(err, ErrGenerationClosed) {
		return request, nil
	}
	if rejected, ok := errors.AsType[*requestRejectedError](err); ok {
		requestID, _ := contexts.GetRequestID(ctx)
		return nil, &llm.ResponseError{
			StatusCode: rejected.status,
			Detail: llm.ErrorDetail{
				Message:   rejected.message,
				Type:      rejectErrorType(rejected.status),
				Code:      rejectErrorCode(rejected.status),
				RequestID: requestID,
			},
		}
	}
	return request, nil
}

func (s *requestHookSession) commitState(installationID int, state json.RawMessage) {
	if s.states == nil {
		s.states = map[int]json.RawMessage{}
	}
	if len(bytes.TrimSpace(state)) == 0 {
		s.states[installationID] = json.RawMessage(`{}`)
		return
	}
	s.states[installationID] = append(json.RawMessage(nil), state...)
}

func (s *requestHookSession) noteRequestOverride(item *LoadedPlugin) {
	if item == nil {
		return
	}
	s.requestOverrides[item.PluginID] = item.ID
}

// passThroughTargets reports only onRequest edits. onAttempt runs after
// pass-through, so its edits cannot be overwritten by replaying the client body.
func (s *requestHookSession) passThroughTargets() []hookOverride {
	if s == nil {
		return nil
	}
	if s.counted == nil {
		s.counted = map[string]struct{}{}
	}
	ids := make([]string, 0, len(s.requestOverrides))
	for id := range s.requestOverrides {
		if _, seen := s.counted[id]; seen {
			continue
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	out := make([]hookOverride, 0, len(ids))
	for _, id := range ids {
		s.counted[id] = struct{}{}
		out = append(out, hookOverride{pluginID: id, installationID: s.requestOverrides[id]})
	}
	return out
}

type requestContextJSON struct {
	ProjectID   *int   `json:"projectId"`
	APIKeyID    *int   `json:"apiKeyId"`
	Model       string `json:"model"`
	APIFormat   string `json:"apiFormat"`
	RequestType string `json:"requestType"`
}

type attemptContextJSON struct {
	Channel   attemptChannelJSON `json:"channel"`
	Model     string             `json:"model"`
	APIFormat string             `json:"apiFormat"`
}

type attemptChannelJSON struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func generationHasRequestHooks(gen *Generation) bool {
	if gen == nil {
		return false
	}
	for _, item := range gen.Plugins() {
		if hookPlugin(item, onRequestExport) || hookPlugin(item, onAttemptExport) {
			return true
		}
	}
	return false
}

func hookPlugin(item *LoadedPlugin, export string) bool {
	if item == nil || !item.Enabled || item.Status != RuntimeStatusActive || item.resource == nil {
		return false
	}
	if item.Manifest.Hooks == nil {
		return false
	}
	switch export {
	case onRequestExport:
		return item.Manifest.Hooks.Request.IsEnabled()
	case onAttemptExport:
		return item.Manifest.Hooks.Attempt.IsEnabled()
	default:
		return false
	}
}

func hookRequestJSON(request *llm.Request) ([]byte, error) {
	if request == nil {
		return []byte("null"), nil
	}
	clone := *request
	clone.RawRequest = nil
	clone.RequestType = ""
	clone.APIFormat = ""
	clone.TransformOptions = llm.TransformOptions{}
	clone.TransformerMetadata = nil
	clone.ProviderExtensions = nil
	return json.Marshal(&clone)
}

func replacePublicRequest(dst *llm.Request, src llm.Request) {
	if dst == nil {
		return
	}
	preserveRequestPayload(&src, dst)
	raw := dst.RawRequest
	requestType := dst.RequestType
	format := dst.APIFormat
	options := dst.TransformOptions
	metadata := dst.TransformerMetadata
	extensions := dst.ProviderExtensions
	stream := dst.Stream
	*dst = src
	dst.RawRequest = raw
	dst.RequestType = requestType
	dst.APIFormat = format
	dst.TransformOptions = options
	dst.TransformerMetadata = metadata
	dst.ProviderExtensions = extensions
	dst.Stream = stream
}

// sameStreamShape treats nil and false as the same non-streaming shape.
// encoding/json omitempty drops an explicit false, so a spread return cannot
// echo it. true versus anything else is a stream change and is rejected.
func sameStreamShape(before, after *bool) bool {
	return streaming(before) == streaming(after)
}

func streaming(value *bool) bool {
	return value != nil && *value
}

func comparablePublic(request *llm.Request) ([]byte, error) {
	raw, err := hookRequestJSON(request)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	delete(obj, "model")
	return json.Marshal(obj)
}

func pluginOverloadedResponse(pluginID, requestID string) *llm.ResponseError {
	return &llm.ResponseError{
		StatusCode: http.StatusTooManyRequests,
		Detail: llm.ErrorDetail{
			Code:      hookOverloadedCode,
			Type:      "rate_limit_error",
			Message:   fmt.Sprintf("plugin %s is at capacity; please retry shortly", pluginID),
			RequestID: requestID,
		},
	}
}

func rejectErrorType(status int) string {
	switch status {
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusForbidden:
		return "permission_error"
	default:
		return "invalid_request_error"
	}
}

func rejectErrorCode(status int) string {
	switch status {
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusForbidden:
		return "permission_denied"
	default:
		return "invalid_request"
	}
}

func (m *Manager) noteCallFailure(installationID int) {
	if m == nil || installationID <= 0 {
		return
	}
	m.execFor(installationID).recordFailure(m.nowCall())
}

func (m *Manager) addHookCounter(installationID int, overloaded bool) {
	if m == nil || installationID <= 0 {
		return
	}
	m.hookMu.Lock()
	defer m.hookMu.Unlock()
	if m.hookCounters == nil {
		m.hookCounters = map[int]hookCounter{}
	}
	counter := m.hookCounters[installationID]
	if overloaded {
		counter.overloaded++
	} else {
		counter.overridden++
	}
	m.hookCounters[installationID] = counter
}

func (m *Manager) hookCounter(installationID int) hookCounter {
	if m == nil || installationID <= 0 {
		return hookCounter{}
	}
	m.hookMu.Lock()
	defer m.hookMu.Unlock()
	return m.hookCounters[installationID]
}
