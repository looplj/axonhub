package agent

import (
	"unicode"
)

// EstimateTokens estimates the token count for a string.
// It uses different ratios for CJK characters vs ASCII/Latin text
// to provide more accurate estimates across languages.
//
// Approximations:
//   - English/ASCII: ~4 characters per token
//   - CJK (Chinese, Japanese, Korean): ~1.5 characters per token
func EstimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}

	var cjkChars, otherChars int
	for _, r := range s {
		if isCJK(r) {
			cjkChars++
		} else if !unicode.IsSpace(r) {
			otherChars++
		}
	}

	// CJK: ~1.5 chars/token, ASCII: ~4 chars/token
	cjkTokens := float64(cjkChars) / 1.5
	otherTokens := float64(otherChars) / 4.0

	total := int(cjkTokens + otherTokens)
	if total == 0 && len(s) > 0 {
		return 1
	}
	return total
}

// isCJK returns true if the rune is a CJK character.
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || // Chinese
		unicode.Is(unicode.Hiragana, r) || // Japanese Hiragana
		unicode.Is(unicode.Katakana, r) || // Japanese Katakana
		unicode.Is(unicode.Hangul, r) // Korean
}

// EstimateMessagesTokens estimates total tokens for a slice of messages.
func EstimateMessagesTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateMessageTokens(msg)
	}
	return total
}

func estimateMessageTokens(msg Message) int {
	tokens := EstimateTokens(string(msg.Role))
	if msg.Content != nil {
		tokens += EstimateTokens(msg.Content.String())
	}
	if msg.ToolUse != nil {
		tokens += EstimateTokens(msg.ToolUse.Name)
		tokens += EstimateTokens(msg.ToolUse.Input)
	}
	if msg.ToolUseID != nil {
		tokens += EstimateTokens(*msg.ToolUseID)
	}
	return tokens
}
