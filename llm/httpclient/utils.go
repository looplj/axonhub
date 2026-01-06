package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/samber/lo"
)

func ReadHTTPRequest(rawReq *http.Request) (*Request, error) {
	req := &Request{
		Method:     rawReq.Method,
		URL:        rawReq.URL.String(),
		Path:       rawReq.URL.Path,
		Query:      rawReq.URL.Query(),
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

// The golang std http client will handle the headers automatically.
var libManagedHeaders = map[string]bool{
	"Content-Length":    true,
	"Transfer-Encoding": true,
	"Accept-Encoding":   true,
	"Host":              true,
}

var blockedHeaders = map[string]bool{
	"Content-Type":      true,
	"Connection":        true,
	"X-Channel-Id":      true,
	"X-Project-Id":      true,
	"X-Real-IP":         true,
	"X-Forwarded-For":   true,
	"X-Forwarded-Proto": true,
	"X-Forwarded-Host":  true,
	"X-Forwarded-Port":  true,
}

var sensitiveHeaders = map[string]bool{
	"Authorization":       true,
	"Api-Key":             true,
	"X-Api-Key":           true,
	"X-Api-Secret":        true,
	"X-Api-Token":         true,
	"X-Goog-Api-Key":      true,
	"X-Google-Api-Key":    true,
	"Cookie":              true,
	"Set-Cookie":          true,
	"Proxy-Authorization": true,
	"WWW-Authenticate":    true,
}

func MergeInboundRequest(dest, src *Request) *Request {
	if src == nil || len(src.Headers) == 0 && len(src.Query) == 0 {
		return dest
	}

	dest.Headers = MergeHTTPHeaders(dest.Headers, src.Headers)

	// Merge query parameters.
	if len(src.Query) > 0 {
		if dest.Query == nil {
			dest.Query = make(url.Values)
		}

		for k, v := range src.Query {
			if _, ok := dest.Query[k]; !ok {
				dest.Query[k] = v
			}
		}
	}

	return dest
}

func MaskSensitiveHeaders(headers http.Header) http.Header {
	result := make(http.Header, len(headers))
	for key, values := range headers {
		var newValues []string
		if _, ok := sensitiveHeaders[key]; !ok {
			newValues = values
		} else {
			newValues = append(newValues, "******")
		}

		result[key] = newValues
	}

	return result
}

// FinalizeAuthHeaders writes the auth config into headers and clears the in-memory auth field.
func FinalizeAuthHeaders(req *Request) (*Request, error) {
	if req.Auth == nil {
		return req, nil
	}

	err := applyAuth(req.Headers, req.Auth)
	if err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	req.Auth = nil

	return req, nil
}

// MergeHTTPHeaders merges the source headers into the destination headers.
// If a header already exists in the destination, it adds non-duplicate values from the source.
// Blocked headers are not merged.
func MergeHTTPHeaders(dest, src http.Header) http.Header {
	for k, v := range src {
		if sensitiveHeaders[k] || libManagedHeaders[k] || blockedHeaders[k] {
			continue
		}

		if existingValues, ok := dest[k]; ok {
			dest[k] = lo.Uniq(append(existingValues, v...))
		} else {
			dest[k] = v
		}
	}

	return dest
}
