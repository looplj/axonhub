package decisions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/tidwall/sjson"
)

type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
	EndpointPath   string              `json:"endpoint_path,omitempty"`
}

type OutboundTransformer struct {
	config *Config
	rawURL bool
}

func NewOutboundTransformer(baseURL, apiKey string) (*OutboundTransformer, error) {
	return NewOutboundTransformerWithConfig(&Config{BaseURL: baseURL, APIKeyProvider: auth.NewStaticKeyProvider(apiKey)})
}

func NewOutboundTransformerWithConfig(config *Config) (*OutboundTransformer, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}
	if config.APIKeyProvider == nil {
		return nil, errors.New("API key provider is required")
	}
	if config.BaseURL == "" {
		return nil, errors.New("base URL is required")
	}
	rawURL := strings.HasSuffix(config.BaseURL, "##")
	if rawURL {
		config.BaseURL = strings.TrimSuffix(config.BaseURL, "##")
	} else if config.EndpointPath != "" {
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "")
	} else {
		config.BaseURL = transformer.NormalizeBaseURL(config.BaseURL, "v1")
	}
	return &OutboundTransformer{config: config, rawURL: rawURL}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIDecisions
}

func (t *OutboundTransformer) AllowPassThroughBody(context.Context, *llm.Request, *httpclient.Request) bool {
	return true
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request == nil || request.Decisions == nil || len(request.Decisions.Body) == 0 {
		return nil, fmt.Errorf("decisions request is nil")
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	body, err := sjson.SetBytes(request.Decisions.Body, "model", request.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to patch decisions model: %w", err)
	}
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	return &httpclient.Request{
		Method:      http.MethodPost,
		URL:         t.buildURL(),
		Headers:     headers,
		Body:        body,
		Auth:        &httpclient.AuthConfig{Type: httpclient.AuthTypeBearer, APIKey: t.config.APIKeyProvider.Get(ctx)},
		RequestType: llm.RequestTypeDecisions.String(),
		APIFormat:   llm.APIFormatOpenAIDecisions.String(),
	}, nil
}

func (t *OutboundTransformer) buildURL() string {
	if t.rawURL {
		return strings.TrimRight(t.config.BaseURL, "/")
	}
	if t.config.EndpointPath != "" {
		return t.config.BaseURL + t.config.EndpointPath
	}
	return t.config.BaseURL + "/decisions"
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	if response == nil {
		return nil, fmt.Errorf("http response is nil")
	}
	if response.StatusCode >= http.StatusMultipleChoices {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: response.StatusCode,
			Body:       response.Body,
			Headers:    response.Headers,
		})
	}
	if len(response.Body) == 0 {
		return &llm.Response{
			RequestType: llm.RequestTypeDecisions,
			APIFormat:   llm.APIFormatOpenAIDecisions,
			Decisions:   &llm.DecisionsResponse{},
		}, nil
	}
	if !json.Valid(response.Body) {
		return nil, fmt.Errorf("%w: invalid decisions response body", transformer.ErrInvalidResponse)
	}
	var wire struct {
		Model   string            `json:"model"`
		Answers []json.RawMessage `json:"answers"`
		Usage   *struct {
			InputTokens int64 `json:"input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(response.Body, &wire); err != nil {
		return nil, fmt.Errorf("%w: failed to decode decisions response: %w", transformer.ErrInvalidResponse, err)
	}
	parsed := &llm.DecisionsResponse{
		Body:    append([]byte(nil), response.Body...),
		Model:   wire.Model,
		Answers: append([]json.RawMessage(nil), wire.Answers...),
	}
	if wire.Usage != nil {
		parsed.Usage = &llm.DecisionsUsage{InputTokens: wire.Usage.InputTokens}
	}
	result := &llm.Response{
		Model:       wire.Model,
		RequestType: llm.RequestTypeDecisions,
		APIFormat:   llm.APIFormatOpenAIDecisions,
		Decisions:   parsed,
	}
	if parsed.Usage != nil {
		result.Usage = &llm.Usage{PromptTokens: parsed.Usage.InputTokens}
	}
	return result, nil
}

func (t *OutboundTransformer) TransformStream(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("%w: decisions does not support streaming", transformer.ErrInvalidRequest)
}

func (t *OutboundTransformer) AggregateStreamChunks(context.Context, *httpclient.Request, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("decisions does not support streaming")
}

func (t *OutboundTransformer) TransformError(_ context.Context, httpErr *httpclient.Error) *llm.ResponseError {
	if httpErr == nil {
		return &llm.ResponseError{StatusCode: http.StatusInternalServerError, Detail: llm.ErrorDetail{Message: http.StatusText(http.StatusInternalServerError), Type: "api_error"}}
	}
	var wire struct {
		Error llm.ErrorDetail `json:"error"`
	}
	_ = json.Unmarshal(httpErr.Body, &wire)
	if wire.Error.Message == "" {
		wire.Error.Message = string(bytes.TrimSpace(httpErr.Body))
	}
	if wire.Error.Type == "" {
		wire.Error.Type = "api_error"
	}
	return &llm.ResponseError{StatusCode: httpErr.StatusCode, Detail: wire.Error, Cause: httpErr}
}

var _ transformer.Outbound = (*OutboundTransformer)(nil)
