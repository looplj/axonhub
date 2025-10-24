package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// WithTrace stores the trace entity in the context.
func WithTrace(ctx context.Context, trace *ent.Trace) context.Context {
	container := getContainer(ctx)
	container.Trace = trace

	return withContainer(ctx, container)
}

// GetTrace retrieves the trace entity from the context.
func GetTrace(ctx context.Context) (*ent.Trace, bool) {
	container := getContainer(ctx)
	return container.Trace, container.Trace != nil
}
