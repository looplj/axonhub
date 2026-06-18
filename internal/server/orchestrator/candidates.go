package orchestrator

import (
	"context"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
	"github.com/looplj/axonhub/llm"
)

// ChannelModelsCandidate represents a resolved channel and its matched model entries.
type ChannelModelsCandidate struct {
	Channel   *biz.Channel
	Priority  int
	Models    []biz.ChannelModelEntry
	APIFormat string // selected endpoint API format for this candidate
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
	associationSignature    string
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

		endpoints := ch.ResolveEndpoints()
		apiFormat := SelectAPIFormat(endpoints, req)

		candidates = append(candidates, &ChannelModelsCandidate{
			Channel:   ch,
			Priority:  0,
			Models:    []biz.ChannelModelEntry{entry},
			APIFormat: apiFormat,
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

	systemSettings := s.SystemService.ModelSettingsOrDefault(ctx)
	developerAssociationCount, modelAssociationCount, developerInheritanceDisabled := effectiveAssociationSourceCounts(systemSettings, model)
	associations := biz.EffectiveModelAssociations(systemSettings, model)
	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "computed effective model associations",
			log.String("model", model.ModelID),
			log.String("developer", model.Developer),
			log.Int("developer_association_count", developerAssociationCount),
			log.Int("model_association_count", modelAssociationCount),
			log.Bool("developer_inheritance_disabled", developerInheritanceDisabled),
			log.Int("effective_association_count", len(associations)),
		)
	}
	if len(associations) == 0 {
		if log.DebugEnabled(ctx) {
			log.Debug(ctx, "model has no associations", log.String("model", req.Model))
		}

		return []*ChannelModelsCandidate{}, nil
	}

	if log.DebugEnabled(ctx) {
		log.Debug(ctx, "model associations found",
			log.String("model", req.Model),
			log.Int("association_count", len(associations)),
			log.Any("associations", associations),
		)
	}

	resolvedCandidates, err := s.resolveAssociations(ctx, model, associations)
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
	associationSignature := modelAssociationSignature(associations)
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
			entry.associationSignature == associationSignature &&
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
		associationSignature:    associationSignature,
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

func modelAssociationSignature(associations []*objects.ModelAssociation) string {
	h := fnv.New64a()
	writeSignatureInt(h, len(associations))
	for _, assoc := range associations {
		writeAssociationSignature(h, assoc)
	}

	return strconv.FormatUint(h.Sum64(), 16)
}

func writeAssociationSignature(h hash.Hash64, assoc *objects.ModelAssociation) {
	if assoc == nil {
		writeSignatureString(h, "<nil>")
		return
	}

	writeSignatureString(h, assoc.Type)
	writeSignatureInt(h, assoc.Priority)
	writeSignatureBool(h, assoc.Disabled)
	writeAssociationWhenSignature(h, assoc.When)
	if assoc.ChannelModel != nil {
		writeSignatureString(h, "channelModel")
		writeSignatureInt(h, assoc.ChannelModel.ChannelID)
		writeSignatureString(h, assoc.ChannelModel.ModelID)
	}
	if assoc.ChannelRegex != nil {
		writeSignatureString(h, "channelRegex")
		writeSignatureInt(h, assoc.ChannelRegex.ChannelID)
		writeSignatureString(h, assoc.ChannelRegex.Pattern)
	}
	if assoc.Regex != nil {
		writeSignatureString(h, "regex")
		writeSignatureString(h, assoc.Regex.Pattern)
		writeExcludeSignature(h, assoc.Regex.Exclude)
	}
	if assoc.ModelID != nil {
		writeSignatureString(h, "modelId")
		writeSignatureString(h, assoc.ModelID.ModelID)
		writeExcludeSignature(h, assoc.ModelID.Exclude)
	}
	if assoc.ChannelTagsModel != nil {
		writeSignatureString(h, "channelTagsModel")
		writeStringSliceSignature(h, assoc.ChannelTagsModel.ChannelTags)
		writeSignatureString(h, assoc.ChannelTagsModel.ModelID)
	}
	if assoc.ChannelTagsRegex != nil {
		writeSignatureString(h, "channelTagsRegex")
		writeStringSliceSignature(h, assoc.ChannelTagsRegex.ChannelTags)
		writeSignatureString(h, assoc.ChannelTagsRegex.Pattern)
	}
}

func writeAssociationWhenSignature(h hash.Hash64, when *objects.ModelAssociationWhen) {
	if when == nil {
		writeSignatureString(h, "when:nil")
		return
	}

	writeSignatureString(h, "when")
	writeSignatureBool(h, when.Enabled)
	writeConditionSignature(h, when.Condition)
}

func writeConditionSignature(h hash.Hash64, condition *objects.Condition) {
	if condition == nil {
		writeSignatureString(h, "condition:nil")
		return
	}

	writeSignatureString(h, string(condition.Type))
	writeSignatureString(h, condition.Logic)
	writeSignatureString(h, condition.Field)
	writeSignatureString(h, condition.Operator)
	writeSignatureString(h, fmt.Sprintf("%T:%v", condition.Value, condition.Value))
	writeSignatureInt(h, len(condition.Conditions))
	for i := range condition.Conditions {
		writeConditionSignature(h, &condition.Conditions[i])
	}
}

func writeExcludeSignature(h hash.Hash64, excludes []*objects.ExcludeAssociation) {
	writeSignatureInt(h, len(excludes))
	for _, exclude := range excludes {
		if exclude == nil {
			writeSignatureString(h, "<nil>")
			continue
		}

		writeSignatureString(h, exclude.ChannelNamePattern)
		writeSignatureIntSlice(h, exclude.ChannelIds)
		writeStringSliceSignature(h, exclude.ChannelTags)
	}
}

func writeStringSliceSignature(h hash.Hash64, values []string) {
	writeSignatureInt(h, len(values))
	for _, value := range values {
		writeSignatureString(h, value)
	}
}

func writeSignatureIntSlice(h hash.Hash64, values []int) {
	writeSignatureInt(h, len(values))
	for _, value := range values {
		writeSignatureInt(h, value)
	}
}

func writeSignatureString(h hash.Hash64, value string) {
	_, _ = io.WriteString(h, value)
	_, _ = h.Write([]byte{0})
}

func writeSignatureInt(h hash.Hash64, value int) {
	writeSignatureString(h, strconv.Itoa(value))
}

func writeSignatureBool(h hash.Hash64, value bool) {
	if value {
		writeSignatureString(h, "1")
		return
	}

	writeSignatureString(h, "0")
}

func effectiveAssociationSourceCounts(systemSettings *biz.SystemModelSettings, m *ent.Model) (developerCount int, modelCount int, developerInheritanceDisabled bool) {
	if m == nil {
		return 0, 0, false
	}

	if m.Settings != nil {
		modelCount = len(m.Settings.Associations)
		developerInheritanceDisabled = m.Settings.DisableDeveloperSettingsInheritance
	}

	if systemSettings == nil || m.Developer == "" {
		return developerCount, modelCount, developerInheritanceDisabled
	}

	for _, developerSettings := range systemSettings.DeveloperSettings {
		if developerSettings == nil || developerSettings.Developer != m.Developer {
			continue
		}

		return len(developerSettings.Associations), modelCount, developerInheritanceDisabled
	}

	return developerCount, modelCount, developerInheritanceDisabled
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
		ctx = contextWithQuotaLimitType(ctx, string(provider_quota.RequestModality(req.Image != nil)))
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
	// SelectedAPIKey, if non-empty, forces the outbound to use this specific API key.
	// Used by the channel key test flow to test a single key.
	SelectedAPIKey string
}

func NewSpecifiedChannelSelector(channelService *biz.ChannelService, channelID objects.GUID) *SpecifiedChannelSelector {
	return &SpecifiedChannelSelector{
		ChannelService: channelService,
		ChannelID:      channelID,
	}
}

func (s *SpecifiedChannelSelector) Select(ctx context.Context, req *llm.Request) ([]*ChannelModelsCandidate, error) {
	var channel *biz.Channel
	var err error
	if s.SelectedAPIKey != "" {
		channel, err = s.ChannelService.GetChannelWithKey(ctx, s.ChannelID.ID, s.SelectedAPIKey)
	} else {
		channel, err = s.ChannelService.GetChannel(ctx, s.ChannelID.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get channel for test: %w", err)
	}

	entries := channel.GetDirectModelEntries()

	entry, ok := entries[req.Model]
	if !ok {
		return nil, fmt.Errorf("model %s not supported in channel %s", req.Model, channel.Name)
	}

	endpoints := channel.ResolveEndpoints()
	apiFormat := SelectAPIFormat(endpoints, req)

	candidate := &ChannelModelsCandidate{
		Channel:   channel,
		Priority:  0,
		Models:    []biz.ChannelModelEntry{entry},
		APIFormat: apiFormat,
	}

	return []*ChannelModelsCandidate{candidate}, nil
}
