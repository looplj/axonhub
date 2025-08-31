package pipeline

import (
	"context"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/pkg/httpclient"
	"github.com/looplj/axonhub/internal/pkg/streams"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

// Process executes the streaming LLM pipeline
// Steps: apply decorators -> outbound transform -> HTTP stream -> outbound stream transform -> inbound stream transform.
func (p *pipeline) stream(
	ctx context.Context,
	request *llm.Request,
) (streams.Stream[*httpclient.StreamEvent], error) {
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

	executor := p.Executor
	if c, ok := p.Outbound.(ChannelCustomizedExecutor); ok {
		executor = c.CustomizeExecutor(executor)
	}

	// Step 3: Execute streaming HTTP request
	outboundStream, err := executor.DoStream(ctx, httpReq)
	if err != nil {
		if httpErr, ok := xerrors.As[*httpclient.Error](err); ok {
			return nil, p.Outbound.TransformError(ctx, httpErr)
		}

		return nil, err
	}

	if log.DebugEnabled(ctx) {
		outboundStream = streams.Map(
			outboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				log.Debug(ctx, "Outbound stream event", log.Any("event", event))
				return event
			},
		)
	}

	llmStream, err := p.Outbound.TransformStream(ctx, outboundStream)
	if err != nil {
		log.Error(ctx, "Failed to transform streaming request", log.Cause(err))
		return nil, err
	}

	if log.DebugEnabled(ctx) {
		llmStream = streams.Map(llmStream, func(event *llm.Response) *llm.Response {
			log.Debug(ctx, "LLM stream event", log.Any("event", event))
			return event
		})
	}

	inboundStream, err := p.Inbound.TransformStream(ctx, llmStream)
	if err != nil {
		log.Error(ctx, "Failed to transform streaming request", log.Cause(err))
		return nil, err
	}

	if log.DebugEnabled(ctx) {
		inboundStream = streams.Map(
			inboundStream,
			func(event *httpclient.StreamEvent) *httpclient.StreamEvent {
				log.Debug(ctx, "Inbound stream event", log.Any("event", event))
				return event
			},
		)
	}

	return inboundStream, nil
}
