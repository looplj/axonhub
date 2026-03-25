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

func stripProviderPrefix(modelID string) string {
	prefixes := []string{"openai/", "anthropic/", "google/", "gemini/", "azure/", "aws/", "bedrock/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(modelID, prefix) {
			return strings.TrimPrefix(modelID, prefix)
		}
	}
	return modelID
}

func ResolveEndpoint(modelID string) (EndpointType, error) {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	if normalized == "" {
		return "", fmt.Errorf("unknown model: %q", modelID)
	}

	// Strip provider prefix if present (e.g., "openai/gpt-5.4" -> "gpt-5.4")
	normalized = stripProviderPrefix(normalized)

	if strings.Contains(normalized, "codex") || isGPT54OrLater(normalized) {
		return EndpointResponses, nil
	}

	if strings.Contains(normalized, "claude") {
		return EndpointMessages, nil
	}

	return EndpointChatCompletions, nil
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
