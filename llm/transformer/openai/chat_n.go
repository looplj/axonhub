package openai

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

const openAIChatNField = "n"

// marshalOpenAIChatRequest emits a Chat request without changing the shared
// OpenAI Request model. The original raw Chat request is the native sidecar for
// a field that has no cross-protocol common semantic; only the Chat outbound
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
	rawN := source[openAIChatNField]
	if len(rawN) == 0 {
		return body, nil
	}

	var outbound map[string]json.RawMessage
	if err := json.Unmarshal(body, &outbound); err != nil {
		return nil, err
	}
	outbound[openAIChatNField] = append(json.RawMessage(nil), rawN...)

	return json.Marshal(outbound)
}
