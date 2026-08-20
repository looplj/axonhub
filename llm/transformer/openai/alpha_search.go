package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// AlphaSearchInboundTransformer accepts the Codex/CPA alpha search envelope
// while preserving all provider-specific fields for transparent forwarding.
type AlphaSearchInboundTransformer struct{}

func NewAlphaSearchInboundTransformer() *AlphaSearchInboundTransformer {
	return &AlphaSearchInboundTransformer{}
}

func (t *AlphaSearchInboundTransformer) TransformRequest(ctx context.Context, httpReq *httpclient.Request) (*llm.Request, error) {
	if httpReq == nil {
		return nil, fmt.Errorf("%w: http request is nil", transformer.ErrInvalidRequest)
	}
	if len(httpReq.Body) == 0 {
		return nil, fmt.Errorf("%w: request body is empty", transformer.ErrInvalidRequest)
	}
	contentType := httpReq.Headers.Get("Content-Type")
	if contentType != "" && !strings.Contains(strings.ToLower(contentType), "application/json") {
		return nil, fmt.Errorf("%w: unsupported content type: %s", transformer.ErrInvalidRequest, contentType)
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(httpReq.Body, &envelope); err != nil || envelope == nil {
		return nil, fmt.Errorf("%w: failed to decode alpha search request: %v", transformer.ErrInvalidRequest, err)
	}
	var model string
	if raw, ok := envelope["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, fmt.Errorf("%w: model is required for alpha search routing", transformer.ErrInvalidRequest)
	}

	return &llm.Request{
		Model:       model,
		Messages:    []llm.Message{},
		RawRequest:  httpReq,
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
		AlphaSearch: &llm.AlphaSearchRequest{Body: append([]byte(nil), httpReq.Body...)},
	}, nil
}

func (t *AlphaSearchInboundTransformer) TransformResponse(ctx context.Context, resp *llm.Response) (*httpclient.Response, error) {
	if resp == nil || resp.AlphaSearch == nil || len(resp.AlphaSearch.Body) == 0 {
		return nil, fmt.Errorf("alpha search response is empty")
	}
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       append([]byte(nil), resp.AlphaSearch.Body...),
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func (t *AlphaSearchInboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return nil, fmt.Errorf("%w: alpha search does not support streaming", transformer.ErrInvalidRequest)
}

func (t *AlphaSearchInboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("alpha search does not support streaming")
}

func (t *AlphaSearchInboundTransformer) TransformError(ctx context.Context, err error) *httpclient.Error {
	return NewInboundTransformer().TransformError(ctx, err)
}

// transformAlphaSearchRequest builds an OpenAI-compatible /alpha/search call.
func (t *OutboundTransformer) transformAlphaSearchRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if llmReq == nil || llmReq.AlphaSearch == nil || len(llmReq.AlphaSearch.Body) == 0 {
		return nil, fmt.Errorf("alpha search request is nil")
	}
	body, err := sjson.SetBytes(llmReq.AlphaSearch.Body, "model", llmReq.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to patch alpha search model: %w", err)
	}
	apiKey := t.config.APIKeyProvider.Get(ctx)
	return &httpclient.Request{
		Method: http.MethodPost,
		URL:    t.buildAlphaSearchURL(),
		Headers: http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}},
		Body: body,
		Auth: &httpclient.AuthConfig{Type: "bearer", APIKey: apiKey},
		RequestType: llm.RequestTypeAlphaSearch.String(),
		APIFormat:   llm.APIFormatOpenAIAlphaSearch.String(),
	}, nil
}

func (t *OutboundTransformer) buildAlphaSearchURL() string {
	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath
	}
	return t.config.BaseURL + "/alpha/search"
}

func (t *OutboundTransformer) transformAlphaSearchResponse(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
	if resp == nil {
		return nil, fmt.Errorf("http response is nil")
	}
	if resp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{StatusCode: resp.StatusCode, Body: resp.Body})
	}
	if len(resp.Body) == 0 {
		return nil, fmt.Errorf("alpha search response body is empty")
	}
	return &llm.Response{
		Model:       "",
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
		AlphaSearch: &llm.AlphaSearchResponse{Body: append([]byte(nil), resp.Body...)},
	}, nil
}
