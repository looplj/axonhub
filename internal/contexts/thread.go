package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// WithThread stores the thread for this request.
// The thread middleware fills only ID, ProjectID, and ThreadID.
func WithThread(ctx context.Context, thread *ent.Thread) context.Context {
	container := getContainer(ctx)
	container.Thread = thread

	return withContainer(ctx, container)
}

// GetThread returns the thread stored for this request.
// Status, timestamps, and edges are unset on the middleware value; load the row to read them.
func GetThread(ctx context.Context) (*ent.Thread, bool) {
	container := getContainer(ctx)
	return container.Thread, container.Thread != nil
}
