package claudecode

import (
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
)

// injectFakeUserIDStructured generates and injects a fake user ID into the request metadata.
func injectFakeUserIDStructured(ctx context.Context, llmReq llm.Request, accountIdentity string) llm.Request {
	if llmReq.Metadata == nil {
		llmReq.Metadata = make(map[string]string)
	}

	existingUserID := llmReq.Metadata["user_id"]
	if existingUserID == "" || ParseUserID(existingUserID) == nil {
		llmReq.Metadata["user_id"] = GenerateUserID(ctx, accountIdentity)
	}

	return llmReq
}

// disableThinkingIfToolChoiceForcedStructured clears ReasoningEffort when tool_choice forces tool use.
// Anthropic API does not allow thinking when tool_choice is "any" or a specific named tool.
// See: https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#important-considerations
// This operates on the structured llm.Request before it's serialized by the base transformer.
func disableThinkingIfToolChoiceForcedStructured(llmReq *llm.Request) *llm.Request {
	if llmReq.ToolChoice == nil {
		return llmReq
	}

	forcesToolUse := false

	if llmReq.ToolChoice.ToolChoice != nil {
		if *llmReq.ToolChoice.ToolChoice == "any" {
			forcesToolUse = true
		}
	} else if llmReq.ToolChoice.NamedToolChoice != nil {
		if llmReq.ToolChoice.NamedToolChoice.Type == "tool" {
			forcesToolUse = true
		}
	}

	if forcesToolUse && llmReq.ReasoningEffort != "" {
		reqCopy := *llmReq
		reqCopy.ReasoningEffort = ""
		reqCopy.ReasoningBudget = nil

		return &reqCopy
	}

	return llmReq
}

// applyClaudeToolPrefixStructured adds a prefix to all tool names in the request.
func applyClaudeToolPrefixStructured(llmReq *llm.Request, prefix string) *llm.Request {
	if prefix == "" {
		return llmReq
	}

	// Prefix tool names in tools array
	for i := range llmReq.Tools {
		if !strings.HasPrefix(llmReq.Tools[i].Function.Name, prefix) {
			llmReq.Tools[i].Function.Name = prefix + llmReq.Tools[i].Function.Name
		}
	}

	// Prefix tool_choice.name if type is "tool"
	if llmReq.ToolChoice != nil && llmReq.ToolChoice.NamedToolChoice != nil {
		if llmReq.ToolChoice.NamedToolChoice.Type == "tool" {
			name := llmReq.ToolChoice.NamedToolChoice.Function.Name
			if name != "" && !strings.HasPrefix(name, prefix) {
				llmReq.ToolChoice.NamedToolChoice.Function.Name = prefix + name
			}
		}
	}

	return llmReq
}

// stripClaudeToolPrefixFromResponse removes the prefix from tool names in the response.
func stripClaudeToolPrefixFromResponse(body []byte, prefix string) []byte {
	if prefix == "" {
		return body
	}

	content := gjson.GetBytes(body, "content")
	if !content.Exists() || !content.IsArray() {
		return body
	}

	content.ForEach(func(index, part gjson.Result) bool {
		if part.Get("type").String() != "tool_use" {
			return true
		}

		name := part.Get("name").String()
		if !strings.HasPrefix(name, prefix) {
			return true
		}

		path := fmt.Sprintf("content.%d.name", index.Int())
		body, _ = sjson.SetBytes(body, path, strings.TrimPrefix(name, prefix))

		return true
	})

	return body
}

// mergeBetasIntoHeader merges beta features into the Anthropic-Beta header.
func mergeBetasIntoHeader(baseBetas string, extraBetas []string) string {
	var parts []string

	existingSet := make(map[string]bool)

	// Add existing betas if present
	baseBetas = strings.TrimSpace(baseBetas)
	if baseBetas != "" {
		for b := range strings.SplitSeq(baseBetas, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				parts = append(parts, b)
				existingSet[b] = true
			}
		}
	}

	// Add extra betas if not already present
	for _, beta := range extraBetas {
		beta = strings.TrimSpace(beta)
		if beta != "" && !existingSet[beta] {
			parts = append(parts, beta)
			existingSet[beta] = true
		}
	}

	return strings.Join(parts, ",")
}

// billingHeaderPrefix is the prefix used to identify billing header system messages.
const billingHeaderPrefix = "x-anthropic-billing-header:"

// RemoveBillingSystemMessages removes system messages that contain the
// x-anthropic-billing-header pattern. These messages are injected by the
// Claude Code CLI to report billing metadata. The returned request is a copy
// when content is removed so retries do not mutate the shared request.
func RemoveBillingSystemMessages(llmReq *llm.Request) *llm.Request {
	if llmReq == nil || len(llmReq.Messages) == 0 {
		return llmReq
	}

	filtered := make([]llm.Message, 0, len(llmReq.Messages))
	changed := false

	for _, msg := range llmReq.Messages {
		if !strings.EqualFold(msg.Role, "system") {
			filtered = append(filtered, msg)

			continue
		}

		if len(msg.Content.MultipleContent) == 0 {
			if msg.Content.Content == nil || !isBillingHeaderText(*msg.Content.Content) {
				filtered = append(filtered, msg)

				continue
			}

			changed = true

			continue
		}

		parts := make([]llm.MessageContentPart, 0, len(msg.Content.MultipleContent))
		for _, part := range msg.Content.MultipleContent {
			if strings.EqualFold(part.Type, "text") && part.Text != nil && isBillingHeaderText(*part.Text) {
				changed = true

				continue
			}

			parts = append(parts, part)
		}

		if len(parts) == 0 {
			continue
		}

		msg.Content.MultipleContent = parts
		filtered = append(filtered, msg)
	}

	if !changed {
		return llmReq
	}

	result := *llmReq
	result.Messages = filtered

	return &result
}

func isBillingHeaderText(text string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(text)), billingHeaderPrefix)
}

// injectClaudeCodeSystemMessageStructured prepends the Claude Code system message.
func injectClaudeCodeSystemMessageStructured(llmReq *llm.Request) *llm.Request {
	claudeCodeMsg := llm.Message{
		Role: "system",
		Content: llm.MessageContent{
			Content: func() *string { s := claudeCodeSystemMessage; return &s }(),
		},
		// Force enable cache_control for Claude Code system message.
		CacheControl: &llm.CacheControl{Type: "ephemeral"},
	}

	if len(llmReq.Messages) > 0 && llmReq.Messages[0].Role == "system" {
		if llmReq.Messages[0].Content.Content != nil &&
			*llmReq.Messages[0].Content.Content == claudeCodeSystemMessage {
			return llmReq
		}
	}

	llmReq.Messages = append([]llm.Message{claudeCodeMsg}, llmReq.Messages...)

	// Ensure array format for system prompts (required for cache_control)
	if llmReq.TransformOptions.ArrayInstructions == nil {
		arrayInstructions := true
		llmReq.TransformOptions.ArrayInstructions = &arrayInstructions
	}

	return llmReq
}
