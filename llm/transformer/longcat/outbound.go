package longcat

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// OutboundTransformer implements transformer.Outbound for Longcat format.
// It inherits from OpenAI transformer but ensures Message Content is never nil.
type OutboundTransformer struct {
	transformer.Outbound
}

// NewOutboundTransformer creates a new Longcat OutboundTransformer.
func NewOutboundTransformer(baseURL, apiKey string) (transformer.Outbound, error) {
	oaiTransformer, err := openai.NewOutboundTransformerWithConfig(&openai.Config{
		Type:    openai.PlatformOpenAI,
		BaseURL: baseURL,
		APIKey:  apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create longcat outbound transformer: %w", err)
	}

	return &OutboundTransformer{
		Outbound: oaiTransformer,
	}, nil
}

// TransformRequest transforms ChatCompletionRequest to Request.
// It ensures Message Content is never nil (Longcat requires the content field to exist).
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

	return t.Outbound.TransformRequest(ctx, chatReq)
}
