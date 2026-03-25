// Package resolver defines model-to-endpoint resolution for Copilot/NanoGPT routing.
package resolver

import (
	"fmt"
	"strconv"
	"strings"
)

type EndpointType string

const (
	EndpointResponses       EndpointType = "responses"
	EndpointMessages        EndpointType = "messages"
	EndpointChatCompletions EndpointType = "chat_completions"
)

var (
	ResponsesModelFixtures = []string{
		"gpt-5.4",
		"gpt-5.4-mini-20250514",
		"codex-mini-latest",
	}

	MessagesModelFixtures = []string{
		"claude-3-opus-20240229",
		"claude-3-7-sonnet-20250219",
	}

	ChatCompletionsModelFixtures = []string{
		"gpt-4o",
		"gpt-4.1-mini-20250101",
		"gemini-2.5-pro",
		"deepseek-chat",
		"qwen-plus",
	}
)

var miscModelFamilyKeywords = []string{
	"gemini",
	"deepseek",
	"qwen",
	"llama",
	"mistral",
	"grok",
	"kimi",
	"doubao",
	"glm",
	"o1",
	"o3",
	"gpt-4",
	"gpt-3",
}

func ResolveEndpoint(modelID string) (EndpointType, error) {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	if normalized == "" {
		return "", fmt.Errorf("unknown model: %q", modelID)
	}

	if strings.Contains(normalized, "codex") || isGPT54OrLater(normalized) {
		return EndpointResponses, nil
	}

	if strings.Contains(normalized, "claude") {
		return EndpointMessages, nil
	}

	if isKnownMiscFamily(normalized) {
		return EndpointChatCompletions, nil
	}

	return "", fmt.Errorf("unknown model: %q", modelID)
}

func isKnownMiscFamily(model string) bool {
	for _, keyword := range miscModelFamilyKeywords {
		if strings.Contains(model, keyword) {
			return true
		}
	}
	return false
}

func isGPT54OrLater(model string) bool {
	if !strings.HasPrefix(model, "gpt-") {
		return false
	}

	version, _, _ := strings.Cut(strings.TrimPrefix(model, "gpt-"), "-")
	major, minor, ok := parseModelVersion(version)
	if !ok {
		return false
	}

	return major > 5 || (major == 5 && minor >= 4)
}

func parseModelVersion(version string) (major int, minor int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) == 0 || parts[0] == "" {
		return 0, 0, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}

	if len(parts) == 1 || parts[1] == "" {
		return major, 0, true
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}

	return major, minor, true
}
