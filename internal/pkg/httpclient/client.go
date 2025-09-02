package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// HttpClient implements the HttpClient interface.
type HttpClient struct {
	client *http.Client
}

// NewHttpClient creates a new HTTP client.
func NewHttpClient() *HttpClient {
	return &HttpClient{
		client: &http.Client{},
	}
}

// NewHttpClientWithClient creates a new HTTP client with a custom http.Client.
func NewHttpClientWithClient(client *http.Client) *HttpClient {
	return &HttpClient{
		client: client,
	}
}

// Do executes the HTTP request.
func (hc *HttpClient) Do(ctx context.Context, request *Request) (*Response, error) {
	log.Debug(ctx, "execute http request", log.Any("request", request))

	rawReq, err := hc.buildHttpRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	rawResp, err := hc.client.Do(rawReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	defer func() {
		err := rawResp.Body.Close()
		if err != nil {
			log.Warn(ctx, "failed to close HTTP response body", log.Cause(err))
		}
	}()

	body, err := io.ReadAll(rawResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if rawResp.StatusCode >= 400 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "HTTP request failed",
				log.String("method", rawReq.Method),
				log.String("url", rawReq.URL.String()),
				log.Any("status_code", rawResp.StatusCode),
				log.String("body", string(body)))
		}

		return nil, &Error{
			Method:     rawReq.Method,
			URL:        rawReq.URL.String(),
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "HTTP request success",
			log.String("method", rawReq.Method),
			log.String("url", rawReq.URL.String()),
			log.Any("status_code", rawResp.StatusCode),
			log.String("body", string(body)))
	}

	// Build generic response
	response := &Response{
		StatusCode:  rawResp.StatusCode,
		Headers:     rawResp.Header,
		Body:        body,
		RawResponse: rawResp,
		Stream:      nil,
	}

	return response, nil
}

// DoStream executes a streaming HTTP request using Server-Sent Events.
func (hc *HttpClient) DoStream(ctx context.Context, request *Request) (streams.Stream[*StreamEvent], error) {
	log.Debug(ctx, "execute stream request", log.Any("request", request))

	rawReq, err := hc.buildHttpRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	// Add streaming headers
	rawReq.Header.Set("Accept", "text/event-stream")
	rawReq.Header.Set("Cache-Control", "no-cache")
	rawReq.Header.Set("Connection", "keep-alive")

	// Execute request
	rawResp, err := hc.client.Do(rawReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP stream request failed: %w", err)
	}

	// Check for HTTP errors before creating stream
	if rawResp.StatusCode >= 400 {
		defer func() {
			err := rawResp.Body.Close()
			if err != nil {
				log.Warn(ctx, "failed to close HTTP response body", log.Cause(err))
			}
		}()

		// Read error body for streaming requests
		body, err := io.ReadAll(rawResp.Body)
		if err != nil {
			return nil, err
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "HTTP stream request failed",
				log.String("method", rawReq.Method),
				log.String("url", rawReq.URL.String()),
				log.Any("status_code", rawResp.StatusCode),
				log.String("body", string(body)))
		}

		return nil, &Error{
			Method:     rawReq.Method,
			URL:        rawReq.URL.String(),
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	// Determine content type and select appropriate decoder
	contentType := rawResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/event-stream" // Default to SSE
	}

	// Try to get a registered decoder for the content type
	decoderFactory, exists := GetDecoder(contentType)
	if !exists {
		// Fallback to default SSE decoder
		log.Debug(ctx, "no decoder found for content type, using default SSE", log.String("content_type", contentType))

		decoderFactory = NewDefaultSSEDecoder
	}

	stream := decoderFactory(ctx, rawResp.Body)

	return stream, nil
}

// buildHttpRequest builds an HTTP request from Request.
func (hc *HttpClient) buildHttpRequest(
	ctx context.Context,
	request *Request,
) (*http.Request, error) {
	var body io.Reader
	if len(request.Body) > 0 {
		body = bytes.NewReader(request.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, request.Method, request.URL, body)
	if err != nil {
		return nil, err
	}

	httpReq.Header = request.Headers
	if httpReq.Header == nil {
		httpReq.Header = make(http.Header)
	}

	httpReq.Header.Set("User-Agent", "axonhub/1.0")

	// Apply authentication
	if request.Auth != nil {
		err = hc.applyAuth(httpReq, request.Auth)
		if err != nil {
			return nil, fmt.Errorf("failed to apply authentication: %w", err)
		}
	}

	return httpReq, nil
}

// applyAuth applies authentication to the HTTP request.
func (hc *HttpClient) applyAuth(req *http.Request, auth *AuthConfig) error {
	switch auth.Type {
	case "bearer":
		if auth.APIKey == "" {
			return fmt.Errorf("bearer token is required")
		}

		req.Header.Set("Authorization", "Bearer "+auth.APIKey)
	case "api_key":
		if auth.HeaderKey == "" {
			return fmt.Errorf("header key is required")
		}

		req.Header.Set(auth.HeaderKey, auth.APIKey)
	default:
		return fmt.Errorf("unsupported auth type: %s", auth.Type)
	}

	return nil
}

// extractHeaders extracts headers from HTTP response.
func (hc *HttpClient) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)

	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0] // Take the first value
		}
	}

	return result
}
