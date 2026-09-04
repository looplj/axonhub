package orchestrator

import (
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
)

var (
	_ transformer.Inbound  = &PersistentInboundTransformer{}
	_ transformer.Outbound = &PersistentOutboundTransformer{}
)

// NewPersistentTransformers creates enhanced persistent transformers with pre-constructed state.
func NewPersistentTransformers(
	state *PersistenceState,
	wrapped transformer.Inbound,
	middlewares ...pipeline.Middleware,
) (*PersistentInboundTransformer, *PersistentOutboundTransformer) {
	outboundLlmRequestMiddlewares := make([]pipeline.OutboundLlmRequestMiddleware, 0, len(middlewares))
	for _, middleware := range middlewares {
		if outboundMiddleware, ok := middleware.(pipeline.OutboundLlmRequestMiddleware); ok {
			outboundLlmRequestMiddlewares = append(outboundLlmRequestMiddlewares, outboundMiddleware)
		}
	}

	inbound := &PersistentInboundTransformer{
		wrapped: wrapped,
		state:   state,
	}
	outbound := &PersistentOutboundTransformer{
		wrapped:                       nil, // Will be set when channel is selected
		state:                         state,
		outboundLlmRequestMiddlewares: outboundLlmRequestMiddlewares,
	}

	return inbound, outbound
}
