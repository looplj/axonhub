package jina

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
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

// OutboundTransformer implements the outbound transformer for Jina APIs (Rerank and Embedding).
type OutboundTransformer struct {
	config *Config
}

// NewOutboundTransformer creates a new RerankOutboundTransformer.
func NewOutboundTransformer(baseURL, apiKey string) (*OutboundTransformer, error) {
	config := &Config{
		BaseURL: baseURL,
		APIKey:  apiKey,
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a transformer with the given config.
func NewOutboundTransformerWithConfig(config *Config) (*OutboundTransformer, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")

	return &OutboundTransformer{
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

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatJinaRerank // Primary format, routing handled in methods
}

// TransformRequest transforms unified llm.Request to HTTP request (rerank or embedding).
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq == nil {
		return nil, fmt.Errorf("llm request is nil")
	}

	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeRerank:
		return t.transformRerankRequest(ctx, llmReq)
	case llm.RequestTypeEmbedding:
		return t.transformEmbeddingRequest(ctx, llmReq)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}
}

// transformRerankRequest handles rerank request transformation.
func (t *OutboundTransformer) transformRerankRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	// Extract rerank request from the unified request
	if llmReq.Rerank == nil {
		return nil, fmt.Errorf("rerank request is nil in llm.Request")
	}

	rerankReq := llmReq.Rerank

	// Validate required fields
	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	if rerankReq.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if len(rerankReq.Documents) == 0 {
		return nil, fmt.Errorf("documents are required")
	}

	// Create Jina rerank request with model from top-level
	jinaRerankReq := RerankRequest{
		Model:           llmReq.Model,
		Query:           rerankReq.Query,
		Documents:       rerankReq.Documents,
		TopN:            rerankReq.TopN,
		ReturnDocuments: rerankReq.ReturnDocuments,
	}

	// Marshal request body
	body, err := json.Marshal(jinaRerankReq)
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

// transformEmbeddingRequest handles embedding request transformation.
func (t *OutboundTransformer) transformEmbeddingRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	if llmReq.Embedding == nil {
		return nil, fmt.Errorf("embedding request is nil in llm.Request")
	}

	embReq := llmReq.Embedding

	if llmReq.Model == "" {
		return nil, fmt.Errorf("model is required")
	}

	jinaEmbReq := EmbeddingRequest{
		Input:          embReq.Input,
		Model:          llmReq.Model,
		Task:           embReq.Task,
		EncodingFormat: embReq.EncodingFormat,
		Dimensions:     embReq.Dimensions,
		User:           embReq.User,
	}

	if jinaEmbReq.Task == "" {
		jinaEmbReq.Task = "text-matching"
	}

	body, err := json.Marshal(jinaEmbReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("Authorization", "Bearer "+t.config.APIKey)

	url := t.buildEmbeddingURL()

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

	if httpReq.TransformerMetadata == nil {
		httpReq.TransformerMetadata = make(map[string]any)
	}

	httpReq.TransformerMetadata["outbound_format_type"] = llm.APIFormatJinaEmbedding.String()

	return httpReq, nil
}

// buildRerankURL constructs the rerank API URL.
func (t *OutboundTransformer) buildRerankURL() string {
	return t.config.BaseURL + "/rerank"
}

// buildEmbeddingURL constructs the embedding API URL.
func (t *OutboundTransformer) buildEmbeddingURL() string {
	return t.config.BaseURL + "/embeddings"
}

// TransformResponse transforms HTTP response to unified llm.Response (rerank or embedding).
func (t *OutboundTransformer) TransformResponse(
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

	// Determine response type from metadata
	var apiFormat llm.APIFormat

	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if formatStr, ok := httpResp.Request.TransformerMetadata["outbound_format_type"].(string); ok {
			apiFormat = llm.APIFormat(formatStr)
		}
	}

	//nolint:exhaustive // Checked.
	switch apiFormat {
	case llm.APIFormatJinaEmbedding:
		return t.transformEmbeddingResponse(ctx, httpResp)
	case llm.APIFormatJinaRerank:
		fallthrough
	default:
		return t.transformRerankResponse(ctx, httpResp)
	}
}

// transformRerankResponse handles rerank response transformation.
func (t *OutboundTransformer) transformRerankResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	// Unmarshal into Jina rerank response
	var jinaResp RerankResponse
	if err := json.Unmarshal(httpResp.Body, &jinaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rerank response: %w", err)
	}

	// Convert to llm.RerankResponse (without Model field)
	llmRerankResp := llm.RerankResponse{
		Object:  jinaResp.Object,
		Results: make([]llm.RerankResult, len(jinaResp.Results)),
	}

	// Convert results
	for i, result := range jinaResp.Results {
		var doc *llm.RerankDocument
		if result.Document != nil {
			doc = &llm.RerankDocument{
				Text: result.Document.Text,
			}
		}

		llmRerankResp.Results[i] = llm.RerankResult{
			Index:          result.Index,
			RelevanceScore: result.RelevanceScore,
			Document:       doc,
		}
	}

	// Convert usage
	if jinaResp.Usage != nil {
		llmRerankResp.Usage = &llm.RerankUsage{
			PromptTokens: jinaResp.Usage.PromptTokens,
			TotalTokens:  jinaResp.Usage.TotalTokens,
		}
	}

	// Build unified response
	llmResp := &llm.Response{
		RequestType: llm.RequestTypeRerank,
		APIFormat:   llm.APIFormatJinaRerank,
		Rerank:      &llmRerankResp,
		Model:       jinaResp.Model,
	}

	// Map usage if available
	if jinaResp.Usage != nil {
		llmResp.Usage = &llm.Usage{
			PromptTokens:     int64(jinaResp.Usage.PromptTokens),
			CompletionTokens: 0,
			TotalTokens:      int64(jinaResp.Usage.TotalTokens),
		}
	}

	return llmResp, nil
}

// transformEmbeddingResponse handles embedding response transformation.
func (t *OutboundTransformer) transformEmbeddingResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	// Unmarshal into Jina embedding response
	var jinaResp EmbeddingResponse
	if err := json.Unmarshal(httpResp.Body, &jinaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedding response: %w", err)
	}

	// Convert to llm.EmbeddingResponse (without Model field)
	llmEmbeddingResp := llm.EmbeddingResponse{
		Object: jinaResp.Object,
		Data:   make([]llm.EmbeddingData, len(jinaResp.Data)),
	}

	// Convert data
	for i, data := range jinaResp.Data {
		llmEmbeddingResp.Data[i] = llm.EmbeddingData{
			Object:    data.Object,
			Embedding: data.Embedding,
			Index:     data.Index,
		}
	}

	// Convert usage
	if jinaResp.Usage.PromptTokens > 0 || jinaResp.Usage.TotalTokens > 0 {
		llmEmbeddingResp.Usage = &llm.EmbeddingUsage{
			PromptTokens: jinaResp.Usage.PromptTokens,
			TotalTokens:  jinaResp.Usage.TotalTokens,
		}
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeEmbedding,
		APIFormat:   llm.APIFormatJinaEmbedding,
		Embedding:   &llmEmbeddingResp,
		Model:       jinaResp.Model,
	}

	if jinaResp.Usage.PromptTokens > 0 || jinaResp.Usage.TotalTokens > 0 {
		llmResp.Usage = &llm.Usage{
			PromptTokens:     jinaResp.Usage.PromptTokens,
			CompletionTokens: 0,
			TotalTokens:      jinaResp.Usage.TotalTokens,
		}
	}

	return llmResp, nil
}

// TransformStream - Rerank doesn't support streaming.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("rerank does not support streaming")
}

// AggregateStreamChunks - Rerank doesn't support streaming.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("rerank does not support streaming")
}

// TransformError transforms HTTP error response to unified error response.
func (t *OutboundTransformer) TransformError(
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
