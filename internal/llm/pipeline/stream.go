package pipeline

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
)

// Process executes the streaming LLM pipeline
// Steps: apply decorators -> outbound transform -> HTTP stream -> outbound stream transform -> inbound stream transform
func (p *pipeline) stream(ctx context.Context, request *llm.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	// Step 1: Apply decorators to the request
	if len(p.decorators) > 0 {
		for _, dec := range p.decorators {
			var err error
			request, err = dec.Decorate(ctx, request)
			if err != nil {
				log.Error(ctx, "Failed to apply decorator", log.Cause(err))
				return nil, err
			}
		}
	}

	// Step 2: Transform to provider-specific HTTP request using outbound transformer
	httpReq, err := p.Outbound.TransformRequest(ctx, request)
	if err != nil {
		log.Error(ctx, "Failed to transform streaming request", log.Cause(err))
		return nil, err
	}

	// Step 3: Execute streaming HTTP request
	httpStream, err := p.HttpClient.DoStream(ctx, httpReq)
	if err != nil {
		log.Error(ctx, "HTTP streaming request failed", log.Cause(err))
		return nil, err
	}

	// Step 4: Transform the HTTP stream through the complete pipeline
	finalStream := streams.MapErr(httpStream, func(src *httpclient.StreamEvent) (*httpclient.StreamEvent, error) {
		// Transform HTTP stream event to LLM response using outbound transformer
		llmResp, err := p.Outbound.TransformStreamChunk(ctx, src)
		if err != nil {
			return nil, err
		}
		log.Debug(ctx, "LLM stream response", log.Any("response", llmResp))

		// Transform LLM response to final HTTP stream event using inbound transformer
		event, err := p.Inbound.TransformStreamChunk(ctx, llmResp)
		if err != nil {
			return nil, err
		}
		log.Debug(ctx, "Final stream event", log.Any("event", event))

		return event, nil
	})

	return finalStream, nil
}
