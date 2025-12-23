package orchestrator

import (
	"context"
	"fmt"
	"slices"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/llm/pipeline"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ModelResolver resolves AxonHub Models to channel+model candidates.
type ModelResolver struct {
	ModelService   *biz.ModelService
	ChannelService *biz.ChannelService
}

// NewModelResolver creates a new ModelResolver.
func NewModelResolver(modelService *biz.ModelService, channelService *biz.ChannelService) *ModelResolver {
	return &ModelResolver{
		ModelService:   modelService,
		ChannelService: channelService,
	}
}

// Resolve attempts to resolve a model name to AxonHub Model associations.
// Returns nil if no AxonHub Model is found (fallback to legacy behavior).
func (r *ModelResolver) Resolve(ctx context.Context, modelName string) ([]*ChannelModelCandidate, *ent.Model, error) {
	// Try to find an enabled AxonHub Model matching the model name
	axonhubModel, err := r.ModelService.GetModelByModelID(ctx, modelName, model.StatusEnabled)
	if err != nil {
		if ent.IsNotFound(err) {
			// No AxonHub Model found, return nil to indicate fallback to legacy
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("failed to query AxonHub Model: %w", err)
	}

	// Model found, resolve associations to candidates
	if axonhubModel.Settings == nil || len(axonhubModel.Settings.Associations) == 0 {
		log.Debug(ctx, "AxonHub Model has no associations",
			log.String("model", modelName))

		return nil, axonhubModel, nil
	}

	// Convert []ModelAssociation to []*ModelAssociation
	associations := make([]*objects.ModelAssociation, len(axonhubModel.Settings.Associations))
	for i := range axonhubModel.Settings.Associations {
		associations[i] = &axonhubModel.Settings.Associations[i]
	}

	candidates, err := r.resolveAssociations(ctx, associations)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve associations: %w", err)
	}

	if len(candidates) == 0 {
		log.Debug(ctx, "No candidates found for AxonHub Model",
			log.String("model", modelName))
	}

	return candidates, axonhubModel, nil
}

// resolveAssociations uses biz.MatchAssociations to resolve model associations
// and converts the results to ChannelModelCandidate.
func (r *ModelResolver) resolveAssociations(ctx context.Context, associations []*objects.ModelAssociation) ([]*ChannelModelCandidate, error) {
	// Get all enabled channels
	enabledChannels := r.ChannelService.GetEnabledChannels()
	if len(enabledChannels) == 0 {
		return []*ChannelModelCandidate{}, nil
	}

	// Convert []*biz.Channel to []biz.Channel for the matching function
	channels := lo.Map(enabledChannels, func(ch *biz.Channel, _ int) biz.Channel {
		return *ch
	})

	// Use the shared MatchAssociations function
	connections, err := biz.MatchAssociations(ctx, associations, channels)
	if err != nil {
		return nil, fmt.Errorf("failed to match associations: %w", err)
	}

	// Convert ModelChannelConnection to ChannelModelCandidate
	candidates := make([]*ChannelModelCandidate, 0, len(connections))
	for _, conn := range connections {
		bizCh, found := lo.Find(enabledChannels, func(c *biz.Channel) bool {
			return c.ID == conn.Channel.ID
		})
		if !found || bizCh == nil {
			continue
		}

		entries := bizCh.GetModelEntries()
		for _, modelID := range conn.ModelIds {
			entry, found := entries[modelID]
			if found {
				candidates = append(candidates, &ChannelModelCandidate{
					Channel:      bizCh,
					RequestModel: entry.RequestModel,
					ActualModel:  entry.ActualModel,
					Priority:     conn.Priority,
				})
			}
		}
	}

	return candidates, nil
}

// filterCandidatesByChannelIDs filters candidates by allowed channel IDs.
func filterCandidatesByChannelIDs(candidates []*ChannelModelCandidate, allowedIDs []int) []*ChannelModelCandidate {
	if len(allowedIDs) == 0 {
		return candidates
	}

	return lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		return lo.Contains(allowedIDs, c.Channel.ID)
	})
}

// filterCandidatesByChannelTags filters candidates by channel tags (OR logic).
func filterCandidatesByChannelTags(candidates []*ChannelModelCandidate, allowedTags []string) []*ChannelModelCandidate {
	if len(allowedTags) == 0 {
		return candidates
	}

	return lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		for _, tag := range c.Channel.Tags {
			if lo.Contains(allowedTags, tag) {
				return true
			}
		}

		return false
	})
}

// sortCandidatesByPriorityAndScore sorts candidates by priority first, then by load balancer score within each priority group.
func sortCandidatesByPriorityAndScore(ctx context.Context, candidates []*ChannelModelCandidate, lb *LoadBalancer) []*ChannelModelCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	// Group by priority
	groups := make(map[int][]*ChannelModelCandidate)
	for _, c := range candidates {
		groups[c.Priority] = append(groups[c.Priority], c)
	}

	// Get sorted priority keys (lower priority value = higher priority)
	priorities := lo.Keys(groups)
	slices.Sort(priorities)

	// Sort each group by LoadBalancer score, then concatenate
	result := make([]*ChannelModelCandidate, 0, len(candidates))

	for _, p := range priorities {
		group := groups[p]

		// Sort group by load balancer score
		sortedGroup := sortCandidatesByScore(ctx, group, lb)
		result = append(result, sortedGroup...)
	}

	return result
}

// sortCandidatesByScore sorts candidates by load balancer score.
func sortCandidatesByScore(ctx context.Context, candidates []*ChannelModelCandidate, lb *LoadBalancer) []*ChannelModelCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	// Calculate scores for each candidate
	type candidateWithScore struct {
		candidate *ChannelModelCandidate
		score     float64
	}

	scored := make([]candidateWithScore, len(candidates))
	for i, c := range candidates {
		// For now, we score based on channel only
		// In the future, we could extend LoadBalanceStrategy to support channel+model scoring
		score := lb.ScoreChannel(ctx, c.Channel)
		scored[i] = candidateWithScore{
			candidate: c,
			score:     score,
		}
	}

	// Sort by score (descending)
	slices.SortFunc(scored, func(a, b candidateWithScore) int {
		if a.score > b.score {
			return -1
		}

		if a.score < b.score {
			return 1
		}

		return 0
	})

	// Extract sorted candidates
	result := make([]*ChannelModelCandidate, len(scored))
	for i, s := range scored {
		result[i] = s.candidate
	}

	return result
}

// extractChannelsFromCandidates extracts unique channels from candidates while preserving order.
func extractChannelsFromCandidates(candidates []*ChannelModelCandidate) []*biz.Channel {
	if len(candidates) == 0 {
		return nil
	}

	seen := make(map[int]bool)
	channels := make([]*biz.Channel, 0, len(candidates))

	for _, c := range candidates {
		if !seen[c.Channel.ID] {
			channels = append(channels, c.Channel)
			seen[c.Channel.ID] = true
		}
	}

	return channels
}

// findCandidateForChannel finds the first candidate matching the given channel.
// Returns nil if no candidate is found for the channel.
func findCandidateForChannel(candidates []*ChannelModelCandidate, channel *biz.Channel) *ChannelModelCandidate {
	for _, c := range candidates {
		if c.Channel.ID == channel.ID {
			return c
		}
	}

	return nil
}

// resolveAxonHubModel creates a middleware that resolves AxonHub Models to channel+model candidates.
// This runs after API key model mapping and before channel selection.
func resolveAxonHubModel(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("resolve-axonhub-model", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		// Skip if already resolved
		if inbound.state.AxonHubModel != nil || len(inbound.state.ChannelModelCandidates) > 0 {
			return llmRequest, nil
		}

		// Skip if ModelResolver is not available
		if inbound.state.ModelResolver == nil {
			return llmRequest, nil
		}

		// Try to resolve AxonHub Model
		candidates, axonhubModel, err := inbound.state.ModelResolver.Resolve(ctx, llmRequest.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve AxonHub Model: %w", err)
		}

		// If no AxonHub Model found, continue with legacy flow
		if axonhubModel == nil {
			log.Debug(ctx, "No AxonHub Model found, using legacy channel selection",
				log.String("model", llmRequest.Model))

			return llmRequest, nil
		}

		// Store AxonHub Model and candidates in state
		inbound.state.AxonHubModel = axonhubModel
		inbound.state.ChannelModelCandidates = candidates

		if len(candidates) > 0 {
			log.Debug(ctx, "Resolved AxonHub Model to candidates",
				log.String("model", llmRequest.Model),
				log.Int("candidate_count", len(candidates)))
		}

		return llmRequest, nil
	})
}

// selectChannelsFromCandidates handles channel selection when AxonHub Model candidates are available.
func selectChannelsFromCandidates(ctx context.Context, inbound *PersistentInboundTransformer, llmRequest *llm.Request) (*llm.Request, error) {
	candidates := inbound.state.ChannelModelCandidates

	// Apply API Key Profile filtering
	if profile := GetActiveProfile(inbound.state.APIKey); profile != nil {
		// Filter by ChannelIDs
		if len(profile.ChannelIDs) > 0 {
			candidates = filterCandidatesByChannelIDs(candidates, profile.ChannelIDs)
		}

		// Filter by ChannelTags
		if len(profile.ChannelTags) > 0 {
			candidates = filterCandidatesByChannelTags(candidates, profile.ChannelTags)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no valid candidates after profile filtering for model %s", biz.ErrInvalidModel, llmRequest.Model)
	}

	// Sort candidates by priority and load balancer score
	if inbound.state.LoadBalancer != nil {
		candidates = sortCandidatesByPriorityAndScore(ctx, candidates, inbound.state.LoadBalancer)
	}

	// Store sorted candidates
	inbound.state.ChannelModelCandidates = candidates

	// Extract channels for compatibility with existing retry logic
	channels := extractChannelsFromCandidates(candidates)
	inbound.state.Channels = channels

	log.Debug(ctx, "selected channels from AxonHub Model candidates",
		log.Int("candidate_count", len(candidates)),
		log.Int("channel_count", len(channels)),
		log.String("model", llmRequest.Model))

	return llmRequest, nil
}

// selectChannelsLegacy handles legacy channel selection when no AxonHub Model is found.
func selectChannelsLegacy(ctx context.Context, inbound *PersistentInboundTransformer, llmRequest *llm.Request) (*llm.Request, error) {
	selector := inbound.state.ChannelSelector

	if profile := GetActiveProfile(inbound.state.APIKey); profile != nil {
		// 先应用 ChannelIDs 过滤
		if len(profile.ChannelIDs) > 0 {
			selector = NewSelectedChannelsSelector(selector, profile.ChannelIDs)
		}

		// 再应用 ChannelTags 过滤（链式装饰器，与 IDs 取交集）
		if len(profile.ChannelTags) > 0 {
			selector = NewTagsFilterSelector(selector, profile.ChannelTags)
		}
	}

	// 应用 Google 原生工具过滤（仅对 Gemini 原生 API 格式生效）
	if inbound.APIFormat() == llm.APIFormatGeminiContents {
		selector = NewGoogleNativeToolsSelector(selector)
	}

	// 应用 Anthropic 原生工具过滤（对所有 API 格式生效）
	// 无论通过 OpenAI 还是 Anthropic 格式入口，只要包含 web_search 工具，
	// 都需要优先路由到支持 Anthropic 原生工具的渠道
	selector = NewAnthropicNativeToolsSelector(selector)

	if inbound.state.LoadBalancer != nil {
		selector = NewLoadBalancedSelector(selector, inbound.state.LoadBalancer)
	}

	candidates, err := selector.Select(ctx, llmRequest)
	if err != nil {
		return nil, err
	}

	log.Debug(ctx, "selected candidates (legacy)",
		log.Int("candidate_count", len(candidates)),
		log.String("model", llmRequest.Model),
	)

	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w: no valid candidates found for model %s", biz.ErrInvalidModel, llmRequest.Model)
	}

	// Extract channels from candidates for backward compatibility
	channels := extractChannelsFromCandidates(candidates)
	inbound.state.Channels = channels

	return llmRequest, nil
}

// chooseModelFromCandidates chooses the model for the current channel from AxonHub Model candidates.
func chooseModelFromCandidates(outbound *PersistentOutboundTransformer, ctx context.Context, llmRequest *llm.Request) (string, error) {
	// Find the candidate for the current channel
	candidate := findCandidateForChannel(outbound.state.ChannelModelCandidates, outbound.state.CurrentChannel)
	if candidate != nil {
		outbound.state.CurrentCandidate = candidate
		log.Debug(ctx, "using pre-resolved model from AxonHub Model candidate",
			log.String("request_model", candidate.RequestModel),
			log.String("actual_model", candidate.ActualModel),
			log.Int("priority", candidate.Priority))

		return candidate.ActualModel, nil
	}

	// Fallback to legacy model resolution
	return outbound.state.CurrentChannel.ChooseModel(llmRequest.Model)
}
