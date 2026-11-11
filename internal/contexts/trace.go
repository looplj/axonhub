package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// WithTrace stores the trace for this request.
// The trace middleware fills only ID, ProjectID, TraceID, and ThreadID.
func WithTrace(ctx context.Context, trace *ent.Trace) context.Context {
	container := getContainer(ctx)
	container.Trace = trace

	return withContainer(ctx, container)
}

// GetTrace returns the trace stored for this request.
// Status, timestamps, and edges are unset on the middleware value; load the row to read them.
func GetTrace(ctx context.Context) (*ent.Trace, bool) {
	container := getContainer(ctx)
	return container.Trace, container.Trace != nil
}
