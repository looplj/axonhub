package shared

import (
	"sort"

	"github.com/looplj/axonhub/llm"
)

// ResponsesLossyDowngradeDiagnosticsKey is deprecated. Summary lives on
// ProviderExtensions.Diagnostics.ResponsesLossy (see llm.ResponsesLossySummaryOf).
// Kept only so old greps still find the former metadata home.
const ResponsesLossyDowngradeDiagnosticsKey = "responses_lossy_downgrade_diagnostics"

// ResponsesLossyDowngradeDiagnostics is an alias of the PE-owned summary type
// for existing call sites in this package and tests.
type ResponsesLossyDowngradeDiagnostics = llm.ResponsesLossySummary

func RecordResponsesLossyDowngradeDiagnostics(llmReq *llm.Request) {
	RecordResponsesLossyDowngradeDiagnosticsForTarget(llmReq, "")
}

// RecordResponsesLossyDowngradeDiagnosticsForTarget records the structured
// Responses lossy summary on ProviderExtensions.Diagnostics and formal
// LossyDowngrade rows when targetProtocol is set.
// targetProtocol empty means summary-only (no formal per-field rows).
func RecordResponsesLossyDowngradeDiagnosticsForTarget(llmReq *llm.Request, targetProtocol llm.APIFormat) {
	requestExt := llm.OpenAIResponsesRequestExtension(llmReq)
	if requestExt == nil {
		return
	}

	diagnostics := llm.ResponsesLossySummary{
		UnknownTopLevelFieldCount:            len(requestExt.RawTopLevelFields),
		ClientMetadataCount:                  len(requestExt.ClientMetadata),
		AdditionalToolsCount:                 len(requestExt.AdditionalTools),
		AdditionalToolsUnrepresentableCount:  requestExt.AdditionalToolsUnrepresentableCount,
		ToolSearchOutputUnrepresentableCount: requestExt.ToolSearchOutputUnrepresentableCount,
		RawInputItemCount:                    len(requestExt.RawInputItems),
	}
	if requestExt.NativeTools != nil {
		diagnostics.NamespaceToolCount = llm.CountOpenAIResponsesNativeToolsByType(requestExt.NativeTools.Raw, "namespace")
		diagnostics.ToolSearchToolCount = llm.CountOpenAIResponsesNativeToolsByType(requestExt.NativeTools.Raw, "tool_search")
	}
	diagnostics.UnknownToolCount = llm.CountUnknownOpenAIResponsesToolFragments(requestExt.RawTools)
	diagnostics.RawOnlyToolCount = len(requestExt.RawTools)
	diagnostics.UnknownInputItemCount = llm.CountUnknownOpenAIResponsesInputFragments(requestExt.RawInputItems)
	additionalToolsLossy := diagnostics.AdditionalToolsCount > 0
	if targetProtocol == llm.APIFormatOpenAIChatCompletion {
		additionalToolsLossy = diagnostics.AdditionalToolsUnrepresentableCount > 0
	}
	diagnostics.LossyDowngrade = diagnostics.UnknownTopLevelFieldCount > 0 ||
		diagnostics.ClientMetadataCount > 0 ||
		diagnostics.NamespaceToolCount > 0 ||
		diagnostics.ToolSearchToolCount > 0 ||
		diagnostics.UnknownToolCount > 0 ||
		diagnostics.RawOnlyToolCount > 0 ||
		additionalToolsLossy ||
		diagnostics.ToolSearchOutputUnrepresentableCount > 0 ||
		diagnostics.RawInputItemCount > 0

	if diagnostics.LossyDowngrade {
		diagExt := llm.EnsureDiagnosticsProviderExtensions(llmReq)
		if diagExt != nil {
			summary := diagnostics
			diagExt.ResponsesLossy = &summary
		}
		// Do not write TransformerMetadata[ResponsesLossyDowngradeDiagnosticsKey].
	}

	if targetProtocol == "" || !diagnostics.LossyDowngrade {
		return
	}

	sourceProtocol := llmReq.APIFormat
	if sourceProtocol == "" {
		sourceProtocol = llm.APIFormatOpenAIResponse
	}

	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "tools[].type=namespace", targetProtocol, diagnostics.NamespaceToolCount > 0)
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "tools[].type=tool_search", targetProtocol, diagnostics.ToolSearchToolCount > 0)
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "input[].type=additional_tools", targetProtocol, additionalToolsLossy)
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "input[].type=tool_search_output.tools[]", targetProtocol, diagnostics.ToolSearchOutputUnrepresentableCount > 0)
	for _, rawItem := range requestExt.RawInputItems {
		llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "input[].type="+rawItem.Type, targetProtocol, rawItem.Type != "")
	}
	for _, rawTool := range requestExt.RawTools {
		llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "tools[].type="+rawTool.Type, targetProtocol, rawTool.Type != "")
	}
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "tools[] raw-only native tool", targetProtocol, diagnostics.RawOnlyToolCount > 0)
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "client_metadata", targetProtocol, diagnostics.ClientMetadataCount > 0)
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "request raw top-level field", targetProtocol, diagnostics.UnknownTopLevelFieldCount > 0)
	rawTopLevelFields := make([]string, 0, len(requestExt.RawTopLevelFields))
	for field := range requestExt.RawTopLevelFields {
		rawTopLevelFields = append(rawTopLevelFields, field)
	}
	sort.Strings(rawTopLevelFields)
	for _, field := range rawTopLevelFields {
		llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, field, targetProtocol, true)
	}
	llm.AddLossyDowngradeIfPresent(llmReq, sourceProtocol, "input[] raw-only item", targetProtocol, diagnostics.RawInputItemCount > 0 && diagnostics.AdditionalToolsCount == 0)
}
