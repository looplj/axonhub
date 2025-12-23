package orchestrator

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
)

// selectCandidates creates a middleware that selects available channel model candidates for the model.
// This is the second step in the inbound pipeline, moved from outbound transformer.
// If no valid candidates are found, it returns ErrInvalidModel to fail fast.
func selectCandidates(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("select-candidates", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		// Only select candidates once
		if len(inbound.state.ChannelModelCandidates) > 0 {
			return llmRequest, nil
		}

		selector := inbound.state.CandidateSelector

		if profile := GetActiveProfile(inbound.state.APIKey); profile != nil {
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

		if inbound.state.LoadBalancer != nil {
			selector = WithLoadBalancedSelector(selector, inbound.state.LoadBalancer)
		}

		candidates, err := selector.Select(ctx, llmRequest)
		if err != nil {
			return nil, err
		}

		log.Debug(ctx, "selected candidates",
			log.Int("candidate_count", len(candidates)),
			log.String("model", llmRequest.Model),
		)

		if len(candidates) == 0 {
			return nil, fmt.Errorf("%w: no valid candidates found for model %s", biz.ErrInvalidModel, llmRequest.Model)
		}

		// Store candidates directly (no need to extract channels)
		inbound.state.ChannelModelCandidates = candidates

		return llmRequest, nil
	})
}
