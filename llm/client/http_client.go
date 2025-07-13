package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/looplj/axonhub/llm/types"
)

// HttpClient interface for making HTTP requests
type HttpClient interface {
	Do(ctx context.Context, request *types.GenericHttpRequest) (*types.GenericHttpResponse, error)
	DoStream(ctx context.Context, request *types.GenericHttpRequest) (*types.GenericHttpResponse, error)
}

// HttpClientImpl implements the HttpClient interface
type HttpClientImpl struct {
	client *http.Client
}

// NewHttpClient creates a new HTTP client
func NewHttpClient(timeout time.Duration) HttpClient {
	return &HttpClientImpl{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Do executes the HTTP request
func (hc *HttpClientImpl) Do(ctx context.Context, request *types.GenericHttpRequest) (*types.GenericHttpResponse, error) {
	startTime := time.Now()
	
	// Create HTTP request
	httpReq, err := hc.buildHttpRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	// Execute request with retry logic
	var httpResp *http.Response
	var lastErr error
	retryCount := 0
	maxRetries := 0
	
	if request.RetryPolicy != nil {
		maxRetries = request.RetryPolicy.MaxRetries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay for retry
			delay := hc.calculateRetryDelay(request.RetryPolicy, attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			retryCount++
		}

		httpResp, lastErr = hc.client.Do(httpReq)
		if lastErr == nil && !hc.shouldRetry(httpResp.StatusCode, request.RetryPolicy) {
			break
		}
		
		if httpResp != nil {
			httpResp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("HTTP request failed after %d retries: %w", retryCount, lastErr)
	}

	latency := time.Since(startTime)

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		httpResp.Body.Close()
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	httpResp.Body.Close()

	// Build generic response
	response := &types.GenericHttpResponse{
		StatusCode:  httpResp.StatusCode,
		Headers:     hc.extractHeaders(httpResp.Header),
		Body:        body,
		Latency:     latency,
		RequestID:   request.RequestID,
		RetryCount:  retryCount,
		RawResponse: httpResp,
	}

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		response.Error = &types.ResponseError{
			Code:    fmt.Sprintf("HTTP_%d", httpResp.StatusCode),
			Message: fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, httpResp.Status),
			Type:    "http_error",
			Details: string(body),
		}
	}

	return response, nil
}

// DoStream executes a streaming HTTP request
func (hc *HttpClientImpl) DoStream(ctx context.Context, request *types.GenericHttpRequest) (*types.GenericHttpResponse, error) {
	startTime := time.Now()
	
	// Create HTTP request
	httpReq, err := hc.buildHttpRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	// Add streaming headers
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Cache-Control", "no-cache")

	// Execute request
	httpResp, err := hc.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP stream request failed: %w", err)
	}

	latency := time.Since(startTime)

	// Build generic response with stream
	response := &types.GenericHttpResponse{
		StatusCode:  httpResp.StatusCode,
		Headers:     hc.extractHeaders(httpResp.Header),
		Latency:     latency,
		RequestID:   request.RequestID,
		Stream:      httpResp.Body, // Keep the stream open
		RawResponse: httpResp,
	}

	// Check for HTTP errors
	if httpResp.StatusCode >= 400 {
		// Read error body for streaming requests
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr == nil {
			response.Body = body
		}
		httpResp.Body.Close()
		
		response.Error = &types.ResponseError{
			Code:    fmt.Sprintf("HTTP_%d", httpResp.StatusCode),
			Message: fmt.Sprintf("HTTP %d: %s", httpResp.StatusCode, httpResp.Status),
			Type:    "http_error",
			Details: string(body),
		}
		response.Stream = nil
	}

	return response, nil
}

// buildHttpRequest builds an HTTP request from GenericHttpRequest
func (hc *HttpClientImpl) buildHttpRequest(ctx context.Context, request *types.GenericHttpRequest) (*http.Request, error) {
	var body io.Reader
	if len(request.Body) > 0 {
		body = bytes.NewReader(request.Body)
	}

	// Use request context if provided, otherwise use the passed context
	reqCtx := ctx
	if request.Context != nil {
		reqCtx = request.Context
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, request.Method, request.URL, body)
	if err != nil {
		return nil, err
	}

	// Set headers
	for key, value := range request.Headers {
		httpReq.Header.Set(key, value)
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
func (hc *HttpClientImpl) applyAuth(req *http.Request, auth *types.AuthConfig) error {
	switch auth.Type {
	case "bearer":
		if auth.Token == "" {
			return fmt.Errorf("bearer token is required")
		}
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	case "api_key":
		if auth.APIKey == "" {
			return fmt.Errorf("API key is required")
		}
		req.Header.Set("Authorization", "Bearer "+auth.APIKey)
	case "basic":
		// Basic auth would require username and password
		// This is a simplified implementation
		if auth.Token == "" {
			return fmt.Errorf("basic auth token is required")
		}
		req.Header.Set("Authorization", "Basic "+auth.Token)
	case "custom":
		for key, value := range auth.Custom {
			req.Header.Set(key, value)
		}
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

// calculateRetryDelay calculates the delay for retry attempts
func (hc *HttpClientImpl) calculateRetryDelay(policy *types.RetryPolicy, attempt int) time.Duration {
	if policy == nil {
		return time.Second
	}

	delay := policy.InitialDelay
	for i := 1; i < attempt; i++ {
		delay = time.Duration(float64(delay) * policy.BackoffFactor)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
			break
		}
	}

	return delay
}

// shouldRetry determines if a request should be retried based on status code
func (hc *HttpClientImpl) shouldRetry(statusCode int, policy *types.RetryPolicy) bool {
	if policy == nil {
		return false
	}

	// Retry on 5xx errors and some 4xx errors
	retryableStatusCodes := []int{429, 500, 502, 503, 504}
	for _, code := range retryableStatusCodes {
		if statusCode == code {
			return true
		}
	}

	return false
}