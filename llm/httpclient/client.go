package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tmaxmax/go-sse"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/log"
	"github.com/looplj/axonhub/pkg/streams"
)

// HttpClientImpl implements the HttpClient interface
type HttpClientImpl struct {
	client *http.Client
}

// NewHttpClient creates a new HTTP client
func NewHttpClient() HttpClient {
	return &HttpClientImpl{
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// Do executes the HTTP request
func (hc *HttpClientImpl) Do(ctx context.Context, request *llm.GenericHttpRequest) (*llm.GenericHttpResponse, error) {
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
		if err := rawResp.Body.Close(); err != nil {
			log.Warn(ctx, "failed to close HTTP response body", log.Cause(err))
		}
	}()

	// Read response body
	body, err := io.ReadAll(rawResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if rawResp.StatusCode >= 400 {
		return nil, llm.GenericHttpError{
			Method:     rawReq.Method,
			URL:        rawReq.URL.String(),
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	// Build generic response
	response := &llm.GenericHttpResponse{
		StatusCode:  rawResp.StatusCode,
		Headers:     rawResp.Header,
		Body:        body,
		RawResponse: rawResp,
		Stream:      nil,
	}
	return response, nil
}

// DoStream executes a streaming HTTP request using Server-Sent Events
func (hc *HttpClientImpl) DoStream(ctx context.Context, request *llm.GenericHttpRequest) (streams.Stream[*llm.GenericHttpResponse], error) {
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
			if err := rawResp.Body.Close(); err != nil {
				log.Warn(ctx, "failed to close HTTP response body", log.Cause(err))
			}
		}()

		// Read error body for streaming requests
		body, err := io.ReadAll(rawResp.Body)
		if err != nil {
			return nil, err
		}

		return nil, llm.GenericHttpError{
			Method:     rawReq.Method,
			URL:        rawReq.URL.String(),
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	// Create SSE stream using go-sse Stream
	sseStream := sse.NewStream(rawResp.Body)

	stream := &sseStreamWrapper{
		ctx: ctx,
		response: &llm.GenericHttpResponse{
			StatusCode:  rawResp.StatusCode,
			Headers:     rawResp.Header,
			RawResponse: rawResp,
		},
		sseStream: sseStream,
		current:   nil,
		err:       nil,
	}

	return stream, nil
}

// sseStreamWrapper implements streams.Stream for Server-Sent Events using go-sse Stream
type sseStreamWrapper struct {
	ctx       context.Context
	response  *llm.GenericHttpResponse
	sseStream *sse.Stream
	current   *llm.GenericHttpResponse
	err       error
}

// Next advances to the next event in the stream
func (s *sseStreamWrapper) Next() bool {
	if s.err != nil {
		return false
	}

	// Check context cancellation
	select {
	case <-s.ctx.Done():
		s.err = s.ctx.Err()
		_ = s.Close()
		return false
	default:
	}

	// Receive next event from go-sse Stream
	event, err := s.sseStream.Recv()
	if err != nil {
		if err == io.EOF {
			// End of stream
			_ = s.Close()
			return false
		}
		s.err = err
		_ = s.Close()
		return false
	}

	log.Debug(s.ctx, "SSE event received", log.Any("event", event))

	// Create response for this event
	s.current = &llm.GenericHttpResponse{
		StatusCode:  s.response.StatusCode,
		Headers:     s.response.Headers,
		Body:        []byte(event.Data),
		RawResponse: s.response.RawResponse,
	}

	return true
}

// Current returns the current event data
func (s *sseStreamWrapper) Current() *llm.GenericHttpResponse {
	return s.current
}

// Err returns any error that occurred during streaming
func (s *sseStreamWrapper) Err() error {
	return s.err
}

// Close closes the stream and releases resources
func (s *sseStreamWrapper) Close() error {
	if s.sseStream != nil {
		err := s.sseStream.Close()
		log.Debug(s.ctx, "SSE stream closed")
		return err
	}
	return nil
}

// buildHttpRequest builds an HTTP request from GenericHttpRequest
func (hc *HttpClientImpl) buildHttpRequest(ctx context.Context, request *llm.GenericHttpRequest) (*http.Request, error) {
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

// applyAuth applies authentication to the HTTP request
func (hc *HttpClientImpl) applyAuth(req *http.Request, auth *llm.AuthConfig) error {
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

// extractHeaders extracts headers from HTTP response
func (hc *HttpClientImpl) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0] // Take the first value
		}
	}
	return result
}
