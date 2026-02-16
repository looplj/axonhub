package httpclient

import (
	"net/http"
	"net/url"
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
	// Register headers for testing append behavior
	RegisterMergeWithAppendHeaders("User-Agent", "Accept")

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
			name: "should add non-duplicate values to existing headers",
			dest: http.Header{
				"Content-Type": []string{"application/json"},
				"Accept":       []string{"application/json"},
			},
			src: http.Header{
				"Accept":     []string{"text/plain", "application/json"},
				"User-Agent": []string{"Mozilla/5.0"},
			},
			want: http.Header{
				"Content-Type": []string{"application/json"},
				"Accept":       []string{"application/json", "text/plain"},
				"User-Agent":   []string{"Mozilla/5.0"},
			},
		},
		{
			name: "should add all non-duplicate values from multiple values",
			dest: http.Header{
				"Accept": []string{"application/json"},
			},
			src: http.Header{
				"Accept": []string{"text/plain", "application/xml", "text/html"},
			},
			want: http.Header{
				"Accept": []string{"application/json", "text/plain", "application/xml", "text/html"},
			},
		},
		{
			name: "should skip all duplicate values",
			dest: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
			src: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
			want: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
		},
		{
			name: "should skip only duplicate values and add new ones",
			dest: http.Header{
				"Accept": []string{"application/json", "text/plain"},
			},
			src: http.Header{
				"Accept": []string{"text/plain", "application/xml", "application/json"},
			},
			want: http.Header{
				"Accept": []string{"application/json", "text/plain", "application/xml"},
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
			name: "should add non-duplicate values when header exists",
			dest: http.Header{
				"User-Agent": []string{"AxonHub/1.0"},
			},
			src: http.Header{
				"Content-Type": []string{"application/json"},
				"User-Agent":   []string{"Mozilla/5.0"},
				"Accept":       []string{"*/*"},
			},
			want: http.Header{
				"User-Agent": []string{"AxonHub/1.0", "Mozilla/5.0"},
				"Accept":     []string{"*/*"},
			},
		},
		{
			name: "should overwrite non-appendable headers",
			dest: http.Header{
				"X-Custom-Header": []string{"old-value"},
			},
			src: http.Header{
				"X-Custom-Header": []string{"new-value"},
			},
			want: http.Header{
				"X-Custom-Header": []string{"new-value"},
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

// TestHeaderMaps_CanonicalForm verifies every key in the three header maps
// matches Go's http.CanonicalHeaderKey form. A mismatch causes silent lookup
// failures when iterating http.Header (which always uses canonical keys).
func TestHeaderMaps_CanonicalForm(t *testing.T) {
	for name, m := range map[string]map[string]bool{
		"blockedHeaders":    blockedHeaders,
		"sensitiveHeaders":  sensitiveHeaders,
		"libManagedHeaders": libManagedHeaders,
	} {
		for k := range m {
			canonical := http.CanonicalHeaderKey(k)
			require.Equal(t, canonical, k, "%s key %q should be canonical form %q", name, k, canonical)
		}
	}
}

// TestMergeHTTPHeaders_BlocksAllHardcodedHeaders builds an http.Header via
// Set() (auto-canonicalized, matching real HTTP server behavior) for every
// header in blockedHeaders, sensitiveHeaders and libManagedHeaders, then
// verifies none of them appear in the merge result.
func TestMergeHTTPHeaders_BlocksAllHardcodedHeaders(t *testing.T) {
	src := make(http.Header)
	for k := range blockedHeaders {
		src.Set(k, "blocked-val")
	}
	for k := range sensitiveHeaders {
		src.Set(k, "sensitive-val")
	}
	for k := range libManagedHeaders {
		src.Set(k, "lib-val")
	}
	src.Set("X-Custom", "keep-me")

	dest := make(http.Header)
	got := MergeHTTPHeaders(dest, src)

	for k := range blockedHeaders {
		require.Empty(t, got.Values(k), "blockedHeaders %q should not be merged", k)
	}
	for k := range sensitiveHeaders {
		require.Empty(t, got.Values(k), "sensitiveHeaders %q should not be merged", k)
	}
	for k := range libManagedHeaders {
		require.Empty(t, got.Values(k), "libManagedHeaders %q should not be merged", k)
	}
	require.Equal(t, "keep-me", got.Get("X-Custom"), "non-blocked header should be merged")
}

// TestMaskSensitiveHeaders_MasksAllHardcodedHeaders verifies every header in
// sensitiveHeaders is masked to "******" by MaskSensitiveHeaders.
func TestMaskSensitiveHeaders_MasksAllHardcodedHeaders(t *testing.T) {
	headers := make(http.Header)
	for k := range sensitiveHeaders {
		headers.Set(k, "secret-value")
	}
	headers.Set("X-Custom", "visible")

	got := MaskSensitiveHeaders(headers)

	for k := range sensitiveHeaders {
		require.Equal(t, []string{"******"}, got.Values(k),
			"sensitiveHeaders %q should be masked", k)
	}
	require.Equal(t, []string{"visible"}, got.Values("X-Custom"))
}

func TestRegisterAppendHeaders(t *testing.T) {
	RegisterMergeWithAppendHeaders("X-New-Append")

	dest := http.Header{"X-New-Append": []string{"old"}}
	src := http.Header{"X-New-Append": []string{"new"}}

	got := MergeHTTPHeaders(dest, src)
	require.Equal(t, []string{"old", "new"}, got["X-New-Append"])
}

func TestMergeHTTPQuery(t *testing.T) {
	tests := []struct {
		name string
		dest url.Values
		src  url.Values
		want url.Values
	}{
		{
			name: "should merge new query parameters",
			dest: url.Values{"q": []string{"golang"}},
			src:  url.Values{"page": []string{"1"}},
			want: url.Values{"q": []string{"golang"}, "page": []string{"1"}},
		},
		{
			name: "should not overwrite existing query parameters",
			dest: url.Values{"q": []string{"golang"}},
			src:  url.Values{"q": []string{"java"}},
			want: url.Values{"q": []string{"golang"}},
		},
		{
			name: "should handle empty src",
			dest: url.Values{"q": []string{"golang"}},
			src:  nil,
			want: url.Values{"q": []string{"golang"}},
		},
		{
			name: "should handle empty dest",
			dest: nil,
			src:  url.Values{"page": []string{"1"}},
			want: url.Values{"page": []string{"1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeHTTPQuery(tt.dest, tt.src)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMergeInboundRequest(t *testing.T) {
	t.Run("should merge both headers and query", func(t *testing.T) {
		dest := &Request{
			Headers: http.Header{"Content-Type": []string{"application/json"}},
			Query:   url.Values{"q": []string{"old"}},
		}
		src := &Request{
			Headers: http.Header{"User-Agent": []string{"Test"}},
			Query:   url.Values{"page": []string{"1"}},
		}

		got := MergeInboundRequest(dest, src)
		require.Equal(t, "application/json", got.Headers.Get("Content-Type"))
		require.Equal(t, "Test", got.Headers.Get("User-Agent"))
		require.Equal(t, "old", got.Query.Get("q"))
		require.Equal(t, "1", got.Query.Get("page"))
	})

	t.Run("should return dest if src is nil", func(t *testing.T) {
		dest := &Request{Headers: http.Header{"X-Test": []string{"val"}}}
		got := MergeInboundRequest(dest, nil)
		require.Equal(t, dest, got)
	})
}
