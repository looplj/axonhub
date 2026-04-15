package orchestrator

import (
	"context"
	"fmt"
	"math/rand/v2"
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

// ChannelModelsCandidate represents a resolved channel and its matched model entries.
type ChannelModelsCandidate struct {
	Channel  *biz.Channel
	Priority int
	Models   []biz.ChannelModelEntry
}

// resolvedAssociationCandidate keeps the association-level metadata produced by
// resolution so request-dependent filtering can run afterwards without mixing
// conditional logic into association matching.
type resolvedAssociationCandidate struct {
	channel  *biz.Channel
	priority int
	models   []biz.ChannelModelEntry
	when     *objects.ModelAssociationWhen
}

// CandidateSelector defines the interface for selecting channel model candidates.
type CandidateSelector interface {
	Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error)
}

// associationCacheEntry stores cached association resolution results.
type associationCacheEntry struct {
	associations            []*objects.ModelAssociation
	candidates              []*resolvedAssociationCandidate
	channelCount            int
	latestChannelUpdateTime time.Time
	latestModelUpdateTime   time.Time
	channelCacheVersion     int64
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

func (s *DefaultSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
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
func (s *DefaultSelector) selectChannelCadidates(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	channels := s.ChannelService.GetEnabledChannels()

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
		log.Debug(ctx, "selected channel candidates for model",
			log.String("model", req.Model),
			log.Int("count", len(candidates)),
			log.Any("candidates", candidates),
		)
	}

	return candidates, nil
}

func (s *DefaultSelector) selectModelCandidates(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	model, err := s.ModelService.GetModelByModelID(ctx, req.Model, model.StatusEnabled)
	if err != nil {
		return nil, fmt.Errorf("failed to query AxonHub Model: %w", err)
	}

	if model.Settings == nil || len(model.Settings.Associations) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "model has no associations", log.String("model", req.Model))
		}

		return []*ChannelModelsCandidate{}, nil
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "model associations found",
			log.String("model", req.Model),
			log.Int("association_count", len(model.Settings.Associations)),
			log.Any("associations", model.Settings.Associations),
		)
	}

	resolvedCandidates, err := s.resolveAssociations(ctx, model, model.Settings.Associations)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve associations: %w", err)
	}

	candidates := filterResolvedCandidatesForRequest(ctx, req, resolvedCandidates)
	if len(candidates) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "no candidates matched request conditions",
				log.String("model", req.Model),
			)
		}

		return []*ChannelModelsCandidate{}, nil
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

// resolveAssociations resolves model associations into an intermediate form that
// still retains each association's `When` condition. The caller can then apply
// request-specific filtering in a dedicated pass after structural matching.
// Results are cached per model ID and invalidated when channel count, latest
// update time, or model update time changes.
func (s *DefaultSelector) resolveAssociations(
	ctx context.Context,
	model *ent.Model,
	associations []*objects.ModelAssociation,
) ([]*resolvedAssociationCandidate, error) {
	// Read version before channels to avoid storing an older channel snapshot with
	// a newer cache version if the enabled-channels cache swaps between the reads.
	// The inverse interleaving only causes a conservative cache miss.
	channelCacheVersion := s.ChannelService.GetCacheVersion()
	channels := s.ChannelService.GetEnabledChannels()
	if len(channels) == 0 {
		return []*resolvedAssociationCandidate{}, nil
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "resolving associations",
			log.String("model", model.ModelID),
			log.Int("enabled_channels", len(channels)),
			log.Any("channel_names", lo.Map(channels, func(ch *biz.Channel, _ int) string { return ch.Name })),
		)
	}

	// Use model ID as cache key
	modelID := model.ModelID
	channelCount := len(channels)
	latestChannelUpdateTime := s.getLatestChannelUpdateTime(channels)
	latestModelUpdateTime := model.UpdatedAt

	// Try to get from cache
	s.cacheMu.RLock()

	if entry, ok := s.associationCache[modelID]; ok {
		// Check if cache is still valid:
		// 1. Channel cache version hasn't changed (most reliable: detects any cache swap)
		// 2. Channel count hasn't changed
		// 3. No channel has been updated
		// 4. Model hasn't been updated
		// 5. Cache hasn't expired (5 minutes)
		if entry.channelCacheVersion == channelCacheVersion &&
			entry.channelCount == channelCount &&
			entry.latestChannelUpdateTime.Equal(latestChannelUpdateTime) &&
			entry.latestModelUpdateTime.Equal(latestModelUpdateTime) &&
			time.Since(entry.cachedAt) < associationCacheTTL {
			s.cacheMu.RUnlock()

			if log.DebugEnabled(ctx) {
				log.Debug(ctx, "using cached association resolution",
					log.String("model_id", modelID),
					log.Int("candidates", len(entry.candidates)),
					log.Duration("age", time.Since(entry.cachedAt)))
			}

			return entry.candidates, nil
		}
	}

	s.cacheMu.RUnlock()

	// Cache miss or invalid, resolve associations first. Request-specific `When`
	// filtering is intentionally deferred to a separate pass afterwards.
	matches := biz.MatchAssociations(associations, channels)

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "association matching results",
			log.String("model", model.ModelID),
			log.Int("matched_associations", len(matches)),
			log.Any("connections", lo.FlatMap(matches, func(match *biz.AssociationMatch, _ int) []map[string]any {
				return lo.Map(match.Connections, func(conn *biz.ModelChannelConnection, _ int) map[string]any {
					return map[string]any{
						"channel_id":   conn.Channel.ID,
						"channel_name": conn.Channel.Name,
						"priority":     conn.Priority,
						"model_count":  len(conn.Models),
						"has_when":     match.Association != nil && match.Association.When != nil,
						"models": lo.Map(conn.Models, func(entry biz.ChannelModelEntry, _ int) map[string]any {
							return map[string]any{
								"request_model": entry.RequestModel,
								"actual_model":  entry.ActualModel,
							}
						}),
					}
				})
			})),
		)
	}

	// Build channel lookup map for O(1) access
	channelMap := make(map[int]*biz.Channel, len(channels))
	for _, ch := range channels {
		channelMap[ch.ID] = ch
	}

	resolvedCandidates := make([]*resolvedAssociationCandidate, 0, len(matches))
	for _, match := range matches {
		for _, conn := range match.Connections {
			bizCh, found := channelMap[conn.Channel.ID]
			if !found || bizCh == nil {
				continue
			}

			resolvedCandidates = append(resolvedCandidates, &resolvedAssociationCandidate{
				channel:  bizCh,
				priority: conn.Priority,
				models:   append([]biz.ChannelModelEntry(nil), conn.Models...),
				when:     match.Association.When,
			})
		}
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "resolved association candidates",
			log.String("model", modelID),
			log.Int("resolved_candidates", len(resolvedCandidates)),
			log.Any("resolved_candidates_detail", lo.Map(resolvedCandidates, func(candidate *resolvedAssociationCandidate, _ int) map[string]any {
				return map[string]any{
					"channel_id":   candidate.channel.ID,
					"channel_name": candidate.channel.Name,
					"priority":     candidate.priority,
					"model_count":  len(candidate.models),
					"has_when":     candidate.when != nil,
				}
			})),
		)
	}

	// Update cache
	s.cacheMu.Lock()
	s.associationCache[modelID] = &associationCacheEntry{
		associations:            append([]*objects.ModelAssociation(nil), associations...),
		candidates:              resolvedCandidates,
		channelCount:            channelCount,
		latestChannelUpdateTime: latestChannelUpdateTime,
		latestModelUpdateTime:   latestModelUpdateTime,
		channelCacheVersion:     channelCacheVersion,
		cachedAt:                time.Now(),
	}
	s.cacheMu.Unlock()

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "cached association resolution",
			log.String("cache_key", model.ModelID),
			log.Int("candidates", len(resolvedCandidates)))
	}

	return resolvedCandidates, nil
}

func aggregateChannelModelCandidates(resolvedCandidates []*resolvedAssociationCandidate) []*ChannelModelsCandidate {
	type candidateKey struct {
		channelID int
		priority  int
	}

	type channelModelKey struct {
		channelID   int
		actualModel string
	}

	candidates := make([]*ChannelModelsCandidate, 0, len(resolvedCandidates))
	candidateIndexByKey := make(map[candidateKey]int, len(resolvedCandidates))
	seenChannelModels := make(map[channelModelKey]struct{}, len(resolvedCandidates))

	for _, resolved := range resolvedCandidates {
		if resolved == nil || resolved.channel == nil {
			continue
		}

		key := candidateKey{channelID: resolved.channel.ID, priority: resolved.priority}

		modelsToAppend := make([]biz.ChannelModelEntry, 0, len(resolved.models))
		for _, entry := range resolved.models {
			modelKey := channelModelKey{
				channelID:   resolved.channel.ID,
				actualModel: entry.ActualModel,
			}
			if _, exists := seenChannelModels[modelKey]; exists {
				continue
			}

			seenChannelModels[modelKey] = struct{}{}

			modelsToAppend = append(modelsToAppend, entry)
		}

		if len(modelsToAppend) == 0 {
			continue
		}

		idx, ok := candidateIndexByKey[key]
		if !ok {
			candidates = append(candidates, &ChannelModelsCandidate{
				Channel:  resolved.channel,
				Priority: resolved.priority,
				Models:   []biz.ChannelModelEntry{},
			})
			idx = len(candidates) - 1
			candidateIndexByKey[key] = idx
		}

		candidates[idx].Models = append(candidates[idx].Models, modelsToAppend...)
	}

	return candidates
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

func (s *SelectedChannelsSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
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
	filtered := lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		_, ok := allowedSet[c.Channel.ID]
		return ok
	})

	return filtered, nil
}

// LoadBalancedSelector is a decorator that sorts candidates using load balancing strategies.
type LoadBalancedSelector struct {
	wrapped               CandidateSelector
	loadBalancer          *LoadBalancer
	policy                RetryPolicyProvider
	systemService         *biz.SystemService
	channelRequestTracker *ChannelRequestTracker

	// explorationState tracks round-robin rotation for exploration candidates.
	explorationState struct {
		mu     sync.Mutex
		counts map[string]int // Per-model exploration counts
	}

	// metricsSamplingState tracks round-robin rotation for alternative candidates.
	metricsSamplingState struct {
		mu     sync.Mutex
		counts map[string]int // Per-model alternative selection counts
	}
}

// WithLoadBalancedSelector creates a selector that applies load balancing to sort candidates.
// The policy is used to determine the retry policy for early stopping.
// The systemService and channelRequestTracker are optional dependencies for advanced load balancing features.
func WithLoadBalancedSelector(wrapped CandidateSelector, loadBalancer *LoadBalancer, policy RetryPolicyProvider, systemService *biz.SystemService, channelRequestTracker *ChannelRequestTracker) *LoadBalancedSelector {
	return &LoadBalancedSelector{
		wrapped:               wrapped,
		loadBalancer:          loadBalancer,
		policy:                policy,
		systemService:         systemService,
		channelRequestTracker: channelRequestTracker,
		explorationState: struct {
			mu     sync.Mutex
			counts map[string]int
		}{
			counts: make(map[string]int),
		},
		metricsSamplingState: struct {
			mu     sync.Mutex
			counts map[string]int
		}{
			counts: make(map[string]int),
		},
	}
}

func (s *LoadBalancedSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
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
	priorityGroups := make(map[int][]*ChannelModelsCandidate)
	for _, c := range candidates {
		priorityGroups[c.Priority] = append(priorityGroups[c.Priority], c)
	}

	// Get sorted priority keys (lower priority value = higher priority)
	priorities := lo.Keys(priorityGroups)

	// Sort priorities: lower value = higher priority
	slices.Sort(priorities)

	// For each priority group, apply load balancing to sort candidates within the group
	// Stop early if we have collected enough candidates
	var result []*ChannelModelsCandidate

	for _, p := range priorities {
		group := priorityGroups[p]

		// Apply load balancing to sort candidates within this priority group.
		useStream := req.Stream != nil && *req.Stream
		sortedCandidates := s.loadBalancer.Sort(ctx, group, req.Model, useStream)

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

	// Exploration mechanism: insert cold channels that have no model-specific data
	// into position 1 (after the winner) so they are tried as a primary fallback,
	// not just as a deep retry slot. This ensures newly added channels get traffic
	// and can gather metrics before being evaluated on performance.
	if len(result) >= 2 {
		selectedSet := make(map[int]bool)
		for _, c := range result {
			selectedSet[c.Channel.ID] = true
		}

		var explorationCandidates []*ChannelModelsCandidate
		for _, c := range candidates {
			if selectedSet[c.Channel.ID] {
				continue
			}
			if s.needsExploration(ctx, c, req.Model) {
				explorationCandidates = append(explorationCandidates, c)
			}
		}

		if len(explorationCandidates) > 0 {
			explorationCandidate := s.selectExplorationCandidate(req.Model, explorationCandidates)

			// Insert at position 1 (after the winner) instead of replacing the last
			// retry slot. This makes cold channels visible as a primary fallback.
			result = slices.Insert(result, 1, explorationCandidate)

			// Trim back to requiredCount to avoid exceeding the configured retry limit.
			if len(result) > requiredCount {
				result = result[:requiredCount]
			}

			log.Info(ctx, "Exploration slot assigned to cold channel",
				log.String("model", req.Model),
				log.Int("channel_id", explorationCandidate.Channel.ID),
				log.String("channel_name", explorationCandidate.Channel.Name),
				log.Int("priority", explorationCandidate.Priority))
		}
	}

	if len(result) > 0 && s.systemService != nil && s.channelRequestTracker != nil {
		metricsSamplingConfig := s.systemService.MetricsSamplingOrDefault(ctx)

		if shouldTriggerMetricsSampling(
			ctx, metricsSamplingConfig, result[0], s.channelRequestTracker, s.loadBalancer,
		) {
			winnerID := result[0].Channel.ID

			// Get winner's score from load balancer strategies for threshold comparison
			winnerScore := s.getCandidateScore(ctx, result[0])
			winnerRequestRate := int(s.channelRequestTracker.GetRequestCount(winnerID))

			// Check if we should sample based on thresholds
			triggerReason := getTriggerReason(metricsSamplingConfig, winnerScore, winnerRequestRate)

			if triggerReason != "" {
				// Check probabilistic sampling BEFORE selecting the alternative
				// to avoid advancing the round-robin counter on rejections.
				// First check with base rate; if it fails, compute the adaptive
				// rate assuming the worst case (0 requests) to give cold channels
				// a chance even when the base rate would reject them.
				effectiveRate := metricsSamplingConfig.SamplingRate
				shouldSample := shouldSampleProbabilistically(effectiveRate)

				if !shouldSample {
					effectiveRate = effectiveSamplingRate(metricsSamplingConfig.SamplingRate, 0)
					shouldSample = shouldSampleProbabilistically(effectiveRate)
				}

				if shouldSample {
					alternative := s.selectAlternativeCandidate(
						ctx,
						req.Model,
						result,
						winnerID,
						metricsSamplingConfig.AlternativeCount,
						triggerReason,
						metricsSamplingConfig.SamplingRate,
						result[0].Channel.Name,
					)

					if alternative != nil {
						result = append(result, alternative)
					}
				} else {
					if log.DebugEnabled(ctx) {
						log.Debug(ctx, "Metrics sampling: probabilistic check skipped sampling",
							log.String("model", req.Model),
							log.Float64("sampling_rate", metricsSamplingConfig.SamplingRate),
							log.Float64("effective_rate", effectiveRate))
					}
				}
			}
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

// needsExploration checks if a candidate has no model-specific performance data.
// This indicates the channel has never been used for this model.
func (s *LoadBalancedSelector) needsExploration(ctx context.Context, candidate *ChannelModelsCandidate, model string) bool {
	// Use the load balancer's strategy to check for model-specific data
	if s.loadBalancer == nil || len(s.loadBalancer.strategies) == 0 {
		return false
	}

	// Check if the performance-aware strategy has metrics for this channel/model
	for _, strategy := range s.loadBalancer.strategies {
		if perf, ok := strategy.(*PerformanceAwareStrategy); ok {
			return perf.NeedsModelData(ctx, candidate.Channel.ID, model)
		}
	}
	return false
}

// selectExplorationCandidate picks an exploration candidate using round-robin rotation.
// This ensures fair distribution of exploration traffic across cold channels.
func (s *LoadBalancedSelector) selectExplorationCandidate(model string, candidates []*ChannelModelsCandidate) *ChannelModelsCandidate {
	s.explorationState.mu.Lock()
	defer s.explorationState.mu.Unlock()

	// Get or initialize the count for this model
	count := s.explorationState.counts[model]

	// Use round-robin rotation based on the count
	index := count % len(candidates)
	selected := candidates[index]

	// Increment the rotation counter
	s.explorationState.counts[model] = count + 1

	return selected
}

// selectAlternativeCandidate selects an alternative candidate using round-robin rotation.
// It selects from the top N runner-up candidates (ranks 2 to N+1), skipping any that are in cooldown.
// If all alternatives are unavailable, it returns nil to fall back to the original winner.
func (s *LoadBalancedSelector) selectAlternativeCandidate(
	ctx context.Context,
	model string,
	candidates []*ChannelModelsCandidate,
	excludedID int,
	alternativeCount int,
	triggerReason string,
	samplingRate float64,
	originalChannelName string,
) *ChannelModelsCandidate {
	if len(candidates) == 0 || alternativeCount <= 0 {
		return nil
	}

	// Filter to get runner-up candidates (not the winner, not in cooldown)
	var alternatives []*ChannelModelsCandidate
	for _, c := range candidates {
		if c.Channel.ID == excludedID {
			continue
		}
		if s.channelRequestTracker != nil && s.channelRequestTracker.IsCoolingDown(c.Channel.ID) {
			continue
		}
		alternatives = append(alternatives, c)
		if len(alternatives) >= alternativeCount {
			break
		}
	}

	if len(alternatives) == 0 {
		return nil
	}

	s.metricsSamplingState.mu.Lock()
	defer s.metricsSamplingState.mu.Unlock()

	// Get or initialize the count for this model
	count := s.metricsSamplingState.counts[model]

	// Use round-robin rotation based on the count
	index := count % len(alternatives)
	selected := alternatives[index]

	// Increment the rotation counter
	s.metricsSamplingState.counts[model] = count + 1

	log.Info(ctx, "Metrics sampling: alternative channel selected",
		log.String("model", model),
		log.Int("original_channel_id", excludedID),
		log.String("original_channel_name", originalChannelName),
		log.Int("alternative_channel_id", selected.Channel.ID),
		log.String("alternative_channel_name", selected.Channel.Name),
		log.String("trigger_reason", triggerReason),
		log.Float64("sampling_rate", samplingRate))

	return selected
}

// shouldTriggerMetricsSampling checks if metrics sampling prerequisites are met.
// Returns false if required dependencies are missing.
func shouldTriggerMetricsSampling(
	ctx context.Context,
	config *biz.MetricsSamplingConfig,
	winner *ChannelModelsCandidate,
	requestTracker *ChannelRequestTracker,
	loadBalancer *LoadBalancer,
) bool {
	if config == nil || !config.Enabled {
		return false
	}

	if config.AlwaysSample {
		return true
	}

	// Need at least one threshold configured
	if config.ScoreThreshold <= 0 && config.RequestRateThreshold <= 0 {
		return false
	}

	return true
}

// getTriggerReason determines if metrics sampling should trigger based on actual metrics.
// Returns empty string if neither threshold is exceeded.
func getTriggerReason(config *biz.MetricsSamplingConfig, winnerScore float64, winnerRequestRate int) string {
	if config == nil {
		return ""
	}

	// AlwaysSample is handled separately in shouldTriggerMetricsSampling
	if config.AlwaysSample {
		return "always_flag"
	}

	// Check score threshold (if configured)
	if config.ScoreThreshold > 0 && winnerScore >= config.ScoreThreshold {
		return "score_threshold"
	}

	// Check request rate threshold (if configured)
	if config.RequestRateThreshold > 0 && winnerRequestRate >= config.RequestRateThreshold {
		return "rate_threshold"
	}

	// No threshold exceeded
	return ""
}

// shouldSampleProbabilistically applies probabilistic sampling based on configured rate.
// Returns true if random value is below the sampling rate (e.g., 0.10 = 10% chance).
//
// #nosec G404 -- Use of math/rand is intentional here; cryptographic randomness is not required
// for load balancing sampling decisions. Predictability is acceptable for this use case.
func shouldSampleProbabilistically(samplingRate float64) bool {
	return rand.Float64() < samplingRate
}

// getCandidateScore calculates the load balancer score for a candidate.
// Returns the sum of all strategy scores.
func (s *LoadBalancedSelector) getCandidateScore(ctx context.Context, candidate *ChannelModelsCandidate) float64 {
	if s.loadBalancer == nil || candidate == nil || candidate.Channel == nil {
		return 0
	}

	return s.loadBalancer.CalculateScore(ctx, candidate.Channel)
}

// effectiveSamplingRate returns an adaptive sampling rate that scales up when the
// alternative channel has fewer than ColdStartMinRequests requests.
// A channel with 0 requests gets near-100% sampling; the rate tapers to the
// configured base rate as the channel accumulates data past ColdStartMinRequests.
func effectiveSamplingRate(baseRate float64, requestCount int64) float64 {
	if baseRate <= 0 {
		return 0
	}
	if requestCount >= int64(ColdStartMinRequests) {
		return baseRate
	}
	deficit := 1.0 - float64(requestCount)/float64(ColdStartMinRequests)
	if deficit < 0 {
		deficit = 0
	}
	return baseRate + (1.0-baseRate)*deficit
}

// TagsFilterSelector is a decorator that filters candidates by allowed channel tags.
type TagsFilterSelector struct {
	wrapped   CandidateSelector
	tags      []string
	matchMode objects.ChannelTagsMatchMode
}

// WithChannelTagsFilterSelector creates a selector that filters by tags and match mode.
// If tags is empty, all candidates from the wrapped selector are returned.
func WithChannelTagsFilterSelector(wrapped CandidateSelector, tags []string, matchMode objects.ChannelTagsMatchMode) *TagsFilterSelector {
	return &TagsFilterSelector{
		wrapped:   wrapped,
		tags:      tags,
		matchMode: matchMode,
	}
}

func (s *TagsFilterSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	candidates, err := s.wrapped.Select(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(s.tags) == 0 {
		return candidates, nil
	}

	candidates = lo.Filter(candidates, func(c *ChannelModelsCandidate, _ int) bool {
		return matchChannelTagsFilter(s.tags, s.matchMode, c.Channel.Tags)
	})

	return candidates, nil
}

func matchChannelTagsFilter(allowedTags []string, matchMode objects.ChannelTagsMatchMode, channelTags []string) bool {
	return objects.MatchChannelTags(allowedTags, matchMode, channelTags)
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

func (s *SpecifiedChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	channel, err := s.ChannelService.GetChannel(ctx, s.ChannelID.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel for test: %w", err)
	}

	entries := channel.GetDirectModelEntries()

	entry, ok := entries[req.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not supported in channel %s", req.Model, channel.Name)
	}

	candidate := &ChannelModelsCandidate{
		Channel:  channel,
		Priority: 0,
		Models:   []biz.ChannelModelEntry{entry},
	}

	return []*ChannelModelsCandidate{candidate}, nil
}
