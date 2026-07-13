package anthropic

import (
	"encoding/json"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/internal/pkg/xjson"
	"github.com/looplj/axonhub/llm/internal/pkg/xurl"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// convertToAnthropicRequest converts ChatCompletionRequest to Anthropic MessageRequest.
// Deprecated: Use convertToAnthropicRequestWithConfig instead.
func convertToAnthropicRequest(chatReq *llm.Request) *MessageRequest {
	return convertToAnthropicRequestWithConfig(chatReq, nil)
}

func convertToAnthropicRequestWithConfig(chatReq *llm.Request, config *Config) *MessageRequest {
	return convertToAnthropicRequestWithThinkingPlan(chatReq, config, resolveThinkingRequestPlan(chatReq, config))
}

func convertToAnthropicRequestWithThinkingPlan(
	chatReq *llm.Request,
	config *Config,
	thinkingPlan thinkingRequestPlan,
) *MessageRequest {
	req := buildBaseRequest(chatReq, thinkingPlan)
	req.Tools = convertToolsAnthropic(chatReq.Tools, config)
	req.Tools = appendAnthropicRawTools(req.Tools, chatReq)
	req.ToolChoice = convertToolChoiceToAnthropic(chatReq.ToolChoice)

	// Map parallel_tool_calls (true=allow) inversely to Anthropic's
	// disable_parallel_tool_use (true=disable), which nests inside tool_choice.
	// Synthesize {type:"auto"} when the client expressed a parallel preference
	// without setting tool_choice; skip when there are no tools (Anthropic
	// requires tools for disable_parallel_tool_use) or tool_choice is "none".
	if len(chatReq.Tools) > 0 && chatReq.ParallelToolCalls != nil {
		if req.ToolChoice == nil {
			req.ToolChoice = &ToolChoice{Type: "auto"}
		}
		if req.ToolChoice.Type != "none" {
			req.ToolChoice.DisableParallelToolUse = lo.ToPtr(!*chatReq.ParallelToolCalls)
		}
	}
	req.Messages = convertMessages(hydrateAnthropicRawContent(chatReq), config)
	req.StopSequences = convertStopSequences(chatReq.Stop)

	// DeepSeek requires assistant messages in history to include a thinking block
	// when thinking is enabled (matching their OpenAI API behavior).
	if config != nil && config.Type == PlatformDeepSeek && isThinkingEnabled(req) {
		ensureAssistantThinkingBlocks(req.Messages)
	}

	// Prefer the full stashed OutputConfig (effort + format/task_budget) when the
	// platform supports output_config. Effort-only conversion already lives in the
	// thinking capability planner; this restores richer same-protocol fields.
	if chatReq.TransformerMetadata != nil && supportsOutputConfig(config) {
		if oc, ok := chatReq.TransformerMetadata[TransformerMetadataKeyOutputConfig].(*OutputConfig); ok && oc != nil {
			req.OutputConfig = oc
		}
	}

	return req
}

// hydrateAnthropicRawContent materializes request-side raw fragments only in a
// short-lived outbound copy. The canonical llm.Request keeps opaque Anthropic
// bytes in ProviderExtensions rather than TransformerMetadata; existing block
// builders can then reuse their ordered-content logic without making metadata
// a persistent provider field bag.
func hydrateAnthropicRawContent(chatReq *llm.Request) *llm.Request {
	if chatReq == nil || chatReq.ProviderExtensions == nil || chatReq.ProviderExtensions.Anthropic == nil ||
		chatReq.ProviderExtensions.Anthropic.Request == nil {
		return chatReq
	}

	fragments := chatReq.ProviderExtensions.Anthropic.Request.RawContentFragments
	if len(fragments) == 0 {
		return chatReq
	}

	cloned := *chatReq
	cloned.Messages = append([]llm.Message(nil), chatReq.Messages...)
	for messageIndex := range cloned.Messages {
		message := &cloned.Messages[messageIndex]
		if len(message.Content.MultipleContent) == 0 {
			continue
		}

		parts := append([]llm.MessageContentPart(nil), message.Content.MultipleContent...)
		for partIndex := range parts {
			part := &parts[partIndex]
			if part.Type != "anthropic_raw_block" {
				continue
			}
			for _, fragment := range fragments {
				if fragment.MessageIndex != messageIndex || fragment.PartIndex != getAnthropicBlockIndex(part.TransformerMetadata) ||
					fragment.NestedInToolResult != (message.Role == "tool") {
					continue
				}
				part.TransformerMetadata = cloneAnthropicTransformerMetadata(part.TransformerMetadata)
				setAnthropicRawBlock(&part.TransformerMetadata, fragment.Raw)
				break
			}
		}
		message.Content.MultipleContent = parts
	}

	return &cloned
}

func cloneAnthropicTransformerMetadata(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(src)+1)
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func isThinkingEnabled(req *MessageRequest) bool {
	return req.Thinking == nil || req.Thinking.Type != "disabled"
}

func ensureAssistantThinkingBlocks(messages []MessageParam) {
	for i, msg := range messages {
		if msg.Role != "assistant" {
			continue
		}

		if hasThinkingBlock(msg) {
			continue
		}

		emptyThinking := ""
		thinkingBlock := MessageContentBlock{
			Type:     "thinking",
			Thinking: &emptyThinking,
		}

		if msg.Content.Content != nil {
			// Convert simple string content to multiple content with thinking + text
			textBlock := MessageContentBlock{
				Type: "text",
				Text: msg.Content.Content,
			}
			messages[i].Content = MessageContent{
				MultipleContent: []MessageContentBlock{thinkingBlock, textBlock},
			}
		} else {
			// Prepend thinking block to existing multiple content
			messages[i].Content.MultipleContent = append(
				[]MessageContentBlock{thinkingBlock},
				messages[i].Content.MultipleContent...,
			)
		}
	}
}

func hasThinkingBlock(msg MessageParam) bool {
	for _, block := range msg.Content.MultipleContent {
		if block.Type == "thinking" {
			return true
		}
	}

	return false
}

func shouldDecodeAnthropicSignature(config *Config) bool {
	if config == nil {
		return true
	}

	switch config.Type {
	case "", PlatformDirect, PlatformClaudeCode, PlatformVertex, PlatformBedrock:
		return true
	default:
		return false
	}
}

func prepareAnthropicReasoning(reasoningContent, reasoningSignature *string, config *Config) (*string, *string) {
	if reasoningSignature == nil || *reasoningSignature == "" {
		return reasoningContent, reasoningSignature
	}

	if shouldDecodeAnthropicSignature(config) {
		if decoded := shared.DecodeAnthropicSignature(reasoningSignature); decoded != nil {
			return reasoningContent, decoded
		}

		return reasoningContent, nil
	}

	return reasoningContent, reasoningSignature
}

// buildBaseRequest creates the base MessageRequest with common fields.
func buildBaseRequest(chatReq *llm.Request, thinkingPlan thinkingRequestPlan) *MessageRequest {
	req := &MessageRequest{
		Model:       chatReq.Model,
		Temperature: chatReq.Temperature,
		TopP:        chatReq.TopP,
		Stream:      chatReq.Stream,
		System:      convertToAnthropicSystemPrompt(chatReq),
		MaxTokens:   resolveMaxTokens(chatReq),
	}

	// Restore service_tier so it survives non-pass-through format conversion.
	if chatReq.ServiceTier != nil {
		req.ServiceTier = *chatReq.ServiceTier
	}

	// Identity: prefer anthropic-native metadata.user_id (authoritative for
	// same-protocol round-trip); fall back to canonical.User so chat/responses
	// -> anthropic routing preserves the user (#13 bridge).
	userID := ""
	if chatReq.Metadata != nil {
		userID = chatReq.Metadata["user_id"]
	}
	if userID == "" && chatReq.User != nil {
		userID = *chatReq.User
	}
	if userID != "" {
		req.Metadata = &AnthropicMetadata{UserID: userID}
	}

	req.Thinking = thinkingPlan.thinking
	req.OutputConfig = thinkingPlan.outputConfig

	// Restore thinking display from TransformerMetadata
	if req.Thinking != nil && chatReq.TransformerMetadata != nil {
		if display, ok := chatReq.TransformerMetadata[TransformerMetadataKeyThinkingDisplay].(string); ok && display != "" {
			req.Thinking.Display = display
		}
	}

	// Restore Anthropic's top-level cache_control (automatic prompt caching).
	// When present we keep it as-is on the upstream request and skip our own
	// per-block breakpoint optimization (handled in TransformRequest).
	if chatReq.TransformerMetadata != nil {
		if raw := asJSONRawMessage(chatReq.TransformerMetadata[TransformerMetadataKeyCacheControl]); len(raw) > 0 {
			var cc CacheControl
			if err := json.Unmarshal(raw, &cc); err == nil && cc.Type != "" {
				req.CacheControl = &cc
			}
		}
	}

	// Restore Anthropic's top-level context_management (context-compression
	// strategy) carried through TransformerMetadata as opaque json.RawMessage.
	if chatReq.TransformerMetadata != nil {
		if cm := asJSONRawMessage(chatReq.TransformerMetadata[TransformerMetadataKeyContextManagement]); len(cm) > 0 {
			req.ContextManagement = cm
		}
	}

	// Restore Anthropic-native container / inference_geo as opaque JSON.
	if chatReq.TransformerMetadata != nil {
		if container := asJSONRawMessage(chatReq.TransformerMetadata[TransformerMetadataKeyContainer]); len(container) > 0 {
			req.Container = container
		}
		if geo := asJSONRawMessage(chatReq.TransformerMetadata[TransformerMetadataKeyInferenceGeo]); len(geo) > 0 {
			req.InferenceGeo = geo
		}
		if mcpServers := asJSONRawMessage(chatReq.TransformerMetadata[TransformerMetadataKeyMCPServers]); len(mcpServers) > 0 {
			req.MCPServers = mcpServers
		}

	}

	// Restore top_k carried through TransformerMetadata (canonical llm.Request
	// has no TopK field). Mirrors the cache_control restoration above.
	if chatReq.TransformerMetadata != nil {
		if topK, ok := chatReq.TransformerMetadata[shared.TransformerMetadataKeyTopK].(*int64); ok && topK != nil {
			req.TopK = topK
		} else if topK, ok := chatReq.TransformerMetadata[transformerMetadataKeyTopKLegacy].(*int64); ok && topK != nil {
			req.TopK = topK
		}
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

// anthropicRawToolFragment preserves adapter-specific tools[] entries with their
// original wire index so same-protocol replay can reinsert them among converted
// function/web_search tools without reordering.
type anthropicRawToolFragment struct {
	OriginalIndex int
	Raw           json.RawMessage
}

// appendAnthropicRawTools re-attaches Anthropic tool declaration fragments at
// their original tools[] indexes. Exclusive raw variants (mcp_toolset /
// non-web-search native tools) are inserted as raw-only tools; function/web_search
// raw fragments replace the common typed reconstruction so Anthropic-only children
// are preserved.
func appendAnthropicRawTools(tools []Tool, chatReq *llm.Request) []Tool {
	frags := anthropicRawToolFragments(chatReq)
	if len(frags) == 0 {
		return tools
	}

	rawByIndex := make(map[int]json.RawMessage, len(frags))
	maxIndex := len(tools) + len(frags) - 1
	for _, frag := range frags {
		if len(frag.Raw) == 0 {
			continue
		}
		rawByIndex[frag.OriginalIndex] = frag.Raw
		if frag.OriginalIndex > maxIndex {
			maxIndex = frag.OriginalIndex
		}
	}
	if len(rawByIndex) == 0 {
		return tools
	}

	out := make([]Tool, 0, maxIndex+1)
	commonIdx := 0
	for i := 0; i <= maxIndex; i++ {
		if raw, ok := rawByIndex[i]; ok {
			tool := Tool{Raw: append(json.RawMessage(nil), raw...)}
			// Best-effort type probe for debug/clarity; MarshalJSON uses Raw.
			var probe struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(raw, &probe)
			if probe.Type != "" {
				tool.Type = probe.Type
			}
			// If this raw fragment overlays a common tool (function/web_search),
			// consume that common tool slot so order stays correct.
			if !isExclusiveAnthropicRawTool(tool) && commonIdx < len(tools) {
				commonIdx++
			}
			out = append(out, tool)
			continue
		}
		if commonIdx < len(tools) {
			out = append(out, tools[commonIdx])
			commonIdx++
		}
	}
	for commonIdx < len(tools) {
		out = append(out, tools[commonIdx])
		commonIdx++
	}
	return out
}

func anthropicRawToolFragments(chatReq *llm.Request) []anthropicRawToolFragment {
	if chatReq == nil || chatReq.TransformerMetadata == nil {
		return nil
	}
	raw, ok := chatReq.TransformerMetadata[TransformerMetadataKeyRawTools]
	if !ok || raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []anthropicRawToolFragment:
		return v
	case []json.RawMessage:
		// Backward-compatible: legacy unindexed fragments append after common tools.
		out := make([]anthropicRawToolFragment, 0, len(v))
		for i, frag := range v {
			if len(frag) == 0 {
				continue
			}
			out = append(out, anthropicRawToolFragment{OriginalIndex: i, Raw: frag})
		}
		return out
	case []any:
		out := make([]anthropicRawToolFragment, 0, len(v))
		for i, item := range v {
			switch frag := item.(type) {
			case anthropicRawToolFragment:
				out = append(out, frag)
			case map[string]any:
				idx := i
				if n, ok := frag["OriginalIndex"].(int); ok {
					idx = n
				} else if n, ok := frag["OriginalIndex"].(float64); ok {
					idx = int(n)
				}
				if rawFrag := asJSONRawMessage(frag["Raw"]); len(rawFrag) > 0 {
					out = append(out, anthropicRawToolFragment{OriginalIndex: idx, Raw: rawFrag})
				}
			default:
				if rawFrag := asJSONRawMessage(item); len(rawFrag) > 0 {
					out = append(out, anthropicRawToolFragment{OriginalIndex: i, Raw: rawFrag})
				}
			}
		}
		return out
	default:
		if frag := asJSONRawMessage(raw); len(frag) > 0 {
			return []anthropicRawToolFragment{{OriginalIndex: 0, Raw: frag}}
		}
		return nil
	}
}

// convertToolsAnthropic converts LLM tools to Anthropic tools.
// If the platform is not direct Anthropic API or Bedrock, anthropic native tools (like web_search) are filtered out.
// Only web_search tool is supported as native tool, other native tools (image_generation, google_*, etc.) are ignored.
func convertToolsAnthropic(tools []llm.Tool, config *Config) []Tool {
	if len(tools) == 0 {
		return nil
	}

	anthropicTools := make([]Tool, 0, len(tools))

	supportsNativeTools := supportsAnthropicNativeTools(config)

	for _, tool := range tools {
		switch tool.Type {
		case llm.ToolTypeFunction:
			anthropicTools = append(anthropicTools, Tool{
				Name:         tool.Function.Name,
				Description:  tool.Function.Description,
				InputSchema:  tool.Function.Parameters,
				CacheControl: convertToAnthropicCacheControl(tool.CacheControl),
			})
		case llm.ToolTypeWebSearch:
			// Already transformed Anthropic native tool type
			// If platform doesn't support native tools, skip this tool
			if !supportsNativeTools {
				continue
			}

			anthropicTool := Tool{
				Type: ToolTypeWebSearch20250305,
				Name: WebSearchFunctionName,
			}
			// Copy web search parameters if available
			if tool.WebSearch != nil {
				anthropicTool.MaxUses = tool.WebSearch.MaxUses
				anthropicTool.Strict = tool.WebSearch.Strict
				anthropicTool.AllowedDomains = tool.WebSearch.AllowedDomains
				anthropicTool.BlockedDomains = tool.WebSearch.BlockedDomains
				anthropicTool.UserLocation = WebSearchToolUserLocation{
					City:     tool.WebSearch.UserLocation.City,
					Country:  tool.WebSearch.UserLocation.Country,
					Region:   tool.WebSearch.UserLocation.Region,
					Timezone: tool.WebSearch.UserLocation.Timezone,
					Type:     tool.WebSearch.UserLocation.Type,
				}
			}

			anthropicTools = append(anthropicTools, anthropicTool)
		default:
			// Ignore other native tools (image_generation, google_*, etc.)
			continue
		}
	}

	return anthropicTools
}

// convertToolChoiceToAnthropic converts llm.ToolChoice to Anthropic ToolChoice.
func convertToolChoiceToAnthropic(src *llm.ToolChoice) *ToolChoice {
	if src == nil {
		return nil
	}

	// String-form tool_choice: "auto", "none", "any", "required"
	if src.ToolChoice != nil {
		choice := *src.ToolChoice

		// OpenAI "required" is equivalent to Anthropic "any"
		if choice == "required" {
			choice = "any"
		}

		return &ToolChoice{
			Type: choice,
		}
	}

	// Named tool_choice: {type: "function", function: {name: "xxx"}}
	if src.NamedToolChoice != nil && src.NamedToolChoice.Function.Name != "" {
		return &ToolChoice{
			Type: "tool",
			Name: lo.ToPtr(src.NamedToolChoice.Function.Name),
		}
	}

	return nil
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
func convertMessages(chatReq *llm.Request, config *Config) []MessageParam {
	messages := make([]MessageParam, 0, len(chatReq.Messages))
	// First, filter out system and developer messages as they are handled separately.
	nonSystemMsgs := lo.Filter(chatReq.Messages, func(msg llm.Message, _ int) bool {
		return msg.Role != "system" && msg.Role != "developer"
	})

	// Track which message indexes have been processed (for user messages with MessageIndex and tool messages)
	processedMessageIndexes := make(map[int]bool)
	processedToolCallIDs := make(map[string]bool)

	for i := 0; i < len(nonSystemMsgs); i++ {
		msg := nonSystemMsgs[i]

		if processedMessageIndexes[i] {
			continue
		}

		switch msg.Role {
		case "tool":
			// Handle standalone tool messages (not following an assistant with tool calls)
			// Group consecutive tool messages into a single user message with tool_results
			if toolMsg, newIndex, created := groupToolResultMessages(nonSystemMsgs, i, processedMessageIndexes, processedToolCallIDs); created {
				messages = append(messages, toolMsg)
				i = newIndex
			}
		case "user":
			// Skip user messages that have MessageIndex and have already been processed
			// (these are merged with tool_result messages)
			if msg.MessageIndex != nil && processedMessageIndexes[*msg.MessageIndex] {
				continue
			}

			if converted, ok := convertUserMessage(msg); ok {
				messages = append(messages, converted...)
			}
		case "assistant":
			// Convert the assistant message.
			if assistantMsg, ok := convertAssistantMessage(msg, config); ok {
				messages = append(messages, assistantMsg...)
			}

			// After an assistant message with tool calls, the next message might be tool results.
			if len(msg.ToolCalls) > 0 {
				// Try to find corresponding tool results, even if not immediately following.
				if toolMsg, ok := findToolResultsForAssistant(nonSystemMsgs, msg.ToolCalls, processedToolCallIDs, processedMessageIndexes); ok {
					messages = append(messages, toolMsg)
				} else if i+1 < len(nonSystemMsgs) {
					// Fallback to grouping consecutive tool messages if no explicit match found (legacy behavior)
					if toolMsg, newIndex, created := groupToolResultMessages(nonSystemMsgs, i+1, processedMessageIndexes, processedToolCallIDs); created {
						messages = append(messages, toolMsg)
						i = newIndex
					}
				}
			}
		}
	}

	return messages
}

// findToolResultsForAssistant looks for tool results matching the given tool calls.
func findToolResultsForAssistant(
	messages []llm.Message,
	toolCalls []llm.ToolCall,
	processedToolCallIDs map[string]bool,
	processedMessageIndexes map[int]bool,
) (MessageParam, bool) {
	var (
		toolResultBlocks []MessageContentBlock
		toolMsgIndexes   = make(map[int]struct{})
	)

	for _, tc := range toolCalls {
		if processedToolCallIDs[tc.ID] {
			continue
		}

		// Look for this tool call ID in all messages
		for i, msg := range messages {
			if msg.Role == "tool" && msg.ToolCallID != nil && *msg.ToolCallID == tc.ID {
				toolResultBlocks = append(toolResultBlocks, convertToToolResultBlock(msg))
				processedToolCallIDs[tc.ID] = true
				processedMessageIndexes[i] = true

				if msg.MessageIndex != nil {
					toolMsgIndexes[*msg.MessageIndex] = struct{}{}
				}

				break
			}
		}
	}

	// Look for related user message with the same MessageIndex
	if len(toolMsgIndexes) > 0 {
		for j := range messages {
			if processedMessageIndexes[j] {
				continue
			}

			userMsg := messages[j]
			if userMsg.Role == "user" && userMsg.MessageIndex != nil {
				if _, ok := toolMsgIndexes[*userMsg.MessageIndex]; ok {
					userBlocks := extractUserContentBlocks(userMsg)
					toolResultBlocks = append(toolResultBlocks, userBlocks...)
					processedMessageIndexes[j] = true
				}
			}
		}
	}

	if len(toolResultBlocks) > 0 {
		return MessageParam{
			Role: "user",
			Content: MessageContent{
				MultipleContent: toolResultBlocks,
			},
		}, true
	}

	return MessageParam{}, false
}

// groupToolResultMessages groups consecutive tool messages and finds related user message content.
// Returns the combined message param, updated index, and whether a message was created.
func groupToolResultMessages(messages []llm.Message, startIndex int, processedIndexes map[int]bool, processedIDs map[string]bool) (MessageParam, int, bool) {
	var (
		toolResultBlocks []MessageContentBlock
		toolMsgIndexes   = make(map[int]struct{})
		currentIndex     = startIndex
	)

	// Group consecutive tool messages
	for currentIndex < len(messages) && messages[currentIndex].Role == "tool" {
		toolMsg := messages[currentIndex]
		if toolMsg.ToolCallID != nil && processedIDs[*toolMsg.ToolCallID] {
			currentIndex++
			continue
		}

		toolResultBlocks = append(toolResultBlocks, convertToToolResultBlock(toolMsg))
		if toolMsg.ToolCallID != nil {
			processedIDs[*toolMsg.ToolCallID] = true
		}

		if toolMsg.MessageIndex != nil {
			toolMsgIndexes[*toolMsg.MessageIndex] = struct{}{}
		}

		processedIndexes[currentIndex] = true
		currentIndex++
	}

	// Look for related user message with the same MessageIndex
	if len(toolMsgIndexes) > 0 {
		for j := range messages {
			if processedIndexes[j] {
				continue
			}

			userMsg := messages[j]
			if userMsg.Role == "user" && userMsg.MessageIndex != nil {
				if _, ok := toolMsgIndexes[*userMsg.MessageIndex]; ok {
					userBlocks := extractUserContentBlocks(userMsg)
					toolResultBlocks = append(toolResultBlocks, userBlocks...)
					processedIndexes[j] = true
				}
			}
		}
	}

	if len(toolResultBlocks) > 0 {
		return MessageParam{
			Role: "user",
			Content: MessageContent{
				MultipleContent: toolResultBlocks,
			},
		}, currentIndex - 1, true
	}

	return MessageParam{}, startIndex, false
}

// extractUserContentBlocks extracts content blocks from a user message.
func extractUserContentBlocks(msg llm.Message) []MessageContentBlock {
	var blocks []MessageContentBlock

	if msg.Content.Content != nil && *msg.Content.Content != "" {
		blocks = append(blocks, MessageContentBlock{
			Type:         "text",
			Text:         msg.Content.Content,
			CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
		})
	} else if len(msg.Content.MultipleContent) > 0 {
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
				if block, ok := convertImageURLToAnthropicBlock(part); ok {
					blocks = append(blocks, block)
				}
			case "document":
				if block, ok := convertDocumentURLToAnthropicBlock(part); ok {
					blocks = append(blocks, block)
				}
			case "anthropic_raw_block":
				// Preserve top-level future/unknown blocks that ride next to
				// tool_result through groupToolResultMessages / findToolResultsForAssistant.
				if raw := getAnthropicRawBlock(part.TransformerMetadata); len(raw) > 0 {
					blocks = append(blocks, MessageContentBlock{
						Type: "anthropic_raw_block",
						Raw:  append(json.RawMessage(nil), raw...),
					})
				}
			}
		}
	}

	return blocks
}

// convertUserMessage handles user message conversion.
func convertUserMessage(msg llm.Message) ([]MessageParam, bool) {
	content, ok := buildMessageContent(msg, nil)
	if !ok {
		return nil, false
	}

	return []MessageParam{{Role: "user", Content: content}}, true
}

// convertAssistantMessage handles assistant message conversion.
func convertAssistantMessage(msg llm.Message, config *Config) ([]MessageParam, bool) {
	return convertAssistantWithToolCalls(msg, config)
}

// convertAssistantWithToolCalls handles assistant messages that have tool calls.
func convertAssistantWithToolCalls(msg llm.Message, config *Config) ([]MessageParam, bool) {
	preBlocks := buildPreBlocks(msg, config)
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
func buildPreBlocks(msg llm.Message, config *Config) []MessageContentBlock {
	var blocks []MessageContentBlock

	// Prefer ReasoningContent, fallback to Reasoning if ReasoningContent is nil
	reasoningContent := msg.ReasoningContent
	if reasoningContent == nil && msg.Reasoning != nil {
		reasoningContent = msg.Reasoning
	}

	reasoningContent, reasoningSignature := prepareAnthropicReasoning(
		reasoningContent,
		msg.ReasoningSignature,
		config,
	)

	if block := buildThinkingBlock(reasoningContent, reasoningSignature); block != nil {
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
func buildMessageContent(msg llm.Message, config *Config) (MessageContent, bool) {
	// Handle simple string content
	if msg.Content.Content != nil {
		if msg.CacheControl != nil || hasThinkingContent(msg) {
			return buildMultipleContentWithThinking(msg, config), true
		}

		return MessageContent{Content: msg.Content.Content}, true
	}

	var blocks []MessageContentBlock

	if hasThinkingContent(msg) {
		// Prefer ReasoningContent, fallback to Reasoning if ReasoningContent is nil
		reasoningContent := msg.ReasoningContent
		if reasoningContent == nil && msg.Reasoning != nil {
			reasoningContent = msg.Reasoning
		}

		reasoningContent, reasoningSignature := prepareAnthropicReasoning(
			reasoningContent,
			msg.ReasoningSignature,
			config,
		)

		if block := buildThinkingBlock(reasoningContent, reasoningSignature); block != nil {
			blocks = append(blocks, *block)
		}

		if block := buildRedactedThinkingBlock(msg.RedactedReasoningContent); block != nil {
			blocks = append(blocks, *block)
		}
	}

	content, ok := convertMultiplePartContent(msg)

	switch {
	case ok && len(blocks) > 0:
		return MessageContent{}, false
	case len(blocks) > 0:
		content.MultipleContent = append(blocks, content.MultipleContent...)
		return content, true
	}

	return content, true
}

// hasThinkingContent checks if message has reasoning content.
func hasThinkingContent(msg llm.Message) bool {
	return (msg.ReasoningContent != nil && *msg.ReasoningContent != "") ||
		(msg.RedactedReasoningContent != nil && *msg.RedactedReasoningContent != "") ||
		(msg.Reasoning != nil && *msg.Reasoning != "")
}

// buildMultipleContentWithThinking creates content blocks including thinking.
func buildMultipleContentWithThinking(msg llm.Message, config *Config) MessageContent {
	blocks := make([]MessageContentBlock, 0, 3)

	// Prefer ReasoningContent, fallback to Reasoning if ReasoningContent is nil
	reasoningContent := msg.ReasoningContent
	if reasoningContent == nil && msg.Reasoning != nil {
		reasoningContent = msg.Reasoning
	}

	reasoningContent, reasoningSignature := prepareAnthropicReasoning(
		reasoningContent,
		msg.ReasoningSignature,
		config,
	)

	if block := buildThinkingBlock(reasoningContent, reasoningSignature); block != nil {
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
		Type:     "thinking",
		Thinking: reasoningContent,
	}
	if reasoningSignature != nil && *reasoningSignature != "" {
		block.Signature = reasoningSignature
	}

	return block
}

// buildRedactedThinkingBlock creates a redacted_thinking block from encrypted content.
func buildRedactedThinkingBlock(redactedContent *string) *MessageContentBlock {
	if redactedContent == nil || *redactedContent == "" {
		return nil
	}

	return &MessageContentBlock{
		Type: "redacted_thinking",
		Data: *redactedContent,
	}
}

func convertToToolResultBlock(msg llm.Message) MessageContentBlock {
	block := MessageContentBlock{
		Type:         "tool_result",
		ToolUseID:    msg.ToolCallID,
		CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
		IsError:      msg.ToolCallIsError,
	}

	// Rebuild from typed parts, including anthropic_raw_block children that
	// preserve unknown nested tool_result content.
	if content := convertToAnthropicTrivialContent(msg.Content); content != nil {
		block.Content = content
	}
	return block
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

// convertDocumentURLToAnthropicBlock converts a canonical document content part
// to an Anthropic document content block (used for PDFs etc.).
func convertDocumentURLToAnthropicBlock(part llm.MessageContentPart) (MessageContentBlock, bool) {
	if part.Document == nil || part.Document.URL == "" {
		return MessageContentBlock{}, false
	}

	url := part.Document.URL
	if parsed := xurl.ParseDataURL(url); parsed != nil {
		return MessageContentBlock{
			Type: "document",
			Source: &ImageSource{
				Type:      "base64",
				MediaType: parsed.MediaType,
				Data:      parsed.Data,
			},
			CacheControl: convertToAnthropicCacheControl(part.CacheControl),
		}, true
	}

	return MessageContentBlock{
		Type: "document",
		Source: &ImageSource{
			Type: "url",
			URL:  part.Document.URL,
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
			case "document":
				if block, ok := convertDocumentURLToAnthropicBlock(part); ok {
					blocks = append(blocks, block)
				}
			case "anthropic_raw_block":
				if raw := getAnthropicRawBlock(part.TransformerMetadata); len(raw) > 0 {
					blocks = append(blocks, MessageContentBlock{
						Type: "anthropic_raw_block",
						Raw:  append(json.RawMessage(nil), raw...),
					})
				}
			}
		}

		return &MessageContent{
			MultipleContent: blocks,
		}
	}

	return nil
}

func systemMessageToParts(msg llm.Message) []SystemPromptPart {
	if msg.Content.Content != nil {
		return []SystemPromptPart{{
			Type:         "text",
			Text:         *msg.Content.Content,
			CacheControl: convertToAnthropicCacheControl(msg.CacheControl),
		}}
	}

	parts := make([]SystemPromptPart, 0, len(msg.Content.MultipleContent))
	for _, part := range msg.Content.MultipleContent {
		if part.Type == "text" && part.Text != nil {
			parts = append(parts, SystemPromptPart{
				Type:         "text",
				Text:         *part.Text,
				CacheControl: convertToAnthropicCacheControl(part.CacheControl),
			})
		}
	}

	return parts
}

func convertToAnthropicSystemPrompt(chatReq *llm.Request) *SystemPrompt {
	// Partition messages into system and developer roles in a single loop for better performance
	var systemOnlyMessages, developerMessages []llm.Message

	for _, msg := range chatReq.Messages {
		switch msg.Role {
		case "system":
			systemOnlyMessages = append(systemOnlyMessages, msg)
		case "developer":
			developerMessages = append(developerMessages, msg)
		}
	}

	systemMessages := append(systemOnlyMessages, developerMessages...)

	// Check if system was originally in array format
	wasArrayFormat := chatReq.TransformOptions.ArrayInstructions != nil && *chatReq.TransformOptions.ArrayInstructions

	switch len(systemMessages) {
	case 0:
		// Leave System as nil when there are no system messages
		return nil
	case 1:
		msg := systemMessages[0]
		parts := systemMessageToParts(msg)

		// If it was originally in array format, preserve that format
		if wasArrayFormat {
			return &SystemPrompt{
				MultiplePrompts: parts,
			}
		}

		// Single string format
		if len(parts) == 1 {
			return &SystemPrompt{
				Prompt: &parts[0].Text,
			}
		}

		return &SystemPrompt{
			MultiplePrompts: parts,
		}
	default:
		// Combine system and developer messages in order
		var parts []SystemPromptPart
		for _, msg := range systemMessages {
			parts = append(parts, systemMessageToParts(msg)...)
		}

		return &SystemPrompt{
			MultiplePrompts: parts,
		}
	}
}

func convertMultiplePartContent(msg llm.Message) (MessageContent, bool) {
	var ordered []orderedContentBlock

	appendOrdered := func(meta map[string]any, b MessageContentBlock) {
		ordered = append(ordered, orderedContentBlock{
			idx:   getAnthropicBlockIndex(meta),
			order: len(ordered),
			block: b,
		})
	}

	// Process content parts in order to preserve original sequence
	for _, part := range msg.Content.MultipleContent {
		switch part.Type {
		case "text":
			if part.Text != nil {
				appendOrdered(part.TransformerMetadata, MessageContentBlock{
					Type:         "text",
					Text:         part.Text,
					CacheControl: convertToAnthropicCacheControl(part.CacheControl),
				})
			}
		case "anthropic_raw_block":
			if raw := getAnthropicRawBlock(part.TransformerMetadata); len(raw) > 0 {
				appendOrdered(part.TransformerMetadata, MessageContentBlock{
					Type: "anthropic_raw_block",
					Raw:  append(json.RawMessage(nil), raw...),
				})
			}
		case "image_url":
			if part.ImageURL != nil && part.ImageURL.URL != "" {
				url := part.ImageURL.URL
				if parsed := xurl.ParseDataURL(url); parsed != nil {
					appendOrdered(part.TransformerMetadata, MessageContentBlock{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: parsed.MediaType,
							Data:      parsed.Data,
						},
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					})
				} else {
					appendOrdered(part.TransformerMetadata, MessageContentBlock{
						Type: "image",
						Source: &ImageSource{
							Type: "url",
							URL:  part.ImageURL.URL,
						},
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					})
				}
			}
		case "document":
			if part.Document != nil && part.Document.URL != "" {
				url := part.Document.URL
				if parsed := xurl.ParseDataURL(url); parsed != nil {
					appendOrdered(part.TransformerMetadata, MessageContentBlock{
						Type: "document",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: parsed.MediaType,
							Data:      parsed.Data,
						},
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					})
				} else {
					appendOrdered(part.TransformerMetadata, MessageContentBlock{
						Type: "document",
						Source: &ImageSource{
							Type: "url",
							URL:  part.Document.URL,
						},
						CacheControl: convertToAnthropicCacheControl(part.CacheControl),
					})
				}
			}
		}
	}

	for _, toolCall := range msg.ToolCalls {
		appendOrdered(toolCall.TransformerMetadata, toolUseBlockFromLLM(toolCall))
	}

	for _, ir := range msg.InlineToolResults {
		if block, ok := toolResultBlockFromInline(ir); ok {
			appendOrdered(ir.TransformerMetadata, block)
		}
	}

	sorted := sortOrderedContentBlocks(ordered)

	blocks := make([]MessageContentBlock, 0, len(sorted))
	for _, ob := range sorted {
		blocks = append(blocks, ob.block)
	}

	if len(blocks) == 0 {
		return MessageContent{}, false
	}

	return MessageContent{
		MultipleContent: blocks,
	}, true
}

func llmAnnotationFromCitation(citation TextCitation) (llm.Annotation, bool) {
	if citation.Type == "" {
		return llm.Annotation{}, false
	}

	annotation := llm.Annotation{Type: citation.Type}
	if citation.URL != "" || citation.Title != "" {
		annotation.URLCitation = &llm.URLCitation{
			URL:   citation.URL,
			Title: citation.Title,
		}
	}

	return annotation, true
}

// marshalAnthropicResponseContentRaw deep-copies each Anthropic response content
// block as json.RawMessage for ProviderExtensions.Anthropic.Response.RawContent.
// Prefer original Raw bytes for unknown/future blocks so nested native fields
// survive same-protocol replay without re-encoding loss.
func marshalAnthropicResponseContentRaw(blocks []MessageContentBlock) []json.RawMessage {
	if len(blocks) == 0 {
		return nil
	}

	out := make([]json.RawMessage, len(blocks))
	for i, block := range blocks {
		if len(block.Raw) > 0 {
			out[i] = append(json.RawMessage(nil), block.Raw...)
			continue
		}
		out[i] = append(json.RawMessage(nil), xjson.MustMarshal(block)...)
	}
	return out
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

	var needRawContent bool
	resp := &llm.Response{
		ID:          anthropicResp.ID,
		Object:      "chat.completion",
		Model:       anthropicResp.Model,
		Created:     0, // Anthropic doesn't provide created timestamp
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatAnthropicMessage,
	}

	// Convert content to message
	var (
		content              llm.MessageContent
		thinkingText         *string
		thinkingSignature    *string
		redactedThinkingData *string
		toolCalls            []llm.ToolCall
		annotations          []llm.Annotation
		textParts            []string
		inlineToolResults    []llm.InlineToolResult
	)

	for i := range anthropicResp.Content {
		block := anthropicResp.Content[i]

		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
				part := llm.MessageContentPart{
					Type:     "text",
					Text:     block.Text,
					ImageURL: &llm.ImageURL{},
				}
				setAnthropicBlockIndex(&part.TransformerMetadata, i)
				content.MultipleContent = append(content.MultipleContent, part)
			}
			if len(block.Citations) > 0 {
				annotations = append(annotations, lo.FilterMap(block.Citations, func(citation TextCitation, _ int) (llm.Annotation, bool) {
					return llmAnnotationFromCitation(citation)
				})...)
				// The common annotation model does not carry every Anthropic citation
				// child (encrypted_index, cited_text, and future variants). Keep the
				// original ordered response content on the existing native sidecar.
				needRawContent = true
			}
		case "image":
			if part, ok := convertImageSourceToLLMImageURLPart(block.Source, block.CacheControl); ok {
				setAnthropicBlockIndex(&part.TransformerMetadata, i)
				content.MultipleContent = append(content.MultipleContent, part)
			}
		case "tool_use":
			if block.ID != "" && block.Name != nil {
				tc := toolCallFromAnthropicBlock(block)
				setAnthropicBlockIndex(&tc.TransformerMetadata, i)
				toolCalls = append(toolCalls, tc)
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
		default:
			switch {
			case isAnthropicSpecialToolUseBlock(block.Type):
				if block.ID != "" && block.Name != nil {
					tc := toolCallFromAnthropicBlock(block)
					setAnthropicBlockIndex(&tc.TransformerMetadata, i)
					toolCalls = append(toolCalls, tc)
				}

				needRawContent = true
			case isAnthropicSpecialToolResultBlock(block.Type):
				ir := inlineToolResultFromBlock(&block)
				setAnthropicBlockIndex(&ir.TransformerMetadata, i)
				inlineToolResults = append(inlineToolResults, ir)

				needRawContent = true
			default:
				// Unknown/future response blocks have no common representation.
				// Retain the original ordered content on the existing Anthropic-only
				// response sidecar for same-protocol replay.
				needRawContent = true
			}
		}
	}

	// Collapse text-only MultipleContent into Content.Content whenever doing
	// so does not lose ordering information needed for Anthropic round-trip:
	// it is safe iff every tool call and every inline tool result has a block
	// index strictly greater than every text part's block index. When a
	// server-side tool appears *between* text blocks, keep MultipleContent so
	// per-part anthropic_block_index metadata survives.
	if len(textParts) > 0 && len(content.MultipleContent) == len(textParts) {
		maxTextIdx := -1

		for _, part := range content.MultipleContent {
			if i := getAnthropicBlockIndex(part.TransformerMetadata); i > maxTextIdx {
				maxTextIdx = i
			}
		}

		safeToCollapse := true

		for _, tc := range toolCalls {
			idx := getAnthropicBlockIndex(tc.TransformerMetadata)
			if idx >= 0 && idx < maxTextIdx {
				safeToCollapse = false
				break
			}
		}

		if safeToCollapse {
			for _, ir := range inlineToolResults {
				idx := getAnthropicBlockIndex(ir.TransformerMetadata)
				if idx >= 0 && idx < maxTextIdx {
					safeToCollapse = false
					break
				}
			}
		}

		if safeToCollapse {
			var allText string
			for _, text := range textParts {
				allText += text
			}

			content.Content = &allText
			content.MultipleContent = nil
		}
	}

	message := &llm.Message{
		Role:                     anthropicResp.Role,
		Content:                  content,
		ToolCalls:                toolCalls,
		ReasoningContent:         thinkingText,
		ReasoningSignature:       shared.EncodeAnthropicSignature(thinkingSignature),
		RedactedReasoningContent: redactedThinkingData,
		Annotations:              annotations,
		InlineToolResults:        inlineToolResults,
	}

	finishReason := convertToLlmFinishReason(anthropicResp.StopReason)

	choice := llm.Choice{
		Index:        0,
		Message:      message,
		FinishReason: finishReason,
	}

	// Carry the original Anthropic stop_reason because several native values
	// collapse in the common finish_reason model (for example stop_sequence and
	// pause_turn both become stop, while refusal becomes content_filter).
	// Anthropic outbound replay consumes this adapter-local metadata; other
	// protocol adapters continue to use the common finish reason.
	if anthropicResp.StopReason != nil {
		switch *anthropicResp.StopReason {
		case "stop_sequence", "pause_turn", "refusal":
			choice.TransformerMetadata = map[string]any{
				TransformerMetadataKeyAnthropicStopReason: *anthropicResp.StopReason,
			}
		}
	}

	resp.Choices = []llm.Choice{choice}

	resp.Usage = convertToLlmUsage(anthropicResp.Usage, platformType)

	// Backfill service_tier from usage so it survives non-streaming conversion,
	// mirroring the streaming path.
	if anthropicResp.Usage != nil {
		resp.ServiceTier = anthropicResp.Usage.ServiceTier
	}
	// Preserve Anthropic-only non-stream response fields on their named owner.
	if anthropicResp.StopSequence != nil || len(anthropicResp.StopDetails) > 0 ||
		(anthropicResp.Usage != nil && len(anthropicResp.Usage.Raw) > 0) ||
		needRawContent {
		ext := llm.EnsureAnthropicResponseExtensions(resp)
		if anthropicResp.StopSequence != nil {
			ext.StopSequence = lo.ToPtr(*anthropicResp.StopSequence)
		}
		ext.StopDetails = append(json.RawMessage(nil), anthropicResp.StopDetails...)
		if anthropicResp.Usage != nil {
			ext.RawUsage = append(json.RawMessage(nil), anthropicResp.Usage.Raw...)
		}
		if needRawContent {
			// Full ordered Anthropic content[] for same-protocol replay when common
			// llm.Response cannot own every block (unknown future blocks, citation
			// native details, special tool blocks). Only Anthropic inbound reads it.
			ext.RawContent = marshalAnthropicResponseContentRaw(anthropicResp.Content)
		}
	}

	return resp
}

// toolCallFromAnthropicBlock builds an llm.ToolCall from an Anthropic
// tool_use-like block (tool_use or any special *_tool_use). For special
// blocks, TransformerMetadata is populated with anthropic_type (+ optional
// anthropic_caller) so the block can be round-tripped.

func toolCallFromAnthropicBlock(block MessageContentBlock) llm.ToolCall {
	repaired := xjson.SafeJSONRawMessage(string(block.Input))

	tc := llm.ToolCall{
		ID:   block.ID,
		Type: "function",
		Function: llm.FunctionCall{
			Name:      *block.Name,
			Arguments: string(repaired),
		},
		CacheControl: convertToLLMCacheControl(block.CacheControl),
	}
	setAnthropicSpecialMeta(&tc.TransformerMetadata, block.Type, block.Caller)

	return tc
}

// toolUseBlockFromLLM converts an llm.ToolCall back to an Anthropic
// MessageContentBlock. For tool calls tagged with anthropic_type, the original
// block type (e.g. "server_tool_use") and caller are restored; otherwise the
// block is emitted as a plain "tool_use".
func toolUseBlockFromLLM(toolCall llm.ToolCall) MessageContentBlock {
	blockType := "tool_use"
	if at := getAnthropicType(toolCall.TransformerMetadata); at != "" {
		blockType = at
	}

	return MessageContentBlock{
		Type:         blockType,
		ID:           toolCall.ID,
		Name:         lo.ToPtr(toolCall.Function.CompositeName()),
		Input:        xjson.SafeJSONRawMessage(toolCall.Function.Arguments),
		CacheControl: convertToAnthropicCacheControl(toolCall.CacheControl),
		Caller:       getAnthropicCaller(toolCall.TransformerMetadata),
	}
}

// toolResultBlockFromInline converts an assistant-inlined tool result back to
// an Anthropic *_tool_result MessageContentBlock. Returns false when the
// inline result lacks an anthropic_type metadata tag (i.e. it did not
// originate from an Anthropic special tool result).
func toolResultBlockFromInline(ir llm.InlineToolResult) (MessageContentBlock, bool) {
	blockType := getAnthropicType(ir.TransformerMetadata)
	if blockType == "" {
		return MessageContentBlock{}, false
	}

	block := MessageContentBlock{
		Type:   blockType,
		Caller: getAnthropicCaller(ir.TransformerMetadata),
	}
	if ir.ToolCallID != "" {
		block.ToolUseID = lo.ToPtr(ir.ToolCallID)
	}

	if ir.IsError {
		block.IsError = lo.ToPtr(true)
	}

	rawContent := getAnthropicToolResultContent(ir.TransformerMetadata)
	if len(rawContent) > 0 {
		content := &MessageContent{}
		content.SetRaw(rawContent)

		block.Content = content
	} else if ir.Output != "" {
		block.Content = &MessageContent{Content: lo.ToPtr(ir.Output)}
	}

	return block, true
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
