package httpclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// HttpClient implements the HttpClient interface.
type HttpClient struct {
	client      *http.Client
	proxyConfig *objects.ProxyConfig
}

// NewHttpClientWithProxy creates a new HTTP client with proxy configuration.
func NewHttpClientWithProxy(proxyConfig *objects.ProxyConfig) *HttpClient {
	transport := &http.Transport{
		Proxy: getProxyFunc(proxyConfig),
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &HttpClient{
		client: &http.Client{
			Transport: transport,
		},
		proxyConfig: proxyConfig,
	}
}

// getProxyFunc returns a proxy function based on the proxy configuration.
func getProxyFunc(config *objects.ProxyConfig) func(*http.Request) (*url.URL, error) {
	// Handle nil config (backward compatibility) - default to environment
	if config == nil {
		return http.ProxyFromEnvironment
	}

	switch config.Type {
	case objects.ProxyTypeDisabled:
		// No proxy - direct connection
		return func(*http.Request) (*url.URL, error) {
			return nil, nil
		}

	case objects.ProxyTypeEnvironment:
		// Use environment variables (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
		return http.ProxyFromEnvironment

	case objects.ProxyTypeURL:
		// Use configured URL with optional authentication
		if config.URL == "" {
			return func(*http.Request) (*url.URL, error) {
				return nil, errors.New("proxy URL is required when type is 'url'")
			}
		}

		proxyURL, err := url.Parse(config.URL)
		if err != nil {
			return func(_ *http.Request) (*url.URL, error) {
				return nil, fmt.Errorf("invalid proxy URL: %w", err)
			}
		}

		if config.Username != "" && config.Password != "" {
			proxyURL.User = url.UserPassword(config.Username, config.Password)
		}

		log.Debug(context.Background(), "use custom proxy", log.Any("proxy_url", proxyURL.Redacted()))

		return http.ProxyURL(proxyURL)

	default:
		// Unknown type - fall back to environment
		return http.ProxyFromEnvironment
	}
}

// NewHttpClient creates a new HTTP client.
func NewHttpClient() *HttpClient {
	return &HttpClient{
		client: &http.Client{},
	}
}

// NewHttpClientWithClient creates a new HTTP client with a custom http.Client.
func NewHttpClientWithClient(client *http.Client) *HttpClient {
	return &HttpClient{
		client: client,
	}
}

// Do executes the HTTP request.
func (hc *HttpClient) Do(ctx context.Context, request *Request) (*Response, error) {
	log.Debug(ctx, "execute http request", log.Any("request", request), log.Any("proxy", hc.proxyConfig))

	rawReq, err := hc.buildHttpRequest(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	rawReq.Header.Set("Accept", "application/json")

	rawResp, err := hc.client.Do(rawReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	defer func() {
		err := rawResp.Body.Close()
		if err != nil {
			log.Warn(ctx, "failed to close HTTP response body", log.Cause(err))
		}
	}()

	body, err := io.ReadAll(rawResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if rawResp.StatusCode >= 400 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "HTTP request failed",
				log.String("method", rawReq.Method),
				log.String("url", rawReq.URL.String()),
				log.Any("status_code", rawResp.StatusCode),
				log.String("body", string(body)))
		}

		return nil, &Error{
			Method:     rawReq.Method,
			URL:        rawReq.URL.String(),
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "HTTP request success",
			log.String("method", rawReq.Method),
			log.String("url", rawReq.URL.String()),
			log.Any("status_code", rawResp.StatusCode),
			log.String("body", string(body)))
	}

	// Build generic response
	response := &Response{
		StatusCode:  rawResp.StatusCode,
		Headers:     rawResp.Header,
		Body:        body,
		RawResponse: rawResp,
		Stream:      nil,
		Request:     request,
		RawRequest:  rawReq,
	}

	return response, nil
}

// DoStream executes a streaming HTTP request using Server-Sent Events.
func (hc *HttpClient) DoStream(ctx context.Context, request *Request) (streams.Stream[*StreamEvent], error) {
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
			err := rawResp.Body.Close()
			if err != nil {
				log.Warn(ctx, "failed to close HTTP response body", log.Cause(err))
			}
		}()

		// Read error body for streaming requests
		body, err := io.ReadAll(rawResp.Body)
		if err != nil {
			return nil, err
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "HTTP stream request failed",
				log.String("method", rawReq.Method),
				log.String("url", rawReq.URL.String()),
				log.Any("status_code", rawResp.StatusCode),
				log.String("body", string(body)))
		}

		return nil, &Error{
			Method:     rawReq.Method,
			URL:        rawReq.URL.String(),
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	// Determine content type and select appropriate decoder
	contentType := rawResp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/event-stream" // Default to SSE
	}

	// Try to get a registered decoder for the content type
	decoderFactory, exists := GetDecoder(contentType)
	if !exists {
		// Fallback to default SSE decoder
		log.Debug(ctx, "no decoder found for content type, using default SSE", log.String("content_type", contentType))

		decoderFactory = NewDefaultSSEDecoder
	}

	stream := decoderFactory(ctx, rawResp.Body)

	return stream, nil
}

// buildHttpRequest builds an HTTP request from Request.
func (hc *HttpClient) buildHttpRequest(
	ctx context.Context,
	request *Request,
) (*http.Request, error) {
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

	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "axonhub/1.0")
	}

	for k := range blockedHeaders {
		httpReq.Header.Del(k)
	}

	if request.Auth != nil {
		err = applyAuth(httpReq.Header, request.Auth)
		if err != nil {
			return nil, fmt.Errorf("failed to apply authentication: %w", err)
		}
	}

	if len(request.Query) > 0 {
		if httpReq.URL.RawQuery != "" {
			httpReq.URL.RawQuery += "&"
		}

		httpReq.URL.RawQuery += request.Query.Encode()
	}

	return httpReq, nil
}

// applyAuth applies authentication to the HTTP request.
func applyAuth(headers http.Header, auth *AuthConfig) error {
	switch auth.Type {
	case "bearer":
		if auth.APIKey == "" {
			return fmt.Errorf("bearer token is required")
		}

		headers.Set("Authorization", "Bearer "+auth.APIKey)
	case "api_key":
		if auth.HeaderKey == "" {
			return fmt.Errorf("header key is required")
		}

		headers.Set(auth.HeaderKey, auth.APIKey)
	default:
		return fmt.Errorf("unsupported auth type: %s", auth.Type)
	}

	return nil
}

// extractHeaders extracts headers from HTTP response.
func (hc *HttpClient) extractHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)

	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0] // Take the first value
		}
	}

	return result
}
