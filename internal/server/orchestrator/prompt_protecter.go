package orchestrator

import (
	"context"

	"github.com/looplj/axonhub/llm"
)

type PromptProtecter interface {
	Protect(ctx context.Context, req *llm.Request) (*llm.Request, error)
}
