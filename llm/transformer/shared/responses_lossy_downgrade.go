package shared

import (
	"encoding/json"

	"github.com/looplj/axonhub/llm"
)

const ResponsesLossyDowngradeDiagnosticsKey = "responses_lossy_downgrade_diagnostics"

type ResponsesLossyDowngradeDiagnostics struct {
	LossyDowngrade            bool
	UnknownTopLevelFieldCount int
	ClientMetadataCount       int
	NamespaceToolCount        int
	ToolSearchToolCount       int
	UnknownToolCount          int
	RawOnlyToolCount          int
	AdditionalToolsCount      int
	RawInputItemCount         int
	UnknownInputItemCount     int
}

func RecordResponsesLossyDowngradeDiagnostics(llmReq *llm.Request) {
	requestExt := openAIResponsesRequestExtensions(llmReq)
	if requestExt == nil {
		return
	}
	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	diagnostics := ResponsesLossyDowngradeDiagnostics{
		UnknownTopLevelFieldCount: len(requestExt.RawTopLevelFields),
		ClientMetadataCount:       len(requestExt.ClientMetadata),
		AdditionalToolsCount:      len(requestExt.AdditionalTools),
		RawInputItemCount:         len(requestExt.RawInputItems),
	}
	if requestExt.NativeTools != nil {
		diagnostics.NamespaceToolCount = countNativeToolsByType(requestExt.NativeTools.Raw, "namespace")
		diagnostics.ToolSearchToolCount = countNativeToolsByType(requestExt.NativeTools.Raw, "tool_search")
	}
	diagnostics.UnknownToolCount = countUnknownToolFragments(requestExt.RawTools)
	diagnostics.RawOnlyToolCount = len(requestExt.RawTools)
	diagnostics.UnknownInputItemCount = countUnknownInputFragments(requestExt.RawInputItems)
	diagnostics.LossyDowngrade = diagnostics.UnknownTopLevelFieldCount > 0 ||
		diagnostics.ClientMetadataCount > 0 ||
		diagnostics.NamespaceToolCount > 0 ||
		diagnostics.ToolSearchToolCount > 0 ||
		diagnostics.UnknownToolCount > 0 ||
		diagnostics.RawOnlyToolCount > 0 ||
		diagnostics.AdditionalToolsCount > 0 ||
		diagnostics.RawInputItemCount > 0

	if diagnostics.LossyDowngrade {
		llmReq.TransformerMetadata[ResponsesLossyDowngradeDiagnosticsKey] = diagnostics
	}
}

func openAIResponsesRequestExtensions(llmReq *llm.Request) *llm.OpenAIResponsesRequestExtensions {
	if llmReq == nil || llmReq.ProviderExtensions == nil || llmReq.ProviderExtensions.OpenAIResponses == nil {
		return nil
	}
	return llmReq.ProviderExtensions.OpenAIResponses.Request
}

func countNativeToolsByType(rawTools []json.RawMessage, toolType string) int {
	count := 0
	for _, raw := range rawTools {
		var tool struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &tool) != nil {
			continue
		}
		if tool.Type == toolType {
			count++
		}
	}
	return count
}

func countUnknownToolFragments(fragments []llm.OpenAIResponsesRawFragment) int {
	count := 0
	for _, fragment := range fragments {
		if llm.IsKnownOpenAIResponsesNativeToolType(fragment.Type) {
			continue
		}
		count++
	}
	return count
}

func countUnknownInputFragments(fragments []llm.OpenAIResponsesRawFragment) int {
	count := 0
	for _, fragment := range fragments {
		if fragment.Type == "additional_tools" || llm.IsKnownOpenAIResponsesInputItemType(fragment.Type) {
			continue
		}
		count++
	}
	return count
}
