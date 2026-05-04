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
			markUnscannedProtectableFragments(inbound, llmRequest)
			return llmRequest, nil
		}

		beforeRequest := CloneRequestForOutboundAttempt(llmRequest)
		var beforeMessages []llm.Message
		if beforeRequest != nil {
			beforeMessages = beforeRequest.Messages
		}

		var (
			protected *llm.Request
			result    biz.PromptProtectionResult
			err       error
		)

		if protecter, ok := inbound.state.PromptProtecter.(interface {
			ProtectWithResult(context.Context, *llm.Request) (biz.PromptProtectionResult, error)
		}); ok {
			result, err = protecter.ProtectWithResult(ctx, llmRequest)
			protected = result.Request
		} else {
			protected, err = inbound.state.PromptProtecter.Protect(ctx, llmRequest)
			result = biz.PromptProtectionResult{Request: protected}
		}

		if err != nil {
			if errors.Is(err, biz.ErrPromptProtectionRejected) {
				inbound.state.PromptProtection.Rejected = true
				applyPromptProtectionResult(inbound, result, beforeMessages, protected)
				if len(inbound.state.PromptProtection.Fragments) == 0 {
					inbound.state.PromptProtection.Fragments = append(inbound.state.PromptProtection.Fragments, PromptProtectionFragmentResult{
						Scope:  "messages",
						Status: PromptProtectionFragmentRejected,
					})
				}

				return nil, fmt.Errorf("%w: %s", transformer.ErrInvalidRequest, promptProtectionRejectedMessage)
			}

			log.Warn(ctx, "failed to protect prompts", log.Cause(err))

			return llmRequest, nil
		}

		if protected == nil {
			return llmRequest, nil
		}

		applyPromptProtectionResult(inbound, result, beforeMessages, protected)
		inbound.state.SetEffectiveSemanticRequest(protected)

		return protected, nil
	})
}

func applyPromptProtectionResult(
	inbound *PersistentInboundTransformer,
	result biz.PromptProtectionResult,
	beforeMessages []llm.Message,
	protected *llm.Request,
) {
	if inbound == nil || protected == nil {
		return
	}

	if !reflect.DeepEqual(beforeMessages, protected.Messages) {
		inbound.state.PromptProtection.Changed = true
		inbound.state.PromptProtection.Fragments = append(inbound.state.PromptProtection.Fragments, PromptProtectionFragmentResult{
			Scope:  "messages",
			Status: PromptProtectionFragmentMatchedChangedReplayable,
		})
		inbound.state.MarkDirty(RequestDirtyMessages, RequestDirtyInputItems)
	}

	if result.Rejected {
		inbound.state.PromptProtection.Rejected = true
	}

	if len(result.FragmentResults) == 0 {
		return
	}

	updateOpenAIResponsesFragmentProtection(protected, result.FragmentResults)

	for _, fragment := range result.FragmentResults {
		status := promptProtectionFragmentStatus(fragment)
		inbound.state.PromptProtection.Fragments = append(inbound.state.PromptProtection.Fragments, PromptProtectionFragmentResult{
			Scope:  fragment.Scope,
			Status: status,
		})

		if fragment.Changed || fragment.DropRequired || fragment.RejectRequired {
			inbound.state.PromptProtection.Changed = true
			inbound.state.MarkDirty(RequestDirtyInputItems)
		}
	}
}

func markUnscannedProtectableFragments(inbound *PersistentInboundTransformer, req *llm.Request) {
	if inbound == nil || req == nil || req.ProviderExtensions == nil ||
		req.ProviderExtensions.OpenAIResponses == nil || req.ProviderExtensions.OpenAIResponses.Request == nil {
		return
	}

	requestExt := req.ProviderExtensions.OpenAIResponses.Request
	for _, fragment := range requestExt.ProtectableFragments {
		inbound.state.PromptProtection.Fragments = append(inbound.state.PromptProtection.Fragments, PromptProtectionFragmentResult{
			Scope:  fragment.Scope,
			Status: PromptProtectionFragmentUnscanned,
		})
	}
}

func updateOpenAIResponsesFragmentProtection(req *llm.Request, results []biz.PromptProtectionFragmentResult) {
	if req == nil || req.ProviderExtensions == nil || req.ProviderExtensions.OpenAIResponses == nil ||
		req.ProviderExtensions.OpenAIResponses.Request == nil {
		return
	}

	requestExt := req.ProviderExtensions.OpenAIResponses.Request
	for i := range requestExt.InputItems {
		item := &requestExt.InputItems[i]
		for _, result := range results {
			if !fragmentResultBelongsToItem(result.Path, item.Protection.TextPaths) {
				continue
			}

			item.Protection.Scanned = true
			item.Protection.TextExtracted = true
			item.Protection.Scope = result.Scope
			item.Protection.ReplayAllowed = result.ReplayAllowed
			item.Protection.Changed = result.Changed
			item.Protection.Status = openAIResponsesProtectionStatus(result)
		}
	}

	for key, textPaths := range requestExt.TopLevelSemanticExtraTextPaths {
		for _, result := range results {
			if !fragmentResultBelongsToItem(result.Path, textPaths) {
				continue
			}

			if requestExt.TopLevelSemanticExtraProtection == nil {
				requestExt.TopLevelSemanticExtraProtection = map[string]llm.OpenAIResponsesRawProtection{}
			}

			current := requestExt.TopLevelSemanticExtraProtection[key]
			if current.Scanned && !current.ReplayAllowed {
				continue
			}

			current.Scanned = true
			current.TextExtracted = true
			current.Scope = result.Scope
			current.ReplayAllowed = result.ReplayAllowed
			current.Changed = current.Changed || result.Changed
			current.Status = openAIResponsesProtectionStatus(result)
			current.TextPaths = append([]string(nil), textPaths...)
			requestExt.TopLevelSemanticExtraProtection[key] = current
		}
	}
}

func fragmentResultBelongsToItem(path string, textPaths []string) bool {
	for _, textPath := range textPaths {
		if path == textPath {
			return true
		}
	}

	return false
}

func promptProtectionFragmentStatus(fragment biz.PromptProtectionFragmentResult) PromptProtectionFragmentStatus {
	if fragment.Rejected || fragment.RejectRequired {
		return PromptProtectionFragmentRejected
	}
	if fragment.DropRequired || (fragment.Changed && !fragment.ReplayAllowed) {
		return PromptProtectionFragmentMatchedChangedUnreplayable
	}
	if fragment.Changed {
		return PromptProtectionFragmentMatchedChangedReplayable
	}
	if fragment.Matched {
		return PromptProtectionFragmentMatchedUnchanged
	}

	return PromptProtectionFragmentScannedClean
}

func openAIResponsesProtectionStatus(fragment biz.PromptProtectionFragmentResult) llm.OpenAIResponsesProtectionStatus {
	if fragment.Rejected || fragment.RejectRequired {
		return llm.OpenAIResponsesProtectionChangedReject
	}
	if fragment.DropRequired || (fragment.Changed && !fragment.ReplayAllowed) {
		return llm.OpenAIResponsesProtectionChangedDrop
	}
	if fragment.Changed {
		return llm.OpenAIResponsesProtectionChangedRewritable
	}
	if fragment.Matched {
		return llm.OpenAIResponsesProtectionMatchedNoChange
	}
	if fragment.NoRules {
		return llm.OpenAIResponsesProtectionEvaluatedNoRules
	}

	return llm.OpenAIResponsesProtectionEvaluatedNoMatch
}
