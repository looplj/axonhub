package openai

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

// Chat-native top-level request fields that are not modeled on the shared
// OpenAI Request struct and must be re-emitted from the original raw request
// for same-protocol Chat -> Chat replay.
var openAIChatRawPreserveFields = []string{
	"n",
	"prompt_cache_retention",
}

// marshalOpenAIChatRequest emits a Chat request without changing the shared
// OpenAI Request model. The original raw Chat request is the native sidecar for
// fields that have no cross-protocol common semantic; only the Chat outbound
// calls this helper.
func marshalOpenAIChatRequest(req *Request, llmReq *llm.Request) ([]byte, error) {
	body, err := json.Marshal(req)
	if err != nil || llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion || llmReq.RawRequest == nil {
		return body, err
	}

	var source map[string]json.RawMessage
	if err := json.Unmarshal(llmReq.RawRequest.Body, &source); err != nil {
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
