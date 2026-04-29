package openai

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

func validateImageOutboundRequest(
	llmReq *llm.Request,
	apiFormat llm.APIFormat,
) error {
	if llmReq == nil {
		return fmt.Errorf("%w: llm request is nil", transformer.ErrInvalidRequest)
	}

	llmReq.APIFormat = apiFormat

	return nil
}

func newImageOutboundConfig(config *Config) (*Config, error) {
	if _, err := NewOutboundTransformerWithConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func transformImageOutboundError(config *Config, ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	base := &OutboundTransformer{config: config}
	return base.TransformError(ctx, rawErr)
}

func transformImageOutboundResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	return transformImageGenerationResponse(httpResp)
}

func transformImageOutboundStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, fmt.Errorf("image generation does not support streaming")
}

func aggregateImageOutboundStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, fmt.Errorf("image generation does not support streaming")
}

type ImageGenerationOutboundTransformer struct {
	config *Config
}

func NewImageGenerationOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	cfg, err := newImageOutboundConfig(config)
	if err != nil {
		return nil, err
	}

	return &ImageGenerationOutboundTransformer{config: cfg}, nil
}

func (t *ImageGenerationOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIImageGeneration
}

func (t *ImageGenerationOutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if err := validateImageOutboundRequest(llmReq, llm.APIFormatOpenAIImageGeneration); err != nil {
		return nil, err
	}

	return buildImageGenerationAPIRequest(ctx, t.config, llmReq)
}

func (t *ImageGenerationOutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	return transformImageOutboundResponse(ctx, httpResp)
}

func (t *ImageGenerationOutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return transformImageOutboundError(t.config, ctx, rawErr)
}

func (t *ImageGenerationOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return transformImageOutboundStream(ctx, stream)
}

func (t *ImageGenerationOutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return aggregateImageOutboundStreamChunks(ctx, chunks)
}

type ImageEditOutboundTransformer struct {
	config *Config
}

func NewImageEditOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	cfg, err := newImageOutboundConfig(config)
	if err != nil {
		return nil, err
	}

	return &ImageEditOutboundTransformer{config: cfg}, nil
}

func (t *ImageEditOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIImageEdit
}

func (t *ImageEditOutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if err := validateImageOutboundRequest(llmReq, llm.APIFormatOpenAIImageEdit); err != nil {
		return nil, err
	}

	return buildImageGenerationAPIRequest(ctx, t.config, llmReq)
}

func (t *ImageEditOutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	return transformImageOutboundResponse(ctx, httpResp)
}

func (t *ImageEditOutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return transformImageOutboundError(t.config, ctx, rawErr)
}

func (t *ImageEditOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return transformImageOutboundStream(ctx, stream)
}

func (t *ImageEditOutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return aggregateImageOutboundStreamChunks(ctx, chunks)
}

type ImageVariationOutboundTransformer struct {
	config *Config
}

func NewImageVariationOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	cfg, err := newImageOutboundConfig(config)
	if err != nil {
		return nil, err
	}

	return &ImageVariationOutboundTransformer{config: cfg}, nil
}

func (t *ImageVariationOutboundTransformer) APIFormat() llm.APIFormat {
	return llm.APIFormatOpenAIImageVariation
}

func (t *ImageVariationOutboundTransformer) TransformRequest(ctx context.Context, llmReq *llm.Request) (*httpclient.Request, error) {
	if err := validateImageOutboundRequest(llmReq, llm.APIFormatOpenAIImageVariation); err != nil {
		return nil, err
	}

	return buildImageGenerationAPIRequest(ctx, t.config, llmReq)
}

func (t *ImageVariationOutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	return transformImageOutboundResponse(ctx, httpResp)
}

func (t *ImageVariationOutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	return transformImageOutboundError(t.config, ctx, rawErr)
}

func (t *ImageVariationOutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return transformImageOutboundStream(ctx, stream)
}

func (t *ImageVariationOutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return aggregateImageOutboundStreamChunks(ctx, chunks)
}
