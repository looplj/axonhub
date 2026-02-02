// Package responses implements the response API for OpenAI.
package responses

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

// ImageGeneration is a permissive structure to carry image generation tool
// parameters. It mirrors the OpenRouter/OpenAI Responses API fields we care
// about, but is intentionally loose to allow forward-compatibility.
type ImageGeneration struct {
	llm.ImageGeneration

	Type  string `json:"type"`
	Model string `json:"model"`
}

type Tool struct {
	// Any of "function", "image_generation" nil.
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`

	// This field is from variant [FunctionTool].
	Parameters map[string]any `json:"parameters,omitempty"`
	// This field is from variant [FunctionTool].
	Strict *bool `json:"strict,omitempty"`

	// This field is for ImageGeneration
	Background string `json:"background,omitempty"`
	// This field is for ImageGeneration
	InputFidelity string `json:"input_fidelity,omitempty"`
	// This field is for ImageGeneration
	Model string `json:"model,omitempty"`
	// This field is for ImageGeneration
	Moderation string `json:"moderation,omitempty"`
	// This field is for ImageGeneration
	OutputCompression *int64 `json:"output_compression,omitempty"`
	// This field is for ImageGeneration
	OutputFormat string `json:"output_format,omitempty"`
	// This field is for ImageGeneration
	PartialImages *int64 `json:"partial_images,omitempty"`
	// This field is for ImageGeneration
	Quality string `json:"quality,omitempty"`
	// This field is for ImageGeneration
	Size string `json:"size,omitempty"`
}

// Request is a struct for OpenAI Responses API creation.
// Reference: github.com/openai/openai-go/v2/responses.ResponseNewParams.
type Request struct {
	Model string `json:"model"`

	// A system (or developer) message inserted into the model's context.
	Instructions string `json:"instructions,omitempty"`

	Temperature *float64 `json:"temperature,omitempty"`

	// Input can be a string prompt or an array of input items.
	Input Input `json:"input"`
	// Tools includes the function/image_generation tools.
	Tools []Tool `json:"tools,omitempty"`
	// Parallel tool calls preference.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`

	// Whether to run the model response in the background.
	Background *bool `json:"background,omitempty"`

	Stream           *bool             `json:"stream,omitempty"`
	Store            *bool             `json:"store,omitempty"`
	ServiceTier      *string           `json:"service_tier,omitempty"`
	SafetyIdentifier *string           `json:"safety_identifier,omitempty"`
	User             *string           `json:"user,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	MaxOutputTokens  *int64            `json:"max_output_tokens,omitempty"`
	MaxToolCalls     *int64            `json:"max_tool_calls,omitempty"`
	Text             *TextOptions      `json:"text,omitempty"`

	// Specify additional output data to include in the model response.
	// e.g., "file_search_call.results", "message.input_image.image_url", "reasoning.encrypted_content"
	Include []string `json:"include,omitempty"`

	// The unique ID of the previous response to the model for multi-turn conversations.
	PreviousResponseID *string `json:"previous_response_id,omitempty"`

	// Reference to a prompt template and its variables.
	// TODO
	// Prompt *Prompt `json:"prompt,omitempty"`

	// Used by OpenAI to cache responses for similar requests.
	PromptCacheKey *string `json:"prompt_cache_key,omitempty"`

	// The retention policy for the prompt cache. Any of "in-memory", "24h".
	PromptCacheRetention *string `json:"prompt_cache_retention,omitempty"`

	// Configuration options for reasoning models.
	Reasoning *Reasoning `json:"reasoning,omitempty"`

	// Options for streaming responses.
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`

	// How the model should select which tool to use.
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	// The truncation strategy. Any of "auto", "disabled".
	Truncation *string `json:"truncation,omitempty"`

	// The conversation that this response belongs to.
	// Conversation *Conversation `json:"conversation,omitempty"`

	// An integer between 0 and 20 specifying the number of most likely tokens to return.
	TopLogprobs *int64 `json:"top_logprobs,omitempty"`

	// Nucleus sampling parameter.
	TopP *float64 `json:"top_p,omitempty"`
}

// Prompt represents a reference to a prompt template.
type Prompt struct {
	ID        string            `json:"id"`
	Version   *string           `json:"version,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

// Reasoning represents configuration options for reasoning models.
type Reasoning struct {
	// The effort level for reasoning. Any of "low", "medium", "high".
	Effort string `json:"effort,omitempty"`
	// Whether to generate a summary of the reasoning. Any of "auto", "concise", "detailed".
	GenerateSummary string `json:"generate_summary,omitempty"`
	// The summary type. Any of "auto", "concise", "detailed".
	Summary string `json:"summary,omitempty"`
	// Maximum number of reasoning tokens.
	MaxTokens *int64 `json:"max_tokens,omitempty"`
}

// StreamOptions represents options for streaming responses.
type StreamOptions struct {
	// When true, stream obfuscation will be enabled.
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty"`
}

// ToolChoice represents how the model should select which tool to use (for requests).
type ToolChoice struct {
	// Mode can be "none", "auto", "required".
	Mode *string `json:"mode,omitempty"`
	// Type for specific tool choice. Any of "function", "file_search", "web_search", "shell" etc.
	Type *string `json:"type,omitempty"`
	// Name of the function for function tool choice.
	Name *string `json:"name,omitempty"`

	// Allow multiple tools to be selected.
	Tools []ToolOption `json:"tools,omitempty"`
}

type ToolOption struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type ToolChoiceAlias ToolChoice

func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	mode, err := xjson.To[string](data)
	if err == nil {
		t.Mode = &mode
		return nil
	}

	tc, err := xjson.To[ToolChoiceAlias](data)
	if err == nil {
		*t = ToolChoice(tc)
		return nil
	}

	return errors.New("invalid tool choice type")
}

func (t *ToolChoice) MarshalJSON() ([]byte, error) {
	if t.Mode != nil && *t.Mode == "auto" {
		return json.Marshal("auto")
	}

	// For other cases, marshal as object
	type Alias ToolChoice

	return json.Marshal(&struct {
		Mode  *string      `json:"mode,omitempty"`
		Type  *string      `json:"type,omitempty"`
		Name  *string      `json:"name,omitempty"`
		Tools []ToolOption `json:"tools,omitempty"`
	}{
		Mode:  t.Mode,
		Type:  t.Type,
		Name:  t.Name,
		Tools: t.Tools,
	})
}

// ResponseToolChoice represents tool_choice in responses, which can be a string or object.
type ResponseToolChoice struct {
	// String value when tool_choice is a simple string like "auto", "none", "required".
	StringValue string
	// Object value when tool_choice is an object.
	ObjectValue *ToolChoice
}

func (r *ResponseToolChoice) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		r.StringValue = str
		return nil
	}

	// Try to unmarshal as object
	var obj ToolChoice
	if err := json.Unmarshal(data, &obj); err == nil {
		r.ObjectValue = &obj
		return nil
	}

	return fmt.Errorf("tool_choice must be a string or object")
}

func (r ResponseToolChoice) MarshalJSON() ([]byte, error) {
	if r.StringValue != "" {
		return json.Marshal(r.StringValue)
	}

	if r.ObjectValue != nil {
		return json.Marshal(r.ObjectValue)
	}

	return []byte("null"), nil
}

// Conversation represents the conversation context for requests.
type Conversation struct {
	// The conversation ID.
	ID *string `json:"id,omitempty"`
}

// ResponseConversation represents the conversation context in responses.
type ResponseConversation struct {
	// The unique ID of the conversation.
	ID string `json:"id"`
}

// ResponseIncompleteDetails contains details about why the response is incomplete.
type ResponseIncompleteDetails struct {
	// The reason why the response is incomplete.
	// Any of "max_output_tokens", "content_filter".
	Reason string `json:"reason"`
}

// ResponseReasoning represents reasoning configuration in responses.
type ResponseReasoning struct {
	// Constrains effort on reasoning for reasoning models.
	// Any of "none", "minimal", "low", "medium", "high", "xhigh".
	Effort string `json:"effort,omitempty"`
	// A summary of the reasoning performed by the model.
	// Any of "auto", "concise", "detailed".
	Summary string `json:"summary,omitempty"`
	// Deprecated: use Summary instead.
	GenerateSummary string `json:"generate_summary,omitempty"`
}

type TextOptions struct {
	// An object specifying the format that the model must output.
	// Configuring { "type": "json_schema" } enables Structured Outputs, which ensures the model will match your supplied JSON schema. Learn more in the Structured Outputs
	// guide.
	// The default format is { "type": "text" } with no additional options.
	// { "type": "json_object" } is also supported.
	Format *TextFormat `json:"format,omitempty"`

	// The verbosity of the response. Any of "low", "medium", "high".
	Verbosity *string `json:"verbosity,omitempty"`
}

// TextFormat specifies the format that the model must output.
type TextFormat struct {
	// The type of the format. Any of "text", "json_object", "json_schema".
	Type string `json:"type,omitempty"`
	// The name of the schema (for json_schema type).
	Name string `json:"name,omitempty"`
	// The JSON schema (for json_schema type).
	Schema json.RawMessage `json:"schema,omitempty"`
}

type Input struct {
	Text  *string
	Items []Item
}

func (i *Input) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		i.Text = &text
		return nil
	}

	var items []Item
	if err := json.Unmarshal(data, &items); err == nil {
		i.Items = items
		return nil
	}

	return fmt.Errorf("invalid input: %w", transformer.ErrInvalidRequest)
}

func (i Input) MarshalJSON() ([]byte, error) {
	if i.Text != nil {
		return json.Marshal(i.Text)
	}

	return json.Marshal(i.Items)
}

type Annotation struct{}

// Item is a unified structure for both input and output items in the Responses API.
// This follows the openai-go pattern where input and output items share the same structure.
// Reference: github.com/openai/openai-go/v3/responses.ResponseOutputItemUnion.
type Item struct {
	// The ID of the item, generated by the server.
	ID string `json:"id,omitempty"`

	// Any of "message", "input_text", "input_image", "input_audio", "output_text",
	// "function_call", "function_call_output", "image_generation_call", "reasoning".
	Type string `json:"type,omitempty"`

	// The annotations of the text output.
	Annotations []Annotation `json:"annotations,omitzero"`

	// Any of "system", "user", "assistant", "developer".
	Role string `json:"role,omitempty"`

	// The content of the message. Can be string or array of content items.
	// For input items: string or array of input items.
	// For output message items: array of ContentItem (stored as []Item with Type="output_text").
	Content *Input `json:"content,omitempty"`

	// Status of the item.
	// Any of "in_progress", "completed", "incomplete".
	Status *string `json:"status,omitempty"`

	// The URL of the image url or base64 encoded image, for input_image type.
	ImageURL *string `json:"image_url,omitempty"`

	// The detail of the image. high, low, or auto, for input_image type.
	Detail *string `json:"detail,omitempty"`

	// Text for output_text/input_text type.
	Text *string `json:"text,omitempty"`

	// Image generation fields

	// Background for image generated, e.g: opaque
	Background *string `json:"background,omitempty"`
	// Output format for image generated, e.g: png
	OutputFormat *string `json:"output_format,omitempty"`
	// Quality for image generated, e.g: low
	Quality *string `json:"quality,omitempty"`
	// Size for image generated, e.g: 1024x1024
	Size *string `json:"size,omitempty"`

	// Result for image_generation_call type.
	Result *string `json:"result,omitempty"`

	// Function call fields
	CallID    string `json:"call_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`

	// Output for function_call_output type.
	Output *Input `json:"output,omitempty"`

	// Reasoning fields (for type="reasoning")
	// Reasoning summary content - array of summary text items.
	Summary []ReasoningSummary `json:"summary,omitempty"`
	// Reasoning text content - array of reasoning text items.
	ReasoningContent []ReasoningContent `json:"reasoning_content,omitempty"`
	// The encrypted content of the reasoning item.
	EncryptedContent *string `json:"encrypted_content,omitempty"`
}

// MarshalJSON omits summary for non-reasoning items and forces an empty array for reasoning items.
func (item Item) MarshalJSON() ([]byte, error) {
	type itemAlias Item

	if item.Type != "reasoning" {
		item.Summary = nil
	} else if item.Summary == nil {
		item.Summary = []ReasoningSummary{}
	}

	if item.Type == "function_call" {
		type functionCallItem struct {
			itemAlias
			Arguments string `json:"arguments"`
		}

		return json.Marshal(functionCallItem{
			itemAlias: itemAlias(item),
			Arguments: item.Arguments,
		})
	}

	return json.Marshal(itemAlias(item))
}

// isOutputMessageContent checks if Content.Items contains output message content items.
func (item Item) isOutputMessageContent() bool {
	if item.Content == nil || len(item.Content.Items) == 0 {
		return false
	}

	for _, ci := range item.Content.Items {
		if ci.Type == "output_text" {
			return true
		}
	}

	return false
}

// GetContentItems returns the content items as []ContentItem for output message items.
// This is a helper method to access content items stored in Content.Items.
func (item Item) GetContentItems() []ContentItem {
	if item.Content == nil || len(item.Content.Items) == 0 {
		return nil
	}

	result := make([]ContentItem, 0, len(item.Content.Items))
	for _, ci := range item.Content.Items {
		text := ""
		if ci.Text != nil {
			text = *ci.Text
		}

		result = append(result, ContentItem{
			Type: ci.Type,
			Text: text,
		})
	}

	return result
}

// SetContentItems sets the content items from []ContentItem for output message items.
// This is a helper method to store content items in Content.Items.
func (item *Item) SetContentItems(items []ContentItem) {
	if len(items) == 0 {
		return
	}

	contentItems := make([]Item, 0, len(items))
	for _, ci := range items {
		contentItems = append(contentItems, Item{
			Type: ci.Type,
			Text: &ci.Text,
		})
	}

	item.Content = &Input{Items: contentItems}
}

// ReasoningSummary represents a summary text from the model.
type ReasoningSummary struct {
	// A summary of the reasoning output from the model.
	Text string `json:"text"`
	// The type of the object. Always "summary_text".
	Type string `json:"type"`
}

// ReasoningContent represents reasoning text from the model.
type ReasoningContent struct {
	// The reasoning text from the model.
	Text string `json:"text"`
	// The type of the reasoning text. Always "reasoning_text".
	Type string `json:"type"`
}

type Response struct {
	// The object type of this resource - always set to "response".
	Object string `json:"object"`
	// Unique identifier for this Response.
	ID string `json:"id"`
	// An error object returned when the model fails to generate a Response.
	Error *Error `json:"error,omitempty"`
	// Unix timestamp (in seconds) of when this Response was created.
	CreatedAt int64 `json:"created_at"`
	// Model ID used to generate the response.
	Model string `json:"model"`
	// An array of content items generated by the model.
	Output []Item `json:"output"`

	// Status of the response.
	// Any of "completed", "failed", "in_progress", "canceled", "queued", or "incomplete".
	Status *string `json:"status,omitempty"`

	// Details about why the response is incomplete.
	IncompleteDetails *ResponseIncompleteDetails `json:"incomplete_details,omitempty"`

	// A system (or developer) message inserted into the model's context.
	Instructions *string `json:"instructions,omitempty"`

	// Set of key-value pairs that can be attached to an object.
	Metadata map[string]string `json:"metadata,omitempty"`

	// Whether to allow the model to run tool calls in parallel.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`

	// What sampling temperature to use, between 0 and 2.
	Temperature *float64 `json:"temperature,omitempty"`

	// How the model should select which tool (or tools) to use.
	ToolChoice *ResponseToolChoice `json:"tool_choice,omitempty"`

	// An array of tools the model may call while generating a response.
	Tools []Tool `json:"tools,omitempty"`

	// An alternative to sampling with temperature, called nucleus sampling.
	TopP *float64 `json:"top_p,omitempty"`

	// Whether to run the model response in the background.
	Background *bool `json:"background,omitempty"`

	// The conversation that this response belongs to.
	Conversation *ResponseConversation `json:"conversation,omitempty"`

	// An upper bound for the number of tokens that can be generated for a response.
	MaxOutputTokens *int64 `json:"max_output_tokens,omitempty"`

	// The maximum number of total calls to built-in tools that can be processed.
	MaxToolCalls *int64 `json:"max_tool_calls,omitempty"`

	// The unique ID of the previous response to the model.
	PreviousResponseID *string `json:"previous_response_id,omitempty"`

	// Reference to a prompt template and its variables.
	Prompt *Prompt `json:"prompt,omitempty"`

	// Used by OpenAI to cache responses for similar requests.
	PromptCacheKey *string `json:"prompt_cache_key,omitempty"`

	// The retention policy for the prompt cache. Any of "in-memory", "24h".
	PromptCacheRetention *string `json:"prompt_cache_retention,omitempty"`

	// Configuration options for reasoning models.
	Reasoning *ResponseReasoning `json:"reasoning,omitempty"`

	// A stable identifier used to help detect users that may be violating usage policies.
	SafetyIdentifier *string `json:"safety_identifier,omitempty"`

	// Specifies the processing type used for serving the request.
	// Any of "auto", "default", "flex", "scale", "priority".
	ServiceTier *string `json:"service_tier,omitempty"`

	// Configuration options for a text response from the model.
	Text *TextOptions `json:"text,omitempty"`

	// An integer between 0 and 20 specifying the number of most likely tokens to return.
	TopLogprobs *int64 `json:"top_logprobs,omitempty"`

	// The truncation strategy to use for the model response. Any of "auto", "disabled".
	Truncation *string `json:"truncation,omitempty"`

	// Represents token usage details.
	Usage *Usage `json:"usage,omitempty"`

	// A stable identifier for your end-users (deprecated, use safety_identifier).
	User *string `json:"user,omitempty"`
}

type ContentItem struct {
	Type        string   `json:"type"`
	Text        string   `json:"text,omitempty"`
	Annotations []string `json:"annotations,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
