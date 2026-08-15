package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/ent/providerquotastatus"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// resolveVisionDelegationSourceModel resolves only image-bearing requests
// before conditional route selection. Vision delegation needs the source model
// settings, while the eventual primary route must be selected after images are
// replaced with text evidence.
func resolveVisionDelegationSourceModel(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("resolve-vision-delegation-source-model", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		state := inbound.state
		if llmRequest == nil || state == nil || state.DelegationDepth > 0 || state.sourceModelResolution != nil ||
			state.ModelService == nil || !detectRequestContentFeatures(llmRequest).hasImage {
			return llmRequest, nil
		}

		sourceModel, err := state.ModelService.GetModelByModelID(ctx, llmRequest.Model, model.StatusEnabled)
		state.sourceModelResolution = &sourceModelResolution{
			modelID: llmRequest.Model,
			model:   sourceModel,
			err:     err,
		}
		if err == nil {
			state.SourceModel = sourceModel
		}

		return llmRequest, nil
	})
}

// selectCandidates creates a middleware that selects available channel model candidates for the model.
// This is the second step in the inbound pipeline, moved from outbound transformer.
// If no valid candidates are found, it returns ErrInvalidModel to fail fast.
func selectCandidates(inbound *PersistentInboundTransformer, quotaProvider ProviderQuotaStatusProvider, systemService QuotaEnforcementSettingsProvider) pipeline.Middleware {
	return pipeline.OnLlmRequest("select-candidates", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		// Only select candidates once
		if len(inbound.state.ChannelModelsCandidates) > 0 {
			return llmRequest, nil
		}
		selector := inbound.state.CandidateSelector

		// Project-level profile filtering (upper boundary)
		if inbound.state.APIKey != nil {
			if project := inbound.state.APIKey.Edges.Project; project != nil {
				if projectProfile := project.GetActiveProfile(); projectProfile != nil {
					if len(projectProfile.ChannelIDs) > 0 {
						selector = WithSelectedChannelsSelector(selector, projectProfile.ChannelIDs)
					}

					if len(projectProfile.ChannelTags) > 0 {
						selector = WithChannelTagsFilterSelector(selector, projectProfile.ChannelTags, projectProfile.ChannelTagsMatchMode)
					}
				}
			}
		}

		// Key-level profile filtering (narrows further within project scope)
		if profile := inbound.state.APIKey.GetActiveProfile(); profile != nil {
			if len(profile.ChannelIDs) > 0 {
				selector = WithSelectedChannelsSelector(selector, profile.ChannelIDs)
			}

			if len(profile.ChannelTags) > 0 {
				selector = WithChannelTagsFilterSelector(selector, profile.ChannelTags, profile.ChannelTagsMatchMode)
			}
		}

		// Apply Google native tools filter (only for Gemini native API format)
		if llmRequest.APIFormat == llm.APIFormatGeminiContents {
			selector = WithGoogleNativeToolsSelector(selector)
		}

		// Apply Anthropic native tools filter (only for Anthropic message API format)
		if llmRequest.APIFormat == llm.APIFormatAnthropicMessage {
			selector = WithAnthropicNativeToolsSelector(selector)
		}

		selector = WithStreamPolicySelector(selector)
		if shouldDeferPrimaryCandidateSelection(inbound.state, llmRequest) {
			// Reject routes that are already impossible after images are removed,
			// before the paid vision request runs. Prompt-token conditions remain
			// provisional because the delegated evidence length is not known yet.
			preflightRequest := cloneVisionDelegationPrimaryRequestForPreflight(llmRequest)
			preflightCtx := withVisionDelegationPreflight(
				withSourceModelResolution(ctx, inbound.state.sourceModelResolution),
			)
			candidates, err := selector.Select(preflightCtx, preflightRequest)
			if err != nil {
				return nil, err
			}
			if len(candidates) == 0 {
				return nil, fmt.Errorf("%w: %s", biz.ErrInvalidModel, llmRequest.Model)
			}

			return llmRequest, nil
		}

		quotaSelector := WithProviderQuotaSelector(selector, quotaProvider, systemService)
		selector = quotaSelector

		if len(inbound.state.LoadBalancers) > 0 {
			selector = WithRoutingPolicyLoadBalancedSelector(
				selector,
				inbound.state.LoadBalancers,
				inbound.state.RetryPolicyProvider,
				inbound.state.RequestService,
				inbound.state.APIKey,
				&inbound.state.RoutingPolicy,
			)
		}

		selectionCtx := withSourceModelResolution(ctx, inbound.state.sourceModelResolution)
		candidates, err := selector.Select(selectionCtx, llmRequest)
		if err != nil {
			return nil, err
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "selected candidates",
				log.Int("candidate_count", len(candidates)),
				log.String("model", llmRequest.Model),
				log.String("load_balance_strategy", inbound.state.RoutingPolicy.LoadBalancerStrategy),
				log.String("trace_sticky_mode", string(inbound.state.RoutingPolicy.TraceStickyMode)),
				log.Any("candidates", lo.Map(candidates, func(candidate *ChannelModelsCandidate, _ int) map[string]any {
					return map[string]any{
						"channel_name": candidate.Channel.Name,
						"channel_id":   candidate.Channel.ID,
						"priority":     candidate.Priority,
						"models": lo.Map(candidate.Models, func(entry biz.ChannelModelEntry, _ int) map[string]any {
							return map[string]any{
								"request_model": entry.RequestModel,
								"actual_model":  entry.ActualModel,
								"source":        entry.Source,
							}
						}),
					}
				})),
			)
		}

		settings := systemService.QuotaEnforcementSettingsOrDefault(ctx)

		if len(candidates) == 0 {
			if settings.Enabled && quotaSelector.FilteredCount > 0 {
				return nil, NewQuotaExhaustedError(llmRequest.Model)
			}
			return nil, fmt.Errorf("%w: %s", biz.ErrInvalidModel, llmRequest.Model)
		}

		if inbound.state.SourceModel == nil {
			inbound.state.SourceModel = candidates[0].SourceModel
		}
		if settings.Enabled && settings.Mode == biz.QuotaEnforcementModeDePrioritize {
			// In DePrioritize mode the quota selector doesn't filter candidates,
			// so we must check quota status again here to determine if all
			// remaining channels are exhausted.
			if areAllChannelsExhausted(ctx, candidates, quotaProvider, llmRequest) {
				return nil, NewQuotaExhaustedError(llmRequest.Model)
			}
		}

		// Store candidates directly (no need to extract channels)
		inbound.state.ChannelModelsCandidates = candidates

		return llmRequest, nil
	})
}

func shouldDeferPrimaryCandidateSelection(state *PersistenceState, request *llm.Request) bool {
	return state != nil && state.DelegationDepth == 0 && state.SourceModel != nil &&
		state.SourceModel.Settings != nil && state.SourceModel.Settings.VisionDelegation.Enabled &&
		detectRequestContentFeatures(request).hasImage
}

func cloneVisionDelegationPrimaryRequestForPreflight(request *llm.Request) *llm.Request {
	cloned := *request
	cloned.Messages = append([]llm.Message(nil), request.Messages...)
	for i := range cloned.Messages {
		cloned.Messages[i].Content = request.Messages[i].Content
		cloned.Messages[i].Content.MultipleContent = append(
			[]llm.MessageContentPart(nil),
			request.Messages[i].Content.MultipleContent...,
		)
	}
	removeVisionImages(&cloned)

	return &cloned
}

func areAllChannelsExhausted(ctx context.Context, candidates []*ChannelModelsCandidate, quotaProvider ProviderQuotaStatusProvider, llmRequest *llm.Request) bool {
	if len(candidates) == 0 || quotaProvider == nil {
		return false
	}

	limitType := provider_quota.RequestModality(llmRequest.Image != nil)

	for _, c := range candidates {
		quotaStatus := quotaProvider.GetQuotaStatus(ctx, c.Channel.ID)
		if quotaStatus == nil {
			return false
		}

		effectiveStatus, _ := quotaStatus.EffectiveStatus(limitType)
		if effectiveStatus != providerquotastatus.StatusExhausted {
			return false
		}
	}

	return true
}
