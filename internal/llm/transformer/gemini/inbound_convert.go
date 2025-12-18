package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	geminioai "github.com/looplj/axonhub/internal/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/shared"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/pkg/xmap"
)

// convertGeminiToLLMRequest converts Gemini GenerateContentRequest to unified Request.
func convertGeminiToLLMRequest(geminiReq *GenerateContentRequest) (*llm.Request, error) {
	chatReq := &llm.Request{
		RawAPIFormat: llm.APIFormatGeminiContents,
	}

	// Convert generation config
	if geminiReq.GenerationConfig != nil {
		gc := geminiReq.GenerationConfig

		if gc.MaxOutputTokens > 0 {
			chatReq.MaxTokens = lo.ToPtr(gc.MaxOutputTokens)
		}

		if gc.Temperature != nil {
			chatReq.Temperature = lo.ToPtr(*gc.Temperature)
		}

		if gc.TopP != nil {
			chatReq.TopP = lo.ToPtr(*gc.TopP)
		}

		if gc.PresencePenalty != nil {
			chatReq.PresencePenalty = lo.ToPtr(*gc.PresencePenalty)
		}

		if gc.FrequencyPenalty != nil {
			chatReq.FrequencyPenalty = lo.ToPtr(*gc.FrequencyPenalty)
		}

		if gc.Seed != nil {
			chatReq.Seed = lo.ToPtr(*gc.Seed)
		}

		if len(gc.StopSequences) > 0 {
			if len(gc.StopSequences) == 1 {
				chatReq.Stop = &llm.Stop{Stop: &gc.StopSequences[0]}
			} else {
				chatReq.Stop = &llm.Stop{MultipleStop: gc.StopSequences}
			}
		}

		// Convert thinking config to reasoning effort and preserve budget
		// Priority 1: Use ThinkingLevel if provided
		// Priority 2: Convert from ThinkingBudget if provided
		if gc.ThinkingConfig != nil {
			rawExtraBody, err := convertGeminiThinkingConfigToGeminiOpenAIExtraBody(gc.ThinkingConfig)
			if err != nil {
				return nil, err
			}

			chatReq.ExtraBody = rawExtraBody

			if gc.ThinkingConfig.ThinkingLevel != "" {
				// ThinkingLevel has priority - use it directly
				chatReq.ReasoningEffort = strings.ToLower(gc.ThinkingConfig.ThinkingLevel)
				// Gemini "minimal" maps to LLM "low" for consistency.
				if chatReq.ReasoningEffort == "minimal" {
					chatReq.ReasoningEffort = "low"
				}
			} else if gc.ThinkingConfig.ThinkingBudget != nil {
				// No ThinkingLevel, convert from ThinkingBudget
				chatReq.ReasoningEffort = thinkingBudgetToReasoningEffort(*gc.ThinkingConfig.ThinkingBudget)
			} else {
				// No level or budget, use default
				chatReq.ReasoningEffort = "medium"
			}
			// Always preserve the original budget if present
			if gc.ThinkingConfig.ThinkingBudget != nil {
				chatReq.ReasoningBudget = gc.ThinkingConfig.ThinkingBudget
			}
		}

		// Convert responseModalities to modalities
		if len(gc.ResponseModalities) > 0 {
			chatReq.Modalities = convertGeminiModalitiesToLLM(gc.ResponseModalities)
		}
	}

	// Convert system instruction
	messages := make([]llm.Message, 0)

	if geminiReq.SystemInstruction != nil {
		systemText := extractTextFromContent(geminiReq.SystemInstruction)
		if systemText != "" {
			messages = append(messages, llm.Message{
				Role: "system",
				Content: llm.MessageContent{
					Content: &systemText,
				},
			})
		}
	}

	// Convert contents to messages
	for i, content := range geminiReq.Contents {
		msg, err := convertGeminiContentToLLMMessage(content, geminiReq.Contents[:i])
		if err != nil {
			return nil, err
		}

		if msg != nil {
			messages = append(messages, *msg)
		}
	}

	chatReq.Messages = messages

	// Convert tools
	if len(geminiReq.Tools) > 0 {
		tools := make([]llm.Tool, 0)

		for _, tool := range geminiReq.Tools {
			// Handle function declarations
			if tool.FunctionDeclarations != nil {
				for _, fd := range tool.FunctionDeclarations {
					parameters := fd.Parameters
					// The gemini sdk use UPPER case for type, but the unified format use lower case.
					parameters, err := xjson.Transform(parameters, func(s *jsonschema.Schema) {
						s.Type = strings.ToLower(s.Type)
					})
					if err != nil {
						// If transform failed, fallback to the original parameters.
						parameters = fd.Parameters
					}

					llmTool := llm.Tool{
						Type: "function",
						Function: llm.Function{
							Name:        fd.Name,
							Description: fd.Description,
							Parameters:  parameters,
						},
					}
					tools = append(tools, llmTool)
				}
			}

			// Handle Google Search tool
			if tool.GoogleSearch != nil {
				llmTool := llm.Tool{
					Type: llm.ToolTypeGoogleSearch,
					Google: &llm.GoogleTools{
						Search: &llm.GoogleSearch{},
					},
				}
				tools = append(tools, llmTool)
			}

			// Handle Code Execution tool
			if tool.CodeExecution != nil {
				llmTool := llm.Tool{
					Type: llm.ToolTypeGoogleCodeExecution,
					Google: &llm.GoogleTools{
						CodeExecution: &llm.GoogleCodeExecution{},
					},
				}
				tools = append(tools, llmTool)
			}

			// Handle URL Context tool
			if tool.UrlContext != nil {
				llmTool := llm.Tool{
					Type: llm.ToolTypeGoogleUrlContext,
					Google: &llm.GoogleTools{
						UrlContext: &llm.GoogleUrlContext{},
					},
				}
				tools = append(tools, llmTool)
			}
		}

		chatReq.Tools = tools
	}

	// Convert tool config
	if geminiReq.ToolConfig != nil && geminiReq.ToolConfig.FunctionCallingConfig != nil {
		fcc := geminiReq.ToolConfig.FunctionCallingConfig
		chatReq.ToolChoice = convertGeminiFunctionCallingConfigToToolChoice(fcc)
	}

	return chatReq, nil
}

func convertGeminiThinkingConfigToGeminiOpenAIExtraBody(thinkingConfig *ThinkingConfig) (json.RawMessage, error) {
	if thinkingConfig == nil {
		return nil, nil
	}

	extraThinkingConfig := &geminioai.ThinkingConfig{
		IncludeThoughts: thinkingConfig.IncludeThoughts,
	}
	if thinkingConfig.ThinkingBudget != nil {
		budget := int(*thinkingConfig.ThinkingBudget)
		extraThinkingConfig.ThinkingBudget = geminioai.NewThinkingBudgetInt(budget)
	}

	if thinkingConfig.ThinkingLevel != "" {
		level := strings.ToLower(thinkingConfig.ThinkingLevel)
		if level == "minimal" {
			level = "low"
		}

		extraThinkingConfig.ThinkingLevel = level
	}

	extraBody := &geminioai.ExtraBody{
		Google: &geminioai.GoogleExtraBody{
			ThinkingConfig: extraThinkingConfig,
		},
	}

	return json.Marshal(extraBody)
}

// convertGeminiContentToLLMMessage converts a Gemini Content to an LLM Message.
func convertGeminiContentToLLMMessage(content *Content, previousContents []*Content) (*llm.Message, error) {
	if content == nil || len(content.Parts) == 0 {
		return nil, nil
	}

	msg := &llm.Message{
		Role: convertGeminiRoleToLLMRole(content.Role),
	}

	var (
		textParts        []llm.MessageContentPart
		toolCalls        []llm.ToolCall
		reasoningContent string
	)

	for _, part := range content.Parts {
		if msg.RedactedReasoningContent == nil && part.ThoughtSignature != "" {
			msg.RedactedReasoningContent = shared.EncodeGeminiThoughtSignature(&part.ThoughtSignature)
		}

		switch {
		case part.Text != "":
			if part.Thought {
				reasoningContent = part.Text
			} else {
				textParts = append(textParts, llm.MessageContentPart{
					Type: "text",
					Text: &part.Text,
				})
			}

		case part.InlineData != nil:
			// Convert inline data to image_url format
			imageURL := fmt.Sprintf("data:%s;base64,%s", part.InlineData.MIMEType, part.InlineData.Data)
			textParts = append(textParts, llm.MessageContentPart{
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL: imageURL,
				},
			})

		case part.FileData != nil:
			// Convert file data to image_url format
			textParts = append(textParts, llm.MessageContentPart{
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL: part.FileData.FileURI,
				},
			})

		case part.FunctionCall != nil:
			argsJSON, _ := json.Marshal(part.FunctionCall.Args)
			tc := llm.ToolCall{
				ID:   part.FunctionCall.ID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			}

			toolCalls = append(toolCalls, tc)

		case part.FunctionResponse != nil:
			// Function response is a separate message in unified format
			responseJSON, _ := json.Marshal(part.FunctionResponse.Response)

			// If FunctionResponse ID is empty, find the matching function call ID from previous contents
			functionResponseID := part.FunctionResponse.ID
			if functionResponseID == "" {
				functionResponseID = findMatchingFunctionCallID(part.FunctionResponse.Name, previousContents)
			}

			return &llm.Message{
				Role:         "tool",
				ToolCallID:   lo.ToPtr(functionResponseID),
				ToolCallName: lo.ToPtr(part.FunctionResponse.Name),
				Content: llm.MessageContent{
					Content: lo.ToPtr(string(responseJSON)),
				},
			}, nil
		}
	}

	// Set content
	if len(textParts) == 1 && textParts[0].Type == "text" {
		msg.Content = llm.MessageContent{
			Content: textParts[0].Text,
		}
	} else if len(textParts) > 0 {
		msg.Content = llm.MessageContent{
			MultipleContent: textParts,
		}
	}

	// Set tool calls
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	// Set reasoning content
	if reasoningContent != "" {
		msg.ReasoningContent = &reasoningContent
	}

	return msg, nil
}

// convertLLMToGeminiResponse converts unified Response to Gemini GenerateContentResponse.
// When isStream is true, it reads from Delta instead of Message in choices.
func convertLLMToGeminiResponse(chatResp *llm.Response, isStream bool) *GenerateContentResponse {
	resp := &GenerateContentResponse{
		ResponseID:   chatResp.ID,
		ModelVersion: chatResp.Model,
	}

	// Convert choices to candidates
	candidates := make([]*Candidate, 0, len(chatResp.Choices))
	for _, choice := range chatResp.Choices {
		candidate := convertLLMChoiceToGeminiCandidate(&choice, isStream)

		// Extract GroundingMetadata from Choice.TransformerMetadata if present
		if gm := xmap.GetPtr[GroundingMetadata](choice.TransformerMetadata, TransformerMetadataKeyGroundingMetadata); gm != nil {
			candidate.GroundingMetadata = gm
		}

		candidates = append(candidates, candidate)
	}

	resp.Candidates = candidates
	resp.UsageMetadata = convertToGeminiUsage(chatResp.Usage)

	return resp
}

// convertLLMChoiceToGeminiCandidate converts an LLM Choice to a Gemini Candidate.
// When isStream is true, it reads from Delta instead of Message.
func convertLLMChoiceToGeminiCandidate(choice *llm.Choice, isStream bool) *Candidate {
	candidate := &Candidate{
		Index: int64(choice.Index),
	}

	var msg *llm.Message

	if isStream {
		// For streaming, prefer Delta
		if choice.Delta != nil {
			msg = choice.Delta
		} else if choice.Message != nil {
			msg = choice.Message
		}
	} else {
		// For non-streaming, prefer Message
		if choice.Message != nil {
			msg = choice.Message
		} else if choice.Delta != nil {
			msg = choice.Delta
		}
	}

	if msg != nil {
		content := &Content{
			Role: "model",
		}

		parts := make([]*Part, 0)

		var (
			lastPart              *Part
			firstFunctionCallPart *Part
		)

		// Add reasoning content (thinking) first if present
		if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
			p := &Part{
				Text:    *msg.ReasoningContent,
				Thought: true,
			}
			parts = append(parts, p)
			lastPart = p
		}

		// Add text content
		if msg.Content.Content != nil && *msg.Content.Content != "" {
			p := &Part{Text: *msg.Content.Content}
			parts = append(parts, p)
			lastPart = p
		} else if len(msg.Content.MultipleContent) > 0 {
			for _, part := range msg.Content.MultipleContent {
				switch part.Type {
				case "text":
					if part.Text != nil {
						p := &Part{Text: *part.Text}
						parts = append(parts, p)
						lastPart = p
					}
				case "image_url":
					if part.ImageURL != nil && part.ImageURL.URL != "" {
						geminiPart := convertImageURLToGeminiPart(part.ImageURL.URL)
						if geminiPart != nil {
							parts = append(parts, geminiPart)
							lastPart = geminiPart
						}
					}
				}
			}
		}

		for _, toolCall := range msg.ToolCalls {
			var args map[string]any
			if toolCall.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
			}

			part := &Part{
				FunctionCall: &FunctionCall{
					ID:   toolCall.ID,
					Name: toolCall.Function.Name,
					Args: args,
				},
			}

			parts = append(parts, part)

			lastPart = part
			if firstFunctionCallPart == nil {
				firstFunctionCallPart = part
			}
		}

		msgThoughtSignature := shared.DecodeGeminiThoughtSignature(msg.RedactedReasoningContent)
		if len(msg.ToolCalls) > 0 && msgThoughtSignature == nil {
			msgThoughtSignature = lo.ToPtr("context_engineering_is_the_way_to_go")
		}

		if msgThoughtSignature != nil && lastPart != nil {
			if firstFunctionCallPart != nil {
				firstFunctionCallPart.ThoughtSignature = *msgThoughtSignature
			} else {
				lastPart.ThoughtSignature = *msgThoughtSignature
			}
		}

		content.Parts = parts
		candidate.Content = content
	}

	// Convert finish reason
	candidate.FinishReason = convertLLMFinishReasonToGemini(choice.FinishReason)

	return candidate
}

// findMatchingFunctionCallID searches backwards through previous contents to find the last
// function call with the given function name and returns its ID.
func findMatchingFunctionCallID(functionName string, previousContents []*Content) string {
	// Search from the end to the beginning to find the most recent matching function call
	for i := len(previousContents) - 1; i >= 0; i-- {
		content := previousContents[i]
		if content == nil {
			continue
		}

		// Look through all parts in this content in reverse order to find the most recent function call
		for j := len(content.Parts) - 1; j >= 0; j-- {
			part := content.Parts[j]
			if part.FunctionCall != nil && part.FunctionCall.Name == functionName && part.FunctionCall.ID != "" {
				return part.FunctionCall.ID
			}
		}
	}

	return ""
}
