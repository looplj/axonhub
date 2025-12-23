package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

// ChannelModelCandidate represents a resolved channel and model pair.
type ChannelModelCandidate struct {
	Channel      *biz.Channel
	RequestModel string
	ActualModel  string
	Priority     int
}

// CandidateSelector defines the interface for selecting channel model candidates.
type CandidateSelector interface {
	Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error)
}

// DefaultSelector directly selects enabled channels supporting the requested model.
type DefaultSelector struct {
	ChannelService *biz.ChannelService
	ModelService   *biz.ModelService // Optional: for AxonHub Model resolution
}

func NewDefaultSelector(channelService *biz.ChannelService, modelService *biz.ModelService) *DefaultSelector {
	return &DefaultSelector{
		ChannelService: channelService,
		ModelService:   modelService,
	}
}

func (s *DefaultSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.selectModelCandidates(ctx, req.Model)
	if err != nil {
		if ent.IsNotFound(err) {
			// Fallback to legacy channel selection
			// TODO: add a setting to enable/disable legacy channel selection
			return s.selectChannelCadidates(ctx, req)
		}

		return nil, err
	}

	return candidates, nil
}

// selectChannelCadidates performs the original channel selection logic.
func (s *DefaultSelector) selectChannelCadidates(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	channels := s.ChannelService.GetEnabledChannels()

	// Single pass: filter channels supporting the model and create candidates
	candidates := make([]*ChannelModelCandidate, 0, len(channels))
	for _, ch := range channels {
		entries := ch.GetModelEntries()

		entry, ok := entries[req.Model]
		if !ok {
			continue
		}

		candidates = append(candidates, &ChannelModelCandidate{
			Channel:      ch,
			RequestModel: entry.RequestModel,
			ActualModel:  entry.ActualModel,
			Priority:     0,
		})
	}

	return candidates, nil
}

// selectModelCandidates attempts to resolve a model name to AxonHub Model associations.
// Returns candidates and the AxonHub Model if found, or nil values for fallback to legacy behavior.
func (s *DefaultSelector) selectModelCandidates(ctx context.Context, modelName string) ([]*ChannelModelCandidate, error) {
	// Query enabled AxonHub Model by model ID
	axonhubModel, err := s.ModelService.GetModelByModelID(ctx, modelName, model.StatusEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to query AxonHub Model: %w", err)
	}

	// Model found, check for associations
	if axonhubModel.Settings == nil || len(axonhubModel.Settings.Associations) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "model has no associations", log.String("model", modelName))
		}

		return []*ChannelModelCandidate{}, nil
	}

	// Resolve associations to candidates
	candidates, err := s.resolveAssociations(ctx, axonhubModel.Settings.Associations)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve associations: %w", err)
	}

	if len(candidates) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "no candidates found for model", log.String("model", modelName))
		}
	}

	return candidates, nil
}

// resolveAssociations uses biz.MatchAssociations to resolve model associations
// and converts the results to ChannelModelCandidate.
func (s *DefaultSelector) resolveAssociations(ctx context.Context, associations []*objects.ModelAssociation) ([]*ChannelModelCandidate, error) {
	channels := s.ChannelService.GetEnabledChannels()
	if len(channels) == 0 {
		return []*ChannelModelCandidate{}, nil
	}

	connections, err := biz.MatchAssociations(ctx, associations, channels)
	if err != nil {
		return nil, fmt.Errorf("failed to match associations: %w", err)
	}

	// Build channel lookup map for O(1) access
	channelMap := make(map[int]*biz.Channel, len(channels))
	for _, ch := range channels {
		channelMap[ch.ID] = ch
	}

	candidates := make([]*ChannelModelCandidate, 0, len(connections))
	for _, conn := range connections {
		bizCh, found := channelMap[conn.Channel.ID]
		if !found || bizCh == nil {
			continue
		}

		// Models are already resolved in MatchAssociations, no need for second lookup
		for _, entry := range conn.Models {
			candidates = append(candidates, &ChannelModelCandidate{
				Channel:      bizCh,
				RequestModel: entry.RequestModel,
				ActualModel:  entry.ActualModel,
				Priority:     conn.Priority,
			})
		}
	}

	return candidates, nil
}

// SelectedChannelsSelector is a decorator that filters candidates by allowed channel IDs.
type SelectedChannelsSelector struct {
	wrapped           CandidateSelector
	allowedChannelIDs []int
}

// WithSelectedChannelsSelector creates a selector that filters by allowed channel IDs.
// If allowedChannelIDs is nil or empty, all candidates from the wrapped selector are returned.
func WithSelectedChannelsSelector(wrapped CandidateSelector, allowedChannelIDs []int) *SelectedChannelsSelector {
	return &SelectedChannelsSelector{
		wrapped:           wrapped,
		allowedChannelIDs: allowedChannelIDs,
	}
}

func (s *SelectedChannelsSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// If no allowed IDs specified, return all candidates
	if len(s.allowedChannelIDs) == 0 {
		return candidates, nil
	}

	// Build allowed set for O(1) lookup
	allowedSet := lo.SliceToMap(s.allowedChannelIDs, func(id int) (int, struct{}) {
		return id, struct{}{}
	})

	// Filter candidates by allowed channel IDs
	filtered := lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		_, ok := allowedSet[c.Channel.ID]
		return ok
	})

	return filtered, nil
}

// LoadBalancedSelector is a decorator that sorts candidates using load balancing strategies.
type LoadBalancedSelector struct {
	wrapped      CandidateSelector
	loadBalancer *LoadBalancer
}

// WithLoadBalancedSelector creates a selector that applies load balancing to sort candidates.
func WithLoadBalancedSelector(wrapped CandidateSelector, loadBalancer *LoadBalancer) *LoadBalancedSelector {
	return &LoadBalancedSelector{
		wrapped:      wrapped,
		loadBalancer: loadBalancer,
	}
}

func (s *LoadBalancedSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(candidates) <= 1 {
		return candidates, nil
	}

	// Apply load balancing to sort candidates
	sortedCandidates := sortCandidatesByPriorityAndScore(ctx, candidates, s.loadBalancer)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Load balanced candidates for model",
			log.String("model", req.Model),
			log.Int("total_candidates", len(candidates)),
			log.Int("sorted_candidates", len(sortedCandidates)))
	}

	return sortedCandidates, nil
}

// TagsFilterSelector is a decorator that filters candidates by allowed channel tags.
// Uses OR logic: a candidate passes if its channel contains any of the allowed tags.
type TagsFilterSelector struct {
	wrapped     CandidateSelector
	allowedTags []string
}

// WithTagsFilterSelector creates a selector that filters by tags.
// If allowedTags is empty, all candidates from the wrapped selector are returned.
func WithTagsFilterSelector(wrapped CandidateSelector, allowedTags []string) *TagsFilterSelector {
	return &TagsFilterSelector{
		wrapped:     wrapped,
		allowedTags: allowedTags,
	}
}

func (s *TagsFilterSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	// If no allowed tags specified, return all candidates
	if len(s.allowedTags) == 0 {
		return candidates, nil
	}

	// Build allowed set for O(1) lookup
	allowedSet := lo.SliceToMap(s.allowedTags, func(tag string) (string, struct{}) {
		return tag, struct{}{}
	})

	// Filter candidates: keep only those whose channel has at least one allowed tag (OR logic)
	filtered := lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		for _, tag := range c.Channel.Tags {
			if _, ok := allowedSet[tag]; ok {
				return true
			}
		}

		return false
	})

	return filtered, nil
}

// SpecifiedChannelSelector allows selecting specific channels (including disabled ones) for testing.
type SpecifiedChannelSelector struct {
	ChannelService *biz.ChannelService
	ChannelID      objects.GUID
}

func NewSpecifiedChannelSelector(channelService *biz.ChannelService, channelID objects.GUID) *SpecifiedChannelSelector {
	return &SpecifiedChannelSelector{
		ChannelService: channelService,
		ChannelID:      channelID,
	}
}

func (s *SpecifiedChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	channel, err := s.ChannelService.GetChannelForTest(ctx, s.ChannelID.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel for test: %w", err)
	}

	if !channel.IsModelSupported(req.Model) {
		return nil, fmt.Errorf("model %s not supported in channel %s", req.Model, channel.Name)
	}

	// Get model entry and create candidate
	entries := channel.GetModelEntries()

	entry, ok := entries[req.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not found in channel %s", req.Model, channel.Name)
	}

	candidate := &ChannelModelCandidate{
		Channel:      channel,
		RequestModel: entry.RequestModel,
		ActualModel:  entry.ActualModel,
		Priority:     0,
	}

	return []*ChannelModelCandidate{candidate}, nil
}
