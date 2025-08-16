package bedrock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

const DefaultVersion = "bedrock-2023-05-31"

var DefaultEndpoints = map[string]bool{
	"/v1/complete": true,
	"/v1/messages": true,
}

// Executor implements a Bedrock-specific executor that handles AWS authentication
// and request transformation for Amazon Bedrock API calls.
type Executor struct {
	// region is the AWS region for Bedrock
	region string
	// AWS config and signer
	config aws.Config
	signer *v4.Signer
	// HTTP client for making requests
	httpClient *http.Client
}

// NewExecutor creates a new Bedrock executor with the specified AWS region.
// It reads AWS credentials from environment variables.
func NewExecutor(region string, accessKeyID, secretAccessKey string) (*Executor, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			aws.NewCredentialsCache(
				credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
			),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Executor{
		region:     region,
		config:     cfg,
		signer:     v4.NewSigner(),
		httpClient: &http.Client{},
	}, nil
}

// Do executes a HTTP request with Bedrock transformations and AWS signing.
func (e *Executor) Do(ctx context.Context, rawReq *httpclient.Request) (*httpclient.Response, error) {
	// Transform the request for Bedrock
	transformedReq, err := e.transformRequest(rawReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, transformedReq.Method, transformedReq.URL, strings.NewReader(string(transformedReq.Body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, values := range transformedReq.Headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Sign the request using AWS SDK v4 signer
	creds, err := e.config.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	hash := sha256.Sum256(transformedReq.Body)

	err = e.signer.SignHTTP(ctx, creds, httpReq, hex.EncodeToString(hash[:]), "bedrock", e.region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute the request
	rawResp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	defer func() {
		err := rawResp.Body.Close()
		if err != nil {
			log.Error(ctx, "failed to close response body", log.Cause(err))
		}
	}()

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

		return nil, &httpclient.Error{
			Method:     rawReq.Method,
			URL:        rawReq.URL,
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	// Convert to httpclient.Response
	body, err := io.ReadAll(rawResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &httpclient.Response{
		StatusCode: rawResp.StatusCode,
		Headers:    rawResp.Header,
		Body:       body,
	}, nil
}

// DoStream executes a streaming HTTP request with Bedrock transformations and AWS signing.
func (e *Executor) DoStream(ctx context.Context, rawReq *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Transform the request for Bedrock
	transformedReq, err := e.transformRequest(rawReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, transformedReq.Method, transformedReq.URL, strings.NewReader(string(transformedReq.Body)))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	for key, values := range transformedReq.Headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Sign the request using AWS SDK v4 signer
	creds, err := e.config.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	hash := sha256.Sum256(transformedReq.Body)

	err = e.signer.SignHTTP(ctx, creds, httpReq, hex.EncodeToString(hash[:]), "bedrock", e.region, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute the request
	rawResp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
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

		return nil, &httpclient.Error{
			Method:     rawReq.Method,
			URL:        rawReq.URL,
			StatusCode: rawResp.StatusCode,
			Status:     rawResp.Status,
			Body:       body,
		}
	}

	// Create stream decoder based on content type
	contentType := rawResp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/vnd.amazon.eventstream") {
		// Use AWS EventStream decoder
		decoder := NewAWSEventStreamDecoder(ctx, rawResp.Body)
		return &decoderStreamWrapper{decoder: decoder}, nil
	}

	// For other content types, create a simple line-based decoder
	return &simpleStreamWrapper{body: rawResp.Body}, nil
}

// simpleStreamWrapper provides a simple stream wrapper for non-EventStream responses.
type simpleStreamWrapper struct {
	body    io.ReadCloser
	current *httpclient.StreamEvent
	err     error
	done    bool
}

func (s *simpleStreamWrapper) Next() bool {
	if s.done || s.err != nil {
		return false
	}

	// Read all data as a single event
	data, err := io.ReadAll(s.body)
	if err != nil {
		s.err = err
		return false
	}

	s.current = &httpclient.StreamEvent{
		Type: "data",
		Data: data,
	}
	s.done = true

	return true
}

func (s *simpleStreamWrapper) Current() *httpclient.StreamEvent {
	return s.current
}

func (s *simpleStreamWrapper) Err() error {
	return s.err
}

func (s *simpleStreamWrapper) Close() error {
	return s.body.Close()
}

// transformRequest transforms the HTTP request for Bedrock API compatibility.
func (e *Executor) transformRequest(request *httpclient.Request) (*httpclient.Request, error) {
	// Create a copy of the request
	transformed := &httpclient.Request{
		Method:    request.Method,
		URL:       request.URL,
		Headers:   make(http.Header),
		Body:      request.Body,
		Auth:      request.Auth,
		RequestID: request.RequestID,
	}

	// Copy headers
	for key, values := range request.Headers {
		for _, value := range values {
			transformed.Headers.Add(key, value)
		}
	}

	// Process request body for Bedrock compatibility
	body := request.Body
	if body != nil {
		// Add anthropic_version if not present
		if !gjson.GetBytes(body, "anthropic_version").Exists() {
			body, _ = sjson.SetBytes(body, "anthropic_version", DefaultVersion)
		}

		// Transform URL for Bedrock if it's a default endpoint
		if DefaultEndpoints[transformed.URL] || transformed.URL == "/v1/messages" {
			model := gjson.GetBytes(body, "model").String()
			stream := gjson.GetBytes(body, "stream").Bool()

			body, _ = sjson.DeleteBytes(body, "model")
			body, _ = sjson.DeleteBytes(body, "stream")

			// Update URL to use Bedrock endpoint format
			if stream {
				transformed.URL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke-with-response-stream", e.region, model)
			} else {
				transformed.URL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke", e.region, model)
			}
		}
	}

	// Set appropriate headers for Bedrock
	transformed.Headers.Set("Content-Type", "application/json")

	if transformed.Headers.Get("Accept") == "" {
		transformed.Headers.Set("Accept", "application/json")
	}

	// For streaming requests, set the appropriate Accept header
	if body != nil && gjson.GetBytes(body, "stream").Bool() {
		transformed.Headers.Set("Accept", "application/vnd.amazon.eventstream")
	}

	return transformed, nil
}

// decoderStreamWrapper wraps a StreamDecoder to implement streams.Stream.
type decoderStreamWrapper struct {
	decoder httpclient.StreamDecoder
}

// Next advances to the next event in the stream.
func (d *decoderStreamWrapper) Next() bool {
	return d.decoder.Next()
}

// Current returns the current event.
func (d *decoderStreamWrapper) Current() *httpclient.StreamEvent {
	return d.decoder.Current()
}

// Err returns any error that occurred during streaming.
func (d *decoderStreamWrapper) Err() error {
	return d.decoder.Err()
}

// Close closes the stream and releases resources.
func (d *decoderStreamWrapper) Close() error {
	return d.decoder.Close()
}

// Ensure Executor implements the pipeline.Executor interface.
var _ pipeline.Executor = (*Executor)(nil)
