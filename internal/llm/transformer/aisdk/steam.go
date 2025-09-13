package aisdk

import (
	"encoding/json"
)

// StreamEvent represents a unified structure for all AI SDK data stream protocol stream events.
// Different event types use different fields based on their specific requirements.
type StreamEvent struct {
	// Common field for all parts
	// Available values:
	// "start",
	// "start-step",
	// "text-start", "text-delta", "text-end",
	// "reasoning-start", "reasoning-delta", "reasoning-end",
	// "source-url","source-document","file"
	// "tool-input-start", "tool-input-delta", "tool-input-available", "tool-output-available",
	// "finish-step",
	// "finish",
	// "error"
	// Other custom data types e.g. "weather-data", Custom data parts allow streaming of arbitrary structured data with type-specific handling.
	Type string `json:"type"`

	// Message-related fields
	MessageID string `json:"messageId,omitempty"` // Used by: start parts

	// Text streaming fields
	ID    string `json:"id,omitempty"`    // Used by: text-start, text-delta, text-end, reasoning-start, reasoning-delta, reasoning-end parts
	Delta string `json:"delta,omitempty"` // Used by: text-delta, reasoning-delta parts for incremental content

	// Tool call fields
	ToolCallID     string          `json:"toolCallId,omitempty"`     // Used by: tool-input-start, tool-input-delta, tool-input-available, tool-output-available parts
	ToolName       string          `json:"toolName,omitempty"`       // Used by: tool-input-start, tool-input-available parts
	InputTextDelta string          `json:"inputTextDelta,omitempty"` // Used by: tool-input-delta parts for streaming tool arguments
	Input          json.RawMessage `json:"input,omitempty"`          // Used by: tool-input-available parts for complete tool arguments
	Output         json.RawMessage `json:"output,omitempty"`         // Used by: tool-output-available parts for tool execution results

	// Error fields
	ErrorText string `json:"errorText,omitempty"` // Used by: error parts

	SourceID string `json:"sourceId,omitempty"` // Used by: source-url,source-document parts
	URL      string `json:"url,omitempty"`      // Used by: source-url, file parts
	// Media type of the source.
	// image/jpeg, image/png, etc.
	MediaType string `json:"mediaType,omitempty"` // Used by: source-document,file parts
	Title     string `json:"title,omitempty"`     // Used by: source-document parts

	Data []byte `json:"data,omitempty"` // Used by: data parts
}
