package orchestrator

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
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

// associationCacheEntry stores cached association resolution results.
type associationCacheEntry struct {
	candidates              []*ChannelModelCandidate
	channelCount            int
	latestChannelUpdateTime time.Time
	latestModelUpdatedAt    time.Time
	cachedAt                time.Time
}

const (
	// associationCacheTTL is the time-to-live for association cache entries.
	// After this duration, cache entries are invalidated even if channels haven't changed.
	associationCacheTTL = 5 * time.Minute
)

// DefaultSelector directly selects enabled channels supporting the requested model.
type DefaultSelector struct {
	ChannelService *biz.ChannelService
	ModelService   *biz.ModelService // Optional: for AxonHub Model resolution
	SystemService  *biz.SystemService

	// Association resolution cache
	cacheMu          sync.RWMutex
	associationCache map[string]*associationCacheEntry
}

func NewDefaultSelector(channelService *biz.ChannelService, modelService *biz.ModelService, systemService *biz.SystemService) *DefaultSelector {
	return &DefaultSelector{
		ChannelService:   channelService,
		ModelService:     modelService,
		SystemService:    systemService,
		associationCache: make(map[string]*associationCacheEntry),
	}
}

func (s *DefaultSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	candidates, err := s.selectModelCandidates(ctx, req)
	if err != nil {
		if ent.IsNotFound(err) {
			// Check if fallback to legacy channel selection is allowed
			settings := s.SystemService.ModelSettingsOrDefault(ctx)
			if settings.FallbackToChannelsOnModelNotFound {
				return s.selectChannelCadidates(ctx, req)
			}

			return nil, fmt.Errorf("%w: %q", biz.ErrInvalidModel, req.Model)
		}

		return nil, fmt.Errorf("%w: %q", err, req.Model)
	}

	return candidates, nil
}

// selectChannelCadidates performs the original channel selection logic.
func (s *DefaultSelector) selectChannelCadidates(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	channels := s.ChannelService.GetEnabledChannels()

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

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "selected channel candidates for model",
			log.String("model", req.Model),
			log.Int("count", len(candidates)),
			log.Any("candidates", candidates),
		)
	}

	return candidates, nil
}

func (s *DefaultSelector) selectModelCandidates(ctx context.Context, req *llm.Request) ([]*ChannelModelCandidate, error) {
	model, err := s.ModelService.GetModelByModelID(ctx, req.Model, model.StatusEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to query AxonHub Model: %w", err)
	}

	if model.Settings == nil || len(model.Settings.Associations) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "model has no associations", log.String("model", req.Model))
		}

		return []*ChannelModelCandidate{}, nil
	}

	candidates, err := s.resolveAssociations(ctx, model, model.Settings.Associations)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve associations: %w", err)
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "selected model candidates for model",
			log.String("model", req.Model),
			log.Int("count", len(candidates)),
			log.Any("candidates", candidates),
		)
	}

	return candidates, nil
}

// resolveAssociations uses biz.MatchAssociations to resolve model associations
// and converts the results to ChannelModelCandidate.
// Results are cached per model ID and invalidated when channel count, latest update time, or model update time changes.
func (s *DefaultSelector) resolveAssociations(ctx context.Context, model *ent.Model, associations []*objects.ModelAssociation) ([]*ChannelModelCandidate, error) {
	channels := s.ChannelService.GetEnabledChannels()
	if len(channels) == 0 {
		return []*ChannelModelCandidate{}, nil
	}

	// Use model ID as cache key
	modelID := model.ModelID
	channelCount := len(channels)
	latestChannelUpdateTime := s.getLatestChannelUpdateTime(channels)
	latestModelUpdatedAt := model.UpdatedAt

	// Try to get from cache
	s.cacheMu.RLock()

	if entry, ok := s.associationCache[modelID]; ok {
		// Check if cache is still valid:
		// 1. Channel count hasn't changed
		// 2. No channel has been updated
		// 3. Model hasn't been updated
		// 4. Cache hasn't expired (5 minutes)
		if entry.channelCount == channelCount &&
			entry.latestChannelUpdateTime.Equal(latestChannelUpdateTime) &&
			entry.latestModelUpdatedAt.Equal(latestModelUpdatedAt) &&
			time.Since(entry.cachedAt) < associationCacheTTL {
			s.cacheMu.RUnlock()

			if log.DebugEnabled(ctx) {
				log.Debug(ctx, "using cached association resolution",
					log.String("modelID", modelID),
					log.Int("candidates", len(entry.candidates)),
					log.Duration("age", time.Since(entry.cachedAt)))
			}

			return entry.candidates, nil
		}
	}

	s.cacheMu.RUnlock()

	// Cache miss or invalid, resolve associations
	connections := biz.MatchAssociations(associations, channels)

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

	// Update cache
	s.cacheMu.Lock()
	s.associationCache[modelID] = &associationCacheEntry{
		candidates:              candidates,
		channelCount:            channelCount,
		latestChannelUpdateTime: latestChannelUpdateTime,
		latestModelUpdatedAt:    latestModelUpdatedAt,
		cachedAt:                time.Now(),
	}
	s.cacheMu.Unlock()

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "cached association resolution",
			log.String("modelID", modelID),
			log.Int("candidates", len(candidates)))
	}

	return candidates, nil
}

// getLatestChannelUpdateTime returns the latest update time among all channels.
func (s *DefaultSelector) getLatestChannelUpdateTime(channels []*biz.Channel) time.Time {
	if len(channels) == 0 {
		return time.Time{}
	}

	latest := channels[0].UpdatedAt
	for _, ch := range channels[1:] {
		if ch.UpdatedAt.After(latest) {
			latest = ch.UpdatedAt
		}
	}

	return latest
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
	policy       RetryPolicyProvider
}

// WithLoadBalancedSelector creates a selector that applies load balancing to sort candidates.
// The policy is used to determine the retry policy for early stopping.
func WithLoadBalancedSelector(wrapped CandidateSelector, loadBalancer *LoadBalancer, policy RetryPolicyProvider) *LoadBalancedSelector {
	return &LoadBalancedSelector{
		wrapped:      wrapped,
		loadBalancer: loadBalancer,
		policy:       policy,
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

	// Get retry policy to determine the required number of candidates
	retryPolicy := s.policy.RetryPolicyOrDefault(ctx)

	requiredCount := 1
	if retryPolicy.Enabled {
		requiredCount = 1 + retryPolicy.MaxChannelRetries
	}

	// Group candidates by priority first (lower priority value = higher priority)
	priorityGroups := make(map[int][]*ChannelModelCandidate)
	for _, c := range candidates {
		priorityGroups[c.Priority] = append(priorityGroups[c.Priority], c)
	}

	// Get sorted priority keys (lower priority value = higher priority)
	priorities := lo.Keys(priorityGroups)

	// Sort priorities: lower value = higher priority
	slices.Sort(priorities)

	// For each priority group, apply load balancing to sort candidates within the group
	// Stop early if we have collected enough candidates
	var result []*ChannelModelCandidate

	for _, p := range priorities {
		group := priorityGroups[p]

		// Apply load balancing to sort candidates within this priority group.
		sortedCandidates := s.loadBalancer.Sort(ctx, group, req.Model)

		// Add candidates, but stop if we have enough
		remaining := requiredCount - len(result)
		if remaining <= 0 {
			break
		}

		if len(sortedCandidates) <= remaining {
			result = append(result, sortedCandidates...)
		} else {
			result = append(result, sortedCandidates[:remaining]...)
			break
		}
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "Load balanced candidates for model",
			log.String("model", req.Model),
			log.Int("total_candidates", len(candidates)),
			log.Int("sorted_candidates", len(result)),
			log.Int("required_count", requiredCount))
	}

	return result, nil
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
	candidates = lo.Filter(candidates, func(c *ChannelModelCandidate, _ int) bool {
		for _, tag := range c.Channel.Tags {
			if _, ok := allowedSet[tag]; ok {
				return true
			}
		}

		return false
	})

	return candidates, nil
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

	entries := channel.GetDirectModelEntries()

	entry, ok := entries[req.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not supported in channel %s", req.Model, channel.Name)
	}

	candidate := &ChannelModelCandidate{
		Channel:      channel,
		RequestModel: entry.RequestModel,
		ActualModel:  entry.ActualModel,
		Priority:     0,
	}

	return []*ChannelModelCandidate{candidate}, nil
}
