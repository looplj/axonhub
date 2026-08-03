package httpclient

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/klauspost/compress/zstd"
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
		ClientIP:   getClientIP(rawReq),
		RawRequest: rawReq,
	}

	body, err := io.ReadAll(rawReq.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) > 0 {
		body, err = decodeRequestBody(body, req.Headers)
		if err != nil {
			return nil, err
		}
	}

	req.Body = body

	return req, nil
}

func decodeRequestBody(body []byte, headers http.Header) ([]byte, error) {
	contentEncoding := headers.Get("Content-Encoding")
	if contentEncoding == "" {
		return body, nil
	}

	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))

	switch encoding {
	case "identity", "":
		return body, nil

	case "gzip", "x-gzip":
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer reader.Close()

		decoded, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress gzip body: %w", err)
		}

		headers.Del("Content-Encoding")
		headers.Del("Content-Length")

		return decoded, nil

	case "deflate":
		// RFC 7230/2616 defines "deflate" as zlib (RFC 1950), but many clients send
		// raw DEFLATE (RFC 1951). Try zlib first, fall back to raw DEFLATE.
		decoded, err := decodeZlibOrFlate(body)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress deflate body: %w", err)
		}

		headers.Del("Content-Encoding")
		headers.Del("Content-Length")

		return decoded, nil

	case "zstd":
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
		}
		defer decoder.Close()

		decoded, err := decoder.DecodeAll(body, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to decode zstd compressed body: %w", err)
		}

		headers.Del("Content-Encoding")
		headers.Del("Content-Length")

		return decoded, nil

	default:
		return nil, fmt.Errorf("unsupported content encoding: %s", contentEncoding)
	}
}

func decodeZlibOrFlate(body []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(body))
	if err == nil {
		defer reader.Close()
		return io.ReadAll(reader)
	}

	flateReader := flate.NewReader(bytes.NewReader(body))
	defer flateReader.Close()
	return io.ReadAll(flateReader)
}

func getClientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		if before, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(before)
		}

		return xff
	}

	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		return ip
	}

	return req.RemoteAddr
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
	"Content-Type": true,
	// Accept is owned by the outbound transformer (e.g. TTS requires */* for binary
	// audio); the inbound client value must not override it.
	"Accept":             true,
	"Connection":         true,
	"Keep-Alive":         true,
	"Proxy-Authenticate": true,
	"Proxy-Connection":   true,
	"Te":                 true,
	"Trailer":            true,
	"Upgrade":            true,
	"X-Channel-Id":       true,
	"X-Project-Id":       true,
	"X-Real-Ip":          true,
	"X-Forwarded-For":    true,
	"X-Forwarded-Proto":  true,
	"X-Forwarded-Host":   true,
	"X-Forwarded-Port":   true,
	"X-Forwarded-Server": true,

	// Browser-only / hop-by-hop-ish headers that should not be forwarded to upstream.
	"Accept-Language":    true,
	"Dnt":                true,
	"Origin":             true,
	"Referer":            true,
	"Sec-Fetch-Dest":     true,
	"Sec-Fetch-Mode":     true,
	"Sec-Fetch-Site":     true,
	"Sec-Fetch-User":     true,
	"Sec-Ch-Ua":          true,
	"Sec-Ch-Ua-Mobile":   true,
	"Sec-Ch-Ua-Platform": true,

	// AxonHub customized headers that should not be forwarded to upstream to avoid recognition.
	// NOTE: user customized trace/thread headers will be sent to upstream.
	"Ah-Trace-Id":  true,
	"Ah-Thread-Id": true,

	// X-Initiator is used by specific channels (e.g. Copilot) for billing control.
	// Block from auto-merge so it is only forwarded by the channel that explicitly needs it.
	"X-Initiator": true,
}

// blockedHeaderPrefixes lists header prefixes that should not be forwarded to upstream.
// For example, Cloudflare adds Cf-* and Cdn-* headers that can cause upstream providers to reject requests.
var blockedHeaderPrefixes = []string{
	"Cf-",
	"Cdn-",
}

// isBlockedHeader checks whether a header (in canonical form) should be blocked from forwarding.
func isBlockedHeader(key string) bool {
	if blockedHeaders[key] {
		return true
	}

	for _, prefix := range blockedHeaderPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
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
	"Www-Authenticate":    true,
}

func IsSensitiveHeader(key string) bool {
	return sensitiveHeaders[http.CanonicalHeaderKey(key)]
}

var mergeWithAppendHeaders = map[string]bool{}

// RegisterMergeWithAppendHeaders registers headers that should be appended instead of overwritten.
// It is not goroutine-safe, should call when init.
func RegisterMergeWithAppendHeaders(headers ...string) {
	for _, h := range headers {
		mergeWithAppendHeaders[http.CanonicalHeaderKey(h)] = true
	}
}

func MergeInboundRequest(dest, src *Request) *Request {
	if src == nil || len(src.Headers) == 0 && len(src.Query) == 0 {
		return dest
	}

	dest.Headers = MergeHTTPHeaders(dest.Headers, src.Headers)

	if !dest.SkipInboundQueryMerge {
		dest.Query = MergeHTTPQuery(dest.Query, src.Query)
	}

	return dest
}

// MergeHTTPQuery merges the source query parameters into the destination query parameters.
// If a key already exists in the destination, it is not overwritten.
func MergeHTTPQuery(dest, src url.Values) url.Values {
	if len(src) == 0 {
		return dest
	}

	if dest == nil {
		dest = make(url.Values)
	}

	for k, v := range src {
		if _, ok := dest[k]; !ok {
			dest[k] = v
		}
	}

	return dest
}

func MaskSensitiveHeaders(headers http.Header) http.Header {
	result := make(http.Header, len(headers))
	for key, values := range headers {
		var newValues []string
		if !IsSensitiveHeader(key) {
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
// If a header is in the mergeWithAppendHeaders list, it adds non-duplicate values from the source.
// Otherwise, it overwrites the destination header with the source values.
// Blocked, sensitive, and library-managed headers are not merged.
func MergeHTTPHeaders(dest, src http.Header) http.Header {
	for k, v := range src {
		if IsSensitiveHeader(k) || libManagedHeaders[k] || isBlockedHeader(k) {
			continue
		}

		if mergeWithAppendHeaders[k] {
			if existingValues, ok := dest[k]; ok {
				dest[k] = lo.Uniq(append(existingValues, v...))
			} else {
				dest[k] = v
			}
		} else {
			dest[k] = v
		}
	}

	return dest
}
