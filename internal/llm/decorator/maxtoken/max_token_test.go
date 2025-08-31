package maxtoken

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/llm"
)

func TestEnsureMaxTokens(t *testing.T) {
	defaultValue := int64(200)
	decorator := EnsureMaxTokens(defaultValue)

	content := "Hello"
	req := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
		// MaxTokens is nil initially
	}

	result, err := decorator.DecorateRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, &defaultValue, result.MaxTokens)
}

func TestEnsureMaxTokens_ExistingValue(t *testing.T) {
	defaultValue := int64(200)
	decorator := EnsureMaxTokens(defaultValue)

	existingValue := int64(100)
	content := "Hello"
	req := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
		MaxTokens: &existingValue, // Already has a value
	}

	result, err := decorator.DecorateRequest(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, &existingValue, result.MaxTokens) // Should remain unchanged
	assert.NotEqual(t, &defaultValue, result.MaxTokens)
}
