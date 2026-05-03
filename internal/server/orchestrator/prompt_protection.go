package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"reflect"

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

		beforeRequest := CloneRequestForOutboundAttempt(llmRequest)
		var beforeMessages []llm.Message
		if beforeRequest != nil {
			beforeMessages = beforeRequest.Messages
		}

		protected, err := inbound.state.PromptProtecter.Protect(ctx, llmRequest)
		if err != nil {
			if errors.Is(err, biz.ErrPromptProtectionRejected) {
				inbound.state.PromptProtection.Rejected = true
				inbound.state.PromptProtection.Fragments = append(inbound.state.PromptProtection.Fragments, PromptProtectionFragmentResult{
					Scope:  "messages",
					Status: PromptProtectionFragmentRejected,
				})

				return nil, fmt.Errorf("%w: %s", transformer.ErrInvalidRequest, promptProtectionRejectedMessage)
			}

			log.Warn(ctx, "failed to protect prompts", log.Cause(err))

			return llmRequest, nil
		}

		if protected == nil {
			return llmRequest, nil
		}

		if !reflect.DeepEqual(beforeMessages, protected.Messages) {
			inbound.state.PromptProtection.Changed = true
			inbound.state.PromptProtection.Fragments = append(inbound.state.PromptProtection.Fragments, PromptProtectionFragmentResult{
				Scope:  "messages",
				Status: PromptProtectionFragmentMatchedChangedReplayable,
			})
			inbound.state.MarkDirty(RequestDirtyMessages, RequestDirtyInputItems)
		}
		inbound.state.SetEffectiveSemanticRequest(protected)

		return protected, nil
	})
}
