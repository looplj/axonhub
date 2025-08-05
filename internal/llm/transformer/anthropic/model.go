package anthropic

import (
	"encoding/json"
	"fmt"
)

// MessageRequest represents the Anthropic Messages API request format.
type MessageRequest struct {
	MaxTokens int64          `json:"max_tokens" validate:"required,gte=1"`
	Messages  []MessageParam `json:"messages"   validate:"required"`
	Model     string         `json:"model"      validate:"required"`

	// Amount of randomness injected into the response.
	//
	// Defaults to `1.0`. Ranges from `0.0` to `1.0`. Use `temperature` closer to `0.0`
	// for analytical / multiple choice, and closer to `1.0` for creative and
	// generative tasks.
	//
	// Note that even with `temperature` of `0.0`, the results will not be fully
	// deterministic.
	Temperature *float64 `json:"temperature,omitempty"`

	// Only sample from the top K options for each subsequent token.
	//
	// Used to remove "long tail" low probability responses.
	// [Learn more technical details here](https://towardsdatascience.com/how-to-sample-from-language-models-682bceb97277).
	//
	// Recommended for advanced use cases only. You usually only need to use
	// `temperature`.
	TopK *int64 `json:"top_k,omitempty"`

	// Use nucleus sampling.
	//
	// In nucleus sampling, we compute the cumulative distribution over all the options
	// for each subsequent token in decreasing probability order and cut it off once it
	// reaches a particular probability specified by `top_p`. You should either alter
	// `temperature` or `top_p`, but not both.
	//
	// Recommended for advanced use cases only. You usually only need to use
	// `temperature`.
	TopP *float64 `json:"top_p,omitempty"`

	// An object describing metadata about the request.
	Metadata *struct {
		UserID string `json:"user_id,omitempty"`
	} `json:"metadata,omitempty"`

	// Determines whether to use priority capacity (if available) or standard capacity
	// for this request.
	//
	// Anthropic offers different levels of service for your API requests. See
	// [service-tiers](https://docs.anthropic.com/en/api/service-tiers) for details.
	//
	// Any of "auto", "standard_only".
	ServiceTier string `json:"service_tier,omitempty"`

	// Custom text sequences that will cause the model to stop generating.
	//
	// Our models will normally stop when they have naturally completed their turn,
	// which will result in a response `stop_reason` of `"end_turn"`.
	//
	// If you want the model to stop generating when it encounters custom strings of
	// text, you can use the `stop_sequences` parameter. If the model encounters one of
	// the custom sequences, the response `stop_reason` value will be `"stop_sequence"`
	// and the response `stop_sequence` value will contain the matched stop sequence.
	StopSequences []string `json:"stop_sequences,omitempty"`

	// System is an optional system prompt.
	System *SystemPrompt `json:"system,omitempty"`

	// Thinking is an optional thinking configuration.
	Thinking *Thinking `json:"thinking,omitempty"`

	// Tools is an optional array of tools.
	Tools []Tool `json:"tools,omitempty"`
	// ToolChoice is an optional tool choice configuration.
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	// Stream is an optional flag to enable streaming.
	Stream *bool `json:"stream,omitempty"`
}

type SystemPrompt struct {
	Prompt *string `json:"prompt,omitempty"`
	// MultiplePrompts is an optional array of system prompts.
	MultiplePrompts []SystemPromptPart `json:"multiple_prompts,omitempty"`
}

func (s *SystemPrompt) MarshalJSON() ([]byte, error) {
	if s.Prompt != nil {
		return json.Marshal(s.Prompt)
	}

	if len(s.MultiplePrompts) > 0 {
		return json.Marshal(s.MultiplePrompts)
	}

	return []byte("null"), nil
}

func (s *SystemPrompt) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.Prompt = &str
		return nil
	}

	var parts []SystemPromptPart
	if err := json.Unmarshal(data, &parts); err == nil {
		s.MultiplePrompts = parts
		return nil
	}

	return fmt.Errorf("invalid system prompt format")
}

type SystemPromptPart struct {
	// Type must be "text".
	Type string `json:"type" validate:"required,oneof=text"`
	Text string `json:"text" validate:"required"`
}

type Thinking struct {
	Type         string `json:"type"          validate:"required,oneof=enabled disabled"`
	BudgetTokens int64  `json:"budget_tokens" validate:"required_if=Type enabled"`
}

type ToolChoice struct {
	Type string `json:"type" validate:"required,oneof=auto none tool any"`

	// DisableParallelToolUse is an optional flag to disable parallel tool use.
	DisableParallelToolUse *bool `json:"disable_parallel_tool_use,omitempty"`

	// Name is an optional name of the tool to use, it is required when Type is tool.
	Name *string `json:"name,omitempty" validate:"required_if=Type tool"`
}

// Tool represents a tool definition for Anthropic API.
type Tool struct {
	Type         string          `json:"type"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

type CacheControl struct {
	Type string `json:"type" validate:"required,oneof=ephemeral"`
}

// InputSchema represents the JSON schema for tool input.
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// MessageParam represents a message in Anthropic format.
type MessageParam struct {
	Role    string         `json:"role"`
	Content MessageContent `json:"content"`
}

// MessageContent supports both string and array formats.
type MessageContent struct {
	Content         *string        `json:"content,omitempty"`
	MultipleContent []ContentBlock `json:"multiple_content,omitempty"`
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	if c.Content != nil {
		return json.Marshal(c.Content)
	}

	return json.Marshal(c.MultipleContent)
}

func (c *MessageContent) UnmarshalJSON(data []byte) error {
	// Handle null values
	if string(data) == "null" {
		return fmt.Errorf("content cannot be null")
	}

	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err == nil {
		c.MultipleContent = blocks
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		c.Content = &str
		return nil
	}

	return fmt.Errorf("invalid content type")
}

// ContentBlock represents different types of content blocks.
type ContentBlock struct {
	// Any of "text", "image", "thinking", "redacted_thinking", "tool_use", "server_tool_use".
	Type string `json:"type"`

	// Text will be present if type is "text".
	Text string `json:"text,omitempty"`

	// Signature will be present if type is "thinking".
	Signature string `json:"signature,omitempty"`

	// Thinking will be present if type is "thinking".
	Thinking string `json:"thinking,omitempty"`

	// Data will be present if type is "redacted_thinking".
	Data string `json:"data,omitempty"`

	// Image will be present if type is "image".
	Source *ImageSource `json:"source,omitempty"`

	// Tool use request
	// tool_use or server_tool_use
	ID    string          `json:"id,omitempty"`
	Name  *string         `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result fields
	ToolUseID *string             `json:"tool_use_id,omitempty"`
	Content   []ToolResultContent `json:"content,omitempty"`
	IsError   *bool               `json:"is_error,omitempty"`
}

type ToolResultContent struct {
	// Type is the type of content.
	// Available values: text, image
	Type string `json:"type"`

	// Text will be present if type is "text".
	Text *string `json:"text,omitempty"`

	// Source will be present if type is "image".
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource represents image source for Anthropic.
type ImageSource struct {
	// Type is the type of image source.
	// Available values: base64, url
	Type string `json:"type"`
	// MediaType is the media type of image.
	// Available values: image/png, image/jpeg, image/gif, image/webp
	MediaType string `json:"media_type"`

	// Data is the image data.
	// If Type is base64, Data is the base64-encoded image data.
	Data string `json:"data"`

	// URL is the URL of the image.
	// It will be present if Type is url.
	URL *string `json:"url,omitempty"`
}

// StreamEvent represents events in Anthropic streaming response.
type StreamEvent struct {
	// Any of "message_start", "message_delta", "message_stop", "content_block_start",
	// "content_block_delta", "content_block_stop".
	Type string `json:"type"`

	// Message will be present if type is "message_start".
	Message *StreamMessage `json:"message,omitempty"`

	// Index will be present if type is "content_block_start" or "content_block_delta".
	Index *int64 `json:"index,omitempty"`

	// ContentBlock will be present if type is "content_block_start".
	ContentBlock *ContentBlock `json:"content_block,omitempty"`

	// Delta will be present if type is "message_delta" or "content_block_delta".
	Delta *StreamDelta `json:"delta,omitempty"`

	Usage *Usage `json:"usage,omitempty"`
}

// StreamDelta represents delta in streaming response.
type StreamDelta struct {
	// Type is the type of delta.
	// Any of "text_delta", "input_json_delta", "citations_delta", "thinking_delta",
	// "signature_delta".
	Type *string `json:"type,omitempty"`

	// Text will be present if type is "text_delta".
	Text *string `json:"text,omitempty"`

	// PartialJSON will be present if type is "input_json_delta".
	PartialJSON *string `json:"partial_json,omitempty"`

	// Thinking will be present if type is "thinking_delta".
	Thinking *string `json:"thinking,omitempty"`

	// Signature will be present if type is "signature_delta".
	Signature *string `json:"signature,omitempty"`

	// For "message_delta"
	// Any of "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn",
	// "refusal".
	StopReason *string `json:"stop_reason,omitempty"`

	// For "message_delta"
	StopSequence *string `json:"stop_sequence,omitempty"`
}

// StreamMessage represents the message part of a stream event.
type StreamMessage struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// Message represents the Anthropic Messages API response format.
type Message struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model"`
	// Any of "end_turn", "max_tokens", "stop_sequence", "tool_use", "pause_turn",
	// "refusal".
	StopReason *string `json:"stop_reason,omitempty"`
	// Which custom stop sequence was generated, if any.
	//
	// This value will be a non-null string if one of your custom stop sequences was
	// generated.
	StopSequence *string `json:"stop_sequence,omitempty"`
	Usage        *Usage  `json:"usage,omitempty"`
}

// Usage represents usage information in Anthropic format.
type Usage struct {
	InputTokens              int64  `json:"input_tokens"`
	OutputTokens             int64  `json:"output_tokens"`
	CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
	ServiceTier              string `json:"service_tier"`
}
