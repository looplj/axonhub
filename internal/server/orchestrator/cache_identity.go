package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/cacheidentity"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// cacheIdentityMiddleware resolves stable session IDs and prompt cache keys
// on the inbound path. It runs before candidate selection so the resolved
// identity survives channel retries.
type cacheIdentityMiddleware struct {
	pipeline.DummyMiddleware

	resolver *cacheidentity.Resolver
}

func enrichCacheIdentity(resolver *cacheidentity.Resolver) pipeline.Middleware {
	return &cacheIdentityMiddleware{resolver: resolver}
}

func (m *cacheIdentityMiddleware) Name() string {
	return "cache-identity"
}

func (m *cacheIdentityMiddleware) OnInboundLlmRequest(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
	if m.resolver == nil {
		return llmRequest, nil
	}

	result := m.resolver.Resolve(ctx, llmRequest)

	// Store resolved values on TransformerMetadata for later use.
	cacheidentity.StoreOnRequest(llmRequest, result)

	// Note: Session ID is stored on TransformerMetadata above. Context-level
	// injection happens later in applyCacheIdentityGating, which correctly
	// returns the updated context. We cannot inject here because
	// OnInboundLlmRequest does not return context.

	if log.DebugEnabled(ctx) {
		redactedKey := result.PromptCacheKey
		if len(redactedKey) > 16 {
			redactedKey = redactedKey[:16] + "..."
		}

		log.Debug(ctx, "cache identity resolved",
			log.String("source", result.Source),
			log.String("cache_key_preview", redactedKey),
			log.Bool("session_injected", result.SessionID != ""),
			log.Bool("cache_key_injected", result.PromptCacheKey != ""),
		)
	}

	return llmRequest, nil
}
