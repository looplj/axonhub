package orchestrator

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
)

type LLMCompatibilitySettingsProvider interface {
	LLMCompatibilitySettingsOrDefault(ctx context.Context) *biz.LLMCompatibilitySettings
}

var responsesTransformerMetadataKeys = map[string]struct{}{
	"include":                {},
	"include_obfuscation":    {},
	"image_output_format":    {},
	"max_tool_calls":         {},
	"prompt_cache_key":       {},
	"prompt_cache_retention": {},
	"truncation":             {},
}

var responsesTransformerMetadataControlKeys = map[string]struct{}{
	"max_tool_calls":         {},
	"prompt_cache_retention": {},
	"truncation":             {},
}

type responsesOnlyDataSummary struct {
	hasRawExtension                bool
	safeExtraKeysCount             int
	semanticExtraKeysCount         int
	metadataExtraKeysCount         int
	rawInputItemsCount             int
	executableToolsCount           int
	customToolMessagesCount        int
	transformerMetadataCount       int
	controlTransformerMetadataKeys int
	previousResponseID             bool
	toolChoiceControl              bool
}

func (s responsesOnlyDataSummary) hasAnyResponsesOnlyData() bool {
	return s.hasRawExtension ||
		s.safeExtraKeysCount > 0 ||
		s.semanticExtraKeysCount > 0 ||
		s.metadataExtraKeysCount > 0 ||
		s.rawInputItemsCount > 0 ||
		s.executableToolsCount > 0 ||
		s.customToolMessagesCount > 0 ||
		s.transformerMetadataCount > 0 ||
		s.previousResponseID ||
		s.toolChoiceControl
}

func (s responsesOnlyDataSummary) hasControlOrExecutableData() bool {
	return s.semanticExtraKeysCount > 0 ||
		s.rawInputItemsCount > 0 ||
		s.executableToolsCount > 0 ||
		s.customToolMessagesCount > 0 ||
		s.controlTransformerMetadataKeys > 0 ||
		s.previousResponseID ||
		s.toolChoiceControl
}

func (s responsesOnlyDataSummary) rejectedCategories(strict bool) []string {
	categories := make([]string, 0, 8)
	if strict && s.hasRawExtension {
		categories = append(categories, "raw_extension")
	}
	if s.semanticExtraKeysCount > 0 {
		categories = append(categories, "semantic_control_top_level_extra")
	}
	if s.rawInputItemsCount > 0 {
		categories = append(categories, "raw_only_input_items")
	}
	if s.executableToolsCount > 0 {
		categories = append(categories, "executable_tools")
	}
	if s.customToolMessagesCount > 0 {
		categories = append(categories, "custom_tool_messages")
	}
	if s.controlTransformerMetadataKeys > 0 {
		categories = append(categories, "control_transformer_metadata")
	}
	if s.previousResponseID {
		categories = append(categories, "previous_response_id")
	}
	if s.toolChoiceControl {
		categories = append(categories, "tool_choice")
	}
	if strict && len(categories) == 0 && s.hasAnyResponsesOnlyData() {
		categories = append(categories, "responses_only_data")
	}

	slices.Sort(categories)

	return categories
}

func (p *PersistentOutboundTransformer) responsesOnlyDataPolicy(ctx context.Context) biz.ResponsesOnlyDataPolicy {
	if p == nil || p.state == nil || p.state.CompatibilitySettingsProvider == nil {
		return biz.ResponsesOnlyDataPolicyDiscard
	}

	settings := p.state.CompatibilitySettingsProvider.LLMCompatibilitySettingsOrDefault(ctx)
	if settings == nil || !settings.ResponsesOnlyDataPolicy.Valid() {
		return biz.ResponsesOnlyDataPolicyDiscard
	}

	return settings.ResponsesOnlyDataPolicy
}

func sanitizeOpenAIResponsesForNonResponsesOutbound(
	req *llm.Request,
	outboundFormat llm.APIFormat,
	policy biz.ResponsesOnlyDataPolicy,
) (*llm.Request, AttemptSanitizeResult, error) {
	result := AttemptSanitizeResult{
		Policy:            policy,
		OutboundAPIFormat: outboundFormat.String(),
	}

	if req == nil || !isResponsesFormat(req.APIFormat) || isResponsesFormat(outboundFormat) {
		return req, result, nil
	}
	if !policy.Valid() {
		policy = biz.ResponsesOnlyDataPolicyDiscard
		result.Policy = policy
	}

	summary := summarizeResponsesOnlyData(req)
	copySummaryToSanitizeResult(&result, summary)
	if !summary.hasAnyResponsesOnlyData() {
		return req, result, nil
	}

	switch policy {
	case biz.ResponsesOnlyDataPolicyReject:
		result.Rejected = true
		result.Reason = "responses_only_data_rejected_for_non_responses_outbound"
		result.RejectedCategories = summary.rejectedCategories(true)

		return req, result, responsesOnlyPolicyError(result.Reason, result.RejectedCategories)

	case biz.ResponsesOnlyDataPolicyDiscardSafeRejectControl:
		if summary.hasControlOrExecutableData() {
			result.Rejected = true
			result.Reason = "responses_only_control_data_rejected_for_non_responses_outbound"
			result.RejectedCategories = summary.rejectedCategories(false)

			return req, result, responsesOnlyPolicyError(result.Reason, result.RejectedCategories)
		}
	}

	sanitized := discardResponsesOnlyData(req)
	result.Changed = true
	result.Reason = "responses_only_data_discarded_for_non_responses_outbound"

	return sanitized, result, nil
}

func responsesOnlyPolicyError(reason string, categories []string) error {
	if len(categories) == 0 {
		return fmt.Errorf("%w: Responses-only data cannot be routed to non-Responses outbound (%s)", transformer.ErrInvalidRequest, reason)
	}

	return fmt.Errorf("%w: Responses-only control/executable data cannot be routed to non-Responses outbound (%s)", transformer.ErrInvalidRequest, strings.Join(categories, ","))
}

func summarizeResponsesOnlyData(req *llm.Request) responsesOnlyDataSummary {
	var summary responsesOnlyDataSummary
	if req == nil {
		return summary
	}

	ext := req.ProviderExtensions.OpenAIResponsesRequest()
	if req.ProviderExtensions != nil && req.ProviderExtensions.OpenAIResponses != nil && req.ProviderExtensions.OpenAIResponses.Request != nil {
		summary.hasRawExtension = true
		summary.safeExtraKeysCount = len(ext.TopLevelExtra)
		summary.semanticExtraKeysCount = len(ext.TopLevelSemanticExtra)
		summary.metadataExtraKeysCount = len(ext.MetadataExtra)
		summary.rawInputItemsCount = countRawOnlyInputItems(ext.InputItems)
		summary.executableToolsCount = countExecutableResponsesTools(ext.Tools)
		if isResponsesToolChoiceControl(ext.ToolChoiceRaw, req.ToolChoice) {
			summary.toolChoiceControl = true
		}
	}

	summary.customToolMessagesCount = countResponseCustomToolMessageFragments(req.Messages)
	summary.executableToolsCount += countResponsesOnlySemanticTools(req.Tools)
	summary.transformerMetadataCount, summary.controlTransformerMetadataKeys = countResponsesTransformerMetadata(req.TransformerMetadata)
	summary.previousResponseID = req.PreviousResponseID != nil && *req.PreviousResponseID != ""
	if req.ToolChoice != nil && req.ToolChoice.NamedToolChoice != nil && req.ToolChoice.NamedToolChoice.Type != "" &&
		req.ToolChoice.NamedToolChoice.Type != llm.ToolTypeFunction {
		summary.toolChoiceControl = true
	}

	return summary
}

func copySummaryToSanitizeResult(result *AttemptSanitizeResult, summary responsesOnlyDataSummary) {
	if result == nil {
		return
	}

	result.DroppedSafeExtraKeysCount = summary.safeExtraKeysCount
	result.DroppedSemanticExtraKeysCount = summary.semanticExtraKeysCount
	result.DroppedMetadataExtraKeysCount = summary.metadataExtraKeysCount
	result.DroppedRawInputItemsCount = summary.rawInputItemsCount
	result.DroppedExecutableToolsCount = summary.executableToolsCount
	result.DroppedCustomToolMessagesCount = summary.customToolMessagesCount
	result.DroppedTransformerMetadataCount = summary.transformerMetadataCount
}

func discardResponsesOnlyData(req *llm.Request) *llm.Request {
	sanitized := CloneRequestForOutboundAttempt(req)
	if sanitized == nil {
		return nil
	}

	if sanitized.ProviderExtensions != nil {
		sanitized.ProviderExtensions.OpenAIResponses = nil
		if sanitized.ProviderExtensions.OpenAIResponses == nil {
			sanitized.ProviderExtensions = nil
		}
	}

	sanitized.PreviousResponseID = nil
	sanitized.Tools = filterResponsesOnlySemanticTools(sanitized.Tools)
	sanitized.Messages = filterResponseCustomToolMessagesForNonResponsesOutbound(sanitized, llm.APIFormatOpenAIChatCompletion).Messages
	sanitized.ToolChoice = sanitizeToolChoiceForNonResponses(sanitized.ToolChoice, sanitized.Tools)
	sanitized.TransformerMetadata = discardResponsesTransformerMetadata(sanitized.TransformerMetadata)

	return sanitized
}

func countRawOnlyInputItems(items []llm.OpenAIResponsesRawItem) int {
	count := 0
	for _, item := range items {
		if item.SemanticKey == "" {
			count++
		}
	}

	return count
}

func countExecutableResponsesTools(tools []llm.OpenAIResponsesRawItem) int {
	count := 0
	for _, tool := range tools {
		if tool.Type != llm.ToolTypeFunction {
			count++
		}
	}

	return count
}

func countResponsesOnlySemanticTools(tools []llm.Tool) int {
	count := 0
	for _, tool := range tools {
		if isResponsesOnlySemanticTool(tool) {
			count++
		}
	}

	return count
}

func filterResponsesOnlySemanticTools(tools []llm.Tool) []llm.Tool {
	if len(tools) == 0 {
		return nil
	}

	filtered := make([]llm.Tool, 0, len(tools))
	for _, tool := range tools {
		if isResponsesOnlySemanticTool(tool) {
			continue
		}
		filtered = append(filtered, tool)
	}
	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

func isResponsesOnlySemanticTool(tool llm.Tool) bool {
	return tool.Type == llm.ToolTypeResponsesCustomTool || tool.ResponseCustomTool != nil
}

func countResponseCustomToolMessageFragments(messages []llm.Message) int {
	count := 0
	removedCallIDs := map[string]struct{}{}
	for _, msg := range messages {
		for _, toolCall := range msg.ToolCalls {
			if toolCall.Type != llm.ToolTypeResponsesCustomTool && toolCall.ResponseCustomToolCall == nil {
				continue
			}
			count++
			if toolCall.ResponseCustomToolCall != nil && toolCall.ResponseCustomToolCall.CallID != "" {
				removedCallIDs[toolCall.ResponseCustomToolCall.CallID] = struct{}{}
			}
			if toolCall.ID != "" {
				removedCallIDs[toolCall.ID] = struct{}{}
			}
		}
	}

	for _, msg := range messages {
		if msg.Role != "tool" || msg.ToolCallID == nil {
			continue
		}
		if _, ok := removedCallIDs[*msg.ToolCallID]; ok {
			count++
		}
	}

	return count
}

func countResponsesTransformerMetadata(metadata map[string]any) (int, int) {
	total := 0
	control := 0
	for key := range metadata {
		if _, ok := responsesTransformerMetadataKeys[key]; !ok {
			continue
		}
		total++
		if _, ok := responsesTransformerMetadataControlKeys[key]; ok {
			control++
		}
	}

	return total, control
}

func discardResponsesTransformerMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}

	filtered := make(map[string]any, len(metadata))
	for key, value := range metadata {
		if _, ok := responsesTransformerMetadataKeys[key]; ok {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}

	return filtered
}

func isResponsesToolChoiceControl(raw []byte, choice *llm.ToolChoice) bool {
	if choice != nil && choice.NamedToolChoice != nil && choice.NamedToolChoice.Type != "" &&
		choice.NamedToolChoice.Type != llm.ToolTypeFunction {
		return true
	}

	text := string(raw)
	if text == "" {
		return false
	}

	for _, marker := range []string{"\"custom\"", "\"tools\"", "\"shell\"", "\"mcp\"", "\"namespace\"", "\"tool_search\""} {
		if strings.Contains(text, marker) {
			return true
		}
	}

	return false
}

func sanitizeToolChoiceForNonResponses(choice *llm.ToolChoice, tools []llm.Tool) *llm.ToolChoice {
	if choice == nil {
		return nil
	}
	if len(tools) == 0 {
		return nil
	}
	if choice.NamedToolChoice != nil && choice.NamedToolChoice.Type != "" && choice.NamedToolChoice.Type != llm.ToolTypeFunction {
		return nil
	}

	return choice
}

func logResponsesOnlyPolicyResult(ctx context.Context, candidate *ChannelModelsCandidate, result AttemptSanitizeResult) {
	fields := []log.Field{
		log.String("policy", string(result.Policy)),
		log.String("outbound_api_format", result.OutboundAPIFormat),
		log.String("reason", result.Reason),
		log.Int("dropped_safe_extra_keys_count", result.DroppedSafeExtraKeysCount),
		log.Int("dropped_semantic_extra_keys_count", result.DroppedSemanticExtraKeysCount),
		log.Int("dropped_metadata_extra_keys_count", result.DroppedMetadataExtraKeysCount),
		log.Int("dropped_raw_input_items_count", result.DroppedRawInputItemsCount),
		log.Int("dropped_executable_tools_count", result.DroppedExecutableToolsCount),
		log.Int("dropped_custom_tool_messages_count", result.DroppedCustomToolMessagesCount),
		log.Int("dropped_transformer_metadata_count", result.DroppedTransformerMetadataCount),
		log.Strings("rejected_categories", result.RejectedCategories),
	}
	if candidate != nil && candidate.Channel != nil {
		fields = append(fields,
			log.Int("channel_id", candidate.Channel.ID),
			log.String("channel", candidate.Channel.Name),
		)
		if len(candidate.Models) > 0 {
			fields = append(fields, log.String("actual_model", candidate.Models[0].ActualModel))
		}
	}

	if result.Rejected {
		log.Warn(ctx, "rejected Responses-only data for non-Responses outbound", fields...)
		return
	}
	if result.Changed {
		log.Info(ctx, "discarded Responses-only data for non-Responses outbound", fields...)
	}
}
