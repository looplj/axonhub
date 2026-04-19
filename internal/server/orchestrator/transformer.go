package orchestrator

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/shared"
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

func enrichOpenAIIdentityContext(ctx context.Context, req *llm.Request) {
	if req == nil {
		return
	}

	if req.TransformerMetadata == nil {
		req.TransformerMetadata = map[string]any{}
	}

	if _, exists := req.TransformerMetadata[shared.TransformerMetadataKeyOpenAIIdentityOwner]; !exists {
		if apiKey, ok := contexts.GetAPIKey(ctx); ok && apiKey != nil {
			if ownerIdentity := apiKeyOwnerIdentity(apiKey); ownerIdentity != "" {
				req.TransformerMetadata[shared.TransformerMetadataKeyOpenAIIdentityOwner] = ownerIdentity
			}
		}
	}
}

func apiKeyOwnerIdentity(apiKey *ent.APIKey) string {
	if apiKey == nil {
		return ""
	}

	if apiKey.UserID > 0 {
		return fmt.Sprintf("user:%d", apiKey.UserID)
	}

	if apiKey.ID > 0 {
		return fmt.Sprintf("api_key:%d", apiKey.ID)
	}

	return ""
}
