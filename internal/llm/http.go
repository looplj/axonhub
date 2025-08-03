package llm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func ReadHTTPRequest(rawReq *http.Request) (*GenericHttpRequest, error) {
	req := &GenericHttpRequest{
		Method:     rawReq.Method,
		URL:        rawReq.URL.String(),
		Headers:    rawReq.Header,
		Body:       []byte{},
		Auth:       &AuthConfig{},
		Streaming:  false,
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

// GenericHttpRequest represents a generic HTTP request that can be adapted to different providers
type GenericHttpRequest struct {
	// HTTP basics
	Method  string      `json:"method"`
	URL     string      `json:"url"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body,omitempty"`

	// Authentication
	Auth *AuthConfig `json:"auth,omitempty"`

	// Streaming support
	Streaming bool `json:"streaming"`

	// Request tracking
	RequestID string `json:"request_id"`

	// Raw HTTP request for advanced use cases
	RawRequest *http.Request `json:"-"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	// Type represents the type of authentication.
	// "bearer", "api_key"
	Type string `json:"type"`

	// APIKey is the API key for the request.
	APIKey string `json:"api_key,omitempty"`

	// HeaderKey is the header key for the request if the type is "api_key".
	HeaderKey string `json:"header_key,omitempty"`
}

// GenericHttpResponse represents a generic HTTP response
type GenericHttpResponse struct {
	// HTTP response basics
	StatusCode int `json:"status_code"`

	// Response headers
	Headers http.Header `json:"headers"`

	// Response body, for the non-streaming response.
	Body []byte `json:"body,omitempty"`

	// Error information
	Error *ResponseError `json:"error,omitempty"`

	// Streaming support
	Stream io.ReadCloser `json:"-"`

	// Request information
	Request *GenericHttpRequest `json:"-"`

	// Raw HTTP response for advanced use cases
	RawResponse *http.Response `json:"-"`

	// Raw HTTP request for advanced use cases
	RawRequest *http.Request `json:"-"`
}

type GenericStreamEvent struct {
	LastEventID string `json:"last_event_id,omitempty"`
	Type        string `json:"type"`
	Data        []byte `json:"data"`
}

type GenericHttpError struct {
	Method     string `json:"method"`
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Status     string `json:"status"`
	Body       []byte `json:"body"`
}

func (e GenericHttpError) Error() string {
	return fmt.Sprintf("%s - %s with status %s", e.Method, e.URL, e.Status)
}

// ResponseError represents an error in the response
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
	Type    string `json:"type"`
	Details string `json:"details,omitempty"`
}

// RequestBuilder helps build GenericHttpRequest
type RequestBuilder struct {
	request *GenericHttpRequest
}

// NewRequestBuilder creates a new request builder
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		request: &GenericHttpRequest{
			Method:  "POST",
			Headers: make(http.Header),
		},
	}
}

// WithMethod sets the HTTP method
func (rb *RequestBuilder) WithMethod(method string) *RequestBuilder {
	rb.request.Method = method
	return rb
}

// WithURL sets the request URL
func (rb *RequestBuilder) WithURL(url string) *RequestBuilder {
	rb.request.URL = url
	return rb
}

// WithHeader adds a header
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.request.Headers.Set(key, value)
	return rb
}

// WithHeaders sets multiple headers
func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		rb.request.Headers.Set(k, v)
	}
	return rb
}

// WithBody sets the request body
func (rb *RequestBuilder) WithBody(body any) *RequestBuilder {
	switch v := body.(type) {
	case []byte:
		rb.request.Body = v
	case string:
		rb.request.Body = []byte(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		rb.request.Body = b
	}
	return rb
}

// WithAuth sets authentication
func (rb *RequestBuilder) WithAuth(auth *AuthConfig) *RequestBuilder {
	rb.request.Auth = auth
	return rb
}

// WithBearerToken sets bearer token authentication
func (rb *RequestBuilder) WithBearerToken(token string) *RequestBuilder {
	rb.request.Auth = &AuthConfig{
		Type:   "bearer",
		APIKey: token,
	}
	return rb
}

// WithAPIKey sets API key authentication
func (rb *RequestBuilder) WithAPIKey(apiKey string) *RequestBuilder {
	rb.request.Auth = &AuthConfig{
		Type:      "api_key",
		HeaderKey: apiKey,
	}
	return rb
}

// WithRequestID sets the request ID
func (rb *RequestBuilder) WithRequestID(requestID string) *RequestBuilder {
	rb.request.RequestID = requestID
	return rb
}

// WithStreaming enables streaming
func (rb *RequestBuilder) WithStreaming(streaming bool) *RequestBuilder {
	rb.request.Streaming = streaming
	return rb
}

// Build returns the built request
func (rb *RequestBuilder) Build() *GenericHttpRequest {
	return rb.request
}
