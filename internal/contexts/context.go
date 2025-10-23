package contexts

import (
	"context"

	"github.com/looplj/axonhub/internal/ent"
)

// ContextKey defines the context key type.
type ContextKey string

const (
	// containerContextKey is used to store the context container in the context.
	containerContextKey ContextKey = "context_container"
)

// WithAPIKey stores the API key entity in the context.
func WithAPIKey(ctx context.Context, apiKey *ent.APIKey) context.Context {
	container := getContainer(ctx)
	container.APIKey = apiKey

	return withContainer(ctx, container)
}

// GetAPIKey retrieves the API key entity from the context.
func GetAPIKey(ctx context.Context) (*ent.APIKey, bool) {
	container := getContainer(ctx)
	return container.APIKey, container.APIKey != nil
}

// GetAPIKeyString retrieves the API key string from the context (for backward compatibility).
func GetAPIKeyString(ctx context.Context) (string, bool) {
	apiKey, ok := GetAPIKey(ctx)
	if !ok || apiKey == nil {
		return "", false
	}

	return apiKey.Key, true
}

// WithUser stores the user entity in the context.
func WithUser(ctx context.Context, user *ent.User) context.Context {
	container := getContainer(ctx)
	container.User = user

	return withContainer(ctx, container)
}

// GetUser retrieves the user entity from the context.
func GetUser(ctx context.Context) (*ent.User, bool) {
	container := getContainer(ctx)
	return container.User, container.User != nil
}

// WithTraceID stores the trace id in the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	container := getContainer(ctx)
	container.TraceID = &traceID

	return withContainer(ctx, container)
}

// GetTraceID retrieves the trace id from the context.
func GetTraceID(ctx context.Context) (string, bool) {
	container := getContainer(ctx)
	if container.TraceID != nil {
		return *container.TraceID, true
	}

	return "", false
}

// WithOperationName stores the operation name in the context.
func WithOperationName(ctx context.Context, name string) context.Context {
	container := getContainer(ctx)
	container.OperationName = &name

	return withContainer(ctx, container)
}

// GetOperationName retrieves the operation name from the context.
func GetOperationName(ctx context.Context) (string, bool) {
	container := getContainer(ctx)
	if container.OperationName != nil {
		return *container.OperationName, true
	}

	return "", false
}

// WithProjectID stores the project ID in the context.
func WithProjectID(ctx context.Context, projectID int) context.Context {
	container := getContainer(ctx)
	container.ProjectID = &projectID

	return withContainer(ctx, container)
}

// GetProjectID retrieves the project ID from the context.
func GetProjectID(ctx context.Context) (int, bool) {
	container := getContainer(ctx)
	if container.ProjectID != nil {
		return *container.ProjectID, true
	}

	return 0, false
}
