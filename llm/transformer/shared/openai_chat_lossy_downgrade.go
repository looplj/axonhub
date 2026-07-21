package shared

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

// RecordOpenAIChatRawRequestLossyDowngrades reports Chat-native raw request
// fields that have no safe non-Chat wire projection. representedFields holds
// source fields which the target adapter already projected exactly.
// Prefers PE.OpenAIChat.Request.RawTopLevelFields; falls back to RawRequest.Body
// for legacy requests that never attached Chat PE.
func RecordOpenAIChatRawRequestLossyDowngrades(
	llmReq *llm.Request,
	targetProtocol llm.APIFormat,
	representedFields map[string]bool,
) {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion || targetProtocol == "" {
		return
	}

	source := openAIChatRawFieldSource(llmReq)
	if len(source) == 0 {
		return
	}

	for _, field := range []string{
		"prompt_cache_retention",
		"n",
		"audio",
		"prediction",
		"moderation",
		"web_search_options",
		"functions",
		"function_call",
	} {
		if representedFields[field] {
			continue
		}
		llm.AddLossyDowngradeIfPresent(llmReq, llm.APIFormatOpenAIChatCompletion, field, targetProtocol, len(source[field]) > 0)
	}
}

func openAIChatRawFieldSource(llmReq *llm.Request) map[string]json.RawMessage {
	if reqExt := llm.OpenAIChatRequestExtension(llmReq); reqExt != nil && len(reqExt.RawTopLevelFields) > 0 {
		return reqExt.RawTopLevelFields
	}
	if llmReq == nil || llmReq.RawRequest == nil || len(llmReq.RawRequest.Body) == 0 {
		return nil
	}
	var source map[string]json.RawMessage
	if err := json.Unmarshal(llmReq.RawRequest.Body, &source); err != nil {
		return nil
	}
	return source
}
