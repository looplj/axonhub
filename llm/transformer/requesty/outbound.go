// Package requesty adapts Requesty (https://requesty.ai) Chat Completions responses.
//
// Requests are plain OpenAI Chat Completions and are built by the OpenAI transformer.
// This package only handles the parts of Requesty responses that the generic OpenAI
// transformer drops:
//   - message.reasoning_signature / delta.reasoning_signature, which carries the signature
//     of the reasoning block (reasoning text itself arrives in reasoning_content).
//   - message.images, used for image output on Chat Completions (non-streaming).
//   - the error envelope {"error":{"origin":"router"|"provider","message":"..."}}.
package requesty

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// DefaultBaseURL is the global Requesty router endpoint.
const DefaultBaseURL = "https://router.requesty.ai/v1"

// Config holds the configuration for the Requesty outbound transformer.
type Config struct {
	BaseURL        string              `json:"base_url,omitempty"` // Defaults to DefaultBaseURL.
	APIKeyProvider auth.APIKeyProvider `json:"-"`
}

// OutboundTransformer wraps the OpenAI transformer and adds the Requesty specific response fields.
type OutboundTransformer struct {
	transformer.Outbound

	oai *openai.OutboundTransformer
}

// NewOutboundTransformer creates a Requesty OutboundTransformer with a static API key.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	})
}

// NewOutboundTransformerWithConfig creates a Requesty OutboundTransformer.
func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	if config == nil {
		return nil, errors.New("invalid Requesty transformer configuration: config cannot be nil")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	t, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        baseURL,
		APIKeyProvider: config.APIKeyProvider,
		ReasoningField: openai.ReasoningFieldContent,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Requesty transformer configuration: %w", err)
	}

	oai, ok := t.(*openai.OutboundTransformer)
	if !ok {
		return nil, fmt.Errorf("invalid Requesty transformer configuration: unexpected outbound type %T", t)
	}

	return &OutboundTransformer{Outbound: oai, oai: oai}, nil
}

// TransformResponse delegates to the OpenAI transformer and adds the Requesty fields on chat responses.
func (t *OutboundTransformer) TransformResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.Response, error) {
	resp, err := t.Outbound.TransformResponse(ctx, httpResp)
	if err != nil || resp == nil || !isChatCompletion(httpResp.Request) {
		return resp, err
	}

	applyMessageExtras(resp, httpResp.Body)

	return resp, nil
}

// TransformStream delegates to the OpenAI transformer per chunk so the Requesty fields are kept.
func (t *OutboundTransformer) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	if !isChatCompletion(req) {
		return t.Outbound.TransformStream(ctx, req, stream)
	}

	return streams.NoNil(streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.TransformStreamChunk(ctx, event)
	})), nil
}

// TransformStreamChunk transforms one Chat Completions stream event.
func (t *OutboundTransformer) TransformStreamChunk(ctx context.Context, event *httpclient.StreamEvent) (*llm.Response, error) {
	resp, err := t.oai.TransformStreamChunk(ctx, event)
	if err != nil {
		var respErr *llm.ResponseError
		if errors.As(err, &respErr) {
			applyErrorOrigin(&respErr.Detail, event.Data)
		}

		return nil, err
	}

	if resp == nil || resp == llm.DoneResponse {
		return resp, nil
	}

	applyMessageExtras(resp, event.Data)

	return resp, nil
}

// TransformError parses the Requesty error envelope.
func (t *OutboundTransformer) TransformError(ctx context.Context, rawErr *httpclient.Error) *llm.ResponseError {
	respErr := t.Outbound.TransformError(ctx, rawErr)
	if rawErr != nil && respErr != nil {
		applyErrorOrigin(&respErr.Detail, rawErr.Body)
	}

	return respErr
}

func isChatCompletion(req *httpclient.Request) bool {
	return req == nil || req.APIFormat == "" || req.APIFormat == string(llm.APIFormatOpenAIChatCompletion)
}

type messageExtras struct {
	ReasoningSignature string                      `json:"reasoning_signature"`
	Images             []openai.MessageContentPart `json:"images"`
}

type responseExtras struct {
	Choices []struct {
		Message *messageExtras `json:"message"`
		Delta   *messageExtras `json:"delta"`
	} `json:"choices"`
}

// applyMessageExtras copies reasoning_signature and images from the raw body onto the converted response.
// The OpenAI conversion keeps choices in body order, so they are matched by position.
func applyMessageExtras(resp *llm.Response, body []byte) {
	if !bytes.Contains(body, []byte(`"reasoning_signature"`)) && !bytes.Contains(body, []byte(`"images"`)) {
		return
	}

	var extras responseExtras
	if err := json.Unmarshal(body, &extras); err != nil {
		return
	}

	for i := range resp.Choices {
		if i >= len(extras.Choices) {
			break
		}

		raw := extras.Choices[i]
		applyToMessage(resp.Choices[i].Message, raw.Message)
		applyToMessage(resp.Choices[i].Delta, raw.Delta)
	}
}

func applyToMessage(msg *llm.Message, extras *messageExtras) {
	if msg == nil || extras == nil {
		return
	}

	if extras.ReasoningSignature != "" {
		signature := extras.ReasoningSignature
		msg.ReasoningSignature = &signature
	}

	var images []llm.MessageContentPart

	for _, img := range extras.Images {
		if img.ImageURL == nil || img.ImageURL.URL == "" {
			continue
		}

		images = append(images, llm.MessageContentPart{
			Type:     "image_url",
			ImageURL: &llm.ImageURL{URL: img.ImageURL.URL},
		})
	}

	if len(images) == 0 {
		return
	}

	parts := msg.Content.MultipleContent
	if msg.Content.Content != nil && *msg.Content.Content != "" {
		parts = append([]llm.MessageContentPart{{Type: "text", Text: msg.Content.Content}}, parts...)
	}

	msg.Content = llm.MessageContent{MultipleContent: append(parts, images...)}
}

type errorEnvelope struct {
	Error struct {
		Origin string `json:"origin"`
	} `json:"error"`
}

// applyErrorOrigin fills the error type and code from the Requesty envelope.
// origin is "router" for errors raised by Requesty itself (auth, balance, unknown model)
// and "provider" for errors passed through from the upstream provider.
func applyErrorOrigin(detail *llm.ErrorDetail, body []byte) {
	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Error.Origin == "" {
		return
	}

	if detail.Type == "" {
		detail.Type = "api_error"
	}

	if detail.Code == "" {
		detail.Code = envelope.Error.Origin
	}
}
