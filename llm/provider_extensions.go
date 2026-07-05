package llm

import (
	"encoding/json"

	"github.com/samber/lo"
)

// ProviderExtensions carries provider/API-format private data that should not
// be serialized through the common llm request/response JSON model.
type ProviderExtensions struct {
	OpenAIResponses *OpenAIResponsesProviderExtensions `json:"-"`
}

type OpenAIResponsesProviderExtensions struct {
	Request *OpenAIResponsesRequestExtensions `json:"-"`
}

type OpenAIResponsesRequestExtensions struct {
	ClientMetadata    map[string]string            `json:"-"`
	RawTopLevelFields map[string]json.RawMessage   `json:"-"`
	NativeTools       *OpenAIResponsesNativeTools  `json:"-"`
	AdditionalTools   []OpenAIResponsesRawFragment `json:"-"`
	RawTools          []OpenAIResponsesRawFragment `json:"-"`
	ToolSignatures    []string                     `json:"-"`
	RawToolChoice     json.RawMessage              `json:"-"`
	RawInputItems     []OpenAIResponsesRawFragment `json:"-"`

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

func CloneProviderExtensions(src *ProviderExtensions) *ProviderExtensions {
	if src == nil {
		return nil
	}

	cloned := &ProviderExtensions{}
	if src.OpenAIResponses != nil {
		cloned.OpenAIResponses = &OpenAIResponsesProviderExtensions{}
		if src.OpenAIResponses.Request != nil {
			cloned.OpenAIResponses.Request = &OpenAIResponsesRequestExtensions{
				ClientMetadata:    cloneStringMap(src.OpenAIResponses.Request.ClientMetadata),
				RawTopLevelFields: cloneRawMessageMap(src.OpenAIResponses.Request.RawTopLevelFields),
				NativeTools:       cloneOpenAIResponsesNativeTools(src.OpenAIResponses.Request.NativeTools),
				AdditionalTools:   cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.AdditionalTools),
				RawTools:          cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.RawTools),
				ToolSignatures:    append([]string(nil), src.OpenAIResponses.Request.ToolSignatures...),
				RawToolChoice:     cloneRawMessage(src.OpenAIResponses.Request.RawToolChoice),
				RawInputItems:     cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.RawInputItems),
				PrependCount:      src.OpenAIResponses.Request.PrependCount,
			}
		}
	}

	return cloned
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
