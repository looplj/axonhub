package stream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/llm"
)

func TestEnsureUsage_StreamEnabled(t *testing.T) {
	decorator := EnsureUsage()

	// Create a request with stream enabled but no stream options
	streamEnabled := true
	req := &llm.Request{
		Stream: &streamEnabled,
		// StreamOptions is nil initially
	}

	// Apply decorator
	result, err := decorator.DecorateRequest(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, result.StreamOptions)
	assert.True(t, result.StreamOptions.IncludeUsage)
}

func TestEnsureUsage_StreamEnabledWithExistingOptions(t *testing.T) {
	decorator := EnsureUsage()

	// Create a request with stream enabled and existing stream options
	streamEnabled := true
	req := &llm.Request{
		Stream: &streamEnabled,
		StreamOptions: &llm.StreamOptions{
			IncludeUsage: false, // Initially false
		},
	}

	// Apply decorator
	result, err := decorator.DecorateRequest(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, result.StreamOptions)
	assert.True(t, result.StreamOptions.IncludeUsage) // Should be forced to true
}

func TestEnsureUsage_StreamDisabled(t *testing.T) {
	decorator := EnsureUsage()

	// Create a request with stream disabled
	streamEnabled := false
	req := &llm.Request{
		Stream: &streamEnabled,
		StreamOptions: &llm.StreamOptions{
			IncludeUsage: false,
		},
	}

	// Apply decorator
	result, err := decorator.DecorateRequest(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, result.StreamOptions)
	assert.False(t, result.StreamOptions.IncludeUsage) // Should remain unchanged
}

func TestEnsureUsage_StreamNil(t *testing.T) {
	decorator := EnsureUsage()

	// Create a request with nil stream
	req := &llm.Request{
		Stream: nil, // Stream is nil
		StreamOptions: &llm.StreamOptions{
			IncludeUsage: false,
		},
	}

	// Apply decorator
	result, err := decorator.DecorateRequest(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, result.StreamOptions)
	assert.False(t, result.StreamOptions.IncludeUsage) // Should remain unchanged
}

func TestEnsureUsage_DecorateResponseNoOp(t *testing.T) {
	decorator := EnsureUsage()

	// Create a response
	resp := &llm.Response{
		ID: "test-id",
	}

	// Apply decorator
	result, err := decorator.DecorateResponse(context.Background(), resp)

	assert.NoError(t, err)
	assert.Equal(t, resp, result) // Should return unchanged response
}
