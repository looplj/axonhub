package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/transformer"
)

func NewSearchOrchestrator(
	channelService *biz.ChannelService,
	modelService *biz.ModelService,
	requestService *biz.RequestService,
	httpClient *httpclient.HttpClient,
	inbound transformer.Inbound,
	systemService *biz.SystemService,
	usageLogService *biz.UsageLogService,
) *ChatCompletionOrchestrator {
	o := NewChatCompletionOrchestrator(
		channelService,
		modelService,
		requestService,
		httpClient,
		inbound,
		systemService,
		usageLogService,
		nil,
		nil,
		nil,
	)
	o.channelSelector = NewSearchSelector(channelService)
	return o
}

// SearchSelector selects enabled search channels supporting the requested model.
// Unlike DefaultSelector, it skips AxonHub Model association resolution since
// search requests route directly by enabled search_* channels.
type SearchSelector struct {
	ChannelService *biz.ChannelService
}

func NewSearchSelector(channelService *biz.ChannelService) *SearchSelector {
	return &SearchSelector{ChannelService: channelService}
}

func (s *SearchSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	channels := s.ChannelService.GetEnabledChannels()

	channels = lo.Filter(channels, func(ch *biz.Channel, _ int) bool {
		return ch.Channel.Type.IsSearch()
	})

	candidates := make([]*ChannelModelsCandidate, 0, len(channels))
	for _, ch := range channels {
		entries := ch.GetModelEntries()

		entry, ok := entries[req.Model]
		if !ok {
			continue
		}

		candidates = append(candidates, &ChannelModelsCandidate{
			Channel:  ch,
			Priority: 0,
			Models:   []biz.ChannelModelEntry{entry},
		})
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "selected search channel candidates",
			log.String("model", req.Model),
			log.Int("count", len(candidates)),
			log.Any("candidates", candidates),
		)
	}

	return candidates, nil
}

// selectSearchCandidates creates a simplified middleware that selects channel model candidates for search requests.
// Unlike selectCandidates, it skips profile-based filtering (channel IDs, tags),
// native tools filters (Google/Anthropic), and stream policy — none of which apply to search.
func selectSearchCandidates(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("select-search-candidates", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		// Only select candidates once
		if len(inbound.state.ChannelModelsCandidates) > 0 {
			return llmRequest, nil
		}

		selector := inbound.state.CandidateSelector

		if inbound.state.LoadBalancer != nil {
			selector = WithLoadBalancedSelector(selector, inbound.state.LoadBalancer, inbound.state.RetryPolicyProvider)
		}

		candidates, err := selector.Select(ctx, llmRequest)
		if err != nil {
			return nil, err
		}

		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "selected search candidates",
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

		inbound.state.ChannelModelsCandidates = candidates

		return llmRequest, nil
	})
}
