package jina

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// RerankInboundTransformer implements the inbound transformer for Jina Rerank API.
type RerankInboundTransformer struct{}

// NewRerankInboundTransformer creates a new RerankInboundTransformer.
func NewRerankInboundTransformer() *RerankInboundTransformer {
	return &RerankInboundTransformer{}
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

	// vLLM extends the Jina text fields with scoring content objects.
	var jinaReq struct {
		RerankRequest
		Query     json.RawMessage `json:"query"`
		Documents json.RawMessage `json:"documents"`
	}
	if err := json.Unmarshal(httpReq.Body, &jinaReq); err != nil {
		return nil, fmt.Errorf("%w: failed to decode rerank request: %w", transformer.ErrInvalidRequest, err)
	}

	// Validate required fields
	if jinaReq.Model == "" {
		return nil, fmt.Errorf("%w: model is required", transformer.ErrInvalidRequest)
	}

	query, queryIsObject, err := decodeRerankInput(jinaReq.Query)
	if err != nil || (!queryIsObject && query == "") {
		return nil, fmt.Errorf("%w: query must be a nonempty string or a content object", transformer.ErrInvalidRequest)
	}

	var documents []json.RawMessage
	if len(jinaReq.Documents) > 0 && jinaReq.Documents[0] == '[' {
		if err := json.Unmarshal(jinaReq.Documents, &documents); err != nil {
			return nil, fmt.Errorf("%w: failed to decode documents: %w", transformer.ErrInvalidRequest, err)
		}
	} else if len(jinaReq.Documents) > 0 {
		documents = []json.RawMessage{jinaReq.Documents}
	}
	if len(documents) == 0 {
		return nil, fmt.Errorf("%w: documents are required", transformer.ErrInvalidRequest)
	}

	native := queryIsObject || jinaReq.Documents[0] != '['
	texts := make([]string, len(documents))
	for i, document := range documents {
		text, isObject, err := decodeRerankInput(document)
		if err != nil {
			return nil, fmt.Errorf("%w: documents[%d] must be a string or a content object", transformer.ErrInvalidRequest, i)
		}
		texts[i] = text
		native = native || isObject
	}

	llmRerankReq := &llm.RerankRequest{
		Query:           query,
		Documents:       texts,
		TopN:            jinaReq.TopN,
		ReturnDocuments: jinaReq.ReturnDocuments,
	}
	if native {
		if err := json.Unmarshal(httpReq.Body, &llmRerankReq.NativeBody); err != nil {
			return nil, fmt.Errorf("%w: failed to decode native rerank body: %w", transformer.ErrInvalidRequest, err)
		}
		llmRerankReq.NativeBody["query"] = jinaReq.Query
		llmRerankReq.NativeBody["documents"] = jinaReq.Documents
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

// decodeRerankInput checks the scoring input envelope. The upstream validates
// the content parts supported by the selected model.
func decodeRerankInput(raw json.RawMessage) (text string, isObject bool, err error) {
	if len(raw) > 0 {
		switch raw[0] {
		case '"':
			err = json.Unmarshal(raw, &text)
			return text, false, err
		case '{':
			var input struct {
				Content []struct {
					Type string `json:"type"`
				} `json:"content"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", false, err
			}
			if len(input.Content) == 0 {
				return "", false, errors.New("content is required")
			}
			for i, part := range input.Content {
				if part.Type == "" {
					return "", false, fmt.Errorf("content[%d].type is required", i)
				}
			}
			return "", true, nil
		}
	}
	return "", false, errors.New("expected a string or a content object")
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
		ID:      llmResp.ID,
		Model:   llmResp.Model,
		Object:  llmRerankResp.Object,
		Results: make([]RerankResult, len(llmRerankResp.Results)),
	}

	// Convert results
	for i, result := range llmRerankResp.Results {
		jinaResp.Results[i] = RerankResult{
			Index:          result.Index,
			RelevanceScore: result.RelevanceScore,
			Document:       result.Document,
		}
	}

	// Convert usage if available
	if llmResp.Usage != nil {
		jinaResp.Usage = &RerankUsage{
			PromptTokens: int(llmResp.Usage.PromptTokens),
			TotalTokens:  int(llmResp.Usage.TotalTokens),
			Cost:         llmResp.Usage.Cost,
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
	if !errors.As(err, &respErr) && errors.Is(err, transformer.ErrInvalidRequest) {
		respErr = &llm.ResponseError{
			StatusCode: http.StatusBadRequest,
			Detail:     llm.ErrorDetail{Type: "invalid_request_error", Message: err.Error()},
		}
	}
	if respErr != nil {
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
