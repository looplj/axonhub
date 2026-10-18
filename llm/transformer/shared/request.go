package shared

import (
	"context"

	"github.com/looplj/axonhub/llm"
)

// requestContextKey is the key carrying the unified request alongside a stream.
type requestContextKey struct{}

// WithRequest returns a context carrying the unified request.
//
// The streaming path consumes the request on the outbound side before the
// inbound transformers run, so the context is their only way back to it. Only
// the request's read-only payload is meaningful here — transformers run
// concurrently with the stream, so the request must be treated as immutable.
//
// This lives in `shared` rather than in `pipeline` so inbound transformers can
// read it without importing the orchestrator package (which owns the pipeline
// and already depends on transformer interfaces).
func WithRequest(ctx context.Context, request *llm.Request) context.Context {
	if request == nil {
		return ctx
	}

	return context.WithValue(ctx, requestContextKey{}, request)
}

// RequestFrom returns the unified request carried by ctx, or nil.
func RequestFrom(ctx context.Context) *llm.Request {
	if ctx == nil {
		return nil
	}

	request, _ := ctx.Value(requestContextKey{}).(*llm.Request)

	return request
}
