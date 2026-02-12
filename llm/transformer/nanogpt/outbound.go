package nanogpt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// Config holds all configuration for the NanoGPT outbound transformer.
type Config struct {
	BaseURL        string              `json:"base_url,omitempty"`
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// OutboundTransformer implements transformer.Outbound for NanoGPT format.
// NanoGPT is compatible with OpenAI API, but handles the reasoning field differently.
// It embeds the OpenAI transformer and overrides response handling.
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformer creates a new NanoGPT OutboundTransformer with legacy parameters.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	config := &Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	}

	return NewOutboundTransformerWithConfig(config)
}

// NewOutboundTransformerWithConfig creates a new NanoGPT OutboundTransformer with unified configuration.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiConfig := &openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	}

	t, err := openai.NewOutboundTransformerWithConfig(oaiConfig)
	if err != nil {
		return nil, fmt.Errorf("invalid NanoGPT transformer configuration: %w", err)
	}

	return &OutboundTransformer{
		Outbound: t,
	}, nil
}

// TransformResponse transforms the HTTP response to llm.Response.
func (t *OutboundTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	if httpResp == nil {
		return nil, fmt.Errorf("http response is nil")
	}

	if httpResp.StatusCode >= 400 {
		// Read response body for diagnostic details
		body := string(httpResp.Body)
		var errResp struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(httpResp.Body, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, errResp.Error)
		}
		// Fallback to raw body if JSON parse fails or no error field
		if len(body) > 0 {
			return nil, fmt.Errorf("HTTP error %d: %s", httpResp.StatusCode, body)
		}
		return nil, fmt.Errorf("HTTP error %d", httpResp.StatusCode)
	}

	if len(httpResp.Body) == 0 {
		return nil, fmt.Errorf("response body is empty")
	}

	var nanoResp Response

	err := json.Unmarshal(httpResp.Body, &nanoResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal chat completion response: %w", err)
	}

	return nanoResp.ToOpenAIResponse().ToLLMResponse(), nil
}

// TransformStream transforms a stream of HTTP events to a stream of llm.Response.
func (t *OutboundTransformer) TransformStream(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	// Filter out upstream DONE events
	filteredStream := streams.Filter(stream, func(event *httpclient.StreamEvent) bool {
		return !bytes.HasPrefix(event.Data, []byte("[DONE]"))
	})

	// Transform remaining events
	transformedStream := streams.MapErr(filteredStream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	})

	// Always append our own DONE event at the end
	return streams.AppendStream(transformedStream, llm.DoneResponse), nil
}

// TransformStreamChunk transforms a single stream event to llm.Response.
func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
	ep := gjson.GetBytes(event.Data, "error")
	if ep.Exists() {
		return nil, &llm.ResponseError{
			Detail: llm.ErrorDetail{
				Message: ep.String(),
			},
		}
	}

	httpResp := &httpclient.Response{
		Body: event.Data,
	}

	return t.TransformResponse(ctx, httpResp)
}

// nanoGPTChunkTransform is a NanoGPT-specific chunk transformer that preserves
// the reasoning field by unmarshaling into nanogpt.Response first.
func nanoGPTChunkTransform(ctx context.Context, chunk *httpclient.StreamEvent) (*openai.Response, error) {
	var nanoResp Response
	if err := json.Unmarshal(chunk.Data, &nanoResp); err != nil {
		return nil, err
	}

	return nanoResp.ToOpenAIResponse(), nil
}

// AggregateStreamChunks aggregates stream chunks into a single response.
func (t *OutboundTransformer) AggregateStreamChunks(ctx context.Context, chunks []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return openai.AggregateStreamChunks(ctx, chunks, nanoGPTChunkTransform)
}
