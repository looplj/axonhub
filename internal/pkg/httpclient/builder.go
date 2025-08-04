package httpclient

import (
	"encoding/json"
	"net/http"
)

// RequestBuilder helps build Request.
type RequestBuilder struct {
	request *Request
}

// NewRequestBuilder creates a new request builder.
func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		request: &Request{
			Method:  "POST",
			Headers: make(http.Header),
		},
	}
}

// WithMethod sets the HTTP method.
func (rb *RequestBuilder) WithMethod(method string) *RequestBuilder {
	rb.request.Method = method
	return rb
}

// WithURL sets the request URL.
func (rb *RequestBuilder) WithURL(url string) *RequestBuilder {
	rb.request.URL = url
	return rb
}

// WithHeader adds a header.
func (rb *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	rb.request.Headers.Set(key, value)
	return rb
}

// WithHeaders sets multiple headers.
func (rb *RequestBuilder) WithHeaders(headers map[string]string) *RequestBuilder {
	for k, v := range headers {
		rb.request.Headers.Set(k, v)
	}
	return rb
}

// WithBody sets the request body.
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

// WithAuth sets authentication.
func (rb *RequestBuilder) WithAuth(auth *AuthConfig) *RequestBuilder {
	rb.request.Auth = auth
	return rb
}

// WithBearerToken sets bearer token authentication.
func (rb *RequestBuilder) WithBearerToken(token string) *RequestBuilder {
	rb.request.Auth = &AuthConfig{
		Type:   "bearer",
		APIKey: token,
	}
	return rb
}

// WithAPIKey sets API key authentication.
func (rb *RequestBuilder) WithAPIKey(apiKey string) *RequestBuilder {
	rb.request.Auth = &AuthConfig{
		Type:      "api_key",
		HeaderKey: apiKey,
	}
	return rb
}

// WithRequestID sets the request ID.
func (rb *RequestBuilder) WithRequestID(requestID string) *RequestBuilder {
	rb.request.RequestID = requestID
	return rb
}

// WithStreaming enables streaming.
func (rb *RequestBuilder) WithStreaming(streaming bool) *RequestBuilder {
	rb.request.Streaming = streaming
	return rb
}

// Build returns the built request.
func (rb *RequestBuilder) Build() *Request {
	return rb.request
}
