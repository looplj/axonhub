package llm

import "encoding/json"

// ProviderExtensions carries provider/API-format private data that should not
// be serialized through the common llm request/response JSON model.
type ProviderExtensions struct {
	OpenAIResponses *OpenAIResponsesProviderExtensions `json:"-"`
}

type OpenAIResponsesProviderExtensions struct {
	Request *OpenAIResponsesRequestExtensions `json:"-"`
}

type OpenAIResponsesRequestExtensions struct {
	ReasoningContext string                       `json:"-"`
	RawTools         []OpenAIResponsesRawFragment `json:"-"`
	ToolSignatures   []string                     `json:"-"`
	RawToolChoice    json.RawMessage              `json:"-"`
	RawInputItems    []OpenAIResponsesRawFragment `json:"-"`
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
				ReasoningContext: src.OpenAIResponses.Request.ReasoningContext,
				RawTools:         cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.RawTools),
				ToolSignatures:   append([]string(nil), src.OpenAIResponses.Request.ToolSignatures...),
				RawToolChoice:    cloneRawMessage(src.OpenAIResponses.Request.RawToolChoice),
				RawInputItems:    cloneOpenAIResponsesRawFragments(src.OpenAIResponses.Request.RawInputItems),
			}
		}
	}

	return cloned
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
