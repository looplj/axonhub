package bus

// Event represents an event on the bus.
type Event struct {
	// Type categorizes the event (e.g., "request", "response", "error").
	Type string
	// Topic is the routing key for publish/subscribe.
	Topic string
	// Metadata carries contextual information for the event.
	Metadata Metadata
	// Payload is the event data.
	Payload any
}

// Metadata carries contextual information for events.
type Metadata struct {
	// ID is a unique identifier for the event, auto-generated if empty on publish.
	ID string
	// Source identifies the origin of the event.
	Source string
	// TraceID is used for distributed tracing correlation.
	TraceID string
	// ThreadID links the event to a specific thread.
	ThreadID string
	// Extra holds arbitrary key-value pairs for extensibility.
	Extra map[string]string
}
