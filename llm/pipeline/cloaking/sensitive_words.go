package cloaking

import (
	"context"
	"strings"

	"github.com/looplj/axonhub/llm"
)

var defaultSensitiveWords = []string{
	"opencode", "cursor", "cline", "roo-code", "copilot",
	"windsurf", "continue", "aider", "avante", "neovim", "zed", "CherryStudio",
}

type SensitiveWordsConfig struct {
	Mode           string
	SensitiveWords *[]string
}

func ApplySensitiveWords(ctx context.Context, req llm.Request, cfg SensitiveWordsConfig) llm.Request {
	if cfg.Mode == "disable" {
		return req
	}

	words := buildWordList(cfg.Mode, cfg.SensitiveWords)
	if len(words) == 0 {
		return req
	}

	for i := range req.Messages {
		obfuscateMessageContent(&req.Messages[i].Content, words)
	}

	return req
}

func buildWordList(mode string, customWords *[]string) []string {
	switch mode {
	case "replace":
		if customWords != nil {
			return *customWords
		}
		return nil
	case "extend":
		result := make([]string, len(defaultSensitiveWords))
		copy(result, defaultSensitiveWords)
		if customWords != nil {
			result = append(result, *customWords...)
		}
		return result
	default:
		return defaultSensitiveWords
	}
}

func obfuscateMessageContent(content *llm.MessageContent, words []string) {
	if content.Content != nil {
		*content.Content = obfuscateText(*content.Content, words)
	}

	for i := range content.MultipleContent {
		part := &content.MultipleContent[i]
		if part.Type == "text" && part.Text != nil {
			*part.Text = obfuscateText(*part.Text, words)
		}
	}
}

func obfuscateText(text string, words []string) string {
	for _, word := range words {
		if word == "" {
			continue
		}
		text = strings.ReplaceAll(text, word, insertZeroWidth(word))
	}
	return text
}
func insertZeroWidth(word string) string {
	if len(word) == 0 {
		return word
	}
	runes := []rune(word)
	var result []rune
	for i, r := range runes {
		result = append(result, r)
		if i < len(runes)-1 {
			result = append(result, '\u200B')
		}
	}
	return string(result)
}
