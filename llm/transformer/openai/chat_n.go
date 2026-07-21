package openai

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

// Chat-native top-level request fields that are not modeled on the shared
// OpenAI Request struct. Owned by ProviderExtensions.OpenAIChat.Request
// (RawTopLevelFields) after inbound attach; same-protocol Chat outbound merges
// them back onto the typed marshal result.
var openAIChatRawPreserveFields = []string{
	"n",
	"prompt_cache_retention",
	"audio",
	"prediction",
	"moderation",
	"web_search_options",
	"functions",
	"function_call",
}

// attachOpenAIChatRequestExtensions captures Chat-only top-level raw fields from
// the inbound wire body onto PE.OpenAIChat.Request.
func attachOpenAIChatRequestExtensions(chatReq *llm.Request, rawBody []byte) {
	if chatReq == nil || len(rawBody) == 0 {
		return
	}

	var source map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &source); err != nil {
		return
	}

	fields := make(map[string]json.RawMessage)
	for _, field := range openAIChatRawPreserveFields {
		raw := source[field]
		if len(raw) == 0 {
			continue
		}
		fields[field] = append(json.RawMessage(nil), raw...)
	}
	if len(fields) == 0 {
		return
	}

	reqExt := llm.EnsureOpenAIChatRequestExtensions(chatReq)
	if reqExt == nil {
		return
	}
	reqExt.RawTopLevelFields = fields
}

// openAIChatRawTopLevelField returns a preserved Chat-only raw field if present.
func openAIChatRawTopLevelField(llmReq *llm.Request, field string) (json.RawMessage, bool) {
	reqExt := llm.OpenAIChatRequestExtension(llmReq)
	if reqExt == nil || len(reqExt.RawTopLevelFields) == 0 {
		return nil, false
	}
	raw, ok := reqExt.RawTopLevelFields[field]
	if !ok || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

// marshalOpenAIChatRequest emits a Chat request without changing the shared
// OpenAI Request model. Chat-only fields come from PE.OpenAIChat when present;
// RawRequest.Body is a legacy fallback for requests that never went through
// Chat inbound attach.
func marshalOpenAIChatRequest(req *Request, llmReq *llm.Request) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil || llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion {
		return body, err
	}

	source := chatRawPreserveSource(llmReq)
	if len(source) == 0 {
		return body, nil
	}

	var outbound map[string]json.RawMessage
	updated := false
	for _, field := range openAIChatRawPreserveFields {
		raw := source[field]
		if len(raw) == 0 {
			continue
		}
		if outbound == nil {
			if err := json.Unmarshal(body, &outbound); err != nil {
				return nil, err
			}
		}
		outbound[field] = append(json.RawMessage(nil), raw...)
		updated = true
	}
	if !updated {
		return body, nil
	}

	return json.Marshal(outbound)
}

func chatRawPreserveSource(llmReq *llm.Request) map[string]json.RawMessage {
	if llmReq == nil {
		return nil
	}
	if reqExt := llm.OpenAIChatRequestExtension(llmReq); reqExt != nil && len(reqExt.RawTopLevelFields) > 0 {
		return reqExt.RawTopLevelFields
	}
	// Legacy fallback: pre-S4 paths that only have RawRequest.
	if llmReq.RawRequest == nil || len(llmReq.RawRequest.Body) == 0 {
		return nil
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(llmReq.RawRequest.Body, &source); err != nil {
		return nil
	}
	return source
}
