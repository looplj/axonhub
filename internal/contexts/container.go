package contexts

import (
	"context"
	"sync"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
)

// contextContainer contains all values in the context.
type contextContainer struct {
	ProjectID     *int
	TraceID       *string
	RequestID     *string
	OperationName *string
	APIKey        *ent.APIKey
	User          *ent.User
	Source        *request.Source
	Thread        *ent.Thread
	Trace         *ent.Trace
	Errors        []error
	mu            sync.RWMutex
}

// getContainer retrieves the existing container from context, or creates a new one and stores it in the context if it doesn't exist.
func getContainer(ctx context.Context) *contextContainer {
	if container, ok := ctx.Value(containerContextKey).(*contextContainer); ok {
		return container
	}

	// If container doesn't exist, create a new one and store it in the context
	container := &contextContainer{}

	return container
}

// withContainer stores the container in the context (if not already stored).
func withContainer(ctx context.Context, container *contextContainer) context.Context {
	if ctx.Value(containerContextKey) == nil {
		return context.WithValue(ctx, containerContextKey, container)
	}

	return ctx
}
