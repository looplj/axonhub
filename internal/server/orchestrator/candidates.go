package orchestrator

import (
	"context"
	"fmt"

	"github.com/samber/lo"

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
}

// NewDefaultSelector creates a basic selector that returns all enabled channels supporting the model.
func NewDefaultSelector(channelService *biz.ChannelService) *DefaultSelector {
	return &DefaultSelector{
		ChannelService: channelService,
	}
}

func (s *DefaultSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
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

// SelectedChannelsSelector is a decorator that filters candidates by allowed channel IDs.
type SelectedChannelsSelector struct {
	wrapped           CandidateSelector
	allowedChannelIDs []int
}

// NewSelectedChannelsSelector creates a selector that filters by allowed channel IDs.
// If allowedChannelIDs is nil or empty, all candidates from the wrapped selector are returned.
func NewSelectedChannelsSelector(wrapped CandidateSelector, allowedChannelIDs []int) *SelectedChannelsSelector {
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

// NewLoadBalancedSelector creates a selector that applies load balancing to sort candidates.
func NewLoadBalancedSelector(wrapped CandidateSelector, loadBalancer *LoadBalancer) *LoadBalancedSelector {
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

// NewTagsFilterSelector creates a selector that filters by tags.
// If allowedTags is empty, all candidates from the wrapped selector are returned.
func NewTagsFilterSelector(wrapped CandidateSelector, allowedTags []string) *TagsFilterSelector {
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
