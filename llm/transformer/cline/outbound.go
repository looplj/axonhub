package cline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the Cline outbound transformer.
type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	EndpointPath   string              `json:"endpoint_path,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// OutboundTransformer implements transformer.Outbound for Cline.
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformer creates a new Cline OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	})
}

// NewOutboundTransformerWithConfig creates a new Cline OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		EndpointPath:   config.EndpointPath,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldReasoning,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid Cline transformer configuration: %w", err)
	}

	return &OutboundTransformer{Outbound: t}, nil
}

// TransformResponse transforms the HTTP response to llm.Response.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	if httpResp.StatusCode >= 400 {
		return nil, t.TransformError(ctx, &httpclient.Error{
			StatusCode: httpResp.StatusCode,
			Body:       httpResp.Body,
			Headers:    httpResp.Headers,
		})
	}

	var wrapped struct {
		Success *bool    `json:"success"`
		Data    Response `json:"data"`
		Error   any      `json:"error"`
		Errors  any      `json:"errors"`
	}
	if err := json.Unmarshal(httpResp.Body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Cline response: %w", err)
	}

	if wrapped.Success != nil {
		if !*wrapped.Success {
			if wrapped.Error != nil {
				return nil, clineResponseError(http.StatusBadGateway, wrapped.Error)
			}
			if wrapped.Errors != nil {
				return nil, clineResponseError(http.StatusBadGateway, wrapped.Errors)
			}

			return nil, clineResponseError(http.StatusBadGateway, "Cline request failed")
		}

		return wrapped.Data.ToOpenAIResponse().ToLLMResponse(), nil
	}

	var resp Response
	if err := json.Unmarshal(httpResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return resp.ToOpenAIResponse().ToLLMResponse(), nil
}

// TransformStream transforms a stream of HTTP events to a stream of llm.Response.
func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	}), nil
}

// TransformStreamChunk transforms a single stream event to llm.Response.
func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
	if event == nil {
		return nil, fmt.Errorf("stream event is nil")
	}
	if bytes.HasPrefix(event.Data, []byte("[DONE]")) {
		return llm.DoneResponse, nil
	}
	if streamErr := parseClineErrorEvent(event); streamErr != nil {
		return nil, streamErr
	}

	return t.TransformResponse(ctx, &httpclient.Response{Body: event.Data})
}

// TransformError transforms HTTP error response to unified error response.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	_ = ctx

	if rawErr == nil {
		return clineResponseError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	}

	return parseClineError(rawErr.StatusCode, rawErr.Body)
}

func clineChunkTransform(ctx context.Context, chunk *httpclient.StreamEvent) (*openai.Response, error) {
	_ = ctx

	if streamErr := parseClineErrorEvent(chunk); streamErr != nil {
		return nil, streamErr
	}

	var resp Response
	if err := json.Unmarshal(chunk.Data, &resp); err != nil {
		return nil, err
	}

	return resp.ToOpenAIResponse(), nil
}

// AggregateStreamChunks aggregates stream chunks into a single response.
func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, req *httpclient.Request, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return openai.AggregateStreamChunks(ctx, chunks, clineChunkTransform)
}
