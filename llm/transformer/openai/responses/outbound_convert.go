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
			items = append(items, Item{
				Type:   "custom_tool_call",
				CallID: tc.ResponseCustomToolCall.CallID,
				Name:   tc.ResponseCustomToolCall.Name,
				Input:  lo.ToPtr(tc.ResponseCustomToolCall.Input),
			})
		} else {
			items = append(items, Item{
				Type:      "function_call",
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}
	}

	var contentItems []Item

	if msg.Content.Content != nil {
		contentItems = append(contentItems, Item{
			Type:        "output_text",
			Text:        msg.Content.Content,
			Annotations: []Annotation{},
		})
	} else {
		for _, p := range msg.Content.MultipleContent {
			if p.Type == "text" && p.Text != nil {
				contentItems = append(contentItems, Item{
					Type:        "output_text",
					Text:        p.Text,
					Annotations: []Annotation{},
				})
			}
		}
	}

	if len(contentItems) > 0 {
		items = append(items, Item{
			Type:    "message",
			Role:    msg.Role,
			Status:  lo.ToPtr("completed"),
			Content: &Input{Items: contentItems},
		})
	}

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
