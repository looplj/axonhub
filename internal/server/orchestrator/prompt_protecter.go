package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

type PromptProtecter interface {
	Protect(ctx context.Context, req *llm.Request) (*llm.Request, error)
}

// PromptProtectionResultProvider exposes matched rules for raw-body patching.
// The base PromptProtecter interface remains compatible with test and external providers.
type PromptProtectionResultProvider interface {
	ProtectWithResult(ctx context.Context, req *llm.Request) (biz.PromptProtectionResult, error)
}
