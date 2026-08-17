package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Tool represents a callable or provider-native model tool.
type Tool struct {
	// Type is the type of the tool.
	// Common values include "function", provider-native search/generation types,
	// and the Responses-specific "responses_custom_tool",
	// "responses_tool_search", and "responses_opaque_tool" variants.
	Type string `json:"type"`

	// Function is the function definition, will be used when Type is "function".
	Function Function `json:"function"`

	// ImageGeneration is the image generation definition, will be used when Type is "image_generation".
	ImageGeneration *ImageGeneration `json:"image_generation,omitempty"`

	// WebSearch is the web search definition, will be used when Type is "web_search".
	WebSearch *WebSearch `json:"web_search,omitempty"`

	// Google contains Google/Gemini-specific grounding tools.
	// This namespace isolates Google's tools from other providers.
	Google *GoogleTools `json:"google,omitempty"`

	// ResponseCustomTool is the custom tool definition for OpenAI Responses API.
	// Will be used when Type is "responses_custom_tool".
	ResponseCustomTool *ResponseCustomTool `json:"response_custom_tool,omitempty"`

	// ResponseToolSearch is the client-executed tool search definition for OpenAI Responses API.
	// Will be used when Type is "responses_tool_search".
	ResponseToolSearch *ResponseToolSearch `json:"response_tool_search,omitempty"`

	// ResponseOpaqueTool identifies a Responses tool that is retained for
	// same-protocol replay but cannot be translated without an executor/codec.
	// Will be used when Type is "responses_opaque_tool".
	ResponseOpaqueTool *ResponseOpaqueTool `json:"response_opaque_tool,omitempty"`

	// CacheControl is serialized in the unified LLM JSON; provider transformers
	// decide whether to emit it upstream.
	CacheControl *CacheControl `json:"cache_control,omitempty"`

	// ResponsesOrigin marks tools whose original Responses declaration is replayed
	// from a raw top-level or position-sensitive input fragment.
	ResponsesOrigin string `json:"-"`

	// ResponsesSourceType retains the original type of a client-executed,
	// function-like Responses tool after it is promoted to the common function IR.
	ResponsesSourceType string `json:"-"`

	// ResponsesRawID identifies the exact raw declaration that produced this tool.
	// It prevents duplicate declarations with equal common-IR fields from being
	// confused during same-protocol replay.
	ResponsesRawID string `json:"-"`

	// ResponsesOriginCallID associates tools promoted from a tool_search_output
	// item with the call whose output declared them.
	ResponsesOriginCallID string `json:"-"`

	// ResponsesNamespaceDescription preserves the description of the Responses
	// namespace wrapper that contained this flattened member tool.
	ResponsesNamespaceDescription string `json:"-"`
}

// Function represents a function definition.
type Function struct {
	Name string `json:"name"`
	// Namespace is populated when a Responses namespace function is flattened for other APIs.
	Namespace   string          `json:"namespace,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	// ParametersJsonSchema is the newer Gemini format that supports full JSON Schema Draft 2020-12
	// including const, enum, and other advanced features. This field is mutually exclusive with Parameters.
	ParametersJsonSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
	Strict               *bool           `json:"strict,omitempty"`
	// DeferLoading marks a Responses function as discoverable through tool search
	// instead of exposing it in the model's initial callable catalog.
	DeferLoading bool `json:"defer_loading,omitempty"`
}

const namespaceFunctionSeparator = "__"

// JoinNamespaceFunctionName returns the protocol's flattened namespace tool
// name. An empty namespace denotes a plain function and is returned unchanged.
func JoinNamespaceFunctionName(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + namespaceFunctionSeparator + name
}

// NamespaceFunctionMemberName validates a flattened function name and returns
// the name without its namespace prefix.
func NamespaceFunctionMemberName(function Function) (string, error) {
	return ValidateNamespaceFunctionName(function.Namespace, function.Name)
}

// ValidateNamespaceFunctionName checks that name is the exact flattening of
// namespace and member and returns the member name.
func ValidateNamespaceFunctionName(namespace, name string) (string, error) {
	if namespace == "" {
		return "", fmt.Errorf("invalid_namespace_tool: namespace is required")
	}
	member := strings.TrimPrefix(name, namespace+namespaceFunctionSeparator)
	if name == "" || member == name || member == "" {
		prefix := namespace + namespaceFunctionSeparator
		return "", fmt.Errorf(
			"invalid_namespace_tool: function %q in namespace %q must use flattened name %q",
			name, namespace, prefix+"<name>",
		)
	}
	return member, nil
}

// SplitNamespaceFunctionName splits a flattened name at its first namespace
// separator. Names without a separator are invalid namespace functions.
func SplitNamespaceFunctionName(name string) (namespace, member string, err error) {
	namespace, member, found := strings.Cut(name, namespaceFunctionSeparator)
	if !found || namespace == "" || member == "" {
		return "", "", fmt.Errorf("invalid_namespace_tool: function %q must use flattened name <namespace>__<name>", name)
	}
	return namespace, member, nil
}

// FunctionCall represents a function call (deprecated).
type FunctionCall struct {
	// The name of the function to call.
	Name string `json:"name"`

	// The namespace that owns the function, for OpenAI Responses namespace tools.
	Namespace string `json:"namespace,omitempty"`

	// The arguments to call the function with, as generated by the model in JSON
	// format. Note that the model does not always generate valid JSON, and may
	// hallucinate parameters not defined by your function schema. Validate the
	// arguments in your code before calling your function.
	Arguments string `json:"arguments"`
}

// ToolCall represents a tool call in the response.
type ToolCall struct {
	ID string `json:"id,omitempty"`

	// Type identifies a function call or a Responses-specific custom/tool-search call.
	// Supported common-model values are "function", "responses_custom_tool",
	// and "responses_tool_search".
	Type string `json:"type,omitempty"`

	Function FunctionCall `json:"function"`

	// ResponseCustomToolCall holds the custom tool call data for OpenAI Responses API.
	// Will be used when Type is "responses_custom_tool".
	ResponseCustomToolCall *ResponseCustomToolCall `json:"response_custom_tool_call,omitempty"`

	// ResponseToolSearchCall holds a client-executed Responses tool search call.
	// Will be used when Type is "responses_tool_search".
	ResponseToolSearchCall *ResponseToolSearchCall `json:"response_tool_search_call,omitempty"`

	// Index is the index of the tool call in the list of tool calls.
	// Cannot use omitempty, as an index of 0 would be omitted, which can break consumers.
	Index int `json:"index"`

	// CacheControl is used for provider-specific cache control (e.g., Anthropic).
	CacheControl *CacheControl `json:"cache_control,omitempty"`

	// TransformerMetadata is used for provider-specific metadata (e.g., Gemini).
	TransformerMetadata map[string]any `json:"transformer_metadata,omitempty"`
}

type ToolFunction struct {
	Name string `json:"name"`
}

// ToolChoice represents the tool choice parameter for function calling.
//
// Tool choice can be a string or a struct.
type ToolChoice struct {
	ToolChoice      *string          `json:"tool_choice,omitempty"`
	NamedToolChoice *NamedToolChoice `json:"named_tool_choice,omitempty"`
	AllowedTools    []ToolOption     `json:"allowed_tools,omitempty"`
	AllowedToolsSet bool             `json:"-"`
}

type ToolOption struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type NamedToolChoice struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

func (t ToolChoice) MarshalJSON() ([]byte, error) {
	if t.AllowedToolsSet {
		type allowedToolChoice struct {
			Mode  *string      `json:"mode,omitempty"`
			Tools []ToolOption `json:"tools"`
		}
		tools := t.AllowedTools
		if tools == nil {
			tools = []ToolOption{}
		}
		return json.Marshal(allowedToolChoice{Mode: t.ToolChoice, Tools: tools})
	}
	if t.ToolChoice != nil {
		return json.Marshal(t.ToolChoice)
	}

	return json.Marshal(t.NamedToolChoice)
}

func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	var str string

	err := json.Unmarshal(data, &str)
	if err == nil {
		*t = ToolChoice{ToolChoice: &str}
		return nil
	}

	var allowed struct {
		Mode  *string         `json:"mode,omitempty"`
		Tools json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(data, &allowed); err == nil && len(allowed.Tools) > 0 {
		if bytes.Equal(bytes.TrimSpace(allowed.Tools), []byte("null")) {
			return errors.New("invalid tool choice: tools must not be null")
		}
		var toolOptions []ToolOption
		if err := json.Unmarshal(allowed.Tools, &toolOptions); err != nil {
			return fmt.Errorf("invalid tool choice tools: %w", err)
		}
		*t = ToolChoice{
			ToolChoice: allowed.Mode, AllowedTools: toolOptions, AllowedToolsSet: true,
		}
		return nil
	}

	var named NamedToolChoice

	err = json.Unmarshal(data, &named)
	if err == nil {
		*t = ToolChoice{NamedToolChoice: &named}
		return nil
	}

	return errors.New("invalid tool choice type")
}

// ImageGeneration is a permissive structure to carry image generation tool
// parameters. It mirrors the OpenRouter/OpenAI Responses API fields we care
// about, but is intentionally loose to allow forward-compatibility.
type ImageGeneration struct {
	Model string `json:"model,omitempty"`
	// One of opaque, transparent.
	Background     string         `json:"background,omitempty"`
	InputFidelity  string         `json:"input_fidelity,omitempty"`
	InputImageMask map[string]any `json:"input_image_mask,omitempty"`
	// One of low, auto.
	Moderation string `json:"moderation,omitempty"`
	// The compression level (0-100%) for the generated images. Default: 100.
	OutputCompression *int64 `json:"output_compression,omitempty"`
	// One of png, webp, or jpeg. Default: png.
	OutputFormat string `json:"output_format,omitempty"`
	// The number of images to generate. Default: 1.
	PartialImages  *int64 `json:"partial_images,omitempty"`
	N              *int64 `json:"n,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	// The quality of the image that will be generated.
	// auto (default value) will automatically select the best quality for the given model.
	// high, medium and low are supported for gpt-image-1.
	// hd and standard are supported for dall-e-3.
	// standard is the only option for dall-e-2.
	Quality string `json:"quality,omitempty"`
	// One of 256x256, 512x512, or 1024x1024. Default: 1024x1024.
	Size  string `json:"size,omitempty"`
	Style string `json:"style,omitempty"`

	// Whether to add a watermark to the generated image. Default: false.
	// It only works for the models support watermark, it will be ignored otherwise.
	Watermark bool `json:"watermark,omitempty"`
}

// GoogleTools contains Google/Gemini-specific grounding tools.
// This namespace isolates Google's tools from other providers,
// allowing for provider-specific implementations without naming conflicts.
type GoogleTools struct {
	// Search enables Google Search grounding for real-time web searches.
	Search *GoogleSearch `json:"search,omitempty"`
	// CodeExecution enables code execution as part of generation.
	CodeExecution *GoogleCodeExecution `json:"code_execution,omitempty"`
	// UrlContext enables URL context grounding for Gemini 2.0+.
	UrlContext *GoogleUrlContext `json:"url_context,omitempty"`
}

// GoogleSearch represents Google Search grounding tool for Gemini.
// This enables the model to perform real-time web searches.
type GoogleSearch struct{}

// GoogleCodeExecution represents code execution tool for Gemini.
// This enables the model to execute code as part of generation.
type GoogleCodeExecution struct{}

// GoogleUrlContext represents URL context grounding tool for Gemini 2.0+.
// This allows the model to fetch and process content from specified URLs.
type GoogleUrlContext struct{}

// ContainsGoogleNativeTools checks if the tools slice contains any Google native tools.
// Google native tools include GoogleSearch, GoogleCodeExecution, and GoogleUrlContext.
// These tools are only supported by native Gemini API format (gemini/gemini_vertex),
// not by OpenAI-compatible endpoints (gemini_openai).
func ContainsGoogleNativeTools(tools []Tool) bool {
	return slices.ContainsFunc(tools, IsGoogleNativeTool)
}

// IsGoogleNativeTool checks if a single tool is a Google native tool.
// Google native tools follow the naming convention "google_*".
func IsGoogleNativeTool(tool Tool) bool {
	return strings.HasPrefix(tool.Type, "google_")
}

// FilterGoogleNativeTools removes Google native tools from the tools slice.
// This is useful as a fallback when routing to channels that don't support native tools.
func FilterGoogleNativeTools(tools []Tool) []Tool {
	if len(tools) == 0 {
		return tools
	}

	filtered := make([]Tool, 0, len(tools))

	for _, tool := range tools {
		if !IsGoogleNativeTool(tool) {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

type WebSearch struct {
	MaxUses        *int64                    `json:"max_uses,omitempty"`
	Strict         *bool                     `json:"strict,omitempty"`
	AllowedDomains []string                  `json:"allowed_domains,omitzero"`
	BlockedDomains []string                  `json:"blocked_domains,omitzero"`
	UserLocation   WebSearchToolUserLocation `json:"user_location,omitzero"`
}

type WebSearchToolUserLocation struct {
	// The city of the user.
	City string `json:"city,omitempty"`
	// The two letter
	// [ISO country code](https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2) of the
	// user.
	Country string `json:"country,omitempty"`
	// The region of the user.
	Region string `json:"region,omitempty"`
	// The [IANA timezone](https://nodatime.org/TimeZones) of the user.
	Timezone string `json:"timezone,omitempty"`
	// This field can be elided, and will marshal its zero value as "approximate".
	Type string `json:"type"`
}

// ResponseCustomTool represents a custom tool definition for the OpenAI Responses API.
// Custom tools use freeform input (not JSON arguments) and a grammar-based format definition.
// This is a Responses API-specific tool type.
type ResponseCustomTool struct {
	// Name is the name of the custom tool.
	Name string `json:"name"`
	// Namespace is an internal identity for custom tools declared inside a
	// Responses namespace. Custom-tool calls do not serialize this field.
	Namespace string `json:"-"`
	// Description is the description of the custom tool.
	Description string `json:"description,omitempty"`
	// Format defines the grammar-based format for the custom tool's input.
	Format *ResponseCustomToolFormat `json:"format,omitempty"`
}

// ResponseCustomToolFormat represents the format definition for a custom tool.
type ResponseCustomToolFormat struct {
	// Type is the format type, e.g. "grammar".
	Type string `json:"type"`
	// Syntax is the grammar syntax, e.g. "lark".
	Syntax string `json:"syntax,omitempty"`
	// Definition is the grammar definition string.
	Definition string `json:"definition,omitempty"`
}

// ResponseCustomToolCall represents a custom tool call from the OpenAI Responses API.
// Unlike function calls which use JSON arguments, custom tool calls use freeform input text.
type ResponseCustomToolCall struct {
	// CallID is the identifier used to map this custom tool call to a tool call output.
	CallID string `json:"call_id"`
	// Name is the name of the custom tool being called.
	Name string `json:"name"`
	// Namespace is the namespace that owns the custom tool being called.
	Namespace string `json:"namespace,omitempty"`
	// Input is the freeform input for the custom tool call generated by the model.
	Input string `json:"input"`
}

// ResponseToolSearch represents a client-executed Responses tool search definition.
type ResponseToolSearch struct {
	Execution   string          `json:"execution,omitempty"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ResponseToolSearchCall represents a client-executed Responses tool search call.
type ResponseToolSearchCall struct {
	CallID    string `json:"call_id"`
	Execution string `json:"execution,omitempty"`
	Arguments string `json:"arguments"`
}

// ResponseOpaqueTool describes an unsupported Responses tool without
// inventing Chat execution semantics for it.
type ResponseOpaqueTool struct {
	SourceType  string `json:"source_type"`
	Name        string `json:"name,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Execution   string `json:"execution,omitempty"`
	Description string `json:"description,omitempty"`
}
