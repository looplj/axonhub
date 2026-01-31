package events

import "time"

// EventType represents the type of event being published
type EventType string

const (
	EventTypeRequestCreated   EventType = "request.created"
	EventTypeRequestUpdated   EventType = "request.updated"
	EventTypeRequestCompleted EventType = "request.completed"
	// Future: EventTypeTraceCreated, EventTypeChannelUpdated, etc.
)

// Topic represents the event topic/channel
type Topic string

const (
	TopicRequests Topic = "requests"
	// Future: TopicTraces, TopicChannels
)

// Event is the base event structure
type Event struct {
	Type      EventType   `json:"type"`
	Topic     Topic       `json:"topic"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// RequestEventPayload contains request-specific event data
type RequestEventPayload struct {
	RequestID int    `json:"request_id"`
	ProjectID int    `json:"project_id"`
	Status    string `json:"status"`
	ModelID   string `json:"model_id,omitempty"`
	Source    string `json:"source,omitempty"`
	Stream    bool   `json:"stream"`

	// Token usage data
	PromptTokens     int64 `json:"prompt_tokens,omitempty"`
	CompletionTokens int64 `json:"completion_tokens,omitempty"`
	TotalTokens      int64 `json:"total_tokens,omitempty"`

	// Cache tokens
	PromptCachedTokens        int64 `json:"prompt_cached_tokens,omitempty"`
	PromptWriteCachedTokens   int64 `json:"prompt_write_cached_tokens,omitempty"`
	PromptWriteCachedTokens5m int64 `json:"prompt_write_cached_tokens_5m,omitempty"`
	PromptWriteCachedTokens1h int64 `json:"prompt_write_cached_tokens_1h,omitempty"`

	// Include enough data for UI to update without additional fetch
	APIKeyID  *int      `json:"api_key_id,omitempty"`
	ChannelID *int      `json:"channel_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
