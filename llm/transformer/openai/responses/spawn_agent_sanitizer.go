package responses

import (
	"encoding/json"
	"os"
)

// maybeSanitizeSpawnAgentArgs checks if AXONHUB_SANITIZE_SPAWN_AGENT_ARGS=1
// and if toolName is "spawn_agent", removes empty "items" field from arguments.
func maybeSanitizeSpawnAgentArgs(toolName string, arguments string) string {
	if os.Getenv("AXONHUB_SANITIZE_SPAWN_AGENT_ARGS") != "1" {
		return arguments
	}
	if toolName != "spawn_agent" {
		return arguments
	}
	return sanitizeSpawnAgentArguments(arguments)
}

// sanitizeSpawnAgentArguments removes the "items" field if it's an empty array
// and "message" is present and non-empty.
func sanitizeSpawnAgentArguments(arguments string) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(arguments), &raw); err != nil {
		return arguments
	}

	// message must exist and be non-empty
	msgRaw, ok := raw["message"]
	if !ok {
		return arguments
	}
	var msgStr string
	if err := json.Unmarshal(msgRaw, &msgStr); err != nil || msgStr == "" {
		return arguments
	}

	// items must exist and be empty array
	itemsRaw, ok := raw["items"]
	if !ok {
		return arguments // no items field, nothing to do
	}
	var items []json.RawMessage
	if err := json.Unmarshal(itemsRaw, &items); err != nil {
		return arguments
	}
	if len(items) != 0 {
		return arguments // items has content, don't touch
	}

	// Remove items and re-serialize
	delete(raw, "items")
	result, err := json.Marshal(raw)
	if err != nil {
		return arguments
	}
	return string(result)
}
