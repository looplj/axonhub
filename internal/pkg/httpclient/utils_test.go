package httpclient

import (
	"net/http"
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

func TestMergeHTTPHeaders(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		dest http.Header
		src  http.Header
		want http.Header
	}{
		{
			name: "given src Authorization header, should skip sensitive header",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
			},
			src: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer 123456"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		{
			name: "given src User-Agent header, should merge them",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
			},
			src: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"Mozilla/5.0"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"Mozilla/5.0"},
			},
		},
		{
			name: "should not override existing dest headers",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"AxonHub/1.0"},
			},
			src: http.Header{
				"User-Agent": []string{"Mozilla/5.0"},
				"Accept":     []string{"*/*"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"AxonHub/1.0"},
				"Accept":       []string{"*/*"},
			},
		},
		{
			name: "should block transport-managed headers and skip sensitive ones",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
			},
			src: http.Header{
				"Authorization":     []string{"Bearer token"},
				"Api-Key":           []string{"key123"},
				"X-Api-Key":         []string{"xkey456"},
				"X-Api-Secret":      []string{"secret789"},
				"X-Api-Token":       []string{"token000"},
				"Content-Type":      []string{"text/plain"},
				"Content-Length":    []string{"100"},
				"Transfer-Encoding": []string{"chunked"},
				"User-Agent":        []string{"Test/1.0"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"Test/1.0"},
			},
		},
		{
			name: "empty src headers should not change dest",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
			},
			src: http.Header{},
			want: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		{
			name: "empty dest headers should merge non-blocked src headers",
			dest: http.Header{},
			src: http.Header{
				"User-Agent":    []string{"Test/1.0"},
				"Accept":        []string{"*/*"},
				"Authorization": []string{"Bearer token"},
			},
			want: http.Header{
				"User-Agent": []string{"Test/1.0"},
				"Accept":     []string{"*/*"},
			},
		},
		{
			name: "should merge multiple custom headers",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
			},
			src: http.Header{
				"X-Request-ID":    []string{"req-123"},
				"X-Trace-ID":      []string{"trace-456"},
				"User-Agent":      []string{"Custom/1.0"},
				"Accept-Encoding": []string{"gzip, deflate"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"X-Request-ID": []string{"req-123"},
				"X-Trace-ID":   []string{"trace-456"},
				"User-Agent":   []string{"Custom/1.0"},
			},
		},
		{
			name: "should handle headers with multiple values",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
			},
			src: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"Accept":       []string{"application/json", "text/plain"},
			},
		},
		{
			name: "should not override existing dest headers",
			dest: http.Header{
				"User-Agent": []string{"AxonHub/1.0"},
			},
			src: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"Mozilla/5.0"},
				"Accept":       []string{"*/*"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"AxonHub/1.0"},
				"Accept":       []string{"*/*"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeHTTPHeaders(tt.dest, tt.src)
			require.Equal(t, tt.want, got)
		})
	}
}
