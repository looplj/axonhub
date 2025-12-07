package responses

import (
	"encoding/json"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
)

func convertToTextOptions(chatReq *llm.Request) *TextOptions {
	if chatReq == nil || chatReq.ResponseFormat == nil {
		return nil
	}

	return &TextOptions{
		Format: &TextFormat{
			Type: chatReq.ResponseFormat.Type,
		},
	}
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

// convertInputFromMessages tries to extract a concise prompt string from the
// request messages, preferring the last user message. If multiple text parts
// exist, they are concatenated with newlines.
func convertInputFromMessages(msgs []llm.Message) Input {
	if len(msgs) == 0 {
		return Input{}
	}

	if len(msgs) == 1 && msgs[0].Content.Content != nil {
		return Input{Text: msgs[0].Content.Content}
	}

	var items []Item

	for _, msg := range msgs {
		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}
		// Collect text from either the simple string content or parts
		if msg.Content.Content != nil {
			items = append(items, Item{
				Type: "message",
				Role: msg.Role,
				Text: msg.Content.Content,
			})
		} else {
			for _, p := range msg.Content.MultipleContent {
				switch p.Type {
				case "text":
					if p.Text != nil {
						items = append(items, Item{
							Type: "message",
							Role: msg.Role,
							Text: p.Text,
						})
					}
				case "image_url":
					if p.ImageURL != nil {
						items = append(items, Item{
							Role:     msg.Role,
							Type:     "input_image",
							ImageURL: &p.ImageURL.URL,
							Detail:   p.ImageURL.Detail,
						})
					}
				}
			}
		}
	}

	return Input{
		Items: items,
	}
}

func convertImageGenerationToTool(src llm.Tool) Tool {
	tool := Tool{
		Type: "image_generation",
	}
	if src.ImageGeneration != nil {
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
func convertStreamOptions(src *llm.StreamOptions) *StreamOptions {
	if src == nil {
		return nil
	}

	return &StreamOptions{
		IncludeObfuscation: nil, // llm.StreamOptions doesn't have this field
	}
}

// convertReasoning converts llm.Request reasoning fields to Responses API Reasoning.
func convertReasoning(req *llm.Request) *Reasoning {
	if req.ReasoningEffort == "" && req.ReasoningBudget == nil {
		return nil
	}

	return &Reasoning{
		Effort:    req.ReasoningEffort,
		MaxTokens: req.ReasoningBudget,
	}
}
