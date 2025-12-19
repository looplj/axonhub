package orchestrator

import (
	"github.com/looplj/axonhub/internal/llm/transformer"
)

var (
	_ transformer.Inbound  = &PersistentInboundTransformer{}
	_ transformer.Outbound = &PersistentOutboundTransformer{}
)

// NewPersistentTransformers creates enhanced persistent transformers with pre-constructed state.
func NewPersistentTransformers(state *PersistenceState, wrapped transformer.Inbound) (*PersistentInboundTransformer, *PersistentOutboundTransformer) {
	return &PersistentInboundTransformer{
			wrapped: wrapped,
			state:   state,
		}, &PersistentOutboundTransformer{
			wrapped: nil, // Will be set when channel is selected
			state:   state,
		}
}
