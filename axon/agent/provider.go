package agent

import (
	"context"
)

// StreamEvent represents a streaming event from the LLM.
type StreamEvent struct {
	Type StreamEventType
	// For text/text_delta
	// If the event is a text delta, Text contains the delta text.
	// If the event is text complete, Text contains the full text.
	Text string

	// For tool_call/tool_call_delta
	// ToolCall contains tool call info (ID, Name for delta; full for complete).
	ToolCall *ToolCall

	// For thinking/thinking_delta
	// Thinking contains thinking content (for thinking_complete).
	Thinking *Thinking // Thinking content (for thinking_complete)

	// For usage/usage_delta
	// Usage contains token usage information.
	Usage *Usage // Token usage

	Error error
}

type Thinking struct {
	Content   string
	Signature string
}

// StreamEventType identifies the kind of streaming event.
type StreamEventType string

const (
	// StreamEventTextDelta indicates a text delta.
	StreamEventTextDelta StreamEventType = "text_delta"
	// StreamEventTextComplete indicates text is complete. Delta contains full text.
	StreamEventTextComplete StreamEventType = "text_complete"
	// StreamEventThinkingDelta indicates a thinking delta.
	StreamEventThinkingDelta StreamEventType = "thinking_delta"
	// StreamEventThinkingComplete indicates thinking is complete.
	// Thinking contains full thinking text, ToolUse.ID contains signature.
	StreamEventThinkingComplete StreamEventType = "thinking_complete"
	// StreamEventToolCallDelta indicates a tool call delta.
	// ToolUse.ID identifies the tool call, ToolUse.Name is tool name.
	// Delta contains incremental JSON fragment (not valid JSON until complete).
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	// StreamEventToolCallComplete indicates a tool call is complete.
	// ToolUse.Input contains valid JSON arguments.
	StreamEventToolCallComplete StreamEventType = "tool_call_complete"
	// StreamEventUsage indicates token usage information.
	StreamEventUsage StreamEventType = "usage"
	// StreamEventDone indicates the stream has completed.
	StreamEventDone StreamEventType = "done"
	// StreamEventError indicates an error occurred.
	StreamEventError StreamEventType = "error"
)

// StopReason describes why the LLM stopped generating.
type StopReason string

const (
	// StopReasonEndTurn indicates normal completion.
	StopReasonEndTurn StopReason = "end_turn"
	// StopReasonToolUse indicates the model wants to call a tool.
	StopReasonToolUse StopReason = "tool_use"
	// StopReasonMaxTokens indicates the response was truncated due to length.
	StopReasonMaxTokens StopReason = "max_tokens"
	// StopReasonError indicates an error stopped generation.
	StopReasonError StopReason = "error"
	// StopReasonAborted indicates the request was cancelled.
	StopReasonAborted StopReason = "aborted"
)

// Usage tracks token consumption for a response.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Response represents an LLM response.
type Response struct {
	Messages   []Message
	StopReason StopReason
	Usage      Usage
}

// Provider defines the LLM provider interface.
type Provider interface {
	// Chat sends messages to the LLM and returns a response.
	Chat(ctx context.Context, model string, tools []ToolDefinition, messages []Message) (Response, error)

	// ChatStream sends messages and returns a streaming response channel.
	ChatStream(ctx context.Context, model string, tools []ToolDefinition, messages []Message) (<-chan StreamEvent, error)
}
