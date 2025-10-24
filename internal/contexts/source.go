package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent/request"
)

// WithSource stores the request source in the context.
func WithSource(ctx context.Context, source request.Source) context.Context {
	container := getContainer(ctx)
	container.Source = &source

	return withContainer(ctx, container)
}

// GetSource retrieves the request source from the context.
func GetSource(ctx context.Context) (request.Source, bool) {
	container := getContainer(ctx)
	if container.Source != nil {
		return *container.Source, true
	}

	return request.SourceAPI, false
}

// GetSourceOrDefault retrieves the request source from the context, or returns the default value if it doesn't exist.
func GetSourceOrDefault(ctx context.Context, defaultSource request.Source) request.Source {
	if source, ok := GetSource(ctx); ok {
		return source
	}

	return defaultSource
}
