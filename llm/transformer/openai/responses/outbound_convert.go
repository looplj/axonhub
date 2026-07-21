package responses

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func convertToTextOptions(chatReq *llm.Request) *TextOptions {
	if chatReq == nil {
		return nil
	}

	// Return nil if neither ResponseFormat nor TextVerbosity is set
	if chatReq.ResponseFormat == nil && chatReq.Verbosity == nil {
		return nil
	}

	result := &TextOptions{
		Verbosity: chatReq.Verbosity,
	}

	if chatReq.ResponseFormat != nil {
		result.Format = &TextFormat{
			Type: chatReq.ResponseFormat.Type,
		}

		// Extract name, schema, strict, and description from json_schema
		if chatReq.ResponseFormat.Type == "json_schema" && len(chatReq.ResponseFormat.JSONSchema) > 0 {
			var jsonSchema rawJSONSchema
			if err := json.Unmarshal(chatReq.ResponseFormat.JSONSchema, &jsonSchema); err == nil {
				result.Format.Name = jsonSchema.Name
				result.Format.Description = jsonSchema.Description
				result.Format.Schema = jsonSchema.Schema
				result.Format.Strict = jsonSchema.Strict
			}
		}
	}

	return result
}

// extractPromptFromMessages tries to extract a concise prompt string from the
// request messages, preferring the last user message. If multiple text parts
// exist, they are concatenated with newlines.
func convertInstructionsFromMessages(msgs []llm.Message) string {
	if len(msgs) == 0 {
		return ""
	}

	var instructions []string

	// find the last user message
	for _, msg := range msgs {
		if msg.Role != "system" {
			continue
		}
		// Collect text from either the simple string content or parts
		if msg.Content.Content != nil {
			instructions = append(instructions, *msg.Content.Content)
		}

		if len(msg.Content.MultipleContent) > 0 {
			var b strings.Builder

			for _, p := range msg.Content.MultipleContent {
				if p.Type == "text" && p.Text != nil {
					if b.Len() > 0 {
						b.WriteString("\n")
					}

					b.WriteString(*p.Text)
				}
			}

			if b.Len() > 0 {
				instructions = append(instructions, b.String())
			}
		}
	}

	return strings.Join(instructions, "\n")
}

// convertInputFromMessages converts LLM messages to Responses API Input format.
// User messages become items with content array containing input_text items.
// Assistant messages become items with type "message" and content array containing output_text items.
// Tool calls become function_call items, tool results become function_call_output items.
func convertInputFromMessages(msgs []llm.Message, transformOptions llm.TransformOptions, metadata map[string]any) Input {
	if len(msgs) == 0 {
		return Input{}
	}

	wasArrayFormat := transformOptions.ArrayInputs != nil && *transformOptions.ArrayInputs

	if len(msgs) == 1 && msgs[0].Content.Content != nil && !wasArrayFormat {
		return Input{Text: msgs[0].Content.Content}
	}

	var items []Item

	// Track tool call types so tool result messages can be encoded correctly.
	// callID -> item type (function_call_output or custom_tool_call_output)
	toolResultItemTypeByCallID := map[string]string{}

	for _, msg := range msgs {
		switch msg.Role {
		case "user", "developer":
			items = append(items, convertUserMessage(msg))
		case "assistant":
			assistantItems := convertAssistantMessage(msg, metadata)
			items = append(items, assistantItems...)

			// Record tool call types for later tool result encoding.
			for _, it := range assistantItems {
				switch it.Type {
				case "function_call":
					if it.CallID != "" {
						toolResultItemTypeByCallID[it.CallID] = "function_call_output"
					}
				case "custom_tool_call":
					if it.CallID != "" {
						toolResultItemTypeByCallID[it.CallID] = "custom_tool_call_output"
					}
				}
			}
		case "tool":
			itemType := "function_call_output"

			if msg.ToolCallID != nil {
				if mapped, ok := toolResultItemTypeByCallID[*msg.ToolCallID]; ok {
					itemType = mapped
				}
			}

			items = append(items, convertToolMessageWithType(msg, itemType))
		}
	}

	return Input{
		Items: items,
	}
}

// convertUserMessage converts a user message to Responses API Item format.
func convertUserMessage(msg llm.Message) Item {
	var contentItems []Item

	if msg.Content.Content != nil {
		contentItems = append(contentItems, Item{
			Type: "input_text",
			Text: msg.Content.Content,
		})
	} else {
		for _, p := range msg.Content.MultipleContent {
			switch p.Type {
			case "text":
				if p.Text != nil {
					contentItems = append(contentItems, Item{
						Type: "input_text",
						Text: p.Text,
					})
				}
			case "image_url":
				if p.ImageURL != nil {
					contentItems = append(contentItems, Item{
						Type:     "input_image",
						ImageURL: &p.ImageURL.URL,
						Detail:   p.ImageURL.Detail,
					})
				}
			case "input_audio":
				if p.InputAudio != nil {
					contentItems = append(contentItems, Item{
						Type:       "input_audio",
						InputAudio: p.InputAudio,
					})
				}
			case "file":
				if file, ok := inputFileItemFromContentPart(p); ok {
					contentItems = append(contentItems, file)
				}
			case "compaction", "compaction_summary":
				if p.Compact != nil {
					contentItems = append(contentItems, compactionItemFromPart(p, p.Type))
				}
			}
		}
	}

	return Item{
		ID:      msg.ID,
		Type:    "message",
		Role:    msg.Role,
		Content: &Input{Items: contentItems},
	}
}

func inputFileItemFromContentPart(part llm.MessageContentPart) (Item, bool) {
	if part.OpenAIChatFile == nil {
		return Item{}, false
	}

	file := Item{
		Type:     "input_file",
		FileData: part.OpenAIChatFile.FileData,
		FileID:   part.OpenAIChatFile.FileID,
		Filename: part.OpenAIChatFile.Filename,
	}
	if fileURL, ok := part.TransformerMetadata[responsesInputFileURLPartTransformerMetadataKey].(*string); ok {
		file.FileURL = fileURL
	}
	if detail, ok := part.TransformerMetadata[responsesInputFileDetailPartTransformerMetadataKey].(*string); ok {
		file.Detail = detail
	}

	return file, true
}

// convertAssistantMessage converts an assistant message to Responses API Item(s) format.
// Returns multiple items if the message contains tool calls.
func convertAssistantMessage(msg llm.Message, metadata map[string]any) []Item {
	var (
		items         []Item
		toolCallItems []Item
	)

	// Handle reasoning content first.
	// For Requests, reasoning is represented as an `input` item with type="reasoning".
	// The Responses API uses the `summary` field to hold the reasoning summary text.
	//
	// Emit a reasoning item when:
	// 1) ResponseReasoningItemID != nil — Responses-native origin (summary-only or with
	//    encrypted content; empty id means omit id), or
	// 2) encrypted content is present — legacy/common path that already used signature
	//    as the gate.
	// Do NOT emit solely because ReasoningContent is set: that would invent Responses
	// reasoning items for Chat/Anthropic cross-protocol text.
	var encryptedContent *string
	if msg.ReasoningSignature != nil {
		if msg.ResponseReasoningItemID != nil {
			// Responses-native identity is authoritative provenance. encrypted_content
			// is opaque and has no documented prefix contract, so preserve the exact
			// value paired with its item id instead of guessing from ciphertext bytes.
			encryptedContent = msg.ReasoningSignature
		} else {
			// The common signature slot is also used by Anthropic and Gemini. Only the
			// legacy cross-protocol path needs format filtering.
			encryptedContent = shared.DecodeOpenAIEncryptedContent(msg.ReasoningSignature)
		}
	}

	emitResponsesReasoning := msg.ResponseReasoningItemID != nil || encryptedContent != nil
	if emitResponsesReasoning {
		summary := []ReasoningSummary{}
		if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
			summary = append(summary, ReasoningSummary{
				Type: "summary_text",
				Text: *msg.ReasoningContent,
			})
		}

		reasoningItemID := ""
		if msg.ResponseReasoningItemID != nil {
			reasoningItemID = *msg.ResponseReasoningItemID
		}
		// Empty id omits via omitempty. Never fall back to Message.ID.
		items = append(items, Item{
			ID:               reasoningItemID,
			Type:             "reasoning",
			EncryptedContent: encryptedContent,
			Summary:          summary,
		})
	}

	// Handle tool calls
	for _, tc := range msg.ToolCalls {
		if tc.ResponseCustomToolCall != nil {
			toolCallItems = append(toolCallItems, Item{
				ID:        tc.ResponseItemID,
				Type:      "custom_tool_call",
				CallID:    tc.ResponseCustomToolCall.CallID,
				Name:      tc.ResponseCustomToolCall.Name,
				Namespace: tc.ResponseCustomToolCall.Namespace,
				Input:     lo.ToPtr(tc.ResponseCustomToolCall.Input),
			})
		} else if tc.OpenAIChatCustomToolCall != nil {
			// Explicit Chat→Responses custom bridge. Chat custom calls live on
			// OpenAIChatCustomToolCall with an empty Function carrier; do not
			// misclassify them as function_call.
			callID := tc.ID
			toolCallItems = append(toolCallItems, Item{
				ID:     tc.ResponseItemID,
				Type:   "custom_tool_call",
				CallID: callID,
				Name:   tc.OpenAIChatCustomToolCall.Name,
				Input:  lo.ToPtr(tc.OpenAIChatCustomToolCall.Input),
			})
		} else {
			// Restore namespace group identity. If the ToolCall already carries
			// a Namespace (e.g. from a Responses function_call input item),
			// use it directly; otherwise fall back to table lookup for
			// composite names produced by the upstream model.
			fcName := tc.Function.Name
			fcNamespace := tc.Function.Namespace
			if fcNamespace == "" {
				fcName, fcNamespace = resolveNamespaceFromMetadata(metadata, tc.Function.Name)
			}
			toolCallItems = append(toolCallItems, Item{
				// ResponseItemID is the Responses item id; tc.ID is call_id only.
				// Do not fall back to call_id when item id is absent.
				ID:        tc.ResponseItemID,
				Type:      "function_call",
				CallID:    tc.ID,
				Name:      fcName,
				Namespace: fcNamespace,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	var contentItems []Item

	flushMessage := func() {
		if len(contentItems) == 0 {
			return
		}

		items = append(items, Item{
			ID:      msg.ID,
			Type:    "message",
			Role:    msg.Role,
			Status:  lo.ToPtr("completed"),
			Content: &Input{Items: contentItems},
		})
		contentItems = nil
	}

	if msg.Content.Content != nil {
		contentItems = append(contentItems, Item{
			Type: "output_text",
			Text: msg.Content.Content,
		})
	} else {
		for _, p := range msg.Content.MultipleContent {
			switch p.Type {
			case "text":
				if p.Text != nil {
					contentItems = append(contentItems, Item{
						Type: "output_text",
						Text: p.Text,
					})
				}
			case "image_url":
				if p.ImageURL != nil {
					contentItems = append(contentItems, Item{
						Type:     "input_image",
						ImageURL: &p.ImageURL.URL,
						Detail:   p.ImageURL.Detail,
					})
				}
			case "compaction", "compaction_summary":
				if p.Compact != nil {
					flushMessage()

					items = append(items, compactionItemFromPart(p, p.Type))
				}
			}
		}
	}

	// In the common assistant flow, the visible message content precedes any
	// subsequent tool calls. Flush message segments before appending tool-call
	// items so the encoded Responses item order matches that expectation.
	flushMessage()

	items = append(items, toolCallItems...)

	return items
}

func convertToolMessageWithType(msg llm.Message, itemType string) Item {
	var output Input

	// Handle simple content first
	if msg.Content.Content != nil {
		output.Text = msg.Content.Content
	} else if len(msg.Content.MultipleContent) > 0 {
		for _, p := range msg.Content.MultipleContent {
			switch p.Type {
			case "text":
				if p.Text != nil {
					output.Items = append(output.Items, Item{
						Type: "input_text",
						Text: p.Text,
					})
				}
			case "image_url":
				if p.ImageURL != nil {
					output.Items = append(output.Items, Item{
						Type:     "input_image",
						ImageURL: &p.ImageURL.URL,
						Detail:   p.ImageURL.Detail,
					})
				}
			case "input_audio":
				if p.InputAudio != nil {
					output.Items = append(output.Items, Item{
						Type:       "input_audio",
						InputAudio: p.InputAudio,
					})
				}
			case "file":
				if file, ok := inputFileItemFromContentPart(p); ok {
					output.Items = append(output.Items, file)
				}
			}
		}
	}

	// Some times the tool result is empty, so we need to add an empty string.
	if output.Text == nil && len(output.Items) == 0 {
		output.Text = lo.ToPtr("")
	}

	return Item{
		ID:     msg.ID,
		Type:   itemType,
		CallID: lo.FromPtr(msg.ToolCallID),
		Output: &output,
	}
}

func convertImageGenerationToTool(src llm.Tool) Tool {
	tool := Tool{
		Type: "image_generation",
	}
	if src.ImageGeneration != nil {
		tool.Model = src.ImageGeneration.Model
		tool.Background = src.ImageGeneration.Background
		tool.InputFidelity = src.ImageGeneration.InputFidelity
		tool.InputImageMask = src.ImageGeneration.InputImageMask
		tool.Moderation = src.ImageGeneration.Moderation
		tool.OutputCompression = src.ImageGeneration.OutputCompression
		tool.OutputFormat = src.ImageGeneration.OutputFormat
		tool.PartialImages = src.ImageGeneration.PartialImages
		tool.Quality = src.ImageGeneration.Quality
		tool.Size = src.ImageGeneration.Size
	}

	return tool
}

func convertWebSearchToTool(src llm.Tool) Tool {
	tool := Tool{
		Type: "web_search",
	}

	if src.WebSearch == nil {
		return tool
	}

	if len(src.WebSearch.AllowedDomains) > 0 {
		tool.Filters = &WebSearchFilters{
			AllowedDomains: append([]string(nil), src.WebSearch.AllowedDomains...),
		}
	}

	location := src.WebSearch.UserLocation
	if location.Type != "" || location.City != "" || location.Country != "" || location.Region != "" || location.Timezone != "" {
		locationType := location.Type
		if locationType == "" {
			locationType = "approximate"
		}
		tool.UserLocation = &WebSearchUserLocation{
			Type:     locationType,
			City:     location.City,
			Country:  location.Country,
			Region:   location.Region,
			Timezone: location.Timezone,
		}
	}

	return tool
}

// convertCustomToTool converts an llm.Tool custom tool to Responses API Tool format.
func convertCustomToTool(src llm.Tool) Tool {
	tool := Tool{
		Type: "custom",
	}
	if src.ResponseCustomTool != nil {
		tool.Name = src.ResponseCustomTool.Name

		tool.Description = src.ResponseCustomTool.Description
		if src.ResponseCustomTool.Format != nil {
			tool.Format = &CustomToolFormat{
				Type:       src.ResponseCustomTool.Format.Type,
				Syntax:     src.ResponseCustomTool.Format.Syntax,
				Definition: src.ResponseCustomTool.Format.Definition,
			}
		}
	}

	return tool
}

// convertChatCustomToTool bridges Chat Completions custom-tool declarations
// (OpenAIChatCustomTool) into the Responses flat custom tool shape.
// Format JSON is Chat-native (nested grammar object or text); map only known
// fields and do not forward unrelated metadata.
func convertChatCustomToTool(src llm.Tool) Tool {
	tool := Tool{Type: "custom"}
	if src.OpenAIChatCustomTool == nil {
		return tool
	}
	chat := src.OpenAIChatCustomTool
	tool.Name = chat.Name
	if chat.Description != nil {
		tool.Description = *chat.Description
	}
	if len(chat.Format) > 0 {
		tool.Format = convertChatCustomToolFormat(chat.Format)
	}
	return tool
}

func convertChatCustomToolFormat(raw json.RawMessage) *CustomToolFormat {
	if len(raw) == 0 {
		return nil
	}

	// Chat grammar wire: {"type":"grammar","grammar":{"syntax":"...","definition":"..."}}
	var nested struct {
		Type    string `json:"type"`
		Grammar *struct {
			Syntax     string `json:"syntax"`
			Definition string `json:"definition"`
		} `json:"grammar"`
		// Also accept already-flat Responses-like shape for resilience.
		Syntax     string `json:"syntax"`
		Definition string `json:"definition"`
	}
	if err := json.Unmarshal(raw, &nested); err != nil {
		return nil
	}
	if nested.Type == "" {
		return nil
	}
	format := &CustomToolFormat{Type: nested.Type}
	if nested.Type == "grammar" {
		if nested.Grammar != nil {
			format.Syntax = nested.Grammar.Syntax
			format.Definition = nested.Grammar.Definition
		} else {
			format.Syntax = nested.Syntax
			format.Definition = nested.Definition
		}
	}
	return format
}

// convertFunctionToTool converts an llm.Tool function to Responses API Tool format.
func convertFunctionToTool(src llm.Tool) Tool {
	tool := Tool{
		Type:        "function",
		Name:        src.Function.Name,
		Description: src.Function.Description,
		Strict:      src.Function.Strict,
	}

	// Convert parameters from json.RawMessage to map[string]any
	if len(src.Function.Parameters) > 0 {
		var params map[string]any
		if err := json.Unmarshal(src.Function.Parameters, &params); err == nil {
			// Handle nil map panic - initialize if nil
			if params == nil {
				params = map[string]any{}
			}

			// OpenAI rejects object schemas that omit properties entirely.
			// Anthropic clients may send {"type":"object"} for no-arg tools, so normalize that here.
			if typeName, ok := params["type"].(string); ok && typeName == "object" {
				if _, ok := params["properties"].(map[string]any); !ok {
					params["properties"] = map[string]any{}
				}
			}

			// For strict mode, additionalProperties must be false and all properties must be required
			// See: https://platform.openai.com/docs/guides/function-calling#strict-mode
			if src.Function.Strict != nil && *src.Function.Strict {
				// Always set additionalProperties: false for strict validation
				// Overwrite any existing value (including true) to ensure false
				params["additionalProperties"] = false

				// When strict mode is enabled, ALL properties must be listed in "required"
				if props, ok := params["properties"].(map[string]any); ok && len(props) > 0 {
					required := make([]string, 0, len(props))
					// First, check if there's an existing required array and preserve it
					if existingRequired, ok := params["required"].([]any); ok {
						for _, r := range existingRequired {
							if s, ok := r.(string); ok {
								required = append(required, s)
							}
						}
					}
					// Add any missing property keys to required
					requiredSet := make(map[string]bool)
					for _, r := range required {
						requiredSet[r] = true
					}

					for key := range props {
						if !requiredSet[key] {
							required = append(required, key)
						}
					}

					params["required"] = required
				}
			}

			tool.Parameters = params
		}
	}

	return tool
}

// convertToolChoice converts llm.ToolChoice to Responses API ToolChoice.
func convertToolChoice(src *llm.ToolChoice) *ToolChoice {
	if src == nil {
		return nil
	}

	if src.ToolChoice != nil {
		// String mode like "none", "auto", "required"
		return &ToolChoice{Mode: src.ToolChoice}
	}
	if src.NamedToolChoice != nil {
		// Specific tool choice
		return &ToolChoice{
			Type: &src.NamedToolChoice.Type,
			Name: &src.NamedToolChoice.Function.Name,
		}
	}
	if src.OpenAIChatCustomToolChoice != nil {
		toolType := "custom"
		return &ToolChoice{
			Type: &toolType,
			Name: &src.OpenAIChatCustomToolChoice.Name,
		}
	}
	if src.OpenAIChatAllowedTools != nil {
		tools := convertOpenAIChatAllowedTools(src.OpenAIChatAllowedTools.Tools)
		if len(tools) == 0 {
			return nil
		}
		toolType := "allowed_tools"
		mode := src.OpenAIChatAllowedTools.Mode
		return &ToolChoice{
			Mode:  &mode,
			Type:  &toolType,
			Tools: tools,
		}
	}

	return nil
}

func convertOpenAIChatAllowedTools(rawTools []json.RawMessage) []ToolOption {
	tools := make([]ToolOption, 0, len(rawTools))
	for _, rawTool := range rawTools {
		var tool struct {
			Type     string `json:"type"`
			Function *struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if json.Unmarshal(rawTool, &tool) != nil || tool.Type != "function" || tool.Function == nil || tool.Function.Name == "" {
			continue
		}
		tools = append(tools, ToolOption{Type: "function", Name: tool.Function.Name})
	}
	return tools
}

func convertStreamOptions(raw json.RawMessage) *StreamOptions {
	if len(raw) == 0 {
		return nil
	}

	var result StreamOptions
	if err := json.Unmarshal(raw, &result); err != nil || result.IncludeObfuscation == nil {
		return nil
	}

	return &result
}

// convertReasoning converts llm.Request reasoning fields to Responses API Reasoning.
// Only one of "reasoning.effort" and "reasoning.max_tokens" can be specified.
// Priority is given to effort when both are present.
func convertReasoning(req *llm.Request) *Reasoning {
	// Restore reasoning.enabled from TransformerMetadata (OpenRouter
	// ReasoningConfig.enabled has no canonical slot; mirrors top_k handling).
	enabled := xmap.GetBoolPtr(req.TransformerMetadata, responsesReasoningEnabledTransformerMetadataKey)

	contextMode := ""
	if req.ProviderExtensions != nil && req.ProviderExtensions.OpenAIResponses != nil && req.ProviderExtensions.OpenAIResponses.Request != nil {
		contextMode = req.ProviderExtensions.OpenAIResponses.Request.ReasoningContext
	}

	// Check if any reasoning-related fields are present
	generateSummaryMeta := ""
	if req.TransformerMetadata != nil {
		if v, ok := req.TransformerMetadata[responsesReasoningGenerateSummaryValueTransformerMetadataKey].(string); ok {
			generateSummaryMeta = v
		}
	}
	hasReasoningFields := req.ReasoningEffort != "" ||
		req.ReasoningBudget != nil ||
		req.ReasoningSummary != nil ||
		enabled != nil ||
		contextMode != "" ||
		generateSummaryMeta != ""
	if !hasReasoningFields {
		return nil
	}

	reasoning := &Reasoning{
		Effort:    req.ReasoningEffort,
		Context:   contextMode,
		MaxTokens: req.ReasoningBudget,
		Enabled:   enabled,
	}

	// If both effort and budget are specified, prioritize effort as per requirement
	if req.ReasoningEffort != "" && req.ReasoningBudget != nil {
		reasoning.MaxTokens = nil // Ignore max_tokens when effort is specified
	}
	// Restore summary / generate_summary as distinct wire fields when possible.
	generateSummary := ""
	generateOnly := false
	if req.TransformerMetadata != nil {
		if v, ok := req.TransformerMetadata[responsesReasoningGenerateSummaryValueTransformerMetadataKey].(string); ok {
			generateSummary = v
		}
		if origin, ok := req.TransformerMetadata[responsesReasoningGenerateSummaryOriginTransformerMetadataKey].(bool); ok && origin {
			generateOnly = true
		}
	}
	if generateOnly && req.ReasoningSummary != nil {
		reasoning.GenerateSummary = *req.ReasoningSummary
	} else {
		if req.ReasoningSummary != nil {
			reasoning.Summary = *req.ReasoningSummary
		}
		if generateSummary != "" {
			reasoning.GenerateSummary = generateSummary
		}
	}

	return reasoning
}

// restoreCacheControl rebuilds the top-level cache_control directive from the
// opaque json.RawMessage carried in TransformerMetadata (mirrors top_k handling).
func restoreCacheControl(meta map[string]any) *CacheControl {
	if raw, ok := meta[shared.TransformerMetadataKeyCacheControl].(json.RawMessage); ok && len(raw) > 0 {
		var cc CacheControl
		if err := json.Unmarshal(raw, &cc); err == nil && cc.Type != "" {
			return &cc
		}
	}
	return nil
}

func annotationToLLM(a Annotation, textRuneOffset int64) llm.Annotation {
	annotation := llm.Annotation{
		Type: a.Type,
	}

	if a.StartIndex != nil {
		annotation.StartIndex = lo.ToPtr(*a.StartIndex + textRuneOffset)
	}

	if a.EndIndex != nil {
		annotation.EndIndex = lo.ToPtr(*a.EndIndex + textRuneOffset)
	}

	if a.URLCitation != nil {
		annotation.URLCitation = &llm.URLCitation{
			URL:   a.URLCitation.URL,
			Title: a.URLCitation.Title,
		}
	}

	return annotation
}

func appendOutputText(textContent *strings.Builder, visibleTextRuneCount *int64, annotations []llm.Annotation, outputItem Item) []llm.Annotation {
	if outputItem.Text == nil {
		return annotations
	}

	textRuneOffset := *visibleTextRuneCount
	textContent.WriteString(*outputItem.Text)
	*visibleTextRuneCount += int64(utf8.RuneCountInString(*outputItem.Text))

	if len(outputItem.Annotations) == 0 {
		return annotations
	}

	for _, annotation := range outputItem.Annotations {
		annotations = append(annotations, annotationToLLM(annotation, textRuneOffset))
	}

	return annotations
}

func appendResponseWebSearchCallMetadata(transformerMetadata map[string]any, outputItem Item) {
	if transformerMetadata == nil || outputItem.Action == nil || outputItem.Action.WebSearch == nil {
		return
	}

	src := outputItem.Action.WebSearch
	action := &WebSearchAction{
		Type:  src.Type,
		Query: src.Query,
	}
	if len(src.Queries) > 0 {
		action.Queries = append([]string(nil), src.Queries...)
	}
	if len(src.Sources) > 0 {
		action.Sources = append([]WebSearchSource(nil), src.Sources...)
	}

	call := Item{
		ID:     outputItem.ID,
		Type:   outputItem.Type,
		Status: outputItem.Status,
		Action: NewWebSearchAction(action),
	}

	existing, _ := transformerMetadata[responsesWebSearchCallsTransformerMetadataKey].([]Item)
	transformerMetadata[responsesWebSearchCallsTransformerMetadataKey] = append(existing, call)
}

// convertOutputToMessage converts Responses API output items into an llm.Message.
// It aggregates text, reasoning, tool calls, image generation,
// compaction and compaction_summary items from the response output.
func convertOutputToMessage(output []Item, transformerMetadata map[string]any) llm.Message {
	var (
		contentParts         []llm.MessageContentPart
		textContent          strings.Builder
		reasoningContent     strings.Builder
		reasoningSignature   *string
		messageID            string
		toolCalls            []llm.ToolCall
		annotations          []llm.Annotation
		visibleTextRuneCount int64
		refusalText          string
	)

	flushText := func() {
		if textContent.Len() == 0 {
			return
		}

		contentParts = append(contentParts, llm.MessageContentPart{
			Type: "text",
			Text: lo.ToPtr(textContent.String()),
		})
		textContent.Reset()
	}

	for _, outputItem := range output {
		switch outputItem.Type {
		case "message":
			if messageID == "" {
				messageID = outputItem.ID
			}

			if outputItem.Content == nil {
				continue
			}
			for _, contentItem := range outputItem.Content.Items {
				switch contentItem.Type {
				case "output_text":
					annotations = appendOutputText(&textContent, &visibleTextRuneCount, annotations, contentItem)
				case "refusal":
					if contentItem.Refusal != nil && *contentItem.Refusal != "" {
						refusalText = *contentItem.Refusal
					}
				}
			}
		case "output_text":
			annotations = appendOutputText(&textContent, &visibleTextRuneCount, annotations, outputItem)
		case "function_call":
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:             outputItem.CallID,
				ResponseItemID: outputItem.ID,
				Status:         lo.FromPtr(outputItem.Status),
				Type:           "function",
				Function: llm.FunctionCall{
					Name:      outputItem.Name,
					Namespace: outputItem.Namespace,
					Arguments: outputItem.Arguments,
				},
			})
		case "custom_tool_call":
			inputStr := ""
			if outputItem.Input != nil {
				inputStr = *outputItem.Input
			}

			toolCalls = append(toolCalls, llm.ToolCall{
				ID:             outputItem.CallID,
				ResponseItemID: outputItem.ID,
				Status:         lo.FromPtr(outputItem.Status),
				Type:           llm.ToolTypeResponsesCustomTool,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{
					CallID:    outputItem.CallID,
					Name:      outputItem.Name,
					Namespace: outputItem.Namespace,
					Input:     inputStr,
				},
			})
		case "reasoning":
			// Prefer content[]/reasoning_text; fall back to summary_text.
			hasReasoningText := false
			for _, part := range outputItem.ReasoningContent {
				if part.Text == "" {
					continue
				}
				reasoningContent.WriteString(part.Text)
				hasReasoningText = true
			}
			if !hasReasoningText {
				for _, summary := range outputItem.Summary {
					reasoningContent.WriteString(summary.Text)
				}
			}
			if transformerMetadata != nil {
				if len(outputItem.ReasoningContent) > 0 {
					transformerMetadata[responsesReasoningTextContentTransformerMetadataKey] = append([]ReasoningContent(nil), outputItem.ReasoningContent...)
				}
				if len(outputItem.Summary) > 0 {
					transformerMetadata[responsesReasoningSummaryContentTransformerMetadataKey] = append([]ReasoningSummary(nil), outputItem.Summary...)
				}
			}

			if outputItem.EncryptedContent != nil && *outputItem.EncryptedContent != "" {
				reasoningSignature = shared.EncodeOpenAIEncryptedContent(outputItem.EncryptedContent)
			}
		case "image_generation_call":
			flushText()

			imageOutputFormat := "png"

			if transformerMetadata != nil {
				if imgFmt, ok := transformerMetadata[responsesImageOutputFormatTransformerMetadataKey].(string); ok && imgFmt != "" {
					imageOutputFormat = imgFmt
				}
			}

			if outputItem.Result != nil && *outputItem.Result != "" {
				contentParts = append(contentParts, llm.MessageContentPart{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: `data:image/` + imageOutputFormat + `;base64,` + *outputItem.Result,
					},
					TransformerMetadata: map[string]any{
						responsesBackgroundTransformerMetadataKey:           outputItem.Background,
						responsesImageGenOutputFormatTransformerMetadataKey: outputItem.OutputFormat,
						responsesImageGenQualityTransformerMetadataKey:      outputItem.Quality,
						responsesImageGenSizeTransformerMetadataKey:         outputItem.Size,
					},
				})
			}
		case "web_search_call":
			appendResponseWebSearchCallMetadata(transformerMetadata, outputItem)
		case "compaction", "compaction_summary":
			flushText()

			encryptedContent := ""
			if outputItem.EncryptedContent != nil {
				encryptedContent = *outputItem.EncryptedContent
			}

			contentParts = append(contentParts, llm.MessageContentPart{
				Type: outputItem.Type,
				Compact: &llm.CompactContent{
					ID:               outputItem.ID,
					EncryptedContent: encryptedContent,
					CreatedBy:        outputItem.CreatedBy,
				},
			})
		case "input_image":
			flushText()

			if outputItem.ImageURL != nil && *outputItem.ImageURL != "" {
				contentParts = append(contentParts, llm.MessageContentPart{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: *outputItem.ImageURL,
					},
				})
			}
		}
	}

	flushText()

	msg := llm.Message{
		ID:          messageID,
		Role:        "assistant",
		ToolCalls:   toolCalls,
		Annotations: annotations,
		Refusal:     refusalText,
	}

	if reasoningContent.Len() > 0 {
		msg.ReasoningContent = lo.ToPtr(reasoningContent.String())
	}

	if reasoningSignature != nil {
		msg.ReasoningSignature = reasoningSignature
	}
	if len(contentParts) == 1 && contentParts[0].Type == "text" && len(toolCalls) == 0 {
		msg.Content = llm.MessageContent{
			Content: contentParts[0].Text,
		}
	} else if len(contentParts) > 0 {
		msg.Content = llm.MessageContent{
			MultipleContent: contentParts,
		}
	}

	return msg
}
