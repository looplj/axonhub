package claudecode

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
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

// injectFakeUserID generates and injects a fake user ID into the request metadata.
func injectFakeUserID(payload []byte) []byte {
	metadata := gjson.GetBytes(payload, "metadata")
	if !metadata.Exists() {
		payload, _ = sjson.SetBytes(payload, "metadata.user_id", generateFakeUserID())
		return payload
	}

	existingUserID := gjson.GetBytes(payload, "metadata.user_id").String()
	if existingUserID == "" || !isValidUserID(existingUserID) {
		payload, _ = sjson.SetBytes(payload, "metadata.user_id", generateFakeUserID())
	}

	return payload
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

// disableThinkingIfToolChoiceForced checks if tool_choice forces tool use and disables thinking.
// Anthropic API does not allow thinking when tool_choice is set to "any" or a specific tool.
// See: https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#important-considerations
func disableThinkingIfToolChoiceForced(body []byte) []byte {
	toolChoiceType := gjson.GetBytes(body, "tool_choice.type").String()
	// "auto" is allowed with thinking, but "any" or "tool" (specific tool) are not
	if toolChoiceType == "any" || toolChoiceType == "tool" {
		// Remove thinking configuration entirely to avoid API error
		body, _ = sjson.DeleteBytes(body, "thinking")
	}

	return body
}

// isClaudeOAuthToken checks if the API key is a Claude OAuth token.
func isClaudeOAuthToken(apiKey string) bool {
	return strings.Contains(apiKey, "sk-ant-oat")
}

// applyClaudeToolPrefix adds a prefix to all tool names in the request.
// This is used for OAuth tokens to add the "proxy_" prefix.
func applyClaudeToolPrefix(body []byte, prefix string) []byte {
	if prefix == "" {
		return body
	}

	// Prefix tool names in tools array
	if tools := gjson.GetBytes(body, "tools"); tools.Exists() && tools.IsArray() {
		tools.ForEach(func(index, tool gjson.Result) bool {
			name := tool.Get("name").String()
			if name == "" || strings.HasPrefix(name, prefix) {
				return true
			}

			path := fmt.Sprintf("tools.%d.name", index.Int())
			body, _ = sjson.SetBytes(body, path, prefix+name)

			return true
		})
	}

	// Prefix tool_choice.name if type is "tool"
	if gjson.GetBytes(body, "tool_choice.type").String() == "tool" {
		name := gjson.GetBytes(body, "tool_choice.name").String()
		if name != "" && !strings.HasPrefix(name, prefix) {
			body, _ = sjson.SetBytes(body, "tool_choice.name", prefix+name)
		}
	}

	// Prefix tool_use names in messages
	if messages := gjson.GetBytes(body, "messages"); messages.Exists() && messages.IsArray() {
		messages.ForEach(func(msgIndex, msg gjson.Result) bool {
			content := msg.Get("content")
			if !content.Exists() || !content.IsArray() {
				return true
			}

			content.ForEach(func(contentIndex, part gjson.Result) bool {
				if part.Get("type").String() != "tool_use" {
					return true
				}

				name := part.Get("name").String()
				if name == "" || strings.HasPrefix(name, prefix) {
					return true
				}

				path := fmt.Sprintf("messages.%d.content.%d.name", msgIndex.Int(), contentIndex.Int())
				body, _ = sjson.SetBytes(body, path, prefix+name)

				return true
			})

			return true
		})
	}

	return body
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
	if len(extraBetas) == 0 {
		return baseBetas
	}

	existingSet := make(map[string]bool)
	for b := range strings.SplitSeq(baseBetas, ",") {
		existingSet[strings.TrimSpace(b)] = true
	}

	var result strings.Builder
	result.WriteString(baseBetas)

	for _, beta := range extraBetas {
		beta = strings.TrimSpace(beta)
		if beta != "" && !existingSet[beta] {
			result.WriteString("," + beta)
			existingSet[beta] = true
		}
	}

	return result.String()
}

// injectClaudeCodeSystemMessage prepends the Claude Code system message to the request.
// It adds cache_control to the injected message.
func injectClaudeCodeSystemMessage(payload []byte) ([]byte, error) {
	system := gjson.GetBytes(payload, "system")

	// Create Claude Code instruction with cache_control
	claudeCodeInstruction := map[string]any{
		"type": "text",
		"text": "You are Claude Code, Anthropic's official CLI for Claude.",
		"cache_control": map[string]string{
			"type": "ephemeral",
		},
	}

	// If system doesn't exist, create it with just the Claude Code instruction
	if !system.Exists() {
		instructions := []any{claudeCodeInstruction}

		instructionsJSON, err := json.Marshal(instructions)
		if err != nil {
			return payload, err
		}

		payload, _ = sjson.SetRawBytes(payload, "system", instructionsJSON)

		return payload, nil
	}

	// If system is an array, check if Claude Code instruction is already first
	if system.IsArray() {
		firstText := gjson.GetBytes(payload, "system.0.text").String()
		if firstText == "You are Claude Code, Anthropic's official CLI for Claude." {
			// Already has the instruction, don't duplicate
			return payload, nil
		}

		// Prepend Claude Code instruction to existing system messages
		instructions := []any{claudeCodeInstruction}

		system.ForEach(func(_, part gjson.Result) bool {
			if part.Get("type").String() == "text" {
				var partMap map[string]any
				if err := json.Unmarshal([]byte(part.Raw), &partMap); err == nil {
					instructions = append(instructions, partMap)
				}
			}

			return true
		})

		instructionsJSON, err := json.Marshal(instructions)
		if err != nil {
			return payload, err
		}

		payload, _ = sjson.SetRawBytes(payload, "system", instructionsJSON)

		return payload, nil
	}

	// If system is a string, convert it to array format with Claude Code instruction first
	if system.Type == gjson.String {
		instructions := []any{
			claudeCodeInstruction,
			map[string]any{
				"type": "text",
				"text": system.String(),
			},
		}

		instructionsJSON, err := json.Marshal(instructions)
		if err != nil {
			return payload, err
		}

		payload, _ = sjson.SetRawBytes(payload, "system", instructionsJSON)

		return payload, nil
	}

	return payload, nil
}
