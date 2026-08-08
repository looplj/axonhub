package orchestrator

import (
	"context"
	"errors"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
)

const promptProtectionRejectedMessage = "request blocked by prompt protection policy"

func protectPrompts(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("protect-prompts", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if inbound.state.PromptProtecter == nil {
			return llmRequest, nil
		}

		protected, matchedRules, err := protectPromptRequest(ctx, inbound.state.PromptProtecter, llmRequest)
		if err != nil {
			if errors.Is(err, biz.ErrPromptProtectionRejected) {
				return nil, fmt.Errorf("%w: %s", transformer.ErrInvalidRequest, promptProtectionRejectedMessage)
			}

			log.Warn(ctx, "failed to protect prompts", log.Cause(err))

			return llmRequest, nil
		}

		inbound.state.PromptProtectionMaskRules = matchedRules

		if protected == nil {
			return llmRequest, nil
		}

		return protected, nil
	})
}

// protectPromptRequest keeps legacy prompt protectors working while allowing the
// rule service to expose mask matches for raw request-body pass-through.
func protectPromptRequest(ctx context.Context, protecter PromptProtecter, request *llm.Request) (*llm.Request, []*ent.PromptProtectionRule, error) {
	resultProvider, ok := protecter.(PromptProtectionResultProvider)
	if !ok {
		protected, err := protecter.Protect(ctx, request)

		return protected, nil, err
	}

	result, err := resultProvider.ProtectWithResult(ctx, request)
	if err != nil {
		return result.Request, nil, err
	}

	return result.Request, result.MatchedRules, nil
}
