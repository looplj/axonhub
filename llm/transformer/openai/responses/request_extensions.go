package responses

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
)

const responsesRequestPreservationDiagnosticsTransformerMetadataKey = "responses_request_preservation_diagnostics"

type requestPreservationDiagnostics struct {
	NativePreservation        bool
	UnknownTopLevelFieldCount int
	NativeToolCount           int
	NamespaceToolCount        int
	ToolSearchToolCount       int
	UnknownToolCount          int
	RawOnlyToolCount          int
	AdditionalToolsCount      int
	RawInputItemCount         int
	UnknownInputItemCount     int
	RawToolChoicePreserved    bool
}

func attachOpenAIResponsesRequestExtensions(chatReq *llm.Request, req *Request, rawBody []byte) {
	if chatReq == nil || req == nil {
		return
	}

	raw := parseRawRequestFragments(rawBody)
	nativeToolSignatures := buildRepresentedToolSignatures(req.Tools)
	requestExt := &llm.OpenAIResponsesRequestExtensions{
		RawTopLevelFields: raw.TopLevelFields,
		NativeTools: &llm.OpenAIResponsesNativeTools{
			Raw:        cloneRawMessages(raw.Tools),
			Signatures: nativeToolSignatures,
		},
		AdditionalTools: buildAdditionalToolsFragments(req.Input, raw.InputItems),
		RawTools:        buildRawOnlyToolFragments(req.Tools, raw.Tools),
		ToolSignatures:  nativeToolSignatures,
		RawToolChoice:   rawUnsupportedToolChoice(req.ToolChoice, raw.ToolChoice),
		RawInputItems:   buildRawOnlyInputFragments(req.Input, raw.InputItems),
	}
	if len(req.ClientMetadata) > 0 {
		requestExt.ClientMetadata = lo.Assign(map[string]string{}, req.ClientMetadata)
	}

	if len(requestExt.ClientMetadata) == 0 && len(requestExt.RawTopLevelFields) == 0 && isEmptyNativeTools(requestExt.NativeTools) && len(requestExt.AdditionalTools) == 0 && len(requestExt.RawTools) == 0 && len(requestExt.RawToolChoice) == 0 && len(requestExt.RawInputItems) == 0 {
		return
	}

	ext := llm.EnsureOpenAIResponsesProviderExtensions(chatReq)
	if ext == nil {
		return
	}
	ext.Request = requestExt
}

type rawRequestFragments struct {
	TopLevelFields map[string]json.RawMessage
	Tools          []json.RawMessage
	ToolChoice     json.RawMessage
	InputItems     []json.RawMessage
}

func isEmptyNativeTools(nativeTools *llm.OpenAIResponsesNativeTools) bool {
	return nativeTools == nil || len(nativeTools.Raw) == 0
}

func parseRawRequestFragments(rawBody []byte) rawRequestFragments {
	if len(rawBody) == 0 {
		return rawRequestFragments{}
	}

	var raw struct {
		Tools      []json.RawMessage `json:"tools"`
		ToolChoice json.RawMessage   `json:"tool_choice"`
		Input      json.RawMessage   `json:"input"`
	}
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return rawRequestFragments{}
	}

	var inputItems []json.RawMessage
	if len(raw.Input) > 0 && json.Unmarshal(raw.Input, &inputItems) != nil {
		inputItems = nil
	}

	return rawRequestFragments{
		TopLevelFields: buildRawUnknownTopLevelFields(rawBody),
		Tools:          raw.Tools,
		ToolChoice:     raw.ToolChoice,
		InputItems:     inputItems,
	}
}

var knownRequestTopLevelFields = requestJSONFields(reflect.TypeOf(Request{}))

func requestJSONFields(requestType reflect.Type) map[string]struct{} {
	fields := make(map[string]struct{}, requestType.NumField())
	for i := 0; i < requestType.NumField(); i++ {
		jsonName := strings.Split(requestType.Field(i).Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}
		fields[jsonName] = struct{}{}
	}
	return fields
}

func buildRawUnknownTopLevelFields(rawBody []byte) map[string]json.RawMessage {
	var obj map[string]json.RawMessage
	if len(rawBody) == 0 || json.Unmarshal(rawBody, &obj) != nil {
		return nil
	}

	unknown := make(map[string]json.RawMessage)
	for key, value := range obj {
		if _, ok := knownRequestTopLevelFields[key]; ok {
			continue
		}
		unknown[key] = cloneRaw(value)
	}

	if len(unknown) == 0 {
		return nil
	}

	return unknown
}

func buildRepresentedToolSignatures(tools []Tool) []string {
	if len(tools) == 0 {
		return nil
	}

	signatures := make([]string, 0, len(tools))
	for _, tool := range tools {
		// A namespace container is expanded into one canonical function per
		// sub-tool by convertToolsToLLM (name = "<namespace>__<sub>"). Mirror
		// that expansion here so the signature count matches the canonical
		// tools produced on the outbound side; otherwise the raw-only merge
		// signature check fails and co-resident raw-only tools are starved.
		if tool.Type == "namespace" {
			for _, subTool := range tool.Tools {
				if subTool.Type != "function" {
					continue
				}
				signatures = append(signatures, "function:"+tool.Name+"__"+subTool.Name)
			}
			continue
		}
		if !isStructurallyRepresentedToolType(tool.Type) {
			continue
		}
		signatures = append(signatures, responseToolSignature(tool))
	}

	return signatures
}

func buildRawOnlyToolFragments(tools []Tool, rawTools []json.RawMessage) []llm.OpenAIResponsesRawFragment {
	if len(tools) == 0 {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0, len(tools))
	for i := range tools {
		// namespace is structurally expanded into canonical functions, so it
		// must not also be carried as a raw fragment (that would duplicate the
		// tools and break the signature-count check). Other non-represented
		// types (file_search, mcp, ...) stay raw.
		if tools[i].Type == "namespace" {
			continue
		}
		if i >= len(rawTools) || len(rawTools[i]) == 0 || isStructurallyRepresentedToolType(tools[i].Type) {
			continue
		}

		fragments = append(fragments, llm.OpenAIResponsesRawFragment{
			Type:          tools[i].Type,
			Name:          tools[i].Name,
			OriginalIndex: i,
			Raw:           cloneRaw(rawTools[i]),
		})
	}

	return fragments
}

func isStructurallyRepresentedToolType(toolType string) bool {
	switch toolType {
	case "function", "image_generation", "web_search", "custom":
		return true
	default:
		return false
	}
}

func responseToolSignature(tool Tool) string {
	switch tool.Type {
	case "function", "custom":
		return tool.Type + ":" + tool.Name
	default:
		return tool.Type
	}
}

func rawUnsupportedToolChoice(choice *ToolChoice, rawChoice json.RawMessage) json.RawMessage {
	if choice == nil || len(rawChoice) == 0 {
		return nil
	}

	var rawObject map[string]json.RawMessage
	if json.Unmarshal(rawChoice, &rawObject) == nil {
		return cloneRaw(rawChoice)
	}

	return nil
}

func buildAdditionalToolsFragments(input Input, rawItems []json.RawMessage) []llm.OpenAIResponsesRawFragment {
	if len(input.Items) == 0 {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0)
	for i := range input.Items {
		item := input.Items[i]
		if item.Type != "additional_tools" || i >= len(rawItems) || len(rawItems[i]) == 0 {
			continue
		}
		fragments = append(fragments, llm.OpenAIResponsesRawFragment{
			Type:          item.Type,
			OriginalIndex: i,
			Raw:           cloneRaw(rawItems[i]),
		})
	}

	return fragments
}

func buildRawOnlyInputFragments(input Input, rawItems []json.RawMessage) []llm.OpenAIResponsesRawFragment {
	if len(input.Items) == 0 {
		return nil
	}

	fragments := make([]llm.OpenAIResponsesRawFragment, 0)
	for i := range input.Items {
		item := input.Items[i]
		if i >= len(rawItems) || len(rawItems[i]) == 0 || isStructurallyRepresentedInputItem(item.Type) {
			continue
		}

		fragments = append(fragments, llm.OpenAIResponsesRawFragment{
			Type:          item.Type,
			Name:          item.Name,
			CallID:        item.CallID,
			OriginalIndex: i,
			Raw:           cloneRaw(rawItems[i]),
		})
	}

	return fragments
}

func isStructurallyRepresentedInputItem(itemType string) bool {
	switch itemType {
	case "", "message", "input_text", "input_image", "input_audio", "function_call", "function_call_output",
		"custom_tool_call", "custom_tool_call_output", "reasoning", "compaction", "compaction_summary":
		return true
	default:
		return false
	}
}

func openAIResponsesRequestExtensions(llmReq *llm.Request) *llm.OpenAIResponsesRequestExtensions {
	if llmReq == nil || llmReq.ProviderExtensions == nil || llmReq.ProviderExtensions.OpenAIResponses == nil {
		return nil
	}
	requestExt := llmReq.ProviderExtensions.OpenAIResponses.Request

	return requestExt
}

func marshalRequestPayload(payload Request, llmReq *llm.Request) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	requestExt := openAIResponsesRequestExtensions(llmReq)
	if requestExt == nil {
		return body, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}

	recordRequestPreservationDiagnostics(llmReq, requestExt)
	mergeRawUnknownTopLevelFields(obj, requestExt.RawTopLevelFields)
	if len(requestExt.ClientMetadata) > 0 {
		clientMetadataRaw, err := json.Marshal(requestExt.ClientMetadata)
		if err != nil {
			return nil, err
		}
		obj["client_metadata"] = clientMetadataRaw
	}

	if tools, ok := mergeNativeTools(obj["tools"], requestExt); ok {
		toolsRaw, err := json.Marshal(tools)
		if err != nil {
			return nil, err
		}
		obj["tools"] = toolsRaw
	}

	if len(requestExt.RawToolChoice) > 0 && rawToolChoiceMatchesCurrentTools(requestExt.RawToolChoice, payload.ToolChoice) {
		obj["tool_choice"] = cloneRaw(requestExt.RawToolChoice)
	}

	if input, ok := mergeRawOnlyInputItems(obj["input"], requestExt); ok {
		inputRaw, err := json.Marshal(input)
		if err != nil {
			return nil, err
		}
		obj["input"] = inputRaw
	}

	return json.Marshal(obj)
}

func recordRequestPreservationDiagnostics(llmReq *llm.Request, requestExt *llm.OpenAIResponsesRequestExtensions) {
	if llmReq == nil || requestExt == nil {
		return
	}
	if llmReq.TransformerMetadata == nil {
		llmReq.TransformerMetadata = map[string]any{}
	}

	diagnostics := requestPreservationDiagnostics{
		UnknownTopLevelFieldCount: len(requestExt.RawTopLevelFields),
		RawOnlyToolCount:          len(requestExt.RawTools),
		AdditionalToolsCount:      len(requestExt.AdditionalTools),
		RawInputItemCount:         len(requestExt.RawInputItems),
		RawToolChoicePreserved:    len(requestExt.RawToolChoice) > 0,
	}
	if requestExt.NativeTools != nil {
		diagnostics.NativeToolCount = len(requestExt.NativeTools.Raw)
		diagnostics.NamespaceToolCount = countNativeToolsByType(requestExt.NativeTools.Raw, "namespace")
		diagnostics.ToolSearchToolCount = countNativeToolsByType(requestExt.NativeTools.Raw, "tool_search")
	}
	diagnostics.UnknownToolCount = countUnknownToolFragments(requestExt.RawTools)
	diagnostics.UnknownInputItemCount = countUnknownInputFragments(requestExt.RawInputItems)
	diagnostics.NativePreservation = diagnostics.UnknownTopLevelFieldCount > 0 ||
		diagnostics.NativeToolCount > 0 ||
		diagnostics.RawOnlyToolCount > 0 ||
		diagnostics.AdditionalToolsCount > 0 ||
		diagnostics.RawInputItemCount > 0 ||
		diagnostics.RawToolChoicePreserved

	llmReq.TransformerMetadata[responsesRequestPreservationDiagnosticsTransformerMetadataKey] = diagnostics
}

func countNativeToolsByType(rawTools []json.RawMessage, toolType string) int {
	count := 0
	for _, raw := range rawTools {
		var tool Tool
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
		if isKnownNativeToolType(fragment.Type) {
			continue
		}
		count++
	}
	return count
}

func isKnownNativeToolType(toolType string) bool {
	switch toolType {
	case "function", "image_generation", "web_search", "custom", "namespace", "tool_search", "file_search", "mcp":
		return true
	default:
		return false
	}
}

func countUnknownInputFragments(fragments []llm.OpenAIResponsesRawFragment) int {
	count := 0
	for _, fragment := range fragments {
		if fragment.Type == "additional_tools" || isStructurallyRepresentedInputItem(fragment.Type) {
			continue
		}
		count++
	}
	return count
}

func mergeRawUnknownTopLevelFields(obj map[string]json.RawMessage, rawFields map[string]json.RawMessage) {
	if len(rawFields) == 0 {
		return
	}

	for key, value := range rawFields {
		if _, exists := obj[key]; exists {
			continue
		}
		obj[key] = cloneRaw(value)
	}
}

func mergeNativeTools(structuredRaw json.RawMessage, requestExt *llm.OpenAIResponsesRequestExtensions) ([]json.RawMessage, bool) {
	if requestExt == nil {
		return nil, false
	}

	var structuredTools []json.RawMessage
	if len(structuredRaw) > 0 {
		if err := json.Unmarshal(structuredRaw, &structuredTools); err != nil {
			return nil, false
		}
	}

	if requestExt.NativeTools != nil && len(requestExt.NativeTools.Raw) > 0 && structuredToolSignaturesMatch(structuredTools, requestExt.NativeTools.Signatures) {
		if tools, ok := mergeNativeToolRawWithStructured(requestExt.NativeTools.Raw, structuredTools); ok {
			return tools, true
		}
	}

	return mergeRawOnlyTools(structuredRaw, requestExt)
}

func mergeNativeToolRawWithStructured(nativeRaw []json.RawMessage, structuredTools []json.RawMessage) ([]json.RawMessage, bool) {
	tools := make([]json.RawMessage, 0, len(nativeRaw))
	structuredIndex := 0

	for _, raw := range nativeRaw {
		var native Tool
		if err := json.Unmarshal(raw, &native); err != nil {
			return nil, false
		}

		if native.Type == "namespace" {
			tools = append(tools, cloneRaw(raw))
			structuredIndex += representedToolCount(native)
			continue
		}

		if !isStructurallyRepresentedToolType(native.Type) {
			tools = append(tools, cloneRaw(raw))
			continue
		}

		if structuredIndex >= len(structuredTools) {
			return nil, false
		}
		merged, ok := mergeNativeToolRawWithStructuredTool(raw, structuredTools[structuredIndex])
		if !ok {
			return nil, false
		}
		tools = append(tools, merged)
		structuredIndex++
	}

	if structuredIndex != len(structuredTools) {
		return nil, false
	}

	return tools, true
}

func representedToolCount(tool Tool) int {
	if tool.Type != "namespace" {
		if isStructurallyRepresentedToolType(tool.Type) {
			return 1
		}
		return 0
	}

	count := 0
	for _, subTool := range tool.Tools {
		if subTool.Type == "function" {
			count++
		}
	}
	return count
}

func mergeNativeToolRawWithStructuredTool(nativeRaw, structuredRaw json.RawMessage) (json.RawMessage, bool) {
	var nativeObj map[string]json.RawMessage
	if err := json.Unmarshal(nativeRaw, &nativeObj); err != nil {
		return nil, false
	}

	var structuredObj map[string]json.RawMessage
	if err := json.Unmarshal(structuredRaw, &structuredObj); err != nil {
		return nil, false
	}

	for key, value := range structuredObj {
		nativeObj[key] = cloneRaw(value)
	}

	merged, err := json.Marshal(nativeObj)
	if err != nil {
		return nil, false
	}
	return merged, true
}

func mergeRawOnlyInputItems(structuredRaw json.RawMessage, requestExt *llm.OpenAIResponsesRequestExtensions) ([]json.RawMessage, bool) {
	if requestExt == nil || len(requestExt.RawInputItems) == 0 {
		return nil, false
	}

	var structuredItems []json.RawMessage
	if len(structuredRaw) > 0 {
		if err := json.Unmarshal(structuredRaw, &structuredItems); err != nil {
			return nil, false
		}
	}

	// PrependCount is the number of messages the prompt pipeline prepended
	// (head) to the canonical request between inbound and outbound. Prepended
	// messages become outbound structured items at the head, so every original
	// input position shifts right by that many slots. Offset raw-only items by
	// PrependCount so they keep their position relative to the user's
	// structured items instead of landing ahead of the injected prepend.
	// Append-only injection grows the tail and does not shift original
	// positions, so it is intentionally not counted here.
	prependCount := requestExt.PrependCount
	if prependCount < 0 {
		prependCount = 0
	}

	total := len(structuredItems) + len(requestExt.RawInputItems)
	items := make([]json.RawMessage, 0, total)
	structuredIndex := 0
	rawByIndex := make(map[int]json.RawMessage, len(requestExt.RawInputItems))
	for _, fragment := range requestExt.RawInputItems {
		if len(fragment.Raw) == 0 || fragment.OriginalIndex < 0 {
			return nil, false
		}
		rawByIndex[fragment.OriginalIndex+prependCount] = cloneRaw(fragment.Raw)
	}

	for i := 0; i < total; i++ {
		if raw, ok := rawByIndex[i]; ok {
			items = append(items, raw)
			continue
		}
		if structuredIndex >= len(structuredItems) {
			return nil, false
		}
		items = append(items, cloneRaw(structuredItems[structuredIndex]))
		structuredIndex++
	}

	if structuredIndex != len(structuredItems) {
		return nil, false
	}

	return items, true
}

func mergeRawOnlyTools(structuredRaw json.RawMessage, requestExt *llm.OpenAIResponsesRequestExtensions) ([]json.RawMessage, bool) {
	if requestExt == nil || len(requestExt.RawTools) == 0 {
		return nil, false
	}

	var structuredTools []json.RawMessage
	if len(structuredRaw) > 0 {
		if err := json.Unmarshal(structuredRaw, &structuredTools); err != nil {
			return nil, false
		}
	}
	if !structuredToolSignaturesMatch(structuredTools, requestExt.ToolSignatures) {
		return nil, false
	}

	total := len(structuredTools) + len(requestExt.RawTools)
	tools := make([]json.RawMessage, 0, total)
	structuredIndex := 0
	rawByIndex := make(map[int]json.RawMessage, len(requestExt.RawTools))
	for _, fragment := range requestExt.RawTools {
		if len(fragment.Raw) == 0 || fragment.OriginalIndex < 0 {
			return nil, false
		}
		rawByIndex[fragment.OriginalIndex] = cloneRaw(fragment.Raw)
	}

	for i := 0; i < total; i++ {
		if raw, ok := rawByIndex[i]; ok {
			tools = append(tools, raw)
			continue
		}
		if structuredIndex >= len(structuredTools) {
			return nil, false
		}
		tools = append(tools, cloneRaw(structuredTools[structuredIndex]))
		structuredIndex++
	}

	if structuredIndex != len(structuredTools) {
		return nil, false
	}

	return tools, true
}

func structuredToolSignaturesMatch(structuredTools []json.RawMessage, expected []string) bool {
	if len(structuredTools) != len(expected) {
		return false
	}

	for i, rawTool := range structuredTools {
		var tool Tool
		if err := json.Unmarshal(rawTool, &tool); err != nil {
			return false
		}
		if responseToolSignature(tool) != expected[i] {
			return false
		}
	}

	return true
}

func rawToolChoiceMatchesCurrentTools(raw json.RawMessage, current *ToolChoice) bool {
	if current == nil {
		return true
	}

	var rawChoice ToolChoice
	if err := json.Unmarshal(raw, &rawChoice); err != nil {
		return false
	}

	currentSignature := toolChoiceSignature(current)
	if currentSignature == "" {
		return true
	}

	return toolChoiceSignature(&rawChoice) == currentSignature
}

func toolChoiceSignature(choice *ToolChoice) string {
	if choice == nil {
		return ""
	}

	if choice.Mode != nil {
		return "mode:" + *choice.Mode
	}

	if choice.Type != nil && choice.Name != nil {
		return "named:" + *choice.Type + ":" + *choice.Name
	}

	if len(choice.Tools) > 0 {
		return "tools"
	}

	return ""
}

func cloneRaw(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}

	return append(json.RawMessage(nil), src...)
}
