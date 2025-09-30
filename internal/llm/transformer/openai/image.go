package openai

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
)

// buildResponsesAPIRequest builds the HTTP request to call the OpenAI Responses API
// for image generation.
func (t *OutboundTransformer) buildResponsesAPIRequest(ctx context.Context, chatReq *llm.Request) (*httpclient.Request, error) {
	chatReq.Stream = lo.ToPtr(false)

	rawReq, err := t.rt.TransformRequest(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to transform request: %w", err)
	}

	if rawReq.Metadata == nil {
		rawReq.Metadata = map[string]string{}
	}

	rawReq.Metadata["outbound_format_type"] = llm.APIFormatOpenAIResponse.String()

	return rawReq, nil
}
