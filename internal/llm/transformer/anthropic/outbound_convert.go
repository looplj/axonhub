package anthropic

import (
	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer/shared"
	"github.com/looplj/axonhub/internal/pkg/xjson"
	"github.com/looplj/axonhub/internal/pkg/xurl"
)

// ToolTypeWebSearch20250305 is an alias to llm.ToolTypeAnthropicWebSearch for package compatibility.
const ToolTypeWebSearch20250305 = llm.ToolTypeAnthropicWebSearch

// convertToAnthropicRequest converts ChatCompletionRequest to Anthropic MessageRequest.
// Deprecated: Use convertToAnthropicRequestWithConfig instead.
func convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
	return convertToAnthropicRequestWithConfig(chatReq, nil)
}

// convertToAnthropicRequestWithConfig converts ChatCompletionRequest to Anthropic MessageRequest with config.
func convertToAnthropicRequestWithConfig(chatReq *llm.Request, config *Config) *MessageRequest {
	req := buildBaseRequest(chatReq, config)
	req.Tools = convertTools(chatReq.Tools)
	req.Messages = convertMessages(chatReq)
	req.StopSequences = convertStopSequences(chatReq.Stop)

	return req
}

// buildBaseRequest creates the base MessageRequest with common fields.
func buildBaseRequest(chatReq *llm.Request, config *Config) *MessageRequest {
	req := &MessageRequest{
		Model:       chatReq.Model,
		Temperature: chatReq.Temperature,
		TopP:        chatReq.TopP,
		Stream:      chatReq.Stream,
		System:      convertToAnthropicSystemPrompt(chatReq),
		MaxTokens:   resolveMaxTokens(chatReq),
	}

	if chatReq.Metadata != nil && chatReq.Metadata["user_id"] != "" {
		req.Metadata = &AnthropicMetadata{UserID: chatReq.Metadata["user_id"]}
	}

	if chatReq.ReasoningEffort != "" {
		req.Thinking = buildThinking(chatReq, config)
	}

	return req
}

// resolveMaxTokens determines the max_tokens value with fallback.
func resolveMaxTokens(chatReq *llm.Request) int64 {
	switch {
	case chatReq.MaxTokens != nil:
		return *chatReq.MaxTokens
	case chatReq.MaxCompletionTokens != nil:
		return *chatReq.MaxCompletionTokens
	default:
		// Set to 8192 tokens to match common model upper limit.
		return 8192
	}
}

// buildThinking creates the Thinking configuration.
func buildThinking(chatReq *llm.Request, config *Config) *Thinking {
	budgetTokens := lo.FromPtrOr(chatReq.ReasoningBudget, getThinkingBudgetTokensWithConfig(chatReq.ReasoningEffort, config))

	return &Thinking{
		Type:         "enabled",
		BudgetTokens: budgetTokens,
	}
}

// convertTools converts LLM tools to Anthropic tools.
func convertTools(tools []llm.Tool) []Tool {
	if len(tools) == 0 {
		return nil
	}

	anthropicTools := make([]Tool, 0, len(tools))

	for _, tool := range tools {
		// Use shared helper to detect Anthropic native tools (web_search)
		if llm.IsAnthropicNativeTool(tool) {
			anthropicTools = append(anthropicTools, Tool{
				Type: ToolTypeWebSearch20250305,
				Name: llm.AnthropicWebSearchFunctionName,
			})
		} else if tool.Type == llm.ToolType {
			anthropicTools = append(anthropicTools, Tool{
				Name:         tool.Function.Name,
				Description:  tool.Function.Description,
				InputSchema:  tool.Function.Parameters,
				CacheControl: convertToAnthropicCacheControl(tool.CacheControl),
			})
		}
	}

	if len(anthropicTools) == 0 {
		return nil
	}

	return anthropicTools
}

// convertStopSequences converts stop sequences.
func convertStopSequences(stop *llm.Stop) []string {
	if stop == nil {
		return nil
	}

	if stop.Stop != nil {
		return []string{*stop.Stop}
	}

	if len(stop.MultipleStop) > 0 {
		return stop.MultipleStop
	}

	return nil
}

// convertMessages converts all messages to Anthropic format.
func convertMessages(chatReq *llm.Request) []MessageParam {
	messages := make([]MessageParam, 0, len(chatReq.Messages))
	processedIndexes := make(map[int]bool)

	for _, msg := range chatReq.Messages {
		if msg.Role == "system" {
			continue
		}

		if converted, ok := convertSingleMessage(msg, chatReq.Messages, processedIndexes); ok {
			messages = append(messages, converted...)
		}
	}

	return messages
}

// convertSingleMessage handles conversion of a single message based on its role.
func convertSingleMessage(msg llm.Message, allMessages []llm.Message, processedIndexes map[int]bool) ([]MessageParam, bool) {
	switch msg.Role {
	case "tool":
		return convertToolMessage(msg, allMessages, processedIndexes)
	case "user":
		if msg.MessageIndex != nil && processedIndexes[*msg.MessageIndex] {
			return nil, false
		}

		return convertUserMessage(msg)
	case "assistant":
		return convertAssistantMessage(msg)
	default:
		return nil, false
	}
}

// convertToolMessage handles tool message conversion (single and parallel).
func convertToolMessage(msg llm.Message, allMessages []llm.Message, processedIndexes map[int]bool) ([]MessageParam, bool) {
	// Single tool call
	if msg.MessageIndex == nil {
		return []MessageParam{{
			Role: "user",
			Content: MessageContent{
				MultipleContent: []MessageContentBlock{convertToToolResultBlock(msg)},
			},
		}}, true
	}

	// Parallel tool calls - skip if already processed
	if processedIndexes[*msg.MessageIndex] {
		return nil, false
	}

	toolMsgs := lo.Filter(allMessages, func(item llm.Message, _ int) bool {
		return item.Role == "tool" && item.MessageIndex != nil && *item.MessageIndex == *msg.MessageIndex
	})

	if len(toolMsgs) == 0 {
		return nil, false
	}

	contentBlocks := buildToolResultBlocks(toolMsgs)
	contentBlocks = appendUserMessageBlocks(contentBlocks, allMessages, *msg.MessageIndex)

	processedIndexes[*msg.MessageIndex] = true

	return []MessageParam{{
		Role:    "user",
		Content: MessageContent{MultipleContent: contentBlocks},
	}}, true
}

// buildToolResultBlocks creates tool_result blocks from tool messages.
func buildToolResultBlocks(toolMsgs []llm.Message) []MessageContentBlock {
	return lo.Map(toolMsgs, func(item llm.Message, _ int) MessageContentBlock {
		return convertToToolResultBlock(item)
	})
}

// appendUserMessageBlocks merges user message content with the same MessageIndex.
func appendUserMessageBlocks(blocks []MessageContentBlock, allMessages []llm.Message, index int) []MessageContentBlock {
	userMsgs := lo.Filter(allMessages, func(item llm.Message, _ int) bool {
		return item.Role == "user" && item.MessageIndex != nil && *item.MessageIndex == index
	})

	for _, userMsg := range userMsgs {
		userBlocks := convertToAnthropicTrivialContent(userMsg.Content).ExtractTrivalBlocks(convertToAnthropicCacheControl(userMsg.CacheControl))
		blocks = append(blocks, userBlocks...)
	}

	return blocks
}

// convertUserMessage handles user message conversion.
func convertUserMessage(msg llm.Message) ([]MessageParam, bool) {
	content, ok := buildMessageContent(msg)
	if !ok {
		return nil, false
	}

	return []MessageParam{{Role: "user", Content: content}}, true
}

// convertAssistantMessage handles assistant message conversion.
func convertAssistantMessage(msg llm.Message) ([]MessageParam, bool) {
	return convertAssistantWithToolCalls(msg)
}

// convertAssistantWithToolCalls handles assistant messages that have tool calls.
func convertAssistantWithToolCalls(msg llm.Message) ([]MessageParam, bool) {
	preBlocks := buildPreBlocks(msg)
	toolContent, hasToolContent := convertMultiplePartContent(msg)

	switch {
	case hasToolContent && len(preBlocks) > 0:
		toolContent.MultipleContent = append(preBlocks, toolContent.MultipleContent...)
	case hasToolContent:
		// Use toolContent directly
	case len(preBlocks) > 0:
		toolContent = buildContentFromBlocks(preBlocks)
	default:
		return nil, false
	}

	return []MessageParam{{Role: "assistant", Content: toolContent}}, true
}

// buildPreBlocks creates thinking and text blocks that precede tool use.
func buildPreBlocks(msg llm.Message) []MessageContentBlock {
	var blocks []MessageContentBlock

	if block := buildThinkingBlock(msg.ReasoningContent, msg.ReasoningSignature); block != nil {
		blocks = append(blocks, *block)
	}

	if block := buildRedactedThinkingBlock(msg.RedactedReasoningContent); block != nil {
		blocks = append(blocks, *block)
	}

	if msg.Content.Content != nil && *msg.Content.Content != "" {
		blocks = append(blocks, MessageContentBlock{
			Type:         "text",
			Text:         msg.Content.Content,
			CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
		})
	}

	return blocks
}

// buildContentFromBlocks converts blocks to MessageContent.
func buildContentFromBlocks(blocks []MessageContentBlock) MessageContent {
	if len(blocks) == 1 && blocks[0].Type == "text" {
		return MessageContent{Content: blocks[0].Text}
	}

	return MessageContent{MultipleContent: blocks}
}

// buildMessageContent creates message content with optional thinking block.
func buildMessageContent(msg llm.Message) (MessageContent, bool) {
	// Handle simple string content
	if msg.Content.Content != nil {
		if msg.CacheControl != nil || hasThinkingContent(msg) {
			return buildMultipleContentWithThinking(msg), true
		}

		return MessageContent{Content: msg.Content.Content}, true
	}

	// Handle multiple content parts
	if len(msg.Content.MultipleContent) > 0 {
		return convertMultiplePartContent(msg)
	}

	return MessageContent{}, false
}

// hasThinkingContent checks if message has reasoning content.
func hasThinkingContent(msg llm.Message) bool {
	return (msg.ReasoningContent != nil && *msg.ReasoningContent != "") ||
		(shared.IsAnthropicRedactedContent(msg.RedactedReasoningContent) && *msg.RedactedReasoningContent != "")
}

// buildMultipleContentWithThinking creates content blocks including thinking.
func buildMultipleContentWithThinking(msg llm.Message) MessageContent {
	blocks := make([]MessageContentBlock, 0, 3)

	if block := buildThinkingBlock(msg.ReasoningContent, msg.ReasoningSignature); block != nil {
		blocks = append(blocks, *block)
	}

	if block := buildRedactedThinkingBlock(msg.RedactedReasoningContent); block != nil {
		blocks = append(blocks, *block)
	}

	blocks = append(blocks, MessageContentBlock{
		Type:         "text",
		Text:         msg.Content.Content,
		CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
	})

	return MessageContent{MultipleContent: blocks}
}

// buildThinkingBlock creates a thinking block from reasoning content.
func buildThinkingBlock(reasoningContent, reasoningSignature *string) *MessageContentBlock {
	if reasoningContent == nil || *reasoningContent == "" {
		return nil
	}

	block := &MessageContentBlock{
		Type:      "thinking",
		Thinking:  reasoningContent,
		Signature: reasoningSignature,
	}

	return block
}

// buildRedactedThinkingBlock creates a redacted_thinking block from encrypted content.
func buildRedactedThinkingBlock(redactedContent *string) *MessageContentBlock {
	if redactedContent == nil || *redactedContent == "" {
		return nil
	}

	if !shared.IsAnthropicRedactedContent(redactedContent) {
		return nil
	}

	return &MessageContentBlock{
		Type: "redacted_thinking",
		Data: *redactedContent,
	}
}

func convertToToolResultBlock(msg llm.Message) MessageContentBlock {
	return MessageContentBlock{
		Type:         "tool_result",
		ToolUseID:    msg.ToolCallID,
		Content:      convertToAnthropicTrivialContent(msg.Content),
		CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
		IsError:      msg.ToolCallIsError,
	}
}

// convertImageURLToAnthropicBlock converts image_url content part to Anthropic MessageContentBlock.
func convertImageURLToAnthropicBlock(part llm.MessageContentPart) (MessageContentBlock, bool) {
	if part.ImageURL == nil || part.ImageURL.URL == "" {
		return MessageContentBlock{}, false
	}

	// Convert OpenAI image format to Anthropic format
	url := part.ImageURL.URL
	if parsed := xurl.ParseDataURL(url); parsed != nil {
		return MessageContentBlock{
			Type: "image",
			Source: &ImageSource{
				Type:      "base64",
				MediaType: parsed.MediaType,
				Data:      parsed.Data,
			},
			CacheControl: convertToAnthropicCacheControl(part.CacheControl),
		}, true
	}

	return MessageContentBlock{
		Type: "image",
		Source: &ImageSource{
			Type: "url",
			URL:  part.ImageURL.URL,
		},
		CacheControl: convertToAnthropicCacheControl(part.CacheControl),
	}, true
}

// convertToAnthropicTrivialContent converts llm.MessageContent to Anthropic MessageContent format.
func convertToAnthropicTrivialContent(content llm.MessageContent) *MessageContent {
	if content.Content != nil {
		return &MessageContent{
			Content: content.Content,
		}
	} else if len(content.MultipleContent) > 0 {
		blocks := make([]MessageContentBlock, 0, len(content.MultipleContent))

		for _, part := range content.MultipleContent {
			switch part.Type {
			case "text":
				if part.Text != nil {
					blocks = append(blocks, MessageContentBlock{
						Type:         "text",
						Text:         part.Text,
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					})
				}
			case "image_url":
				if block, ok := convertImageURLToAnthropicBlock(part); ok {
					blocks = append(blocks, block)
				}
			}
		}

		return &MessageContent{
			MultipleContent: blocks,
		}
	}

	return nil
}

func convertToAnthropicSystemPrompt(chatReq *llm.Request) *SystemPrompt {
	systemMessages := lo.Filter(chatReq.Messages, func(msg llm.Message, _ int) bool {
		return msg.Role == "system"
	})

	// Check if system was originally in array format
	wasArrayFormat := chatReq.TransformerMetadata != nil && chatReq.TransformerMetadata["anthropic_system_array_format"] == "true"

	switch len(systemMessages) {
	case 0:
		// Leave System as nil when there are no system messages
		return nil
	case 1:
		// If it was originally in array format, preserve that format
		if wasArrayFormat {
			return &SystemPrompt{
				MultiplePrompts: []SystemPromptPart{{
					Type:         "text",
					Text:         *systemMessages[0].Content.Content,
					CacheControl: convertToAnthropicCacheControl(systemMessages[0].CacheControl),
				}},
			}
		}

		return &SystemPrompt{
			Prompt: systemMessages[0].Content.Content,
		}
	default:
		return &SystemPrompt{
			MultiplePrompts: lo.Map(systemMessages, func(msg llm.Message, _ int) SystemPromptPart {
				part := SystemPromptPart{
					Type:         "text",
					Text:         *msg.Content.Content,
					CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
				}

				return part
			}),
		}
	}
}

func convertMultiplePartContent(msg llm.Message) (MessageContent, bool) {
	blocks := make([]MessageContentBlock, 0, len(msg.Content.MultipleContent))

	// Process content parts in order to preserve original sequence
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				blocks = append(blocks, MessageContentBlock{
					Type:         "text",
					Text:         part.Text,
					CacheControl: convertToAnthropicCacheControl(part.CacheControl),
				})
			}
		case "image_url":
			if part.ImageURL != nil && part.ImageURL.URL != "" {
				// Convert OpenAI image format to Anthropic format
				url := part.ImageURL.URL
				if parsed := xurl.ParseDataURL(url); parsed != nil {
					block := MessageContentBlock{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: parsed.MediaType,
							Data:      parsed.Data,
						},
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					}

					blocks = append(blocks, block)
				} else {
					block := MessageContentBlock{
						Type: "image",
						Source: &ImageSource{
							Type: "url",
							URL:  part.ImageURL.URL,
						},
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					}

					blocks = append(blocks, block)
				}
			}
		}
	}

	for _, toolCall := range msg.ToolCalls {
		// Use safe JSON repair/fallback for tool input
		blocks = append(blocks, MessageContentBlock{
			Type:         "tool_use",
			ID:           toolCall.ID,
			Name:         &toolCall.Function.Name,
			Input:        xjson.SafeJSONRawMessage(toolCall.Function.Arguments),
			CacheControl: convertToAnthropicCacheControl(toolCall.CacheControl),
		})
	}

	if len(blocks) == 0 {
		return MessageContent{}, false
	}

	return MessageContent{
		MultipleContent: blocks,
	}, true
}

// convertToLlmResponse converts Anthropic Message to unified Response format.
func convertToLlmResponse(anthropicResp *Message, platformType PlatformType) *llm.Response {
	if anthropicResp == nil {
		return &llm.Response{
			ID:      "",
			Object:  "chat.completion",
			Model:   "",
			Created: 0,
		}
	}

	resp := &llm.Response{
		ID:      anthropicResp.ID,
		Object:  "chat.completion",
		Model:   anthropicResp.Model,
		Created: 0, // Anthropic doesn't provide created timestamp
	}

	// Convert content to message
	var (
		content              llm.MessageContent
		thinkingText         *string
		thinkingSignature    *string
		redactedThinkingData *string
		toolCalls            []llm.ToolCall
		textParts            []string
	)

	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
				content.MultipleContent = append(content.MultipleContent, llm.MessageContentPart{
					Type:     "text",
					Text:     block.Text,
					ImageURL: &llm.ImageURL{},
				})
			}
		case "image":
			if block.Source != nil {
				content.MultipleContent = append(content.MultipleContent, llm.MessageContentPart{
					Type: "image",
					ImageURL: &llm.ImageURL{
						URL: block.Source.Data,
					},
				})
			}
		case "tool_use":
			if block.ID != "" && block.Name != nil {
				// Repair or safely fallback invalid JSON from provider
				repaired := xjson.SafeJSONRawMessage(string(block.Input))
				toolCall := llm.ToolCall{
					ID:   block.ID,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      *block.Name,
						Arguments: string(repaired),
					},
				}
				toolCalls = append(toolCalls, toolCall)
			}
		case "thinking":
			if block.Thinking != nil {
				thinkingText = block.Thinking
			}

			thinkingSignature = block.Signature
		case "redacted_thinking":
			if block.Data != "" {
				redactedThinkingData = &block.Data
			}
		}
	}

	// If we only have text content and no other types, set Content.Content
	if len(textParts) > 0 && len(content.MultipleContent) == len(textParts) {
		// Join all text parts
		var allText string
		for _, text := range textParts {
			allText += text
		}

		content.Content = &allText
		// Clear MultipleContent since we're using the simple string format
		content.MultipleContent = nil
	}

	message := &llm.Message{
		Role:                     anthropicResp.Role,
		Content:                  content,
		ToolCalls:                toolCalls,
		ReasoningContent:         thinkingText,
		ReasoningSignature:       thinkingSignature,
		RedactedReasoningContent: redactedThinkingData,
	}

	choice := llm.Choice{
		Index:        0,
		Message:      message,
		FinishReason: convertToLlmFinishReason(anthropicResp.StopReason),
	}

	resp.Choices = []llm.Choice{choice}

	resp.Usage = convertToLlmUsage(anthropicResp.Usage, platformType)

	return resp
}

func convertToLlmFinishReason(stopReason *string) *string {
	if stopReason == nil {
		return nil
	}

	switch *stopReason {
	case "end_turn":
		return lo.ToPtr("stop")
	case "max_tokens":
		return lo.ToPtr("length")
	case "stop_sequence", "pause_turn":
		return lo.ToPtr("stop")
	case "tool_use":
		return lo.ToPtr("tool_calls")
	case "refusal":
		return lo.ToPtr("content_filter")
	default:
		return stopReason
	}
}
