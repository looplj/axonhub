package gemini

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
)

// convertLLMToGeminiRequest converts unified Request to Gemini GenerateContentRequest.
func convertLLMToGeminiRequest(chatReq *llm.Request) *GenerateContentRequest {
	req := &GenerateContentRequest{}

	// Convert generation config
	gc := &GenerationConfig{}
	hasGenerationConfig := false

	if chatReq.MaxTokens != nil {
		gc.MaxOutputTokens = *chatReq.MaxTokens
		hasGenerationConfig = true
	} else if chatReq.MaxCompletionTokens != nil {
		gc.MaxOutputTokens = *chatReq.MaxCompletionTokens
		hasGenerationConfig = true
	}

	if chatReq.Temperature != nil {
		gc.Temperature = lo.ToPtr(*chatReq.Temperature)
		hasGenerationConfig = true
	}

	if chatReq.TopP != nil {
		gc.TopP = lo.ToPtr(*chatReq.TopP)
		hasGenerationConfig = true
	}

	if chatReq.PresencePenalty != nil {
		gc.PresencePenalty = lo.ToPtr(*chatReq.PresencePenalty)
		hasGenerationConfig = true
	}

	if chatReq.FrequencyPenalty != nil {
		gc.FrequencyPenalty = lo.ToPtr(*chatReq.FrequencyPenalty)
		hasGenerationConfig = true
	}

	if chatReq.Seed != nil {
		gc.Seed = lo.ToPtr(*chatReq.Seed)
		hasGenerationConfig = true
	}

	if chatReq.Stop != nil {
		if chatReq.Stop.Stop != nil {
			gc.StopSequences = []string{*chatReq.Stop.Stop}
		} else if len(chatReq.Stop.MultipleStop) > 0 {
			gc.StopSequences = chatReq.Stop.MultipleStop
		}

		hasGenerationConfig = true
	}

	// Convert reasoning effort to thinking config
	if chatReq.ReasoningEffort != "" {
		gc.ThinkingConfig = &ThinkingConfig{
			IncludeThoughts: true,
			ThinkingBudget:  lo.ToPtr(reasoningEffortToThinkingBudget(chatReq.ReasoningEffort)),
		}
		hasGenerationConfig = true
	}

	// Convert modalities to responseModalities
	if len(chatReq.Modalities) > 0 {
		gc.ResponseModalities = convertLLMModalitiesToGemini(chatReq.Modalities)
		hasGenerationConfig = true
	}

	if hasGenerationConfig {
		req.GenerationConfig = gc
	}

	// Convert messages
	var systemInstruction *Content

	contents := make([]*Content, 0)

	for _, msg := range chatReq.Messages {
		switch msg.Role {
		case "system":
			// Collect system messages into system instruction
			text := extractTextFromLLMMessage(&msg)
			if text != "" {
				if systemInstruction == nil {
					systemInstruction = &Content{
						Parts: []*Part{{Text: text}},
					}
				} else {
					// Append to existing system instruction
					existingText := extractTextFromContent(systemInstruction)
					systemInstruction.Parts = []*Part{{Text: existingText + "\n" + text}}
				}
			}

		case "tool":
			// Tool response - need to find the corresponding function call
			content := convertLLMToolMessageToGeminiContent(&msg)
			if content != nil {
				contents = append(contents, content)
			}

		default:
			content := convertLLMMessageToGeminiContent(&msg)
			if content != nil {
				contents = append(contents, content)
			}
		}
	}

	req.SystemInstruction = systemInstruction
	req.Contents = contents

	// Convert tools
	if len(chatReq.Tools) > 0 {
		functionDeclarations := make([]*FunctionDeclaration, 0, len(chatReq.Tools))
		for _, tool := range chatReq.Tools {
			if tool.Type == "function" {
				fd := &FunctionDeclaration{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  tool.Function.Parameters,
				}
				functionDeclarations = append(functionDeclarations, fd)
			}
		}

		if len(functionDeclarations) > 0 {
			req.Tools = []*Tool{{FunctionDeclarations: functionDeclarations}}
		}
	}

	// Convert tool choice
	if chatReq.ToolChoice != nil {
		req.ToolConfig = convertLLMToolChoiceToGeminiToolConfig(chatReq.ToolChoice)
	}

	return req
}

// convertLLMMessageToGeminiContent converts an LLM Message to Gemini Content.
func convertLLMMessageToGeminiContent(msg *llm.Message) *Content {
	content := &Content{
		Role: convertLLMRoleToGeminiRole(msg.Role),
	}

	parts := make([]*Part, 0)

	// Add reasoning content (thinking) first if present
	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		parts = append(parts, &Part{
			Text:    *msg.ReasoningContent,
			Thought: true,
		})
	}

	// Add text content
	if msg.Content.Content != nil && *msg.Content.Content != "" {
		parts = append(parts, &Part{Text: *msg.Content.Content})
	} else if len(msg.Content.MultipleContent) > 0 {
		for _, part := range msg.Content.MultipleContent {
			switch part.Type {
			case "text":
				if part.Text != nil {
					parts = append(parts, &Part{Text: *part.Text})
				}
			case "image_url":
				if part.ImageURL != nil && part.ImageURL.URL != "" {
					geminiPart := convertImageURLToGeminiPart(part.ImageURL.URL)
					if geminiPart != nil {
						parts = append(parts, geminiPart)
					}
				}
			}
		}
	}

	// Add tool calls
	for _, toolCall := range msg.ToolCalls {
		var args map[string]any
		if toolCall.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
		}

		parts = append(parts, &Part{
			FunctionCall: &FunctionCall{
				ID:   toolCall.ID,
				Name: toolCall.Function.Name,
				Args: args,
			},
		})
	}

	if len(parts) == 0 {
		return nil
	}

	content.Parts = parts

	return content
}

// convertLLMToolMessageToGeminiContent converts an LLM tool message to Gemini Content.
func convertLLMToolMessageToGeminiContent(msg *llm.Message) *Content {
	content := &Content{
		Role: "user", // Function responses come from user role in Gemini
	}

	var responseData map[string]any
	if msg.Content.Content != nil {
		_ = json.Unmarshal([]byte(*msg.Content.Content), &responseData)
	}

	if responseData == nil {
		responseData = map[string]any{"result": lo.FromPtrOr(msg.Content.Content, "")}
	}

	id := ""
	if msg.ToolCallID != nil {
		id = *msg.ToolCallID
	}

	content.Parts = []*Part{
		{
			FunctionResponse: &FunctionResponse{
				ID:       id,
				Name:     lo.FromPtrOr(msg.ToolCallName, ""),
				Response: responseData,
			},
		},
	}

	return content
}

// convertGeminiToLLMResponse converts Gemini GenerateContentResponse to unified Response.
// When isStream is true, it sets Delta instead of Message in choices.
func convertGeminiToLLMResponse(geminiResp *GenerateContentResponse, isStream bool) *llm.Response {
	resp := &llm.Response{
		ID:      geminiResp.ResponseID,
		Model:   geminiResp.ModelVersion,
		Created: time.Now().Unix(),
	}

	// Set object type based on stream mode
	if isStream {
		resp.Object = "chat.completion.chunk"
	} else {
		resp.Object = "chat.completion"
	}

	// Generate ID if not present
	if resp.ID == "" {
		resp.ID = "chatcmpl-" + uuid.New().String()
	}

	// Convert candidates to choices
	choices := make([]llm.Choice, 0, len(geminiResp.Candidates))
	for _, candidate := range geminiResp.Candidates {
		choice := convertGeminiCandidateToLLMChoice(candidate, isStream)
		choices = append(choices, choice)
	}

	resp.Choices = choices
	resp.Usage = convertToLLMUsage(geminiResp.UsageMetadata)

	return resp
}

// convertGeminiCandidateToLLMChoice converts a Gemini Candidate to an LLM Choice.
// When isStream is true, it sets Delta instead of Message.
func convertGeminiCandidateToLLMChoice(candidate *Candidate, isStream bool) llm.Choice {
	choice := llm.Choice{
		Index: int(candidate.Index),
	}

	var hasToolCall bool

	if candidate.Content != nil {
		msg := &llm.Message{
			Role: "assistant",
		}

		var (
			textParts        []string
			contentParts     []llm.MessageContentPart
			toolCalls        []llm.ToolCall
			reasoningContent string
		)

		for _, part := range candidate.Content.Parts {
			switch {
			case part.Text != "":
				if part.Thought {
					reasoningContent = part.Text
				} else {
					textParts = append(textParts, part.Text)
				}

			case part.InlineData != nil:
				// Convert inline data (image) to image_url format
				imageURL := "data:" + part.InlineData.MIMEType + ";base64," + part.InlineData.Data
				contentParts = append(contentParts, llm.MessageContentPart{
					Type: "image_url",
					ImageURL: &llm.ImageURL{
						URL: imageURL,
					},
				})

			case part.FunctionCall != nil:
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   part.FunctionCall.ID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}

		// Set content - prefer multipart if we have images
		if len(contentParts) > 0 {
			// Add text parts to content parts
			for _, text := range textParts {
				contentParts = append([]llm.MessageContentPart{{
					Type: "text",
					Text: lo.ToPtr(text),
				}}, contentParts...)
			}

			msg.Content = llm.MessageContent{
				MultipleContent: contentParts,
			}
		} else if len(textParts) > 0 {
			allText := strings.Join(textParts, "")
			msg.Content = llm.MessageContent{
				Content: &allText,
			}
		}

		// Set tool calls
		if len(toolCalls) > 0 {
			hasToolCall = true
			msg.ToolCalls = toolCalls
		}

		// Set reasoning content
		if reasoningContent != "" {
			msg.ReasoningContent = &reasoningContent
		}

		// Set Delta for streaming, Message for non-streaming
		if isStream {
			choice.Delta = msg
		} else {
			choice.Message = msg
		}
	}

	// Convert finish reason
	choice.FinishReason = convertGeminiFinishReasonToLLM(candidate.FinishReason, hasToolCall)

	return choice
}
