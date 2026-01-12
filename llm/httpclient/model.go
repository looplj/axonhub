package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/looplj/axonhub/llm/streams"
)

// Request represents a generic HTTP request that can be adapted to different providers.
type Request struct {
	// HTTP basics
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	Path        string      `json:"path"`
	Query       url.Values  `json:"query"`
	Headers     http.Header `json:"headers"`
	ContentType string      `json:"content_type"`
	Body        []byte      `json:"body,omitempty"`

	// JSONBody is a json representation of the request body.
	// For some scenario, the request body is not a json, but we still need to marshal it to json for the request.
	// For example, the image edit api.
	// If the JSONBody is not empty, will use the JSONBody to save in to the log.
	JSONBody []byte `json:"json_body,omitempty"`

	// Authentication
	Auth *AuthConfig `json:"auth,omitempty"`

	// Request tracking
	RequestID string `json:"request_id"`

	// Raw HTTP request for advanced use cases
	RawRequest *http.Request `json:"-"`

	// Metadata for advanced use cases
	Metadata map[string]string `json:"-"`

	// TransformerMetadata stores transformer-specific metadata for preserving format during transformations.
	// This supports any type of value for flexibility.
	TransformerMetadata map[string]any `json:"-"`
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

const (
	AuthTypeBearer = "bearer"
	AuthTypeAPIKey = "api_key"
)

// Response represents a generic HTTP response.
type Response struct {
	// HTTP response basics
	StatusCode int `json:"status_code"`

	// Response headers
	Headers http.Header `json:"headers"`

	// Response body, for the non-streaming response.
	Body []byte `json:"body,omitempty"`

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

// StreamDecoder defines the interface for decoding streaming responses.
type StreamDecoder = streams.Stream[*StreamEvent]

// StreamDecoderFactory is a function that creates a StreamDecoder from a ReadCloser.
type StreamDecoderFactory func(ctx context.Context, rc io.ReadCloser) StreamDecoder

type _StreamEventJSON struct {
	LastEventID string `json:"last_event_id,omitempty"`
	Type        string `json:"type"`
	Data        string `json:"data"`
}

func EncodeStreamEventToJSON(event *StreamEvent) ([]byte, error) {
	return json.Marshal(_StreamEventJSON{
		LastEventID: event.LastEventID,
		Type:        event.Type,
		Data:        string(event.Data),
	})
}
