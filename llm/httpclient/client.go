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

// HttpClient interface for making HTTP requests
type HttpClient interface {
	// Do executes a HTTP request and returns a HTTP response.
	Do(ctx context.Context, request *llm.GenericHttpRequest) (*llm.GenericHttpResponse, error)

	// DoStream a HTTP request with streaming response
	DoStream(ctx context.Context, request *llm.GenericHttpRequest) (streams.Stream[*llm.GenericHttpResponse], error)
}

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
	// Create HTTP request
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

	// Create SSE stream
	return &sseStream{
		ctx: ctx,
		response: &llm.GenericHttpResponse{
			StatusCode:  rawResp.StatusCode,
			Headers:     rawResp.Header,
			RawResponse: rawResp,
		},
		body:   rawResp.Body,
		events: sse.Read(rawResp.Body, nil),
		closed: false,
	}, nil
}

// sseStream implements streams.Stream for Server-Sent Events
type sseStream struct {
	ctx      context.Context
	response *llm.GenericHttpResponse
	body     io.ReadCloser
	events   func(func(sse.Event, error) bool)
	current  *llm.GenericHttpResponse
	err      error
	closed   bool
	started  bool
}

// Next advances to the next event in the stream
func (s *sseStream) Next() bool {
	if s.closed || s.err != nil {
		return false
	}

	// Check context cancellation
	select {
	case <-s.ctx.Done():
		s.err = s.ctx.Err()
		err := s.Close()
		if err != nil {
			log.Warn(s.ctx, "failed to close SSE stream", log.Cause(err))
		}
		return false
	default:
	}

	// If this is an error response, return it once
	if s.response.Error != nil && !s.started {
		s.current = s.response
		s.started = true
		s.err = fmt.Errorf("stream error: %s", s.response.Error.Message)
		return true
	}

	// Read next SSE event
	eventReceived := false
	s.events(func(event sse.Event, err error) bool {
		if err != nil {
			s.err = fmt.Errorf("SSE event error: %w", err)
			return false
		}

		// Create response for this event
		s.current = &llm.GenericHttpResponse{
			StatusCode:  s.response.StatusCode,
			Headers:     s.response.Headers,
			Body:        []byte(event.Data),
			RawResponse: s.response.RawResponse,
		}
		eventReceived = true
		return false // Stop after first event
	})

	if !eventReceived {
		// No more events, stream is done
		err := s.Close()
		if err != nil {
			log.Warn(s.ctx, "failed to close SSE stream", log.Cause(err))
		}
		return false
	}

	return true
}

// Current returns the current event data
func (s *sseStream) Current() *llm.GenericHttpResponse {
	return s.current
}

// Err returns any error that occurred during streaming
func (s *sseStream) Err() error {
	return s.err
}

// Close closes the stream and releases resources
func (s *sseStream) Close() error {
	if s.closed {
		return nil
	}

	s.closed = true
	if s.body != nil {
		return s.body.Close()
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

	// Set headers
	for key, values := range request.Headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

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
