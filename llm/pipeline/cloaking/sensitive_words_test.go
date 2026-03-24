package cloaking

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/stretchr/testify/assert"
)

func TestSensitiveWordsMiddleware(t *testing.T) {
	ctx := context.Background()

	t.Run("disable mode does not modify content", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use opencode and cursor")}},
			},
		}
		cfg := SensitiveWordsConfig{Mode: "disable"}
		result := ApplySensitiveWords(ctx, req, cfg)
		assert.Equal(t, "I use opencode and cursor", *result.Messages[0].Content.Content)
	})

	t.Run("default mode inserts zero-width spaces in default words", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use opencode")}},
			},
		}
		cfg := SensitiveWordsConfig{Mode: ""}
		result := ApplySensitiveWords(ctx, req, cfg)
		assert.Contains(t, *result.Messages[0].Content.Content, "\u200B")
		assert.Contains(t, *result.Messages[0].Content.Content, "o\u200Bp\u200Be\u200Bn\u200Bc\u200Bo\u200Bd\u200Be")
	})

	t.Run("extend mode merges default and custom words", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use opencode and axonhub")}},
			},
		}
		customWords := []string{"axonhub"}
		cfg := SensitiveWordsConfig{Mode: "extend", SensitiveWords: &customWords}
		result := ApplySensitiveWords(ctx, req, cfg)
		content := *result.Messages[0].Content.Content
		assert.Contains(t, content, "o\u200Bp\u200Be\u200Bn\u200Bc\u200Bo\u200Bd\u200Be")
		assert.Contains(t, content, "a\u200Bx\u200Bo\u200Bn\u200Bh\u200Bu\u200Bb")
	})

	t.Run("replace mode uses only custom words", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use opencode and axonhub")}},
			},
		}
		customWords := []string{"axonhub"}
		cfg := SensitiveWordsConfig{Mode: "replace", SensitiveWords: &customWords}
		result := ApplySensitiveWords(ctx, req, cfg)
		content := *result.Messages[0].Content.Content
		assert.NotContains(t, content, "o\u200Bp\u200Be\u200Bn\u200Bc\u200Bo\u200Bd\u200Be")
		assert.Contains(t, content, "opencode")
		assert.Contains(t, content, "a\u200Bx\u200Bo\u200Bn\u200Bh\u200Bu\u200Bb")
	})

	t.Run("SensitiveWords omitted uses default words", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use cursor")}},
			},
		}
		cfg := SensitiveWordsConfig{Mode: "extend", SensitiveWords: nil}
		result := ApplySensitiveWords(ctx, req, cfg)
		assert.Contains(t, *result.Messages[0].Content.Content, "c\u200Bu\u200Br\u200Bs\u200Bo\u200Br")
	})

	t.Run("SensitiveWords empty array uses default words", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use cursor")}},
			},
		}
		emptyWords := []string{}
		cfg := SensitiveWordsConfig{Mode: "extend", SensitiveWords: &emptyWords}
		result := ApplySensitiveWords(ctx, req, cfg)
		assert.Contains(t, *result.Messages[0].Content.Content, "c\u200Bu\u200Br\u200Bs\u200Bo\u200Br")
	})

	t.Run("SensitiveWords with custom word in extend mode", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{Role: "user", Content: llm.MessageContent{Content: strPtr("I use cursor and axonhub")}},
			},
		}
		customWords := []string{"axonhub"}
		cfg := SensitiveWordsConfig{Mode: "extend", SensitiveWords: &customWords}
		result := ApplySensitiveWords(ctx, req, cfg)
		content := *result.Messages[0].Content.Content
		assert.Contains(t, content, "c\u200Bu\u200Br\u200Bs\u200Bo\u200Br")
		assert.Contains(t, content, "a\u200Bx\u200Bo\u200Bn\u200Bh\u200Bu\u200Bb")
	})

	t.Run("handles MultipleContent with text parts", func(t *testing.T) {
		req := llm.Request{
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						MultipleContent: []llm.MessageContentPart{
							{Type: "text", Text: strPtr("I use opencode")},
							{Type: "image_url", ImageURL: &llm.ImageURL{URL: "http://example.com/img.png"}},
						},
					},
				},
			},
		}
		cfg := SensitiveWordsConfig{Mode: ""}
		result := ApplySensitiveWords(ctx, req, cfg)
		assert.Contains(t, *result.Messages[0].Content.MultipleContent[0].Text, "o\u200Bp\u200Be\u200Bn\u200Bc\u200Bo\u200Bd\u200Be")
		assert.Equal(t, "http://example.com/img.png", result.Messages[0].Content.MultipleContent[1].ImageURL.URL)
	})
}

func strPtr(s string) *string {
	return &s
}
