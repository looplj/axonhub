package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
)

// RequestHooks places request and attempt callbacks at separate pipeline stages.
// Request owns shared session cleanup through OnProcessEnd; Attempt must not
// release that session a second time.
type RequestHooks struct {
	Request pipeline.Middleware
	Attempt pipeline.Middleware
}

// RequestHookProvider captures request-scoped hooks, or returns empty hooks when
// none are enabled. Acquisition errors must not silently bypass configured hooks.
type RequestHookProvider interface {
	Middleware(context.Context) (RequestHooks, error)
	PassThroughApplied(context.Context, int)
}

// PluginIntegration is the plugin surface the chat orchestrator calls.
// Event publication stays on EventBus. This struct does not republish events.
type PluginIntegration struct {
	Hooks RequestHookProvider
}

// RequestHooks acquires request-scoped hooks. Nil providers yield empty hooks.
func (p PluginIntegration) RequestHooks(ctx context.Context) (RequestHooks, error) {
	if p.Hooks == nil {
		return RequestHooks{}, nil
	}
	return p.Hooks.Middleware(ctx)
}

// requestStage returns the middlewares registered before persistent transformers.
func (h RequestHooks) requestStage() []pipeline.Middleware {
	if h.Request == nil {
		return nil
	}
	return []pipeline.Middleware{h.Request}
}

// attemptStage returns the raw-body hook placed after pass-through.
func (h RequestHooks) attemptStage(outbound *PersistentOutboundTransformer) []pipeline.Middleware {
	if h.Attempt == nil {
		return nil
	}
	return []pipeline.Middleware{ApplyRequestHookAttempt(outbound, h.Attempt)}
}

// ApplyRequestHookAttempt schedules raw-body hooks after pass-through and before
// performance recording, execution persistence, and channel admission. If a hook
// rejects for capacity, those later OnOutboundRawRequest callbacks have not run
// for this attempt, but the pipeline still broadcasts OnOutboundRawError to ALL
// middleware. Health observers must ignore isLocalAdmissionError, and execution
// persistence must ignore plugin overload because this attempt has no execution.
// state.Perf may still refer to the previous attempt. Cleanup callbacks must
// remain safe before admission.
func ApplyRequestHookAttempt(outbound *PersistentOutboundTransformer, hook pipeline.Middleware) pipeline.Middleware {
	return &requestHookAttempt{Middleware: hook, outbound: outbound}
}

// Preserve the supplied middleware's lifecycle callbacks while observing the
// attempt result for retry policy and persistence.
type requestHookAttempt struct {
	pipeline.Middleware

	outbound *PersistentOutboundTransformer
}

func (m *requestHookAttempt) OnOutboundRawRequest(ctx context.Context, request *httpclient.Request) (*httpclient.Request, error) {
	next, err := m.Middleware.OnOutboundRawRequest(ctx, request)
	if err != nil {
		if isPluginOverloaded(err) {
			m.outbound.state.PluginOverloaded = true
		}
		return nil, err
	}
	m.outbound.state.RawProviderRequest = next
	return next, nil
}
