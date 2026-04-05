package bus

import "context"

type contextKey string

const (
	eventContextKey    contextKey = "bus_event"
	metadataContextKey contextKey = "bus_metadata"
)

// ContextWithEvent returns a new context carrying the given event.
func ContextWithEvent(ctx context.Context, event Event) context.Context {
	return context.WithValue(ctx, eventContextKey, event)
}

// EventFromContext extracts an event from the context.
// Returns the event and true if present, or a zero Event and false otherwise.
func EventFromContext(ctx context.Context) (Event, bool) {
	event, ok := ctx.Value(eventContextKey).(Event)
	return event, ok
}

// ContextWithMetadata returns a new context carrying the given metadata.
func ContextWithMetadata(ctx context.Context, meta Metadata) context.Context {
	return context.WithValue(ctx, metadataContextKey, meta)
}

// MetadataFromContext extracts metadata from the context.
// Returns the metadata and true if present, or a zero Metadata and false otherwise.
func MetadataFromContext(ctx context.Context) (Metadata, bool) {
	meta, ok := ctx.Value(metadataContextKey).(Metadata)
	return meta, ok
}
