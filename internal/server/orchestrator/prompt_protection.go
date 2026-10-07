package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
)

const promptProtectionRejectedMessage = "request blocked by prompt protection policy"

// protectPrompts masks or rejects prompts and snapshots the result for safe raw replay.
func protectPrompts(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("protect-prompts", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		if inbound.state.PromptProtecter == nil {
			return llmRequest, nil
		}

		originalTexts := promptProtectionTexts(llmRequest)
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
			protected = llmRequest
		}
		if protected.RawRequest == nil {
			// A legacy protector may return a new request without transport metadata.
			// Keep the inbound source available for safe replay and model mapping.
			protected.RawRequest = llmRequest.RawRequest
		}

		protectedTexts := promptProtectionTexts(protected)
		if llmRequest.APIFormat != llm.APIFormatOpenAIDecisions && (len(matchedRules) > 0 || !slices.Equal(originalTexts, protectedTexts)) {
			// Legacy protectors can change prompts without reporting rules. They must
			// also validate raw replay instead of silently restoring the original text.
			inbound.state.PromptProtectionBodyCheck = &promptProtectionBodyCheck{
				inbound:  inbound.wrapped,
				raw:      llmRequest.RawRequest,
				expected: protectedTexts,
			}
		}
		inbound.state.LlmRequest = protected

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
