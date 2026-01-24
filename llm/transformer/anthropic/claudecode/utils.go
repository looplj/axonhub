package claudecode

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
)

// userIDPattern matches Claude Code format: user_[64-hex]_account__session_[uuid-v4].
var userIDPattern = regexp.MustCompile(`^user_[a-fA-F0-9]{64}_account__session_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// generateFakeUserID generates a fake user ID in Claude Code format.
// Format: user_[64-hex-chars]_account__session_[UUID-v4].
func generateFakeUserID() string {
	hexBytes := make([]byte, 32)
	_, _ = rand.Read(hexBytes)
	hexPart := hex.EncodeToString(hexBytes)
	uuidPart := uuid.New().String()

	return "user_" + hexPart + "_account__session_" + uuidPart
}

// isValidUserID checks if a user ID matches Claude Code format.
func isValidUserID(userID string) bool {
	return userIDPattern.MatchString(userID)
}

// injectFakeUserIDStructured generates and injects a fake user ID into the request metadata.
func injectFakeUserIDStructured(llmReq *llm.Request) *llm.Request {
	if llmReq.Metadata == nil {
		llmReq.Metadata = make(map[string]string)
	}

	existingUserID := llmReq.Metadata["user_id"]
	if existingUserID == "" || !isValidUserID(existingUserID) {
		llmReq.Metadata["user_id"] = generateFakeUserID()
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

// isClaudeOAuthToken checks if the API key is a Claude OAuth token.
func isClaudeOAuthToken(apiKey string) bool {
	return strings.Contains(apiKey, "sk-ant-oat")
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

// injectClaudeCodeSystemMessageStructured prepends the Claude Code system message with cache_control.
func injectClaudeCodeSystemMessageStructured(llmReq *llm.Request) *llm.Request {
	claudeCodeMsg := llm.Message{
		Role: "system",
		Content: llm.MessageContent{
			Content: func() *string { s := claudeCodeSystemMessage; return &s }(),
		},
		CacheControl: &llm.CacheControl{
			Type: "ephemeral",
		},
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
