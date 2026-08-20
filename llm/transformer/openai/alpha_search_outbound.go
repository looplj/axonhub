package openai

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tidwall/sjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

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
		Method:      http.MethodPost,
		URL:         t.buildAlphaSearchURL(),
		Headers:     http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}},
		Body:        body,
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: apiKey},
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
