package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

// selectCandidates creates a middleware that selects available channel model candidates for the model.
// This is the second step in the inbound pipeline, moved from outbound transformer.
// If no valid candidates are found, it returns ErrInvalidModel to fail fast.
func selectCandidates(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("select-candidates", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		// Only select candidates once
		if len(inbound.state.ChannelModelsCandidates) > 0 {
			return llmRequest, nil
		}

		selector := inbound.state.CandidateSelector

		if profile := inbound.state.APIKey.GetActiveProfile(); profile != nil {
			// 先应用 ChannelIDs 过滤
			if len(profile.ChannelIDs) > 0 {
				selector = WithSelectedChannelsSelector(selector, profile.ChannelIDs)
			}

			// 再应用 ChannelTags 过滤（链式装饰器，与 IDs 取交集）
			if len(profile.ChannelTags) > 0 {
				selector = WithTagsFilterSelector(selector, profile.ChannelTags)
			}
		}

		// 应用 Google 原生工具过滤（仅对 Gemini 原生 API 格式生效）
		if inbound.APIFormat() == llm.APIFormatGeminiContents {
			selector = WithGoogleNativeToolsSelector(selector)
		}

		// 应用 Anthropic 原生工具过滤（对所有 API 格式生效）
		// 无论通过 OpenAI 还是 Anthropic 格式入口，只要包含 web_search 工具，
		// 都需要优先路由到支持 Anthropic 原生工具的渠道
		selector = WithAnthropicNativeToolsSelector(selector)

		selector = WithStreamPolicySelector(selector)

		if inbound.state.LoadBalancer != nil {
			selector = WithLoadBalancedSelector(selector, inbound.state.LoadBalancer, inbound.state.RetryPolicyProvider)
		}

		candidates, err := selector.Select(ctx, llmRequest)
		if err != nil {
			return nil, err
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "selected candidates",
				log.Int("candidate_count", len(candidates)),
				log.String("model", llmRequest.Model),
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

		if len(candidates) == 0 {
			return nil, fmt.Errorf("%w: %s", biz.ErrInvalidModel, llmRequest.Model)
		}

		// Store candidates directly (no need to extract channels)
		inbound.state.ChannelModelsCandidates = candidates

		return llmRequest, nil
	})
}
