package anthropic

import (
	"encoding/json"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/axon/agent"
)

func convertMessages(messages []agent.Message) ([]anthropic.TextBlockParam, []anthropic.MessageParam) {
	var system []anthropic.TextBlockParam
	var params []anthropic.MessageParam

	processedRoundIndex := make(map[int]bool)

	for _, msg := range messages {
		switch msg.Role {
		case agent.RoleSystem:
			if msg.Content == nil {
				continue
			}
			if msg.Content.Text != nil {
				system = append(system, anthropic.TextBlockParam{Text: *msg.Content.Text})
			} else {
				for _, part := range msg.Content.Parts {
					if part.Type == agent.ContentPartText {
						system = append(system, anthropic.TextBlockParam{Text: part.Text})
					}
				}
			}
		case agent.RoleUser:
			params = append(params, anthropic.NewUserMessage(contentToBlocks(msg)...))
		case agent.RoleAssistant:
			if processedRoundIndex[msg.RoundIndex] {
				continue
			}

			blocks := aggregateAssistantBlocks(messages, msg.RoundIndex)
			params = append(params, anthropic.NewAssistantMessage(blocks...))
			processedRoundIndex[msg.RoundIndex] = true
		case agent.RoleTool:
			isError := msg.IsError != nil && *msg.IsError
			content := ""
			if msg.Content != nil {
				content = msg.Content.String()
			}
			toolUseID := ""
			if msg.ToolUseID != nil {
				toolUseID = *msg.ToolUseID
			}
			params = append(params, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(toolUseID, content, isError),
			))
		}
	}

	return system, params
}

func aggregateAssistantBlocks(messages []agent.Message, roundIndex int) []anthropic.ContentBlockParamUnion {
	var thinkingBlocks []anthropic.ContentBlockParamUnion
	var textBlocks []anthropic.ContentBlockParamUnion
	var toolUseBlocks []anthropic.ContentBlockParamUnion

	for _, msg := range messages {
		if msg.Role != agent.RoleAssistant {
			continue
		}

		if msg.RoundIndex != roundIndex {
			continue
		}

		if msg.Content != nil {
			if msg.Content.Text != nil {
				textBlocks = append(textBlocks, anthropic.NewTextBlock(*msg.Content.Text))
			} else {
				for _, part := range msg.Content.Parts {
					switch part.Type {
					case agent.ContentPartText:
						textBlocks = append(textBlocks, anthropic.NewTextBlock(part.Text))
					case agent.ContentPartThinking:
						thinkingBlocks = append(thinkingBlocks, anthropic.NewThinkingBlock(part.ThinkingSignature, part.Thinking))
					case agent.ContentPartRedactedThinking:
						thinkingBlocks = append(thinkingBlocks, anthropic.NewRedactedThinkingBlock(part.Data))
					}
				}
			}
		}

		if msg.ToolUse != nil {
			var input any
			if msg.ToolUse.Input != "" {
				_ = json.Unmarshal([]byte(msg.ToolUse.Input), &input)
			}
			toolUseBlocks = append(toolUseBlocks, anthropic.NewToolUseBlock(msg.ToolUse.ID, input, msg.ToolUse.Name))
		}
	}

	var blocks []anthropic.ContentBlockParamUnion
	blocks = append(blocks, thinkingBlocks...)
	blocks = append(blocks, textBlocks...)
	blocks = append(blocks, toolUseBlocks...)
	return blocks
}

func contentToBlocks(msg agent.Message) []anthropic.ContentBlockParamUnion {
	if msg.Content == nil {
		return nil
	}

	if msg.Content.Text != nil {
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(*msg.Content.Text)}
	}

	return lo.Map(msg.Content.Parts, func(part agent.ContentPart, _ int) anthropic.ContentBlockParamUnion {
		switch part.Type {
		case agent.ContentPartText:
			return anthropic.NewTextBlock(part.Text)
		case agent.ContentPartImage:
			if part.URL != "" {
				return anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: part.URL})
			}
			return anthropic.NewImageBlock(anthropic.Base64ImageSourceParam{
				Data:      part.Data,
				MediaType: anthropic.Base64ImageSourceMediaType(part.MimeType),
			})
		default:
			return anthropic.NewTextBlock(part.Text)
		}
	})
}

func assistantToBlocks(msg agent.Message) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	if msg.ToolUse != nil {
		var input any
		if msg.ToolUse.Input != "" {
			_ = json.Unmarshal([]byte(msg.ToolUse.Input), &input)
		}
		blocks = append(blocks, anthropic.NewToolUseBlock(msg.ToolUse.ID, input, msg.ToolUse.Name))
	}

	if msg.Content != nil {
		if msg.Content.Text != nil {
			blocks = append(blocks, anthropic.NewTextBlock(*msg.Content.Text))
		} else {
			for _, part := range msg.Content.Parts {
				switch part.Type {
				case agent.ContentPartText:
					blocks = append(blocks, anthropic.NewTextBlock(part.Text))
				case agent.ContentPartThinking:
					blocks = append(blocks, anthropic.NewThinkingBlock(part.ThinkingSignature, part.Thinking))
				case agent.ContentPartRedactedThinking:
					blocks = append(blocks, anthropic.NewRedactedThinkingBlock(part.Data))
				}
			}
		}
	}

	return blocks
}

func convertTools(tools []agent.ToolDefinition) []anthropic.ToolUnionParam {
	return lo.Map(tools, func(t agent.ToolDefinition, _ int) anthropic.ToolUnionParam {
		schemaBytes, _ := json.Marshal(t.Parameters)
		var properties any
		var required []string

		var schema struct {
			Properties any      `json:"properties"`
			Required   []string `json:"required"`
		}
		if json.Unmarshal(schemaBytes, &schema) == nil {
			properties = schema.Properties
			required = schema.Required
		}

		return anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
					Required:   required,
				},
			},
		}
	})
}

func convertResponse(resp *anthropic.Message) agent.Response {
	var msgs []agent.Message
	var parts []agent.ContentPart

	for _, block := range resp.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			parts = append(parts, agent.ContentPart{
				Type: agent.ContentPartText,
				Text: variant.Text,
			})
		case anthropic.ThinkingBlock:
			parts = append(parts, agent.ContentPart{
				Type:              agent.ContentPartThinking,
				Thinking:          variant.Thinking,
				ThinkingSignature: variant.Signature,
			})
		case anthropic.RedactedThinkingBlock:
			parts = append(parts, agent.ContentPart{
				Type: agent.ContentPartRedactedThinking,
				Data: variant.Data,
			})
		case anthropic.ToolUseBlock:
			msgs = append(msgs, agent.Message{
				Role: agent.RoleAssistant,
				ToolUse: &agent.ToolUse{
					ID:    variant.ID,
					Name:  variant.Name,
					Input: string(variant.Input),
				},
			})
		}
	}

	if len(parts) > 0 {
		contentMsg := agent.Message{
			Role:    agent.RoleAssistant,
			Content: &agent.Content{Parts: parts},
		}
		msgs = append([]agent.Message{contentMsg}, msgs...)
	}

	return agent.Response{
		Messages:   msgs,
		StopReason: convertStopReason(resp.StopReason),
		Usage: agent.Usage{
			InputTokens:  int(resp.Usage.InputTokens),
			OutputTokens: int(resp.Usage.OutputTokens),
		},
	}
}

func convertStopReason(reason anthropic.StopReason) agent.StopReason {
	switch reason {
	case anthropic.StopReasonEndTurn:
		return agent.StopReasonEndTurn
	case anthropic.StopReasonToolUse:
		return agent.StopReasonToolUse
	case anthropic.StopReasonMaxTokens:
		return agent.StopReasonMaxTokens
	default:
		return agent.StopReasonEndTurn
	}
}
