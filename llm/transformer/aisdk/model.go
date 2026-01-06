package aisdk

import (
	"encoding/json"
)

// Request represents the AI SDK request format.
type Request struct {
	Messages []UIMessage `json:"messages"`
	Model    string      `json:"model,omitempty"`
	Stream   *bool       `json:"stream,omitempty"`
	Tools    []Tool      `json:"tools,omitempty"`
}

// UIMessage represents a message in AI SDK format.
type UIMessage struct {
	ID string `json:"id,omitempty"`
	// Role of the message, e.g. "user", "assistant", "system".
	Role    string          `json:"role"`
	Content interface{}     `json:"content"` // can be string or array of content parts
	Parts   []UIMessagePart `json:"parts"`
}

// Tool represents a tool in AI SDK format.
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a tool function in AI SDK format.
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// Usage represents token usage information for AI model requests and responses.
type Usage struct {
	PromptTokens     *int64 `json:"promptTokens,omitempty"`     // Number of tokens in the input prompt
	CompletionTokens *int64 `json:"completionTokens,omitempty"` // Number of tokens in the generated completion
}

type UIMessagePart struct {
	// Type of the UI message part.
	// Supported values (mirrors TS):
	// - "text"
	// - "reasoning"
	// - "file"
	// - "source-url"
	// - "source-document"
	// - "step-start"
	// - "dynamic-tool"
	// - "tool-<NAME>"
	// - "data-<NAME>"
	// Note: The struct is unified; only a subset of fields are used depending on the type.
	Type string `json:"type"`

	// State of the part.
	// For text/reasoning: "streaming" | "done".
	// For tool/dynamic-tool: "input-streaming" | "input-available" | "output-available" | "output-error".
	State string `json:"state,omitempty"`

	// =============================
	// Text / Reasoning parts
	// =============================
	// Text content for types: "text", "reasoning".
	Text string `json:"text,omitempty"`
	// For streaming text blocks: used by "text-delta" parts to carry incremental content.
	Delta string `json:"delta,omitempty"`
	// Provider metadata for types: "text", "reasoning" (and also file/source parts below).
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`

	// =============================
	// Source URL part
	// =============================
	// For type: "source-url".
	SourceID string `json:"sourceId,omitempty"`
	URL      string `json:"url,omitempty"`
	Title    string `json:"title,omitempty"`

	// =============================
	// Source Document part
	// =============================
	// For type: "source-document".
	MediaType string `json:"mediaType,omitempty"`
	Filename  string `json:"filename,omitempty"`
	// Title is reused here as well.

	// =============================
	// File part
	// =============================
	// For type: "file".
	// Uses MediaType, Filename, URL, ProviderMetadata.

	// =============================
	// Data part (custom data types)
	// =============================
	// For type: "data-<NAME>".
	// Optional ID for the data part.
	ID string `json:"id,omitempty"`
	// Arbitrary data payload; shape depends on <NAME>.
	Data json.RawMessage `json:"data,omitempty"`

	// =============================
	// Tool parts (named tools and dynamic-tool)
	// =============================
	// Common fields for both "tool-<NAME>" and "dynamic-tool".
	ToolCallID     string          `json:"toolCallId,omitempty"`     // All tool invocations
	ToolName       string          `json:"toolName,omitempty"`       // Dynamic tool or derived name from type
	InputTextDelta string          `json:"inputTextDelta,omitempty"` // For streaming tool arguments (tool-input-delta)
	Input          interface{}     `json:"input,omitempty"`          // tool input (tool-input-available)
	Output         interface{}     `json:"output,omitempty"`         // tool output (tool-output-available)
	ErrorText      string          `json:"errorText,omitempty"`      // error text (output-error)
	RawInput       json.RawMessage `json:"rawInput,omitempty"`       // transitional raw input (output-error)
	// Whether the provider executed the tool. Only for named tools (UIToolInvocation), optional.
	ProviderExecuted *bool `json:"providerExecuted,omitempty"`
	// Provider metadata for the tool call. Present for input-available/output-* states.
	CallProviderMetadata json.RawMessage `json:"callProviderMetadata,omitempty"`
	// Indicates preliminary output for state: output-available.
	Preliminary *bool `json:"preliminary,omitempty"`
}
