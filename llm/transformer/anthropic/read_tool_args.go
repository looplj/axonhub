package anthropic

import (
	"encoding/json"
	"strings"
)

func sanitizeReadToolInput(name string, input json.RawMessage) json.RawMessage {
	if !isReadToolName(name) || len(input) == 0 {
		return input
	}

	sanitized, ok := removeEmptyReadPages(string(input))
	if !ok {
		return input
	}

	return json.RawMessage(sanitized)
}

func removeEmptyReadPages(arguments string) (string, bool) {
	return sanitizeEmptyReadPages(arguments, false)
}

func normalizeReadToolArguments(arguments string) (string, bool) {
	return sanitizeEmptyReadPages(arguments, true)
}

func sanitizeEmptyReadPages(arguments string, unchangedOK bool) (string, bool) {
	if arguments == "" {
		return arguments, false
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return arguments, false
	}

	pages, ok := args["pages"].(string)
	if !ok || pages != "" {
		return arguments, unchangedOK
	}

	delete(args, "pages")

	sanitized, err := json.Marshal(args)
	if err != nil {
		return arguments, false
	}

	return string(sanitized), true
}

func isReadToolName(name string) bool {
	return strings.EqualFold(name, "read")
}
