package responses

import (
	"os"
	"strconv"
	"strings"
)

func responsesStreamTextConfig() (string, int) {
	mode := strings.ToLower(strings.TrimSpace(firstEnv("AXONHUB_RESPONSES_STREAM_TEXT", "OLLAMA_RESPONSES_STREAM_TEXT")))
	switch mode {
	case "", "strict":
		mode = "strict"
	case "prefix", "always":
	default:
		mode = "strict"
	}

	prefixChars := parseUint(firstEnv("AXONHUB_RESPONSES_STREAM_TEXT_PREFIX_CHARS", "OLLAMA_RESPONSES_STREAM_TEXT_PREFIX_CHARS"))
	if mode == "prefix" && prefixChars <= 0 {
		mode = "strict"
	}

	return mode, prefixChars
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func parseUint(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return int(parsed)
}
