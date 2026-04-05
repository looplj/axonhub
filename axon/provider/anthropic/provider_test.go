package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("creates provider with default headers", func(t *testing.T) {
		provider := New("https://api.anthropic.com", "test-api-key")

		require.NotNil(t, provider)
		assert.NotNil(t, provider.client)
		assert.Equal(t, defaultThreadHeader, provider.threadHeader)
		assert.Equal(t, defaultTraceHeader, provider.traceHeader)
	})

	t.Run("creates provider with custom headers", func(t *testing.T) {
		provider := New(
			"https://api.anthropic.com",
			"test-api-key",
			WithThreadHeader("X-Custom-Thread"),
			WithTraceHeader("X-Custom-Trace"),
		)

		require.NotNil(t, provider)
		assert.Equal(t, "X-Custom-Thread", provider.threadHeader)
		assert.Equal(t, "X-Custom-Trace", provider.traceHeader)
	})

	t.Run("creates provider with empty base URL", func(t *testing.T) {
		provider := New("", "test-api-key")

		require.NotNil(t, provider)
	})

	t.Run("creates provider with empty API key", func(t *testing.T) {
		provider := New("https://api.anthropic.com", "")

		require.NotNil(t, provider)
	})
}

func TestWithThreadHeader(t *testing.T) {
	provider := &Provider{}
	opt := WithThreadHeader("X-Thread-ID")
	opt(provider)

	assert.Equal(t, "X-Thread-ID", provider.threadHeader)
}

func TestWithTraceHeader(t *testing.T) {
	provider := &Provider{}
	opt := WithTraceHeader("X-Trace-ID")
	opt(provider)

	assert.Equal(t, "X-Trace-ID", provider.traceHeader)
}

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, "AH-Thread-Id", defaultThreadHeader)
	assert.Equal(t, "AH-Trace-Id", defaultTraceHeader)
	assert.Equal(t, 8192, defaultMaxTokens)
}
