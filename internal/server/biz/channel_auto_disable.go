package biz

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

var compiledAPIKeyRuleRegexes sync.Map

func (svc *ChannelService) markChannelUnavailable(ctx context.Context, channelID int, responseStatusCode int, threshold int, actualCount int) {
	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Only disable channels that are currently enabled to avoid repeated disabling
	// of the same channel under sustained error traffic, which would keep resetting
	// the cache debounce timer and prevent the cache from ever refreshing.
	affected, err := svc.db.Channel.Update().
		Where(
			channel.ID(channelID),
			channel.StatusEQ(channel.StatusEnabled),
		).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(deriveErrorMessage(responseStatusCode)).
		Save(ctx)
	if err != nil {
		log.Error(ctx, "Failed to disable channel on unrecoverable error",
			log.Int("channel_id", channelID),
			log.Int("error_code", responseStatusCode),
			log.Cause(err),
		)

		return
	}

	if affected == 0 {
		log.Debug(ctx, "Channel already disabled, skipping",
			log.Int("channel_id", channelID),
			log.Int("error_code", responseStatusCode),
		)

		// Another instance may have already disabled the channel in DB while this
		// instance still serves it from a stale in-memory cache. Force a local
		// refresh so candidate selection stops using the channel immediately.
		if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
			log.Warn(ctx, "Failed to refresh local cache for already-disabled channel",
				log.Int("channel_id", channelID),
				log.Cause(err),
			)
		}

		return
	}

	log.Warn(ctx, "Channel disabled due to unrecoverable error",
		log.Int("channel_id", channelID),
		log.Int("error_code", responseStatusCode),
	)

	// Fetch the updated channel for webhook notification
	updatedChannel, err := svc.db.Channel.Get(ctx, channelID)
	if err != nil {
		log.Error(ctx, "Failed to fetch disabled channel for webhook notification",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
	} else {
		notifyCtx := context.WithoutCancel(ctx)
		go svc.WebhookNotifier.NotifyChannelAutoDisabled(notifyCtx, ChannelAutoDisabledEvent{
			ChannelID:       updatedChannel.ID,
			ChannelName:     updatedChannel.Name,
			ChannelProvider: updatedChannel.Type.String(),
			ChannelBaseURL:  updatedChannel.BaseURL,
			ChannelStatus:   updatedChannel.Status.String(),
			StatusCode:      responseStatusCode,
			Threshold:       threshold,
			ActualCount:     actualCount,
			Reason:          deriveErrorMessage(responseStatusCode),
			OccurredAt:      time.Now(),
		})
	}

	// Synchronously reload the local cache to immediately stop selecting this channel.
	// This avoids the debounce delay that could keep the disabled channel in the candidate pool.
	if err := svc.enabledChannelsCache.Load(ctx, true); err != nil {
		log.Warn(ctx, "Failed to synchronously reload channels after auto-disable",
			log.Int("channel_id", channelID),
			log.Cause(err),
		)
	}

	// Also notify other instances via the watcher for cross-instance cache invalidation.
	svc.asyncReloadChannels()
}

// checkAndHandleChannelError checks if the channel should be disabled based on the error status code.
func (svc *ChannelService) checkAndHandleChannelError(ctx context.Context, perf *PerformanceRecord, policy *RetryPolicy) bool {
	for _, statusConfig := range policy.AutoDisableChannel.Statuses {
		if statusConfig.Status != perf.ResponseStatusCode {
			continue
		}

		svc.channelErrorCountsLock.Lock()

		if svc.channelErrorCounts[perf.ChannelID] == nil {
			svc.channelErrorCounts[perf.ChannelID] = make(map[int]int)
		}

		svc.channelErrorCounts[perf.ChannelID][perf.ResponseStatusCode]++
		count := svc.channelErrorCounts[perf.ChannelID][perf.ResponseStatusCode]
		svc.channelErrorCountsLock.Unlock()

		if count >= statusConfig.Times {
			svc.markChannelUnavailable(ctx, perf.ChannelID, perf.ResponseStatusCode, statusConfig.Times, count)
			svc.channelErrorCountsLock.Lock()
			delete(svc.channelErrorCounts, perf.ChannelID)
			svc.channelErrorCountsLock.Unlock()

			return true
		}
	}

	return false
}

// checkAndHandleAPIKeyError checks if the API key should be disabled based on the error status code.
// Returns true if the API key was disabled.
func (svc *ChannelService) checkAndHandleAPIKeyError(ctx context.Context, perf *PerformanceRecord, policy *RetryPolicy) bool {
	for _, statusConfig := range policy.AutoDisableChannel.Statuses {
		if statusConfig.Status != perf.ResponseStatusCode {
			continue
		}

		svc.apiKeyErrorCountsLock.Lock()

		if svc.apiKeyErrorCounts[perf.ChannelID] == nil {
			svc.apiKeyErrorCounts[perf.ChannelID] = make(map[string]map[int]int)
		}

		if svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey] == nil {
			svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey] = make(map[int]int)
		}

		svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey][perf.ResponseStatusCode]++
		count := svc.apiKeyErrorCounts[perf.ChannelID][perf.APIKey][perf.ResponseStatusCode]
		svc.apiKeyErrorCountsLock.Unlock()

		if count >= statusConfig.Times {
			reason := fmt.Sprintf("Auto-disabled after %d consecutive errors with status %d", count, perf.ResponseStatusCode)
			if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason); err != nil {
				log.Error(ctx, "Failed to disable API key",
					log.Int("channel_id", perf.ChannelID),
					log.Int("error_code", perf.ResponseStatusCode),
					log.Cause(err),
				)

				return false
			}

			svc.apiKeyErrorCountsLock.Lock()
			delete(svc.apiKeyErrorCounts[perf.ChannelID], perf.APIKey)
			svc.apiKeyErrorCountsLock.Unlock()

			return true
		}
	}

	return false
}

// EvaluateAPIKeyRulesForFailure evaluates channel-scoped API key rules for a
// failure that was persisted outside the normal performance middleware path.
func (svc *ChannelService) EvaluateAPIKeyRulesForFailure(
	ctx context.Context,
	channelID int,
	apiKey string,
	responseStatusCode int,
	errorMessage string,
) bool {
	if channelID == 0 || apiKey == "" {
		return false
	}

	_, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID:          channelID,
		APIKey:             apiKey,
		ResponseStatusCode: responseStatusCode,
		ErrorMessage:       errorMessage,
	})
	return acted
}

// checkAndHandleChannelAPIKeyRules evaluates rules in declaration order. The
// first matching rule owns the failure so one request cannot increment several
// overlapping counters or execute multiple actions.
func (svc *ChannelService) checkAndHandleChannelAPIKeyRules(ctx context.Context, perf *PerformanceRecord) (matched, acted bool) {
	rulePrefix := perf.APIKey + ":rule:"
	ch := svc.GetEnabledChannel(perf.ChannelID)
	if ch == nil || len(ch.Policies.APIKeyAutoDisableRules) == 0 {
		svc.clearAPIKeyRuleCounts(perf.ChannelID, rulePrefix)
		return false, false
	}

	for ruleIndex, rule := range ch.Policies.APIKeyAutoDisableRules {
		if !matchesAPIKeyRule(rule, perf) {
			continue
		}

		ruleKey := apiKeyRuleCounterKey(perf.APIKey, ruleIndex, rule)
		// Every failure accepted by one rule contributes to that rule's single
		// consecutive counter, including alternating configured status codes.
		const countKey = 0

		svc.apiKeyErrorCountsLock.Lock()
		if svc.apiKeyErrorCounts[perf.ChannelID] == nil {
			svc.apiKeyErrorCounts[perf.ChannelID] = make(map[string]map[int]int)
		}
		if svc.apiKeyRuleActionsInFlight == nil {
			svc.apiKeyRuleActionsInFlight = make(map[int]map[string]bool)
		}
		if svc.apiKeyRuleActionsInFlight[perf.ChannelID] == nil {
			svc.apiKeyRuleActionsInFlight[perf.ChannelID] = make(map[string]bool)
		}
		for key := range svc.apiKeyErrorCounts[perf.ChannelID] {
			if strings.HasPrefix(key, rulePrefix) && key != ruleKey {
				delete(svc.apiKeyErrorCounts[perf.ChannelID], key)
			}
		}
		for key := range svc.apiKeyRuleActionsInFlight[perf.ChannelID] {
			if strings.HasPrefix(key, rulePrefix) && key != ruleKey {
				delete(svc.apiKeyRuleActionsInFlight[perf.ChannelID], key)
			}
		}
		if svc.apiKeyErrorCounts[perf.ChannelID][ruleKey] == nil {
			svc.apiKeyErrorCounts[perf.ChannelID][ruleKey] = make(map[int]int)
		}
		svc.apiKeyErrorCounts[perf.ChannelID][ruleKey][countKey]++
		count := svc.apiKeyErrorCounts[perf.ChannelID][ruleKey][countKey]
		threshold := max(rule.Times, 1)
		_, actionInFlight := svc.apiKeyRuleActionsInFlight[perf.ChannelID][ruleKey]
		shouldAct := count >= threshold && !actionInFlight
		if shouldAct {
			svc.apiKeyErrorCounts[perf.ChannelID][ruleKey][countKey] -= threshold
			svc.apiKeyRuleActionsInFlight[perf.ChannelID][ruleKey] = false
		}
		svc.apiKeyErrorCountsLock.Unlock()

		if shouldAct {
			actionSucceeded := svc.executeAPIKeyRuleAction(ctx, perf, rule, count)
			svc.apiKeyErrorCountsLock.Lock()
			if streakReset, stillClaimed := svc.apiKeyRuleActionsInFlight[perf.ChannelID][ruleKey]; stillClaimed {
				delete(svc.apiKeyRuleActionsInFlight[perf.ChannelID], ruleKey)
				if !actionSucceeded && !streakReset {
					if svc.apiKeyErrorCounts[perf.ChannelID] == nil {
						svc.apiKeyErrorCounts[perf.ChannelID] = make(map[string]map[int]int)
					}
					if svc.apiKeyErrorCounts[perf.ChannelID][ruleKey] == nil {
						svc.apiKeyErrorCounts[perf.ChannelID][ruleKey] = make(map[int]int)
					}
					svc.apiKeyErrorCounts[perf.ChannelID][ruleKey][countKey] += threshold
				}
				if svc.apiKeyErrorCounts[perf.ChannelID][ruleKey][countKey] == 0 {
					delete(svc.apiKeyErrorCounts[perf.ChannelID], ruleKey)
				}
			}
			svc.apiKeyErrorCountsLock.Unlock()
			return true, actionSucceeded
		}

		return true, false
	}

	svc.clearAPIKeyRuleCounts(perf.ChannelID, rulePrefix)

	return false, false
}

func (svc *ChannelService) clearAPIKeyRuleCounts(channelID int, rulePrefix string) {
	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()

	for key := range svc.apiKeyErrorCounts[channelID] {
		if strings.HasPrefix(key, rulePrefix) {
			delete(svc.apiKeyErrorCounts[channelID], key)
		}
	}
	for key := range svc.apiKeyRuleActionsInFlight[channelID] {
		if strings.HasPrefix(key, rulePrefix) {
			delete(svc.apiKeyRuleActionsInFlight[channelID], key)
		}
	}
}

func apiKeyRuleCounterKey(apiKey string, ruleIndex int, rule objects.APIKeyAutoDisableRule) string {
	disableDurationMinutes := 0
	if rule.DisableDurationMinutes != nil {
		disableDurationMinutes = *rule.DisableDurationMinutes
	}

	return fmt.Sprintf(
		"%s:rule:%d:%v:%v:%d:%s:%d",
		apiKey,
		ruleIndex,
		rule.StatusCodes,
		rule.KeywordPatterns,
		rule.Times,
		rule.Action,
		disableDurationMinutes,
	)
}

func matchesAPIKeyRule(rule objects.APIKeyAutoDisableRule, perf *PerformanceRecord) bool {
	if len(rule.StatusCodes) > 0 && !slices.Contains(rule.StatusCodes, perf.ResponseStatusCode) {
		return false
	}
	if len(rule.KeywordPatterns) == 0 {
		return true
	}
	if perf.ErrorMessage == "" {
		return false
	}

	lowerMessage := strings.ToLower(perf.ErrorMessage)
	for _, pattern := range rule.KeywordPatterns {
		if re := compiledAPIKeyRuleRegex(pattern); re != nil {
			if re.MatchString(perf.ErrorMessage) {
				return true
			}
			continue
		}
		// Patterns may be plain keywords or regular expressions. Treat syntax
		// that is not a valid expression as a case-insensitive literal keyword.
		if strings.Contains(lowerMessage, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

func compiledAPIKeyRuleRegex(pattern string) *regexp.Regexp {
	cacheKey := "(?i)" + pattern
	if cached, ok := compiledAPIKeyRuleRegexes.Load(cacheKey); ok {
		re, _ := cached.(*regexp.Regexp)
		return re
	}

	re, err := regexp.Compile(cacheKey)
	if err != nil {
		compiledAPIKeyRuleRegexes.Store(cacheKey, (*regexp.Regexp)(nil))
		return nil
	}
	compiledAPIKeyRuleRegexes.Store(cacheKey, re)
	return re
}

func (svc *ChannelService) executeAPIKeyRuleAction(
	ctx context.Context,
	perf *PerformanceRecord,
	rule objects.APIKeyAutoDisableRule,
	count int,
) bool {
	reason := fmt.Sprintf("Disabled by channel API key rule after %d consecutive errors", count)
	if rule.Action == objects.APIKeyAutoDisableActionPermanent {
		if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason); err != nil {
			log.Error(ctx, "Failed to permanently disable API key by channel rule",
				log.Int("channel_id", perf.ChannelID),
				log.Cause(err),
			)
			return false
		}
		result, err := svc.DeleteDisabledAPIKeys(ctx, perf.ChannelID, []string{perf.APIKey})
		if err != nil {
			log.Error(ctx, "Failed to delete API key disabled by channel rule",
				log.Int("channel_id", perf.ChannelID),
				log.Cause(err),
			)
			return false
		}

		// Channels must retain at least one credential. Keep that last key
		// permanently disabled when deletion cannot remove it, otherwise the
		// delete helper would make the rule a no-op by re-enabling the channel.
		if result.Message == "ONE_KEY_PRESERVED" {
			if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason); err != nil {
				log.Error(ctx, "Failed to keep preserved API key disabled by channel rule",
					log.Int("channel_id", perf.ChannelID),
					log.Cause(err),
				)
				return false
			}
		}
		return true
	}

	var expiresAt *time.Time
	if rule.DisableDurationMinutes != nil {
		disabledUntil := time.Now().Add(time.Duration(*rule.DisableDurationMinutes) * time.Minute)
		expiresAt = &disabledUntil
		reason = fmt.Sprintf("Temporarily disabled for %d minutes by channel API key rule after %d consecutive errors", *rule.DisableDurationMinutes, count)
	}

	if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason, expiresAt); err != nil {
		log.Error(ctx, "Failed to temporarily disable API key by channel rule",
			log.Int("channel_id", perf.ChannelID),
			log.Cause(err),
		)
		return false
	}

	return true
}
