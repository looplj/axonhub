package jina

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// RerankError represents an error response from the rerank API.
type RerankError struct {
	StatusCode int
	Message    string
}

func (e *RerankError) Error() string {
	return fmt.Sprintf("rerank error (status %d): %s", e.StatusCode, e.Message)
}

// Config holds configuration for Jina transformer.
type Config struct {
	BaseURL string `json:"base_url,omitempty"`
	APIKey  string `json:"api_key,omitempty"`
}

// RerankOutboundTransformer implements the outbound transformer for Jina Rerank API.
type RerankOutboundTransformer struct {
	config *Config
}

// NewRerankOutboundTransformer creates a new RerankOutboundTransformer.
func NewRerankOutboundTransformer(baseURL, apiKey string) (*RerankOutboundTransformer, error) {
	config := &Config{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		APIKey:  apiKey,
	}

	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &RerankOutboundTransformer{
		config: config,
	}, nil
}

// NewRerankOutboundTransformerWithConfig creates a transformer with the given config.
func NewRerankOutboundTransformerWithConfig(config *Config) (*RerankOutboundTransformer, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	config.BaseURL = strings.TrimSuffix(config.BaseURL, "/")

	return &RerankOutboundTransformer{
		config: config,
	}, nil
}

func validateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}

	if config.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	return nil
}

func (t *RerankOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatJinaRerank
}

// TransformRequest transforms unified llm.Request to HTTP rerank request.
func (t *RerankOutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeRerank:
		// continue
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	// Extract rerank request from the unified request
	if llmReq.Rerank == nil {
		return nil, fmt.Errorf("rerank request is nil in llm.Request")
	}

	rerankReq := llmReq.Rerank

	// Validate required fields
	if rerankReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if rerankReq.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if len(rerankReq.Documents) == 0 {
		return nil, fmt.Errorf("documents are required")
	}

	// Marshal request body
	body, err := json.Marshal(rerankReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank request: %w", err)
	}

	// Prepare headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("Authorization", "Bearer "+t.config.APIKey)

	// Build URL
	url := t.buildRerankURL()

	httpReq := &httpclient.Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		Body:    body,
		Auth: &httpclient.AuthConfig{
			Type:   "bearer",
			APIKey: t.config.APIKey,
		},
	}

	// Set metadata for response routing
	if httpReq.TransformerMetadata == nil {
		httpReq.TransformerMetadata = make(map[string]any)
	}

	httpReq.TransformerMetadata["outbound_format_type"] = llm.APIFormatJinaRerank.String()

	return httpReq, nil
}

// buildRerankURL constructs the rerank API URL.
func (t *RerankOutboundTransformer) buildRerankURL() string {
	if strings.HasSuffix(t.config.BaseURL, "/v1") {
		return t.config.BaseURL + "/rerank"
	}

	return t.config.BaseURL + "/v1/rerank"
}

// TransformResponse transforms HTTP rerank response to unified llm.Response.
func (t *RerankOutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	// Check HTTP status codes
	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
		})
	}

	// Check for empty response body
	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	// Unmarshal rerank response
	var rerankResp llm.RerankResponse
	if err := json.Unmarshal(httpResp.Body, &rerankResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rerank response: %w", err)
	}

	// Build unified response - only assign the needed fields
	llmResp := &llm.Response{
		RequestType: llm.RequestTypeRerank,
		APIFormat:   llm.APIFormatJinaRerank,
		Rerank:      &rerankResp,
	}

	// Map usage if available
	if rerankResp.Usage != nil {
		llmResp.Usage = &llm.Usage{
			PromptTokens:     int64(rerankResp.Usage.PromptTokens),
			CompletionTokens: 0,
			TotalTokens:      int64(rerankResp.Usage.TotalTokens),
		}
	}

	return llmResp, nil
}

// TransformStream - Rerank doesn't support streaming.
func (t *RerankOutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("rerank does not support streaming")
}

// AggregateStreamChunks - Rerank doesn't support streaming.
func (t *RerankOutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("rerank does not support streaming")
}

// TransformError transforms HTTP error response to unified error response.
func (t *RerankOutboundTransformer) TransformError(
	ctx context.Context,
	httpErr *httpclient.Error,
) *llm.ResponseError {
	if httpErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	// Try to parse Jina error format
	var jinaError struct {
		Error llm.ErrorDetail `json:"error"`
	}

	err := json.Unmarshal(httpErr.Body, &jinaError)
	if err == nil && jinaError.Error.Message != "" {
		return &llm.ResponseError{
			StatusCode: httpErr.StatusCode,
			Detail:     jinaError.Error,
		}
	}

	// If JSON parsing fails, use upstream status text
	return &llm.ResponseError{
		StatusCode: httpErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(httpErr.StatusCode),
			Type:    "api_error",
		},
	}
}
