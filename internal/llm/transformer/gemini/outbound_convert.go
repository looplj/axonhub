package gemini

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	geminioai "github.com/looplj/axonhub/internal/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/internal/llm/transformer/shared"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/pkg/xurl"
)

// convertLLMToGeminiRequest converts unified Request to Gemini GenerateContentRequest.
func convertLLMToGeminiRequest(chatReq *llm.Request) *GenerateContentRequest {
	return convertLLMToGeminiRequestWithConfig(chatReq, nil)
}

// convertLLMToGeminiRequestWithConfig converts unified Request to Gemini GenerateContentRequest with config.
//
//nolint:maintidx // Checked.
func convertLLMToGeminiRequestWithConfig(chatReq *llm.Request, config *Config) *GenerateContentRequest {
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

	// Priority 1: Parse ExtraBody from llm.Request (higher priority)
	var hasExtraBodyThinkingConfig bool

	if len(chatReq.ExtraBody) > 0 {
		extraBody := geminioai.ParseExtraBody(chatReq.ExtraBody)
		if extraBody != nil && extraBody.Google != nil && extraBody.Google.ThinkingConfig != nil {
			// Convert geminioai.ThinkingConfig to gemini.ThinkingConfig
			geminiThinkingConfig := &ThinkingConfig{
				IncludeThoughts: extraBody.Google.ThinkingConfig.IncludeThoughts,
			}

			// Priority 1: Use ThinkingLevel if present (takes absolute priority)
			if extraBody.Google.ThinkingConfig.ThinkingLevel != "" {
				level := extraBody.Google.ThinkingConfig.ThinkingLevel
				// Map "minimal" to "low" for consistency
				if strings.ToLower(level) == "minimal" {
					level = "low"
				}

				geminiThinkingConfig.ThinkingLevel = level
				// Don't set ThinkingBudget when ThinkingLevel is present
			} else if extraBody.Google.ThinkingConfig.ThinkingBudget != nil {
				// Priority 2: Use ThinkingBudget if no level
				if extraBody.Google.ThinkingConfig.ThinkingBudget.IntValue != nil {
					// Integer budget: use as ThinkingBudget
					geminiThinkingConfig.ThinkingBudget = lo.ToPtr(int64(*extraBody.Google.ThinkingConfig.ThinkingBudget.IntValue))
				} else if extraBody.Google.ThinkingConfig.ThinkingBudget.StringValue != nil {
					// String budget: convert to ThinkingLevel for standard values
					strVal := strings.ToLower(*extraBody.Google.ThinkingConfig.ThinkingBudget.StringValue)
					switch strVal {
					case "low", "minimal":
						geminiThinkingConfig.ThinkingLevel = "low"
					case "medium":
						geminiThinkingConfig.ThinkingLevel = "medium"
					case "high":
						geminiThinkingConfig.ThinkingLevel = "high"
					default:
						// Unknown string value, use as-is
						geminiThinkingConfig.ThinkingLevel = strVal
					}
				}
			}

			gc.ThinkingConfig = geminiThinkingConfig
			hasGenerationConfig = true
			hasExtraBodyThinkingConfig = true
		}
	}

	// Convert reasoning effort to thinking config
	// Priority: ExtraBody > ReasoningBudget > ReasoningEffort > default
	if !hasExtraBodyThinkingConfig {
		if chatReq.ReasoningBudget != nil {
			// Priority 1: Use ReasoningBudget if provided
			gc.ThinkingConfig = &ThinkingConfig{
				IncludeThoughts: true,
				// Gemini max thinking budget is 24576
				ThinkingBudget: lo.ToPtr(min(*chatReq.ReasoningBudget, 24576)),
			}
			hasGenerationConfig = true
		} else if chatReq.ReasoningEffort != "" {
			// Priority 2: Convert from ReasoningEffort if provided
			// Use ThinkingLevel for standard effort values (low, medium, high)
			// to preserve the original format when possible
			thinkingConfig := &ThinkingConfig{
				IncludeThoughts: true,
			}

			// For standard effort levels, use ThinkingLevel
			switch strings.ToLower(chatReq.ReasoningEffort) {
			case "none", "low", "medium", "high":
				thinkingConfig.ThinkingLevel = chatReq.ReasoningEffort
			default:
				// For non-standard effort values, convert to budget
				thinkingBudget := reasoningEffortToThinkingBudgetWithConfig(chatReq.ReasoningEffort, config)
				thinkingConfig.ThinkingBudget = lo.ToPtr(min(thinkingBudget, 24576))
			}

			gc.ThinkingConfig = thinkingConfig
			hasGenerationConfig = true
		}
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
		case "system", "developer":
			// Collect system and developer messages into system instruction
			parts := extractPartsFromLLMMessage(&msg)
			if len(parts) > 0 {
				if systemInstruction == nil {
					systemInstruction = &Content{
						Parts: parts,
					}
				} else {
					// Append to existing system instruction
					systemInstruction.Parts = append(systemInstruction.Parts, parts...)
				}
			}

		case "tool":
			// Tool response - need to find the corresponding function call
			content := convertLLMToolResultToGeminiContent(&msg, contents)
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
		tools := make([]*Tool, 0)
		functionDeclarations := make([]*FunctionDeclaration, 0)

		var functionTool *Tool

		for _, tool := range chatReq.Tools {
			switch tool.Type {
			case "function":
				params := tool.Function.Parameters

				params, err := xjson.CleanSchema(params, "$schema", "additionalProperties")
				if err != nil {
					// ignore
					params = tool.Function.Parameters
				}

				fd := &FunctionDeclaration{
					Name:        tool.Function.Name,
					Description: tool.Function.Description,
					Parameters:  params,
				}
				functionDeclarations = append(functionDeclarations, fd)

				if functionTool == nil {
					functionTool = &Tool{}
					tools = append(tools, functionTool)
				}

			case llm.ToolTypeGoogleSearch:
				if tool.Google != nil && tool.Google.Search != nil {
					tools = append(tools, &Tool{GoogleSearch: &GoogleSearch{}})
				}

			case llm.ToolTypeGoogleCodeExecution:
				if tool.Google != nil && tool.Google.CodeExecution != nil {
					tools = append(tools, &Tool{CodeExecution: &CodeExecution{}})
				}

			case llm.ToolTypeGoogleUrlContext:
				if tool.Google != nil && tool.Google.UrlContext != nil {
					tools = append(tools, &Tool{UrlContext: &UrlContext{}})
				}
			}
		}

		if functionTool != nil {
			functionTool.FunctionDeclarations = functionDeclarations
		}

		if len(tools) > 0 {
			req.Tools = tools
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
	if msg == nil {
		return nil
	}

	content := &Content{
		Role: convertLLMRoleToGeminiRole(msg.Role),
	}

	parts := make([]*Part, 0)

	var (
		firstFunctionCallPart *Part
		lastPart              *Part
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
				// Handle image_url type
				if part.ImageURL != nil && part.ImageURL.URL != "" {
					geminiPart := convertImageURLToGeminiPart(part.ImageURL.URL)
					if geminiPart != nil {
						parts = append(parts, geminiPart)
						lastPart = geminiPart
					}
				}
			case "document":
				// Handle document type (PDF, Word, etc.)
				if part.Document != nil && part.Document.URL != "" {
					geminiPart := convertDocumentURLToGeminiPart(part.Document)
					if geminiPart != nil {
						parts = append(parts, geminiPart)
						lastPart = geminiPart
					}
				}
			}
		}
	}

	// https://ai.google.dev/gemini-api/docs/thought-signatures#model-behavior
	// Gemini 3 Pro, Gemini 3 Pro Image and Gemini 2.5 models behave differently with thought signatures.
	// For Gemini 3 Pro Image see the thinking process section of the image generation guide.
	// Gemini 3 Pro and Gemini 2.5 models behave differently with thought signatures in function calls:
	//     If there are function calls in a response,
	//         Gemini 3 Pro will always have the signature on the first function call part. It is mandatory to return that part.
	//         Gemini 2.5 will have the signature in the first part (regardless of type). It is optional to return that part.
	//     If there are no function calls in a response,
	//         Gemini 3 Pro will have the signature on the last part if the model generates a thought.
	//         Gemini 2.5 won't have a signature in any part.

	// Add tool calls
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

	// https://ai.google.dev/gemini-api/docs/gemini-3#migrating_from_other_models
	// If there are tool calls but no thought signature, use a default one.
	// This field is not compatible with OpenAI sdk, so we use the default value.
	// We try the best to support this fields to keep this fields in the chat conversions, so we use the RedactedReasoningContent to hold the field,
	// And this field will be preserved during claude code trace, will not degrade the gemini model performance.
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

	if len(parts) == 0 {
		return nil
	}

	content.Parts = parts

	return content
}

// convertLLMToolResultToGeminiContent converts an LLM tool message to Gemini Content.
func convertLLMToolResultToGeminiContent(msg *llm.Message, contents []*Content) *Content {
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

	fp := &FunctionResponse{
		ID:       lo.FromPtr(msg.ToolCallID),
		Name:     lo.FromPtr(msg.ToolCallName),
		Response: responseData,
	}

	// Anthropic‘s tool result doesn't have name, so we need to find it by tool call id.
	if fp.Name == "" && fp.ID != "" {
		fp.Name = findToolNameByToolCallID(contents, fp.ID)
	}

	content.Parts = []*Part{
		{FunctionResponse: fp},
	}

	return content
}

func findToolNameByToolCallID(contents []*Content, id string) string {
	for _, content := range contents {
		for _, part := range content.Parts {
			if part.FunctionCall != nil && part.FunctionCall.ID == id {
				return part.FunctionCall.Name
			}
		}
	}

	return ""
}

// convertGeminiToLLMResponse converts Gemini GenerateContentResponse to unified Response.
// When isStream is true, it sets Delta instead of Message in choices.
func convertGeminiToLLMResponse(geminiResp *GenerateContentResponse, isStream bool) *llm.Response {
	resp, _ := convertGeminiToLLMResponseWithState(geminiResp, isStream, 0)
	return resp
}

// TransformerMetadataKeyGroundingMetadata is the key for storing GroundingMetadata in TransformerMetadata.
const TransformerMetadataKeyGroundingMetadata = "gemini_grounding_metadata"

// convertGeminiToLLMResponseWithState converts Gemini response with tool call index tracking.
// Returns the response and the next tool call index to use.
func convertGeminiToLLMResponseWithState(geminiResp *GenerateContentResponse, isStream bool, toolCallIndexOffset int) (*llm.Response, int) {
	resp := &llm.Response{
		ID:          geminiResp.ResponseID,
		Model:       geminiResp.ModelVersion,
		Created:     time.Now().Unix(),
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatGeminiContents,
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
	nextToolCallIndex := toolCallIndexOffset

	for _, candidate := range geminiResp.Candidates {
		var choice llm.Choice

		choice, nextToolCallIndex = convertGeminiCandidateToLLMChoiceWithState(candidate, isStream, nextToolCallIndex)

		// Store GroundingMetadata in Choice.TransformerMetadata if present
		if candidate.GroundingMetadata != nil {
			if choice.TransformerMetadata == nil {
				choice.TransformerMetadata = map[string]any{}
			}

			choice.TransformerMetadata[TransformerMetadataKeyGroundingMetadata] = candidate.GroundingMetadata
		}

		choices = append(choices, choice)
	}

	resp.Choices = choices
	resp.Usage = convertToLLMUsage(geminiResp.UsageMetadata)

	return resp, nextToolCallIndex
}

// convertGeminiCandidateToLLMChoiceWithState converts a Gemini Candidate to an LLM Choice with tool call index tracking.
// Returns the choice and the next tool call index to use.
func convertGeminiCandidateToLLMChoiceWithState(candidate *Candidate, isStream bool, toolCallIndexOffset int) (llm.Choice, int) {
	choice := llm.Choice{
		Index: int(candidate.Index),
	}

	var hasToolCall bool

	nextToolCallIndex := toolCallIndexOffset

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
			if msg.RedactedReasoningContent == nil && part.ThoughtSignature != "" {
				msg.RedactedReasoningContent = shared.EncodeGeminiThoughtSignature(&part.ThoughtSignature)
			}

			switch {
			case part.Text != "":
				if part.Thought {
					reasoningContent = part.Text
				} else {
					textParts = append(textParts, part.Text)
				}

			case part.InlineData != nil:
				// Convert inline data based on MIME type
				dataURL := xurl.BuildDataURL(part.InlineData.MIMEType, part.InlineData.Data, true)

				if isDocumentMIMEType(part.InlineData.MIMEType) {
					// Document type (PDF, Word, etc.)
					contentParts = append(contentParts, llm.MessageContentPart{
						Type: "document",
						Document: &llm.DocumentURL{
							URL:      dataURL,
							MIMEType: part.InlineData.MIMEType,
						},
					})
				} else {
					// Image type
					contentParts = append(contentParts, llm.MessageContentPart{
						Type: "image_url",
						ImageURL: &llm.ImageURL{
							URL: dataURL,
						},
					})
				}

			case part.FunctionCall != nil:
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				tc := llm.ToolCall{
					Index: nextToolCallIndex,
					ID:    part.FunctionCall.ID,
					Type:  "function",
					Function: llm.FunctionCall{
						Name:      part.FunctionCall.Name,
						Arguments: string(argsJSON),
					},
				}
				// Gemini may response empty tool call ID.
				if tc.ID == "" {
					tc.ID = uuid.NewString()
				}

				toolCalls = append(toolCalls, tc)
				nextToolCallIndex++
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

	return choice, nextToolCallIndex
}
