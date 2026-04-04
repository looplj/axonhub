package anthropic

import (
	"errors"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/agent"
)

func TestWrapAPIError(t *testing.T) {
	t.Run("non-API error", func(t *testing.T) {
		originalErr := errors.New("some error")
		wrapped := wrapAPIError(originalErr)

		assert.Contains(t, wrapped.Error(), "some error")
		assert.Contains(t, wrapped.Error(), "anthropic:")

		var providerErr *agent.ProviderError
		assert.False(t, errors.As(wrapped, &providerErr))
	})

	t.Run("API error with status code", func(t *testing.T) {
		apiErr := &anthropic.Error{
			StatusCode: 500,
		}

		wrapped := wrapAPIError(apiErr)

		var providerErr *agent.ProviderError
		require.True(t, errors.As(wrapped, &providerErr))
		assert.Equal(t, 500, providerErr.StatusCode)
		assert.False(t, providerErr.IsClientError())
	})

	t.Run("API error with 400 status", func(t *testing.T) {
		apiErr := &anthropic.Error{
			StatusCode: 400,
		}

		wrapped := wrapAPIError(apiErr)

		var providerErr *agent.ProviderError
		require.True(t, errors.As(wrapped, &providerErr))
		assert.Equal(t, 400, providerErr.StatusCode)
		assert.True(t, providerErr.IsClientError())
		assert.Contains(t, wrapped.Error(), "status 400")
	})
}

func TestExtractErrorMessage(t *testing.T) {
	t.Run("empty raw JSON", func(t *testing.T) {
		apiErr := &anthropic.Error{}
		msg := extractErrorMessage(apiErr)

		assert.Empty(t, msg)
	})
}
