package claudecode

import (
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
)

const claudeCodeBillingCCHMetadataKey = "claudecode_billing_cch"

// injectFakeUserIDStructured generates and injects a fake user ID into the request metadata.
func injectFakeUserIDStructured(ctx context.Context, llmReq llm.Request) llm.Request {
	if llmReq.Metadata == nil {
		llmReq.Metadata = make(map[string]string)
	}

	existingUserID := llmReq.Metadata["user_id"]
	if existingUserID == "" || ParseUserID(existingUserID) == nil {
		llmReq.Metadata["user_id"] = GenerateUserID(ctx)
	}

	return llmReq
}

// extractAndRemoveBetas extracts the "betas" array from the body and removes it.
// Returns the extracted betas as a string slice and the modified body.
func extractAndRemoveBetas(body []byte) ([]string, []byte) {
	betasResult := gjson.GetBytes(body, "betas")
	if !betasResult.Exists() {
		return nil, body
	}

	var betas []string

	if betasResult.IsArray() {
		for _, item := range betasResult.Array() {
			if s := strings.TrimSpace(item.String()); s != "" {
				betas = append(betas, s)
			}
		}
	} else if s := strings.TrimSpace(betasResult.String()); s != "" {
		betas = append(betas, s)
	}

	body, _ = sjson.DeleteBytes(body, "betas")

	return betas, body
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
		for _, b := range strings.Split(baseBetas, ",") {
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

// removeBillingSystemMessages removes system messages that contain the
// x-anthropic-billing-header pattern. These messages are injected by the
// Claude Code CLI to report billing metadata. For non-official (non-OAuth)
// channels, these messages should be stripped to avoid leaking client info.
func removeBillingSystemMessages(llmReq *llm.Request) *llm.Request {
	if len(llmReq.Messages) == 0 {
		return llmReq
	}

	filtered := make([]llm.Message, 0, len(llmReq.Messages))

	for _, msg := range llmReq.Messages {
		if msg.Role == "system" && msg.Content.Content != nil &&
			strings.HasPrefix(strings.TrimSpace(*msg.Content.Content), billingHeaderPrefix) {
			continue
		}

		filtered = append(filtered, msg)
	}

	llmReq.Messages = filtered

	return llmReq
}

func ensureBillingSystemMessageCCH(llmReq *llm.Request) *llm.Request {
	if llmReq == nil || len(llmReq.Messages) == 0 {
		return llmReq
	}

	cch := ""
	if llmReq.TransformerMetadata != nil {
		if v, ok := llmReq.TransformerMetadata[claudeCodeBillingCCHMetadataKey]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				cch = strings.TrimSpace(s)
			}
		}
	}
	if cch == "" {
		return llmReq
	}

	for i := range llmReq.Messages {
		msg := &llmReq.Messages[i]
		if msg.Role != "system" {
			continue
		}

		if msg.Content.Content != nil {
			updated, changed := ensureBillingHeaderCCHInText(*msg.Content.Content, cch)
			if changed {
				*msg.Content.Content = updated
			}
		}

		if len(msg.Content.MultipleContent) > 0 {
			for j := range msg.Content.MultipleContent {
				part := &msg.Content.MultipleContent[j]
				if part.Type != "text" || part.Text == nil {
					continue
				}

				updated, changed := ensureBillingHeaderCCHInText(*part.Text, cch)
				if changed {
					*part.Text = updated
				}
			}
		}
	}

	return llmReq
}

func ensureBillingHeaderCCHInText(text string, cch string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return text, false
	}

	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, billingHeaderPrefix) {
		return text, false
	}

	rest := strings.TrimSpace(trimmed[len(billingHeaderPrefix):])
	if rest == "" {
		return text, false
	}

	parts := strings.Split(rest, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		if strings.HasPrefix(strings.ToLower(p), "cch=") {
			return text, false
		}
	}

	out := strings.TrimSpace(trimmed)
	if !strings.HasSuffix(out, ";") {
		out += ";"
	}
	out += " cch=" + strings.TrimSpace(cch) + ";"

	return out, true
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
