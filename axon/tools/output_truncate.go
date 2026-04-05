package tools

import (
	"strings"
	"unicode/utf8"
)

const (
	defaultToolOutputMaxChars = 12_000
	defaultToolOutputMaxLines = 500
)

func truncateToolOutputLines(text string, maxLines int, hint string) string {
	if maxLines <= 0 {
		maxLines = defaultToolOutputMaxLines
	}

	idx := -1
	for range maxLines {
		found := strings.IndexByte(text[idx+1:], '\n')
		if found == -1 {
			return text
		}

		idx += found + 1
	}

	if idx >= len(text)-1 {
		return text
	}

	return strings.TrimSuffix(text[:idx+1], "\n") + buildToolOutputTruncationSuffix(hint)
}

func truncateToolOutput(text string, maxChars int, hint string) string {
	if maxChars <= 0 {
		maxChars = defaultToolOutputMaxChars
	}

	if len(text) <= maxChars || utf8.RuneCountInString(text) <= maxChars {
		return text
	}

	suffix := buildToolOutputTruncationSuffix(hint)

	suffixRunes := utf8.RuneCountInString(suffix)
	if suffixRunes >= maxChars {
		return truncateToolOutputRunes(text, maxChars)
	}

	return truncateToolOutputRunes(text, maxChars-suffixRunes) + suffix
}

func buildToolOutputTruncationSuffix(hint string) string {
	suffix := "\n... (truncated)"
	if hint != "" {
		suffix += " " + hint
	}

	return suffix + "\n"
}

func truncateToolOutputRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}

	runeCount := 0
	for i := range text {
		if runeCount == limit {
			return text[:i]
		}

		runeCount++
	}

	return text
}
