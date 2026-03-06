package agent

import (
	"fmt"
	"strings"
)

func cloneMessages(in []Message) []Message {
	out := make([]Message, len(in))
	copy(out, in)
	return out
}

func summarizeMessages(messages []Message) string {
	const (
		maxLines = 80
		maxChars = 6000
	)

	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	written := 0

	for _, msg := range messages {
		if written >= maxLines || sb.Len() >= maxChars {
			break
		}

		text := extractMessageText(msg)
		if text == "" {
			continue
		}

		line := fmt.Sprintf("[%s] %s", msg.Role, text)
		line = truncateString(line, 240)
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(line)
		written++
	}

	return strings.TrimSpace(sb.String())
}

func extractMessageText(msg Message) string {
	if msg.ToolUse != nil {
		return fmt.Sprintf("tool_call: %s %s", msg.ToolUse.Name, truncateString(msg.ToolUse.Input, 180))
	}
	if msg.Content == nil {
		return ""
	}
	return strings.TrimSpace(msg.Content.String())
}

func mergeSummary(existing, next string, maxChars int) string {
	existing = strings.TrimSpace(existing)
	next = strings.TrimSpace(next)

	if next == "" {
		return existing
	}

	var merged string
	if existing == "" {
		merged = next
	} else {
		merged = existing + "\n" + next
	}

	if maxChars <= 0 || len(merged) <= maxChars {
		return merged
	}

	return merged[len(merged)-maxChars:]
}

func truncateString(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
