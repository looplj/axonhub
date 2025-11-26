package httpclient

import (
	"fmt"
	"io"
	"net/http"
)

func ReadHTTPRequest(rawReq *http.Request) (*Request, error) {
	req := &Request{
		Method:     rawReq.Method,
		URL:        rawReq.URL.String(),
		Headers:    rawReq.Header,
		Body:       nil,
		Auth:       &AuthConfig{},
		RequestID:  "",
		RawRequest: rawReq,
	}

	body, err := io.ReadAll(rawReq.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	req.Body = body

	return req, nil
}

// IsHTTPStatusCodeRetryable checks if an HTTP status code is retryable.
// 4xx status codes are generally not retryable except for 429 (Too Many Requests).
// 5xx status codes are typically retryable.
func IsHTTPStatusCodeRetryable(statusCode int) bool {
	if statusCode == http.StatusTooManyRequests {
		return true // 429 is retryable (rate limiting)
	}

	if statusCode >= 400 && statusCode < 500 {
		return false // Other 4xx errors are not retryable
	}

	if statusCode >= 500 {
		return true // 5xx errors are retryable
	}

	return false // Non-error status codes don't need retrying
}

var BlockedHeaders = map[string]bool{
	"Content-Length":    true,
	"Transfer-Encoding": true,
	// The client will handle it automatically.
	"Accept-Encoding": true,
	"Authorization":   true,
	"Api-Key":         true,
	"X-Api-Key":       true,
	"X-Api-Secret":    true,
	"X-Api-Token":     true,
}

// MergeHTTPHeaders merges the source headers into the destination headers if the key not present in the destination headers.
// Blocked headers are not merged.
func MergeHTTPHeaders(dest, src http.Header) http.Header {
	for k, v := range src {
		if _, ok := dest[k]; !ok && !BlockedHeaders[k] {
			dest[k] = v
		}
	}

	return dest
}
