package deepseek

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

type OutboundTransformer struct {
	transformer.Outbound

	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
	completion     transformer.Outbound
}

func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid DeepSeek transformer configuration: %w", err)
	}

	baseURL := transformer.NormalizeBaseURL(config.BaseURL, "v1")

	completionT, err := openai.NewCompletionOutboundTransformer(&openai.Config{
		BaseURL:        strings.TrimSuffix(baseURL, "/v1") + "/beta#",
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid DeepSeek completion transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		Outbound:       t,
		completion:     completionT,
	}, nil
}

type Request struct {
	openai.Request

	Thinking *Thinking `json:"thinking,omitempty"`
}

type Thinking struct {
	Type string `json:"type"`
}

func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	//nolint:exhaustive // Checked.
	switch llmReq.RequestType {
	case llm.RequestTypeChat, "":
		// continue
	case llm.RequestTypeCompletion:
		return t.completion.TransformRequest(ctx, llmReq)
	case llm.RequestTypeCompact:
		return nil, fmt.Errorf("%w: compact is only supported by OpenAI Responses API", transformer.ErrInvalidRequest)
	default:
		return nil, fmt.Errorf("%w: %s is not supported", transformer.ErrInvalidRequest, llmReq.RequestType)
	}

	if len(llmReq.Messages) == 0 {
		return nil, fmt.Errorf("%w: messages are required", transformer.ErrInvalidRequest)
	}

	oaiReq := openai.RequestFromLLM(llmReq)

	if oaiReq.ResponseFormat != nil && oaiReq.ResponseFormat.Type == "json_schema" {
		oaiReq.ResponseFormat.Type = "json_object"
		oaiReq.ResponseFormat.JSONSchema = nil
	}

	dsReq := Request{
		Request: *oaiReq,
	}

	thinkingDisabled := llmReq.ReasoningEffort == "none"

	dsReq.Thinking = &Thinking{
		Type: "enabled",
	}
	if thinkingDisabled {
		dsReq.Thinking.Type = "disabled"
	}

	if !thinkingDisabled {
		for i := range dsReq.Messages {
			if dsReq.Messages[i].Role == "assistant" && dsReq.Messages[i].ReasoningContent == nil {
				dsReq.Messages[i].ReasoningContent = lo.ToPtr("")
			}
		}
	}

	body, err := json.Marshal(dsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")

	apiKey := t.APIKeyProvider.Get(ctx)

	auth := &httpclient.AuthConfig{
		Type:   "bearer",
		APIKey: apiKey,
	}

	url := t.BaseURL + "/chat/completions"

	return &httpclient.Request{
		Method:    http.MethodPost,
		URL:       url,
		Headers:   headers,
		Body:      body,
		Auth:      auth,
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
	}, nil
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	if req.RequestType == string(llm.RequestTypeCompletion) {
		return t.completion.TransformStream(ctx, req, stream)
	}

	return t.Outbound.TransformStream(ctx, req, stream)
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp.Request != nil && httpResp.Request.RequestType == string(llm.RequestTypeCompletion) {
		return t.completion.TransformResponse(ctx, httpResp)
	}

	return t.Outbound.TransformResponse(ctx, httpResp)
}

func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return t.Outbound.TransformError(ctx, rawErr)
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, req *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	if req.RequestType == string(llm.RequestTypeCompletion) {
		return t.completion.AggregateStreamChunks(ctx, req, chunks)
	}

	return t.Outbound.AggregateStreamChunks(ctx, req, chunks)
}
