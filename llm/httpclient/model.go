package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

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
	ClientIP  string `json:"client_ip"`

	// RequestType is the type of the request, ref to llm.RequestType.
	// For example, "chat", "image", "embedding", etc.
	// If empty, will use the "chat" request type.
	RequestType string `json:"request_type"`

	// APIFormat is the format of the API response,ref to llm.APIFormat.
	APIFormat string `json:"api_format"`

	// Raw HTTP request for advanced use cases
	RawRequest *http.Request `json:"-"`

	// Metadata for advanced use cases
	Metadata map[string]string `json:"-"`

	// TransformerMetadata stores transformer-specific metadata for preserving format during transformations.
	// This supports any type of value for flexibility.
	TransformerMetadata map[string]any `json:"-"`

	// SkipInboundQueryMerge when set to true, prevents query parameters from the original
	// inbound request from being merged into this request during MergeInboundRequest.
	SkipInboundQueryMerge bool `json:"-"`
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

	// BinaryStreamDoneEventType marks EOF for non-SSE streaming responses.
	BinaryStreamDoneEventType = "binary.done"
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
	// Size optionally carries the byte size of a binary chunk that was elided
	// from Data for persistence (e.g. raw TTS audio chunks). It lets stream
	// aggregators report total bytes without retaining the audio payload.
	Size int `json:"size,omitempty"`
}

// IsBinaryAudioChunk reports whether the event carries a raw binary audio payload
// (e.g. TTS audio/* or application/octet-stream chunks) as opposed to an SSE event
// or the binary.done sentinel.
func (e *StreamEvent) IsBinaryAudioChunk() bool {
	if e == nil || e.Type == "" || e.Type == BinaryStreamDoneEventType {
		return false
	}

	t := strings.ToLower(strings.TrimSpace(e.Type))

	return strings.HasPrefix(t, "audio/") || t == "application/octet-stream"
}

// SummarizeBinaryChunk returns a copy of the event suitable for persistence:
// for raw binary audio chunks the audio bytes are dropped and only the byte
// count is kept in Size, so persistence buffers never accumulate the full
// audio payload. Non-binary events are returned as-is.
func SummarizeBinaryChunk(event *StreamEvent) *StreamEvent {
	if event == nil || !event.IsBinaryAudioChunk() {
		return event
	}

	return &StreamEvent{
		LastEventID: event.LastEventID,
		Type:        event.Type,
		Size:        len(event.Data),
	}
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
