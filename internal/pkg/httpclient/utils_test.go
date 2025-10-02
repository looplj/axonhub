package httpclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsHTTPStatusCodeRetryable(t *testing.T) {
	t.Run("429 is retryable", func(t *testing.T) {
		require.True(t, IsHTTPStatusCodeRetryable(429))
	})

	t.Run("4xx errors (except 429) are not retryable", func(t *testing.T) {
		require.False(t, IsHTTPStatusCodeRetryable(400))
		require.False(t, IsHTTPStatusCodeRetryable(401))
		require.False(t, IsHTTPStatusCodeRetryable(403))
		require.False(t, IsHTTPStatusCodeRetryable(404))
		require.False(t, IsHTTPStatusCodeRetryable(422))
	})

	t.Run("5xx errors are retryable", func(t *testing.T) {
		require.True(t, IsHTTPStatusCodeRetryable(500))
		require.True(t, IsHTTPStatusCodeRetryable(502))
		require.True(t, IsHTTPStatusCodeRetryable(503))
		require.True(t, IsHTTPStatusCodeRetryable(504))
	})

	t.Run("non-error status codes are not retryable", func(t *testing.T) {
		require.False(t, IsHTTPStatusCodeRetryable(200))
		require.False(t, IsHTTPStatusCodeRetryable(201))
		require.False(t, IsHTTPStatusCodeRetryable(301))
		require.False(t, IsHTTPStatusCodeRetryable(302))
	})
}
