package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/tmaxmax/go-sse"
)

func TestHttpClientImpl_Do(t *testing.T) {
	tests := []struct {
		name           string
		request        *Request
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		errReg         *regexp.Regexp
		validate       func(*Response) bool
	}{
		{
			name: "successful request",
			request: &Request{
				Method: http.MethodPost,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{"test": "data"}`),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"response": "success"}`))
			},
			wantErr: false,
			validate: func(resp *Response) bool {
				return resp.StatusCode == http.StatusOK &&
					string(resp.Body) == `{"response": "success"}`
			},
		},
		{
			name: "request with authentication",
			request: &Request{
				Method: http.MethodPost,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{"test": "data"}`),
				Auth: &AuthConfig{
					Type:   "bearer",
					APIKey: "test-token",
				},
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error": "unauthorized"}`))
					return
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"response": "authenticated"}`))
			},
			wantErr: false,
			validate: func(resp *Response) bool {
				return resp.StatusCode == http.StatusOK &&
					string(resp.Body) == `{"response": "authenticated"}`
			},
		},
		{
			name: "HTTP error response",
			request: &Request{
				Method: http.MethodPost,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{"test": "data"}`),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "bad request"}`))
			},
			wantErr: true,
			errReg:  regexp.MustCompile("POST - http://127.0.0.1:\\d+ with status 400 Bad Request"),
			validate: func(resp *Response) bool {
				return resp == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			// Update request URL to point to test server
			tt.request.URL = server.URL

			// Create client
			client := NewHttpClient()

			// Execute request
			result, err := client.Do(t.Context(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Do() expected error but got none")
					return
				}

				if tt.errReg != nil && !tt.errReg.MatchString(err.Error()) {
					t.Errorf("Do() error = %v, want error containing %v", err, tt.errReg)
				}

				return
			}

			if err != nil {
				t.Errorf("Do() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("Do() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("Do() validation failed for result: %+v", result)
			}
		})
	}
}

func TestHttpClientImpl_DoStream(t *testing.T) {
	tests := []struct {
		name           string
		request        *Request
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		errContains    string
		validate       func(stream interface{}) bool
	}{
		{
			name: "successful streaming request",
			request: &Request{
				Method: http.MethodPost,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{"stream": true}`),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				// Check streaming headers
				if r.Header.Get("Accept") != "text/event-stream" {
					t.Errorf(
						"Expected Accept header to be text/event-stream, got %s",
						r.Header.Get("Accept"),
					)
				}

				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Cache-Control", "no-cache")
				w.Header().Set("Connection", "keep-alive")
				w.WriteHeader(http.StatusOK)

				// Write SSE events
				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Error("ResponseWriter does not support flushing")
					return
				}

				events := []string{
					"data: {\"id\": \"1\", \"content\": \"Hello\"}\n\n",
					"data: {\"id\": \"2\", \"content\": \"World\"}\n\n",
					"data: [DONE]\n\n",
				}

				for _, event := range events {
					fmt.Fprint(w, event)
					flusher.Flush()
					time.Sleep(10 * time.Millisecond) // Small delay between events
				}
			},
			wantErr: false,
			validate: func(stream interface{}) bool {
				// This is a basic validation - in a real test we'd iterate through the stream
				return stream != nil
			},
		},
		{
			name: "HTTP error in streaming request",
			request: &Request{
				Method: http.MethodPost,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{"stream": true}`),
			},
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "unauthorized"}`))
			},
			wantErr: true,
			validate: func(stream interface{}) bool {
				return stream == nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			// Update request URL to point to test server
			tt.request.URL = server.URL

			// Create client
			client := NewHttpClient()

			// Execute streaming request
			result, err := client.DoStream(t.Context(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DoStream() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("DoStream() error = %v, want error containing %v", err, tt.errContains)
				}

				return
			}

			if err != nil {
				t.Errorf("DoStream() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("DoStream() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("DoStream() validation failed for result: %+v", result)
			}

			// Clean up stream
			if result != nil {
				result.Close()
			}
		})
	}
}

func TestHttpClientImpl_buildHttpRequest(t *testing.T) {
	client := &HttpClient{
		client: &http.Client{Timeout: 5 * time.Second},
	}

	tests := []struct {
		name        string
		request     *Request
		wantErr     bool
		errContains string
		validate    func(*http.Request) bool
	}{
		{
			name: "basic request",
			request: &Request{
				Method: http.MethodPost,
				URL:    "https://api.example.com/test",
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: []byte(`{"test": "data"}`),
			},
			wantErr: false,
			validate: func(req *http.Request) bool {
				return req.Method == http.MethodPost &&
					req.URL.String() == "https://api.example.com/test" &&
					req.Header.Get("Content-Type") == "application/json"
			},
		},
		{
			name: "request with bearer auth",
			request: &Request{
				Method: http.MethodPost,
				URL:    "https://api.example.com/test",
				Auth: &AuthConfig{
					Type:   "bearer",
					APIKey: "test-token",
				},
			},
			wantErr: false,
			validate: func(req *http.Request) bool {
				return req.Header.Get("Authorization") == "Bearer test-token"
			},
		},
		{
			name: "request with api_key auth",
			request: &Request{
				Method: http.MethodPost,
				URL:    "https://api.example.com/test",
				Auth: &AuthConfig{
					Type:      "api_key",
					APIKey:    "test-key",
					HeaderKey: "X-API-Key",
				},
			},
			wantErr: false,
			validate: func(req *http.Request) bool {
				return req.Header.Get("X-Api-Key") == "test-key"
			},
		},
		{
			name: "invalid URL",
			request: &Request{
				Method: http.MethodPost,
				URL:    "://invalid-url",
			},
			wantErr:     true,
			errContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.buildHttpRequest(t.Context(), tt.request)

			if tt.wantErr {
				if err == nil {
					t.Errorf("buildHttpRequest() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf(
						"buildHttpRequest() error = %v, want error containing %v",
						err,
						tt.errContains,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("buildHttpRequest() unexpected error = %v", err)
				return
			}

			if result == nil {
				t.Errorf("buildHttpRequest() returned nil result")
				return
			}

			if tt.validate != nil && !tt.validate(result) {
				t.Errorf("buildHttpRequest() validation failed for result: %+v", result)
			}
		})
	}
}

func TestHttpClientImpl_applyAuth(t *testing.T) {
	client := &HttpClient{}

	tests := []struct {
		name        string
		auth        *AuthConfig
		wantErr     bool
		errContains string
		validate    func(*http.Request) bool
	}{
		{
			name: "bearer auth",
			auth: &AuthConfig{
				Type:   "bearer",
				APIKey: "test-token",
			},
			wantErr: false,
			validate: func(req *http.Request) bool {
				return req.Header.Get("Authorization") == "Bearer test-token"
			},
		},
		{
			name: "api_key auth",
			auth: &AuthConfig{
				Type:      "api_key",
				APIKey:    "test-key",
				HeaderKey: "X-API-Key",
			},
			wantErr: false,
			validate: func(req *http.Request) bool {
				return req.Header.Get("X-Api-Key") == "test-key"
			},
		},
		{
			name: "bearer auth without token",
			auth: &AuthConfig{
				Type: "bearer",
			},
			wantErr:     true,
			errContains: "bearer token is required",
		},
		{
			name: "api_key auth without header key",
			auth: &AuthConfig{
				Type:   "api_key",
				APIKey: "test-key",
			},
			wantErr:     true,
			errContains: "header key is required",
		},
		{
			name: "unsupported auth type",
			auth: &AuthConfig{
				Type: "oauth",
			},
			wantErr:     true,
			errContains: "unsupported auth type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
			err := client.applyAuth(req, tt.auth)

			if tt.wantErr {
				if err == nil {
					t.Errorf("applyAuth() expected error but got none")
					return
				}

				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf(
						"applyAuth() error = %v, want error containing %v",
						err,
						tt.errContains,
					)
				}

				return
			}

			if err != nil {
				t.Errorf("applyAuth() unexpected error = %v", err)
				return
			}

			if tt.validate != nil && !tt.validate(req) {
				t.Errorf("applyAuth() validation failed for request: %+v", req.Header)
			}
		})
	}
}

func TestHttpClientImpl_extractHeaders(t *testing.T) {
	client := &HttpClient{}

	headers := http.Header{
		"Content-Type":  []string{"application/json"},
		"Authorization": []string{"Bearer token"},
		"X-Custom":      []string{"value1", "value2"}, // Multiple values
		"Empty-Header":  []string{},                   // Empty values
	}

	result := client.extractHeaders(headers)

	expected := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token",
		"X-Custom":      "value1", // Should take first value
	}

	for key, expectedValue := range expected {
		if result[key] != expectedValue {
			t.Errorf("extractHeaders() key %s = %v, want %v", key, result[key], expectedValue)
		}
	}

	// Empty-Header should not be in result
	if _, exists := result["Empty-Header"]; exists {
		t.Errorf("extractHeaders() should not include headers with empty values")
	}
}

// Test SSE Stream implementation.
func TestSSEStream(t *testing.T) {
	// Create a mock response body with SSE data
	sseData := `data: {"id": "1", "content": "Hello"}

data: {"id": "2", "content": "World"}

data: [DONE]

`
	body := io.NopCloser(strings.NewReader(sseData))

	stream := &sseStreamWrapper{
		ctx:       t.Context(),
		sseStream: sse.NewStream(body),
	}

	// Test that we can close the stream
	err := stream.Close()
	if err != nil {
		t.Errorf("Close() unexpected error = %v", err)
	}

	// Test that closing again doesn't error
	err = stream.Close()
	if err != nil {
		t.Errorf("Close() second call unexpected error = %v", err)
	}

	// Test Current() and Err() methods
	if stream.Current() != nil {
		t.Errorf("Current() should return nil when no event has been read")
	}

	if stream.Err() != nil {
		t.Errorf("Err() should return nil initially")
	}
}
