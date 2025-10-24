package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// WithThread stores the thread entity in the context.
func WithThread(ctx context.Context, thread *ent.Thread) context.Context {
	container := getContainer(ctx)
	container.Thread = thread

	return withContainer(ctx, container)
}

// GetThread retrieves the thread entity from the context.
func GetThread(ctx context.Context) (*ent.Thread, bool) {
	container := getContainer(ctx)
	return container.Thread, container.Thread != nil
}
