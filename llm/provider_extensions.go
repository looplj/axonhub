package llm

import (
	"encoding/json"

	"github.com/samber/lo"
)

// ProviderExtensions carries provider/API-format private data that should not
// be serialized through the common llm request/response JSON model.
type ProviderExtensions struct {
	OpenAIResponses *OpenAIResponsesProviderExtensions `json:"-"`
	OpenAIChat      *OpenAIChatProviderExtensions      `json:"-"`
	Anthropic       *AnthropicProviderExtensions       `json:"-"`
	Diagnostics     *DiagnosticsProviderExtensions     `json:"-"`
}

// DiagnosticsProviderExtensions carries cross-protocol conversion diagnostics.
// It is not provider-native request/response storage and must not be serialized
// to upstream providers.
type DiagnosticsProviderExtensions struct {
	LossyDowngrades []LossyDowngrade `json:"-"`
	// ResponsesLossy is a structured same-event summary for Responses-native
	// losses (counts). Formal per-field rows still live in LossyDowngrades.
	// Do not store this bag in TransformerMetadata.
	ResponsesLossy *ResponsesLossySummary `json:"-"`
}

// ResponsesLossySummary counts Responses-native losses for one conversion.
// Owned by ProviderExtensions.Diagnostics (not TransformerMetadata).
type ResponsesLossySummary struct {
	LossyDowngrade                       bool
	UnknownTopLevelFieldCount            int
	ClientMetadataCount                  int
	NamespaceToolCount                   int
	ToolSearchToolCount                  int
	UnknownToolCount                     int
	RawOnlyToolCount                     int
	AdditionalToolsCount                 int
	AdditionalToolsUnrepresentableCount  int
	ToolSearchOutputUnrepresentableCount int
	RawInputItemCount                    int
	UnknownInputItemCount                int
}

type OpenAIResponsesProviderExtensions struct {
	Request  *OpenAIResponsesRequestExtensions  `json:"-"`
	Response *OpenAIResponsesResponseExtensions `json:"-"`
}

// OpenAIResponsesResponseExtensions carries Responses-native response body
// fragments that have no stable common llm.Response semantic.
type OpenAIResponsesResponseExtensions struct {
	// Status preserves a Responses-native lifecycle value that has no shared
	// Chat finish_reason equivalent (for example queued or in_progress).
	Status            *string                    `json:"-"`
	RawTopLevelFields map[string]json.RawMessage `json:"-"`
	// RawOutputItems preserves Responses-native output items that cannot be
	// represented by the canonical response without losing their original
	// structure, ordering, or identity. They are replayed only by the Responses
	// inbound adapter at their original output[] positions.
	RawOutputItems []OpenAIResponsesRawFragment `json:"-"`
	// RawStreamEvents preserves Responses SSE events that have no canonical
	// chunk representation. The Responses stream emitter replays them only to
	// the same protocol family.
	RawStreamEvents []OpenAIResponsesRawStreamEvent `json:"-"`
}

type OpenAIResponsesRawStreamEvent struct {
	Type string          `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

// OpenAIChatProviderExtensions carries Chat Completions request fragments that
// are not modeled on the shared openai.Request / llm.Request surface.
type OpenAIChatProviderExtensions struct {
	Request *OpenAIChatRequestExtensions `json:"-"`
}

// OpenAIChatRequestExtensions holds same-protocol Chat-only top-level raw fields
// (n, prompt_cache_retention, audio, prediction, moderation, web_search_options,
// deprecated functions / function_call). Replayed only by Chat outbound merge.
type OpenAIChatRequestExtensions struct {
	// RawTopLevelFields maps wire field name → original JSON value.
	RawTopLevelFields map[string]json.RawMessage `json:"-"`
}


// AnthropicProviderExtensions carries Anthropic-native request/response/stream
// data that has no stable common llm representation.
type AnthropicProviderExtensions struct {
	Request  *AnthropicRequestExtensions  `json:"-"`
	Response *AnthropicResponseExtensions `json:"-"`
}

// AnthropicRequestExtensions carries Anthropic-native request fragments that
// have no stable common llm.Request representation.
type AnthropicRequestExtensions struct {
	// Container / InferenceGeo / MCPServers are same-protocol Anthropic body
	// fields (opaque JSON). Primary owner is PE, not TransformerMetadata.
	Container     json.RawMessage `json:"-"`
	InferenceGeo  json.RawMessage `json:"-"`
	MCPServers    json.RawMessage `json:"-"`
	// RawContentFragments preserves unknown/future Anthropic request content
	// blocks (including nested tool_result children) as ordered raw JSON
	// fragments. They are replayed only by the Anthropic outbound adapter.
	RawContentFragments []AnthropicRawContentFragment `json:"-"`
}

// AnthropicRawContentFragment stores one Anthropic-native content-block JSON
// fragment with enough routing metadata for same-protocol ordered replay.
// MessageIndex is the canonical llm.Request.Messages index that owns the
// fragment. PartIndex is the MultipleContent / nested tool_result child index.
// NestedInToolResult marks fragments that belong to a tool message's nested
// tool_result content rather than a top-level message content array.
type AnthropicRawContentFragment struct {
	MessageIndex       int             `json:"-"`
	PartIndex          int             `json:"-"`
	NestedInToolResult bool            `json:"-"`
	Raw                json.RawMessage `json:"-"`
}

type AnthropicResponseExtensions struct {
	StopSequence *string         `json:"-"`
	StopDetails  json.RawMessage `json:"-"`
	RawUsage     json.RawMessage `json:"-"`
	// RawContent preserves the complete Anthropic response content[] array for
	// same-protocol non-stream replay when common llm.Response cannot own every
	// block. Only the Anthropic inbound adapter may consume it.
	RawContent      []json.RawMessage         `json:"-"`
	RawStreamEvents []AnthropicRawStreamEvent `json:"-"`
}

type AnthropicRawStreamEvent struct {
	Type string          `json:"-"`
	Raw  json.RawMessage `json:"-"`
}

type OpenAIResponsesRequestExtensions struct {
	// ReasoningContext is the Responses-native reasoning.context scope
	// (e.g. Responses Lite / internal features).
	ReasoningContext string `json:"-"`
	// Include carries the Responses-native top-level include directive. It is
	// intentionally not part of TransformerMetadata because Chat and Anthropic
	// do not share its wire semantics.
	Include              []string                   `json:"-"`
	MaxToolCalls         *int64                     `json:"-"`
	PromptCacheRetention *string                    `json:"-"`
	Truncation           *string                    `json:"-"`
	Background           *bool                      `json:"-"`
	ClientMetadata       map[string]string          `json:"-"`
	RawPrompt            json.RawMessage            `json:"-"`
	RawTopLevelFields    map[string]json.RawMessage `json:"-"`
	// RawStreamOptions preserves Responses-native stream_options exactly,
	// including nested extension fields. It is intentionally separate from
	// RawTopLevelFields so known stream_options never inflate
	// UnknownTopLevelFieldCount / false LossyDowngrade diagnostics.
	RawStreamOptions json.RawMessage             `json:"-"`
	NativeTools      *OpenAIResponsesNativeTools `json:"-"`

	// AdditionalTools carries raw input items with type="additional_tools".
	// They are replayed from this field, not RawInputItems, so diagnostics can
	// distinguish lazy tool declarations from other raw-only input items.
	AdditionalTools []OpenAIResponsesRawFragment `json:"-"`

	// AdditionalToolsCanonicalTools contains declarations from additional_tools
	// that have a stable cross-protocol llm.Tool representation. Responses
	// same-protocol replay continues to use AdditionalTools raw fragments.
	AdditionalToolsCanonicalTools []Tool `json:"-"`

	// AdditionalToolsUnrepresentableCount records declarations that cannot be
	// projected into a canonical tool and remain lossy cross-protocol.
	AdditionalToolsUnrepresentableCount int `json:"-"`

	// ToolSearchOutputCanonicalTools contains the tools dynamically loaded by
	// a client-executed tool_search_output item that have a stable
	// cross-protocol llm.Tool representation. The original input item remains
	// in RawInputItems for Responses same-protocol replay.
	ToolSearchOutputCanonicalTools []Tool `json:"-"`

	// ToolSearchOutputUnrepresentableCount records loaded declarations that
	// cannot be projected into a canonical tool for a non-Responses target.
	ToolSearchOutputUnrepresentableCount int                          `json:"-"`
	RawTools                             []OpenAIResponsesRawFragment `json:"-"`
	ToolSignatures                       []string                     `json:"-"`
	RawToolChoice                        json.RawMessage              `json:"-"`
	RawInputItems                        []OpenAIResponsesRawFragment `json:"-"`

	// PrependCount records how many messages were prepended to the canonical
	// request by the prompt pipeline between inbound and outbound. The
	// outbound Responses merge uses it to offset raw-only input items so they
	// keep their original position relative to the user's structured items
	// instead of landing ahead of the injected prepend (see
	// mergeRawOnlyInputItems). Set by the prompt pipeline when it prepends.
	PrependCount int `json:"-"`
}

type OpenAIResponsesNativeTools struct {
	Raw        []json.RawMessage `json:"-"`
	Signatures []string          `json:"-"`
}

type OpenAIResponsesRawFragment struct {
	Type          string          `json:"-"`
	Name          string          `json:"-"`
	CallID        string          `json:"-"`
	OriginalIndex int             `json:"-"`
	Raw           json.RawMessage `json:"-"`
}

func EnsureOpenAIResponsesProviderExtensions(req *Request) *OpenAIResponsesProviderExtensions {
	if req == nil {
		return nil
	}

	if req.ProviderExtensions == nil {
		req.ProviderExtensions = &ProviderExtensions{}
	}

	if req.ProviderExtensions.OpenAIResponses == nil {
		req.ProviderExtensions.OpenAIResponses = &OpenAIResponsesProviderExtensions{}
	}

	return req.ProviderExtensions.OpenAIResponses
}

// OpenAIResponsesRequestExtension returns the source-owned Responses request
// sidecar without creating provider extensions.
func OpenAIResponsesRequestExtension(req *Request) *OpenAIResponsesRequestExtensions {
	if req == nil || req.ProviderExtensions == nil || req.ProviderExtensions.OpenAIResponses == nil {
		return nil
	}
	return req.ProviderExtensions.OpenAIResponses.Request
}

func EnsureOpenAIResponsesResponseExtensions(resp *Response) *OpenAIResponsesResponseExtensions {
	if resp == nil {
		return nil
	}
	if resp.ProviderExtensions == nil {
		resp.ProviderExtensions = &ProviderExtensions{}
	}
	if resp.ProviderExtensions.OpenAIResponses == nil {
		resp.ProviderExtensions.OpenAIResponses = &OpenAIResponsesProviderExtensions{}
	}
	if resp.ProviderExtensions.OpenAIResponses.Response == nil {
		resp.ProviderExtensions.OpenAIResponses.Response = &OpenAIResponsesResponseExtensions{}
	}
	return resp.ProviderExtensions.OpenAIResponses.Response
}

func EnsureAnthropicProviderExtensions(req *Request) *AnthropicProviderExtensions {
	if req == nil {
		return nil
	}
	if req.ProviderExtensions == nil {
		req.ProviderExtensions = &ProviderExtensions{}
	}
	if req.ProviderExtensions.Anthropic == nil {
		req.ProviderExtensions.Anthropic = &AnthropicProviderExtensions{}
	}
	return req.ProviderExtensions.Anthropic
}

func EnsureAnthropicRequestExtensions(req *Request) *AnthropicRequestExtensions {
	ext := EnsureAnthropicProviderExtensions(req)
	if ext == nil {
		return nil
	}
	if ext.Request == nil {
		ext.Request = &AnthropicRequestExtensions{}
	}
	return ext.Request
}

func EnsureAnthropicResponseExtensions(resp *Response) *AnthropicResponseExtensions {
	if resp == nil {
		return nil
	}
	if resp.ProviderExtensions == nil {
		resp.ProviderExtensions = &ProviderExtensions{}
	}
	if resp.ProviderExtensions.Anthropic == nil {
		resp.ProviderExtensions.Anthropic = &AnthropicProviderExtensions{}
	}
	if resp.ProviderExtensions.Anthropic.Response == nil {
		resp.ProviderExtensions.Anthropic.Response = &AnthropicResponseExtensions{}
	}
	return resp.ProviderExtensions.Anthropic.Response
}


func EnsureOpenAIChatProviderExtensions(req *Request) *OpenAIChatProviderExtensions {
	if req == nil {
		return nil
	}
	if req.ProviderExtensions == nil {
		req.ProviderExtensions = &ProviderExtensions{}
	}
	if req.ProviderExtensions.OpenAIChat == nil {
		req.ProviderExtensions.OpenAIChat = &OpenAIChatProviderExtensions{}
	}
	return req.ProviderExtensions.OpenAIChat
}

func EnsureOpenAIChatRequestExtensions(req *Request) *OpenAIChatRequestExtensions {
	ext := EnsureOpenAIChatProviderExtensions(req)
	if ext == nil {
		return nil
	}
	if ext.Request == nil {
		ext.Request = &OpenAIChatRequestExtensions{}
	}
	return ext.Request
}

func OpenAIChatRequestExtension(req *Request) *OpenAIChatRequestExtensions {
	if req == nil || req.ProviderExtensions == nil || req.ProviderExtensions.OpenAIChat == nil {
		return nil
	}
	return req.ProviderExtensions.OpenAIChat.Request
}

func EnsureDiagnosticsProviderExtensions(req *Request) *DiagnosticsProviderExtensions {
	if req == nil {
		return nil
	}

	if req.ProviderExtensions == nil {
		req.ProviderExtensions = &ProviderExtensions{}
	}

	if req.ProviderExtensions.Diagnostics == nil {
		req.ProviderExtensions.Diagnostics = &DiagnosticsProviderExtensions{}
	}

	return req.ProviderExtensions.Diagnostics
}

func CloneProviderExtensions(src *ProviderExtensions) *ProviderExtensions {
	if src == nil {
		return nil
	}

	cloned := &ProviderExtensions{}
	if src.OpenAIResponses != nil {
		cloned.OpenAIResponses = &OpenAIResponsesProviderExtensions{}
		if src.OpenAIResponses.Request != nil {
			cloned.OpenAIResponses.Request = &OpenAIResponsesRequestExtensions{
				ReasoningContext:                         src.OpenAIResponses.Request.ReasoningContext,
				Include:                              append([]string(nil), src.OpenAIResponses.Request.Include...),
				MaxToolCalls:                         cloneInt64Ptr(src.OpenAIResponses.Request.MaxToolCalls),
				PromptCacheRetention:                 cloneStringPtr(src.OpenAIResponses.Request.PromptCacheRetention),
				Truncation:                           cloneStringPtr(src.OpenAIResponses.Request.Truncation),
				Background:                           cloneBoolPtr(src.OpenAIResponses.Request.Background),
				ClientMetadata:                       cloneStringMap(src.OpenAIResponses.Request.ClientMetadata),
				RawPrompt:                            cloneRawMessage(src.OpenAIResponses.Request.RawPrompt),
				RawTopLevelFields:                    cloneRawMessageMap(src.OpenAIResponses.Request.RawTopLevelFields),
				RawStreamOptions:                     cloneRawMessage(src.OpenAIResponses.Request.RawStreamOptions),
				NativeTools:                          cloneOpenAIResponsesNativeTools(src.OpenAIResponses.Request.NativeTools),
				AdditionalTools:                      cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.AdditionalTools),
				AdditionalToolsCanonicalTools:        append([]Tool(nil), src.OpenAIResponses.Request.AdditionalToolsCanonicalTools...),
				AdditionalToolsUnrepresentableCount:  src.OpenAIResponses.Request.AdditionalToolsUnrepresentableCount,
				ToolSearchOutputCanonicalTools:       append([]Tool(nil), src.OpenAIResponses.Request.ToolSearchOutputCanonicalTools...),
				ToolSearchOutputUnrepresentableCount: src.OpenAIResponses.Request.ToolSearchOutputUnrepresentableCount,
				RawTools:                             cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.RawTools),
				ToolSignatures:                       append([]string(nil), src.OpenAIResponses.Request.ToolSignatures...),
				RawToolChoice:                        cloneRawMessage(src.OpenAIResponses.Request.RawToolChoice),
				RawInputItems:                        cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.RawInputItems),
				PrependCount:                         src.OpenAIResponses.Request.PrependCount,
			}
		}
		if src.OpenAIResponses.Response != nil {
			cloned.OpenAIResponses.Response = &OpenAIResponsesResponseExtensions{
				RawTopLevelFields: cloneRawMessageMap(src.OpenAIResponses.Response.RawTopLevelFields),
				RawOutputItems:    cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Response.RawOutputItems),
				RawStreamEvents:   cloneOpenAIResponsesRawStreamEvents(src.OpenAIResponses.Response.RawStreamEvents),
			}
			if src.OpenAIResponses.Response.Status != nil {
				cloned.OpenAIResponses.Response.Status = lo.ToPtr(*src.OpenAIResponses.Response.Status)
			}
		}
	}
	if src.OpenAIChat != nil {
		cloned.OpenAIChat = &OpenAIChatProviderExtensions{}
		if src.OpenAIChat.Request != nil {
			cloned.OpenAIChat.Request = &OpenAIChatRequestExtensions{
				RawTopLevelFields: cloneRawMessageMap(src.OpenAIChat.Request.RawTopLevelFields),
			}
		}
	}
	if src.Anthropic != nil {
		cloned.Anthropic = &AnthropicProviderExtensions{}
		if src.Anthropic.Request != nil {
			cloned.Anthropic.Request = &AnthropicRequestExtensions{
				Container:           cloneRawMessage(src.Anthropic.Request.Container),
				InferenceGeo:        cloneRawMessage(src.Anthropic.Request.InferenceGeo),
				MCPServers:          cloneRawMessage(src.Anthropic.Request.MCPServers),
				RawContentFragments: cloneAnthropicRawContentFragments(src.Anthropic.Request.RawContentFragments),
			}
		}
		if src.Anthropic.Response != nil {
			cloned.Anthropic.Response = &AnthropicResponseExtensions{
				StopDetails:     cloneRawMessage(src.Anthropic.Response.StopDetails),
				RawUsage:        cloneRawMessage(src.Anthropic.Response.RawUsage),
				RawContent:      cloneRawMessages(src.Anthropic.Response.RawContent),
				RawStreamEvents: cloneAnthropicRawStreamEvents(src.Anthropic.Response.RawStreamEvents),
			}
			if src.Anthropic.Response.StopSequence != nil {
				cloned.Anthropic.Response.StopSequence = lo.ToPtr(*src.Anthropic.Response.StopSequence)
			}
		}
	}

	if src.Diagnostics != nil {
		cloned.Diagnostics = &DiagnosticsProviderExtensions{
			LossyDowngrades: append([]LossyDowngrade(nil), src.Diagnostics.LossyDowngrades...),
		}
		if src.Diagnostics.ResponsesLossy != nil {
			summary := *src.Diagnostics.ResponsesLossy
			cloned.Diagnostics.ResponsesLossy = &summary
		}
	}

	return cloned
}

func cloneAnthropicRawContentFragments(src []AnthropicRawContentFragment) []AnthropicRawContentFragment {
	if len(src) == 0 {
		return nil
	}
	out := make([]AnthropicRawContentFragment, len(src))
	for i := range src {
		out[i] = src[i]
		out[i].Raw = cloneRawMessage(src[i].Raw)
	}
	return out
}

func cloneAnthropicRawStreamEvents(src []AnthropicRawStreamEvent) []AnthropicRawStreamEvent {
	if len(src) == 0 {
		return nil
	}
	out := make([]AnthropicRawStreamEvent, len(src))
	for i, event := range src {
		out[i] = AnthropicRawStreamEvent{Type: event.Type, Raw: cloneRawMessage(event.Raw)}
	}
	return out
}

func cloneOpenAIResponsesRawStreamEvents(src []OpenAIResponsesRawStreamEvent) []OpenAIResponsesRawStreamEvent {
	if len(src) == 0 {
		return nil
	}
	out := make([]OpenAIResponsesRawStreamEvent, len(src))
	for i, event := range src {
		out[i] = OpenAIResponsesRawStreamEvent{Type: event.Type, Raw: cloneRawMessage(event.Raw)}
	}
	return out
}

func cloneOpenAIResponsesNativeTools(src *OpenAIResponsesNativeTools) *OpenAIResponsesNativeTools {
	if src == nil {
		return nil
	}

	return &OpenAIResponsesNativeTools{
		Raw:        cloneRawMessages(src.Raw),
		Signatures: append([]string(nil), src.Signatures...),
	}
}

func cloneRawMessages(src []json.RawMessage) []json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	out := make([]json.RawMessage, len(src))
	for i := range src {
		out[i] = cloneRawMessage(src[i])
	}

	return out
}

func cloneOpenAIResponsesRawFragments(src []OpenAIResponsesRawFragment) []OpenAIResponsesRawFragment {
	if len(src) == 0 {
		return nil
	}

	out := make([]OpenAIResponsesRawFragment, len(src))
	for i := range src {
		out[i] = src[i]
		out[i].Raw = cloneRawMessage(src[i].Raw)
	}

	return out
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	return append(json.RawMessage(nil), src...)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	return lo.Assign(map[string]string{}, src)
}

func cloneInt64Ptr(src *int64) *int64 {
	if src == nil {
		return nil
	}

	value := *src
	return &value
}

func cloneStringPtr(src *string) *string {
	if src == nil {
		return nil
	}

	value := *src
	return &value
}

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}

	value := *src
	return &value
}

func cloneRawMessageMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	out := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		out[key] = cloneRawMessage(value)
	}

	return out
}
