package responses

import (
	"encoding/json"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/pkg/xmap"
	"github.com/looplj/axonhub/llm"
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
		if msg.Role != "system" && msg.Role != "developer" {
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
func convertInputFromMessages(msgs []llm.Message, transformOptions llm.TransformOptions) Input {
	if len(msgs) == 0 {
		return Input{}
	}

	wasArrayFormat := transformOptions.ArrayInputs != nil && *transformOptions.ArrayInputs

	if len(msgs) == 1 && msgs[0].Content.Content != nil && !wasArrayFormat {
		return Input{Text: msgs[0].Content.Content}
	}

	var items []Item

	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			items = append(items, convertUserMessage(msg))
		case "assistant":
			items = append(items, convertAssistantMessage(msg)...)
		case "tool":
			items = append(items, convertToolMessage(msg))
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
		Role:    msg.Role,
		Content: &Input{Items: contentItems},
	}
}

// convertAssistantMessage converts an assistant message to Responses API Item(s) format.
// Returns multiple items if the message contains tool calls.
func convertAssistantMessage(msg llm.Message) []Item {
	var items []Item

	// Handle tool calls
	for _, tc := range msg.ToolCalls {
		items = append(items, Item{
			Type:      "function_call",
			CallID:    tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
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

// convertToolMessage converts a tool result message to Responses API Item format.
func convertToolMessage(msg llm.Message) Item {
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
		Type:   "function_call_output",
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
	if req.ReasoningEffort == "" && req.ReasoningBudget == nil {
		return nil
	}

	// If both effort and budget are specified, prioritize effort as per requirement
	if req.ReasoningEffort != "" && req.ReasoningBudget != nil {
		return &Reasoning{
			Effort:    req.ReasoningEffort,
			MaxTokens: nil, // Ignore max_tokens when effort is specified
		}
	}

	return &Reasoning{
		Effort:    req.ReasoningEffort,
		MaxTokens: req.ReasoningBudget,
	}
}
