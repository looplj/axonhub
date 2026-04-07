package responses

import (
	"encoding/json"
	"strings"

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
func convertInputFromMessages(msgs []llm.Message, transformOptions llm.TransformOptions, scope shared.TransportScope) Input {
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
			assistantItems := convertAssistantMessage(msg, scope)
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
			case "compaction", "compaction_summary":
				if p.Compact != nil {
					contentItems = append(contentItems, compactionItemFromPart(p, p.Type))
				}
			}
		}
	}

	return Item{
		Type:    "message",
		Role:    msg.Role,
		Content: &Input{Items: contentItems},
	}
}

// convertAssistantMessage converts an assistant message to Responses API Item(s) format.
// Returns multiple items if the message contains tool calls.
func convertAssistantMessage(msg llm.Message, scope shared.TransportScope) []Item {
	var items []Item
	var toolCallItems []Item

	// Handle reasoning content first.
	// For Requests, reasoning is represented as an `input` item with type="reasoning".
	// The Responses API uses the `summary` field to hold the reasoning summary text.
	var encryptedContent *string
	if msg.ReasoningSignature != nil {
		encryptedContent = shared.DecodeOpenAIEncryptedContentInScope(msg.ReasoningSignature, scope)
	}

	if encryptedContent != nil {
		summary := []ReasoningSummary{}
		if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
			summary = append(summary, ReasoningSummary{
				Type: "summary_text",
				Text: *msg.ReasoningContent,
			})
		}

		items = append(items, Item{
			Type:             "reasoning",
			EncryptedContent: encryptedContent,
			Summary:          summary,
		})
	}

	// Handle tool calls
	for _, tc := range msg.ToolCalls {
		if tc.ResponseCustomToolCall != nil {
			toolCallItems = append(toolCallItems, Item{
				Type:   "custom_tool_call",
				CallID: tc.ResponseCustomToolCall.CallID,
				Name:   tc.ResponseCustomToolCall.Name,
				Input:  lo.ToPtr(tc.ResponseCustomToolCall.Input),
			})
		} else {
			toolCallItems = append(toolCallItems, Item{
				Type:      "function_call",
				CallID:    tc.ID,
				Name:      tc.Function.Name,
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
			if p.Type == "text" && p.Text != nil {
				output.Items = append(output.Items, Item{
					Type: "input_text",
					Text: p.Text,
				})
			}
		}
	}

	// Some times the tool result is empty, so we need to add an empty string.
	if output.Text == nil && len(output.Items) == 0 {
		output.Text = lo.ToPtr("")
	}

	return Item{
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
		tool.Moderation = src.ImageGeneration.Moderation
		tool.OutputCompression = src.ImageGeneration.OutputCompression
		tool.OutputFormat = src.ImageGeneration.OutputFormat
		tool.PartialImages = src.ImageGeneration.PartialImages
		tool.Quality = src.ImageGeneration.Quality
		tool.Size = src.ImageGeneration.Size
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

	result := &ToolChoice{}

	if src.ToolChoice != nil {
		// String mode like "none", "auto", "required"
		result.Mode = src.ToolChoice
	} else if src.NamedToolChoice != nil {
		// Specific tool choice
		result.Type = &src.NamedToolChoice.Type
		result.Name = &src.NamedToolChoice.Function.Name
	}

	return result
}

// convertStreamOptions converts llm.StreamOptions to Responses API StreamOptions.
// IncludeObfuscation is read from TransformerMetadata since it's a Responses API specific field.
func convertStreamOptions(src *llm.StreamOptions, metadata map[string]any) *StreamOptions {
	if src == nil {
		return nil
	}

	includeObfuscation := xmap.GetBoolPtr(metadata, "include_obfuscation")
	if includeObfuscation == nil {
		return nil
	}

	return &StreamOptions{
		IncludeObfuscation: includeObfuscation,
	}
}

// convertReasoning converts llm.Request reasoning fields to Responses API Reasoning.
// Only one of "reasoning.effort" and "reasoning.max_tokens" can be specified.
// Priority is given to effort when both are present.
func convertReasoning(req *llm.Request) *Reasoning {
	// Check if any reasoning-related fields are present
	hasReasoningFields := req.ReasoningEffort != "" ||
		req.ReasoningBudget != nil ||
		req.ReasoningSummary != nil
	if !hasReasoningFields {
		return nil
	}

	reasoning := &Reasoning{
		Effort:    req.ReasoningEffort,
		MaxTokens: req.ReasoningBudget,
	}

	// Handle summary field (generate_summary is already merged at inbound)
	if req.ReasoningSummary != nil {
		reasoning.Summary = *req.ReasoningSummary
	}

	// If both effort and budget are specified, prioritize effort as per requirement
	if req.ReasoningEffort != "" && req.ReasoningBudget != nil {
		reasoning.MaxTokens = nil // Ignore max_tokens when effort is specified
	}

	return reasoning
}

// convertOutputToMessage converts Responses API output items into an llm.Message.
// It aggregates text, reasoning, tool calls, image generation,
// compaction and compaction_summary items from the response output.
func convertOutputToMessage(output []Item, scope shared.TransportScope, transformerMetadata map[string]any) llm.Message {
	var (
		contentParts       []llm.MessageContentPart
		textContent        strings.Builder
		reasoningContent   strings.Builder
		reasoningSignature *string
		messageID          string
		toolCalls          []llm.ToolCall
	)

	appendText := func(text string) {
		if text == "" {
			return
		}
		textContent.WriteString(text)
	}

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
			for _, contentItem := range outputItem.GetContentItems() {
				if contentItem.Type == "output_text" {
					appendText(contentItem.Text)
				}
			}
		case "output_text":
			if outputItem.Text != nil {
				appendText(*outputItem.Text)
			}
		case "function_call":
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   outputItem.CallID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      outputItem.Name,
					Arguments: outputItem.Arguments,
				},
			})
		case "custom_tool_call":
			inputStr := ""
			if outputItem.Input != nil {
				inputStr = *outputItem.Input
			}
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   outputItem.CallID,
				Type: llm.ToolTypeResponsesCustomTool,
				ResponseCustomToolCall: &llm.ResponseCustomToolCall{
					CallID: outputItem.CallID,
					Name:   outputItem.Name,
					Input:  inputStr,
				},
			})
		case "reasoning":
			for _, summary := range outputItem.Summary {
				reasoningContent.WriteString(summary.Text)
			}
			if outputItem.EncryptedContent != nil && *outputItem.EncryptedContent != "" {
				reasoningSignature = shared.EncodeOpenAIEncryptedContentInScope(outputItem.EncryptedContent, scope)
			}
		case "image_generation_call":
			flushText()
			imageOutputFormat := "png"
			if transformerMetadata != nil {
				if imgFmt, ok := transformerMetadata["image_output_format"].(string); ok && imgFmt != "" {
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
						"background":    outputItem.Background,
						"output_format": outputItem.OutputFormat,
						"quality":       outputItem.Quality,
						"size":          outputItem.Size,
					},
				})
			}
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
		ID:        messageID,
		Role:      "assistant",
		ToolCalls: toolCalls,
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
