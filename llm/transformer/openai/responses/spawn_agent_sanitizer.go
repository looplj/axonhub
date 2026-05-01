package responses

import (
	"encoding/json"
	"os"
)

// maybeSanitizeSpawnAgentArgs checks if spawn_agent argument sanitization is enabled
// via the AXONHUB_SANITIZE_SPAWN_AGENT_ARGS environment variable, and if so,
// sanitizes the tool call arguments.
func maybeSanitizeSpawnAgentArgs(args string) string {
	if os.Getenv("AXONHUB_SANITIZE_SPAWN_AGENT_ARGS") != "1" {
		return args
	}
	return sanitizeSpawnAgentArguments(args)
}

// sanitizeSpawnAgentArguments cleans spawn_agent tool call arguments:
// If the arguments contain a non-empty "message" field alongside an empty "items" array,
// the "items" field is removed to prevent downstream parsing issues.
func sanitizeSpawnAgentArguments(args string) string {
	if args == "" {
		return args
	}

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(args), &parsed); err != nil {
		return args
	}

	// Check if "message" is present and non-empty
	msgRaw, hasMsg := parsed["message"]
	if !hasMsg {
		return args
	}
	var msgStr string
	if err := json.Unmarshal(msgRaw, &msgStr); err != nil || msgStr == "" {
		return args
	}

	// Check if "items" is present and empty array
	itemsRaw, hasItems := parsed["items"]
	if !hasItems {
		return args
	}
	var items []json.RawMessage
	if err := json.Unmarshal(itemsRaw, &items); err != nil || len(items) > 0 {
		return args
	}

	// Remove empty items and re-serialize
	delete(parsed, "items")
	result, err := json.Marshal(parsed)
	if err != nil {
		return args
	}
	return string(result)
}
