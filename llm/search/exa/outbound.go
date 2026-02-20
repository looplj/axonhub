package exa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

type OutboundTransformer struct {
	config *Config
}

func NewOutboundTransformerWithConfig(config *Config) (*OutboundTransformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.APIKeyProvider == nil {
		return nil, fmt.Errorf("api key provider is required")
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}

	config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")

	return &OutboundTransformer{config: config}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatExaSearch
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil || llmReq.Search == nil {
		return nil, fmt.Errorf("%w: search request is nil", transformer.ErrInvalidRequest)
	}

	sreq := llmReq.Search
	if sreq.Query == "" {
		return nil, fmt.Errorf("%w: query is required", transformer.ErrInvalidRequest)
	}

	maxResults := 5
	if sreq.MaxResults != nil {
		maxResults = *sreq.MaxResults
	}

	bodyObj := SearchRequest{
		Query:          sreq.Query,
		NumResults:     maxResults,
		IncludeDomains: sreq.AllowedDomains,
		ExcludeDomains: sreq.BlockedDomains,
		Contents:       &RequestContents{Text: true},
	}

	body, err := json.Marshal(bodyObj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal exa search request: %w", err)
	}

	// Merge extra_body into provider request body.
	if len(sreq.ExtraBody) > 0 && string(sreq.ExtraBody) != "null" && string(sreq.ExtraBody) != "{}" {
		body = append([]byte(nil), body...)
		var extra any
		if err := json.Unmarshal(sreq.ExtraBody, &extra); err == nil {
			if m, ok := extra.(map[string]any); ok {
				for k, v := range m {
					body, _ = sjson.SetBytes(body, k, v)
				}
			}
		}
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	apiKey := t.config.APIKeyProvider.Get(ctx)

	req := &httpclient.Request{
		Method:      http.MethodPost,
		URL:         t.config.BaseURL + "/search",
		Headers:     headers,
		Body:        body,
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: apiKey},
		RequestType: llm.RequestTypeSearch.String(),
		APIFormat:   llm.APIFormatExaSearch.String(),
		TransformerMetadata: map[string]any{
			"query": sreq.Query,
		},
	}

	return req, nil
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("%w: http response is nil", transformer.ErrInvalidResponse)
	}

	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{StatusCode: httpResp.StatusCode, Body: httpResp.Body})
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("%w: response body is empty", transformer.ErrInvalidResponse)
	}

	var resp SearchResponse
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("%w: failed to decode exa response: %w", transformer.ErrInvalidResponse, err)
	}

	results := make([]llm.SearchResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		score := r.Score
		results = append(results, llm.SearchResult{
			Title:         r.Title,
			URL:           r.URL,
			Content:       r.Text,
			Score:         score,
			PublishedDate: r.PublishedDate,
			Favicon:       r.Favicon,
		})
	}

	var queryStr string
	if httpResp.Request != nil && httpResp.Request.TransformerMetadata != nil {
		if q, ok := httpResp.Request.TransformerMetadata["query"].(string); ok {
			queryStr = q
		}
	}

	llmResp := &llm.Response{
		RequestType: llm.RequestTypeSearch,
		APIFormat:   llm.APIFormatExaSearch,
		Search: &llm.SearchResponse{
			Query:        queryStr,
			Results:      results,
			ResponseTime: 0,
		},
	}

	llmResp.Usage = &llm.Usage{Quantity: 1}

	return llmResp, nil
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("exa search does not support streaming")
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("exa search does not support streaming")
}

func (t *OutboundTransformer) TransformError(ctx context.Context, httpErr *httpclient.Error) *llm.ResponseError {
	if httpErr == nil {
		return &llm.ResponseError{
			StatusCode: http.StatusInternalServerError,
			Detail: llm.ErrorDetail{
				Message: http.StatusText(http.StatusInternalServerError),
				Type:    "api_error",
			},
		}
	}

	// Exa typically returns JSON bodies; best-effort parse error.message if present.
	var upstream struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(httpErr.Body, &upstream); err == nil {
		msg := upstream.Message
		if msg == "" && upstream.Error.Message != "" {
			msg = upstream.Error.Message
		}
		if msg != "" {
			return &llm.ResponseError{
				StatusCode: httpErr.StatusCode,
				Detail: llm.ErrorDetail{
					Message: msg,
					Type:    "api_error",
				},
			}
		}
	}

	return &llm.ResponseError{
		StatusCode: httpErr.StatusCode,
		Detail: llm.ErrorDetail{
			Message: http.StatusText(httpErr.StatusCode),
			Type:    "api_error",
		},
	}
}
