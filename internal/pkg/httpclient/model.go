package httpclient

import (
	"io"
	"net/http"
)

// Request represents a generic HTTP request that can be adapted to different providers.
type Request struct {
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

// AuthConfig represents authentication configuration.
type AuthConfig struct {
	// Type represents the type of authentication.
	// "bearer", "api_key"
	Type string `json:"type"`

	// APIKey is the API key for the request.
	APIKey string `json:"api_key,omitempty"`

	// HeaderKey is the header key for the request if the type is "api_key".
	HeaderKey string `json:"header_key,omitempty"`
}

// Response represents a generic HTTP response.
type Response struct {
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
	Request *Request `json:"-"`

	// Raw HTTP response for advanced use cases
	RawResponse *http.Response `json:"-"`

	// Raw HTTP request for advanced use cases
	RawRequest *http.Request `json:"-"`
}

type StreamEvent struct {
	LastEventID string `json:"last_event_id,omitempty"`
	Type        string `json:"type"`
	Data        []byte `json:"data"`
}
