package jina

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/transformer"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// RerankInboundTransformer implements the inbound transformer for Jina Rerank API.
type RerankInboundTransformer struct{}

// NewRerankInboundTransformer creates a new RerankInboundTransformer.
func NewRerankInboundTransformer() *RerankInboundTransformer {
	return &RerankInboundTransformer{}
}

func (t *RerankInboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatJinaRerank
}

// TransformRequest transforms HTTP rerank request to unified llm.Request.
func (t *RerankInboundTransformer) TransformRequest(
	ctx context.Context,
	httpReq *httpclient.Request,
) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	// Parse the jina rerank request
	var jinaReq RerankRequest
	if err := json.Unmarshal(httpReq.Body, &jinaReq); err != nil {
		return nil, fmt.Errorf("%w: failed to decode rerank request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if jinaReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	if jinaReq.Query == "" {
		return nil, fmt.Errorf("%w: query is required", transformer.ErrInvalidRequest)
	}

	if len(jinaReq.Documents) == 0 {
		return nil, fmt.Errorf("%w: documents are required", transformer.ErrInvalidRequest)
	}

	// Convert to unified llm.RerankRequest
	llmRerankReq := &llm.RerankRequest{
		Query:           jinaReq.Query,
		Documents:       jinaReq.Documents,
		TopN:            jinaReq.TopN,
		ReturnDocuments: jinaReq.ReturnDocuments,
	}

	// Build unified request
	llmReq := &llm.Request{
		Model:       jinaReq.Model,
		RequestType: llm.RequestTypeRerank,
		APIFormat:   llm.APIFormatJinaRerank,
		Rerank:      llmRerankReq,
		RawRequest:  httpReq,
	}

	return llmReq, nil
}

// TransformResponse transforms unified llm.Response to HTTP rerank response.
func (t *RerankInboundTransformer) TransformResponse(
	ctx context.Context,
	llmResp *llm.Response,
) (*httpclient.Response, error) {
	if llmResp == nil {
		return nil, fmt.Errorf("llm response is nil")
	}

	if llmResp.Rerank == nil {
		return nil, fmt.Errorf("rerank response is nil")
	}

	// Convert llm.RerankResponse to jina.RerankResponse
	llmRerankResp := llmResp.Rerank
	jinaResp := RerankResponse{
		Model:   llmResp.Model,
		Object:  llmRerankResp.Object,
		Results: make([]RerankResult, len(llmRerankResp.Results)),
	}

	// Convert results
	for i, result := range llmRerankResp.Results {
		var doc *RerankDocument
		if result.Document != nil {
			doc = &RerankDocument{
				Text: result.Document.Text,
			}
		}

		jinaResp.Results[i] = RerankResult{
			Index:          result.Index,
			RelevanceScore: result.RelevanceScore,
			Document:       doc,
		}
	}

	// Convert usage if available
	if llmRerankResp.Usage != nil {
		jinaResp.Usage = &RerankUsage{
			PromptTokens: llmRerankResp.Usage.PromptTokens,
			TotalTokens:  llmRerankResp.Usage.TotalTokens,
		}
	}

	// Marshal the jina rerank response
	body, err := json.Marshal(jinaResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rerank response: %w", err)
	}

	// Build HTTP response
	httpResp := &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: body,
	}

	return httpResp, nil
}

// TransformError transforms unified error response to HTTP error response.
func (t *RerankInboundTransformer) TransformError(
	ctx context.Context,
	err error,
) *httpclient.Error {
	if err == nil {
		return &httpclient.Error{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte(`{"error":{"message":"Internal server error","type":"api_error"}}`),
		}
	}

	// Try to extract ResponseError if possible
	var respErr *llm.ResponseError
	if errors.As(err, &respErr) {
		// Build error response body
		errorBody := map[string]any{
			"error": respErr.Detail,
		}

		body, marshalErr := json.Marshal(errorBody)
		if marshalErr != nil {
			return &httpclient.Error{
				StatusCode: http.StatusInternalServerError,
				Body:       []byte(`{"error":{"message":"Failed to marshal error","type":"api_error"}}`),
			}
		}

		return &httpclient.Error{
			StatusCode: respErr.StatusCode,
			Body:       body,
		}
	}

	// Generic error handling
	return &httpclient.Error{
		StatusCode: http.StatusInternalServerError,
		Body:       []byte(`{"error":{"message":"Internal server error","type":"api_error"}}`),
	}
}

// TransformStream - Rerank doesn't support streaming.
func (t *RerankInboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*llm.Response],
) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("rerank does not support streaming")
}

// AggregateStreamChunks - Rerank doesn't support streaming.
func (t *RerankInboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("rerank does not support streaming")
}
