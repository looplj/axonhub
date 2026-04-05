package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

const (
	defaultSearchModel      = "__search"
	defaultSearchMaxResults = 10
	maxSearchMaxResults     = 20
)

type inboundRequest struct {
	Query          string          `json:"query"`
	Model          string          `json:"model"`
	MaxResults     *int            `json:"max_results,omitempty"`
	AllowedDomains []string        `json:"allowed_domains,omitempty"`
	BlockedDomains []string        `json:"blocked_domains,omitempty"`
	ExtraBody      json.RawMessage `json:"extra_body,omitempty"`
}

// InboundTransformer implements the inbound transformer for OpenAI-style /v1/search.
type InboundTransformer struct{}

func NewInboundTransformer() *InboundTransformer {
	return &InboundTransformer{}
}

func (t *InboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatAxonHubSearch
}

func (t *InboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}

	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}

	var raw inboundRequest
	if err := json.Unmarshal(httpReq.Body, &raw); err != nil {
		return nil, fmt.Errorf("%w: failed to decode search request: %w", transformer.ErrInvalidRequest, err)
	}

	if raw.Query == "" {
		return nil, fmt.Errorf("%w: query is required", transformer.ErrInvalidRequest)
	}

	model := raw.Model
	if model == "" {
		model = defaultSearchModel
	}

	maxResults := raw.MaxResults
	if maxResults == nil {
		v := defaultSearchMaxResults
		maxResults = &v
	}
	if *maxResults <= 0 || *maxResults > maxSearchMaxResults {
		return nil, fmt.Errorf("%w: max_results must be between 1 and %d", transformer.ErrInvalidRequest, maxSearchMaxResults)
	}

	llmReq := &llm.Request{
		Model:       model,
		Messages:    []llm.Message{}, // Search does not use messages
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeSearch,
		APIFormat:   llm.APIFormatTavilySearch,
		Stream:      nil,
		Search: &llm.SearchRequest{
			Query:          raw.Query,
			MaxResults:     maxResults,
			AllowedDomains: raw.AllowedDomains,
			BlockedDomains: raw.BlockedDomains,
			ExtraBody:      raw.ExtraBody,
		},
	}

	return llmReq, nil
}

func (t *InboundTransformer) TransformResponse(ctx context.Context, llmResp *llm.Response) (*httpclient.Response, error) {
	if llmResp == nil || llmResp.Search == nil {
		return nil, fmt.Errorf("%w: search response is nil", transformer.ErrInvalidResponse)
	}

	body, err := json.Marshal(llmResp.Search)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to encode search response: %w", transformer.ErrInvalidResponse, err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    headers,
		Body:       body,
	}, nil
}

func (t *InboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: search does not support streaming", transformer.ErrInvalidRequest)
}

func (t *InboundTransformer) TransformError(ctx context.Context, rawErr error) *httpclient.Error {
	chatInbound := openai.NewInboundTransformer()
	return chatInbound.TransformError(ctx, rawErr)
}

func (t *InboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("search does not support streaming")
}
