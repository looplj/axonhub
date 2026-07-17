package shared

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

// RecordOpenAIChatRawRequestLossyDowngrades reports Chat-native raw request
// fields that have no safe non-Chat wire projection. representedFields holds
// source fields which the target adapter already projected exactly.
func RecordOpenAIChatRawRequestLossyDowngrades(
	llmReq *llm.Request,
	targetProtocol llm.APIFormat,
	representedFields map[string]bool,
) {
	if llmReq == nil || llmReq.APIFormat != llm.APIFormatOpenAIChatCompletion || llmReq.RawRequest == nil || targetProtocol == "" {
		return
	}

	var source map[string]json.RawMessage
	if err := json.Unmarshal(llmReq.RawRequest.Body, &source); err != nil {
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
