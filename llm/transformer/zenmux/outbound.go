package zenmux

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

var ErrVideoTaskDeleteUnsupported = errors.New("zenmux video task deletion is unsupported")

type Config struct {
	BaseURL        string
	EndpointPath   string
	APIKeyProvider auth.APIKeyProvider
}

type OutboundTransformer struct {
	transformer.Outbound
	baseURL        string
	videoPath      string
	apiKeyProvider auth.APIKeyProvider
}

func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	})
}

func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, errors.New("config is nil")
	}
	if config.APIKeyProvider == nil {
		return nil, errors.New("API key provider is required")
	}
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, errors.New("base URL is required")
	}

	baseURL := strings.TrimRight(config.BaseURL, "/")
	endpointPath := strings.TrimSpace(config.EndpointPath)
	if endpointPath == "" {
		endpointPath = "/videos"
	}
	endpointPath = "/" + strings.Trim(endpointPath, "/")
	chatTransformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        baseURL,
		EndpointPath:   "/chat/completions",
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ZenMux chat transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound:       chatTransformer,
		baseURL:        baseURL,
		videoPath:      endpointPath,
		apiKeyProvider: config.APIKeyProvider,
	}, nil
}

func (t *OutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatZenmuxVideo
}

func (t *OutboundTransformer) TransformRequest(ctx context.Context, request *llm.Request) (*httpclient.Request, error) {
	if request == nil || request.RequestType != llm.RequestTypeVideo {
		return t.Outbound.TransformRequest(ctx, request)
	}

	return t.buildVideoRequest(ctx, request)
}

func (t *OutboundTransformer) TransformResponse(ctx context.Context, response *httpclient.Response) (*llm.Response, error) {
	if response != nil && response.Request != nil && response.Request.APIFormat == llm.APIFormatZenmuxVideo.String() {
		return t.parseCreateResponse(response)
	}

	return t.Outbound.TransformResponse(ctx, response)
}

func (t *OutboundTransformer) TransformStream(ctx context.Context, request *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return t.Outbound.TransformStream(ctx, request, stream)
}

func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, request *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return t.Outbound.AggregateStreamChunks(ctx, request, chunks)
}

var _ transformer.Outbound = (*OutboundTransformer)(nil)
var _ transformer.VideoTaskOutbound = (*OutboundTransformer)(nil)
