package longcat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// OutboundTransformer implements transformer.Outbound for Longcat format.
// It inherits from OpenAI transformer but ensures Message Content always uses
// the multiple content (array) format, as required by models like LongCat-Flash-Omni.
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformer creates a new Longcat OutboundTransformer.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	return NewOutboundTransformerWithConfig(&Config{
		BaseURL:        baseURL,
		APIKeyProvider: auth.NewStaticKeyProvider(apiKey),
	})
}

type Config struct {
	BaseURL        string
	APIKeyProvider auth.APIKeyProvider
}

func NewOutboundTransformerWithConfig(config *Config) (transformer.Outbound, error) {
	oaiTransformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		PlatformType:   openai.PlatformOpenAI,
		BaseURL:        config.BaseURL,
		APIKeyProvider: config.APIKeyProvider,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create longcat outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound: oaiTransformer,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Request.
// It forces all message content into the multiple content (array) format,
// because Longcat models reject plain string content with "json format error".
func (t *OutboundTransformer) TransformRequest(
	ctx context.Context,
	chatReq *llm.Request,
) (*httpclient.Request, error) {
	if chatReq == nil {
		return nil, fmt.Errorf("chat completion request is nil")
	}

	// Ensure all messages have non-nil content
	for i := range chatReq.Messages {
		if chatReq.Messages[i].Content.Content == nil && len(chatReq.Messages[i].Content.MultipleContent) == 0 {
			chatReq.Messages[i].Content.Content = lo.ToPtr("")
		}
	}

	httpReq, err := t.Outbound.TransformRequest(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	// Unmarshal into OpenAI request, then convert to longcat request
	// so that MessageContent.MarshalJSON always produces array format.
	var oaiReq openai.Request
	if err := json.Unmarshal(httpReq.Body, &oaiReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal openai request: %w", err)
	}

	lcReq := Request{Request: oaiReq}
	lcReq.Messages = lo.Map(oaiReq.Messages, func(m openai.Message, _ int) Message {
		return Message{
			Message: m,
			Content: MessageContent{MessageContent: m.Content},
		}
	})

	httpReq.Body, err = json.Marshal(lcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal longcat request: %w", err)
	}

	return httpReq, nil
}
