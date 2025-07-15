package types

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// GenericHttpRequest represents a generic HTTP request that can be adapted to different providers
type GenericHttpRequest struct {
	// HTTP basics
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    []byte            `json:"body,omitempty"`

	// Request configuration
	Timeout         time.Duration `json:"timeout"`
	RetryPolicy     *RetryPolicy  `json:"retry_policy,omitempty"`
	FollowRedirects bool          `json:"follow_redirects"`

	// Provider-specific configurations
	ProviderConfig map[string]interface{} `json:"provider_config,omitempty"`

	// Authentication
	Auth *AuthConfig `json:"auth,omitempty"`

	// Context and metadata
	Context  context.Context   `json:"-"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// Streaming support
	Streaming bool `json:"streaming"`

	// Request tracking
	RequestID string    `json:"request_id"`
	Timestamp time.Time `json:"timestamp"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type   string            `json:"type"` // "bearer", "api_key", "basic", "custom"
	Token  string            `json:"token,omitempty"`
	APIKey string            `json:"api_key,omitempty"`
	Custom map[string]string `json:"custom,omitempty"`
}

// RetryPolicy defines retry behavior
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	RetryableErrors []string      `json:"retryable_errors,omitempty"`
}

// GenericHttpResponse represents a generic HTTP response
type GenericHttpResponse struct {
	// HTTP response basics
	StatusCode int         `json:"status_code"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body,omitempty"`

	// Response metadata
	Latency    time.Duration `json:"latency"`
	RequestID  string        `json:"request_id"`
	Provider   string        `json:"provider"`
	RetryCount int           `json:"retry_count"`
	CacheHit   bool          `json:"cache_hit"`

	// Error information
	Error *ResponseError `json:"error,omitempty"`

	// Streaming support
	Stream io.ReadCloser `json:"-"`

	// Raw HTTP response for advanced use cases
	RawResponse *http.Response `json:"-"`
}

// ResponseError represents an error in the response
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
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
			Method:          "POST",
			Headers:         make(map[string]string),
			Timeout:         30 * time.Second,
			FollowRedirects: true,
			Metadata:        make(map[string]string),
			Timestamp:       time.Now(),
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
	rb.request.Headers[key] = value
	return rb
}

// WithHeaders sets multiple headers
func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		rb.request.Headers[k] = v
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

// WithTimeout sets the request timeout
func (rb *RequestBuilder) WithTimeout(timeout time.Duration) *RequestBuilder {
	rb.request.Timeout = timeout
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
		Type:  "bearer",
		Token: token,
	}
	return rb
}

// WithAPIKey sets API key authentication
func (rb *RequestBuilder) WithAPIKey(apiKey string) *RequestBuilder {
	rb.request.Auth = &AuthConfig{
		Type:   "api_key",
		APIKey: apiKey,
	}
	return rb
}

// WithRetryPolicy sets retry policy
func (rb *RequestBuilder) WithRetryPolicy(policy *RetryPolicy) *RequestBuilder {
	rb.request.RetryPolicy = policy
	return rb
}

// WithContext sets the request context
func (rb *RequestBuilder) WithContext(ctx context.Context) *RequestBuilder {
	rb.request.Context = ctx
	return rb
}

// WithMetadata adds metadata
func (rb *RequestBuilder) WithMetadata(key, value string) *RequestBuilder {
	rb.request.Metadata[key] = value
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

// WithProviderConfig sets provider-specific configuration
func (rb *RequestBuilder) WithProviderConfig(config map[string]interface{}) *RequestBuilder {
	rb.request.ProviderConfig = config
	return rb
}

// Build returns the built request
func (rb *RequestBuilder) Build() *GenericHttpRequest {
	return rb.request
}
