package biz

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aptible/supercronic/cronexpr"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

var compiledAPIKeyRuleRegexes sync.Map

type autoDisableScope string

const (
	autoDisableScopeChannel autoDisableScope = "channel"
	autoDisableScopeGlobal  autoDisableScope = "global"
)

func (svc *ChannelService) markChannelUnavailable(
	ctx context.Context,
	channelID int,
	responseStatusCode int,
	threshold int,
	actualCount int,
	expiresAt *time.Time,
	reason string,
) {
	ctx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
	defer cancel()

	if reason == "" {
		reason = deriveErrorMessage(responseStatusCode)
	}

	// Only disable channels that are currently enabled to avoid repeated disabling
	// of the same channel under sustained error traffic, which would keep resetting
	// the cache debounce timer and prevent the cache from ever refreshing.
	update := svc.db.Channel.Update().
		Where(
			channel.ID(channelID),
			channel.StatusEQ(channel.StatusEnabled),
		).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage(reason).
		SetAutoDisabledAt(time.Now())
	if expiresAt != nil {
		update.SetAutoDisableExpiresAt(*expiresAt)
	} else {
		update.ClearAutoDisableExpiresAt()
	}

	affected, err := update.Save(ctx)
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
		svc.asyncNotifyChannelAutoDisabled(ctx, ChannelAutoDisabledEvent{
			ChannelID:       updatedChannel.ID,
			ChannelName:     updatedChannel.Name,
			ChannelProvider: updatedChannel.Type.String(),
			ChannelBaseURL:  updatedChannel.BaseURL,
			ChannelStatus:   updatedChannel.Status.String(),
			StatusCode:      responseStatusCode,
			Threshold:       threshold,
			ActualCount:     actualCount,
			Reason:          reason,
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

func (svc *ChannelService) asyncNotifyChannelAutoDisabled(ctx context.Context, event ChannelAutoDisabledEvent) {
	notifyCtx := context.WithoutCancel(ctx)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(notifyCtx, "channel auto-disabled webhook notification panicked", log.Any("panic", r))
			}
		}()

		svc.WebhookNotifier.NotifyChannelAutoDisabled(notifyCtx, event)
	}()
}

// EvaluateAPIKeyRulesForFailure evaluates auto-disable rules for a failure that
// was persisted outside the normal performance middleware path. Empty API keys
// stay on RecordPerformance; this bypass exists only for keyed failures.
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

	_, acted := svc.evaluateAutoDisableForFailure(ctx, &PerformanceRecord{
		ChannelID:          channelID,
		APIKey:             apiKey,
		ResponseStatusCode: responseStatusCode,
		ErrorMessage:       errorMessage,
	})
	return acted
}

func (svc *ChannelService) evaluateAutoDisableForFailure(ctx context.Context, perf *PerformanceRecord) (matched, acted bool) {
	ch := svc.GetEnabledChannel(perf.ChannelID)
	if ch == nil {
		return false, false
	}

	switch ch.Policies.EffectiveAutoDisableMode() {
	case objects.APIKeyAutoDisableModeOff:
		return false, false
	case objects.APIKeyAutoDisableModeCustom:
		matched, acted = svc.evaluateRules(ctx, perf, ch.Policies.APIKeyAutoDisableRules, autoDisableScopeChannel)
		if matched {
			return matched, acted
		}
	}

	policy := svc.SystemService.RetryPolicyOrDefault(ctx)
	if !policy.AutoDisableChannel.Enabled || len(policy.AutoDisableChannel.Rules) == 0 {
		return false, false
	}

	return svc.evaluateRules(ctx, perf, policy.AutoDisableChannel.Rules, autoDisableScopeGlobal)
}

// checkAndHandleChannelAPIKeyRules evaluates the channel's own rules only. Tests
// and internal callers that already know they want the channel layer use this.
func (svc *ChannelService) checkAndHandleChannelAPIKeyRules(ctx context.Context, perf *PerformanceRecord) (matched, acted bool) {
	ch := svc.GetEnabledChannel(perf.ChannelID)
	if ch == nil {
		return false, false
	}

	return svc.evaluateRules(ctx, perf, ch.Policies.APIKeyAutoDisableRules, autoDisableScopeChannel)
}

// evaluateRules finds the first matching rule and increments that rule's
// independent counter. Other rules and unmatched failures leave counts alone.
func (svc *ChannelService) evaluateRules(
	ctx context.Context,
	perf *PerformanceRecord,
	rules []objects.APIKeyAutoDisableRule,
	scope autoDisableScope,
) (matched, acted bool) {
	if len(rules) == 0 {
		return false, false
	}

	for ruleIndex, rule := range rules {
		if !matchesAPIKeyRule(rule, perf) {
			continue
		}

		ruleKey := apiKeyRuleCounterKey(autoDisableCounterIdentity(perf), scope, ruleIndex, rule)
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
			actionSucceeded := svc.executeMatchedRuleAction(ctx, perf, rule, count)
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

	return false, false
}

func (svc *ChannelService) clearAutoDisableCountsOnSuccess(perf *PerformanceRecord) {
	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()

	identity := autoDisableCounterIdentity(perf)
	prefix := identity + ":"
	for key := range svc.apiKeyErrorCounts[perf.ChannelID] {
		if key == identity || strings.HasPrefix(key, prefix) {
			delete(svc.apiKeyErrorCounts[perf.ChannelID], key)
		}
	}
	for key := range svc.apiKeyRuleActionsInFlight[perf.ChannelID] {
		if key == identity || strings.HasPrefix(key, prefix) {
			svc.apiKeyRuleActionsInFlight[perf.ChannelID][key] = true
		}
	}
}

func autoDisableCounterIdentity(perf *PerformanceRecord) string {
	if perf.APIKey != "" {
		return perf.APIKey
	}

	return fmt.Sprintf("channel:%d", perf.ChannelID)
}

func apiKeyRuleCounterKey(identity string, scope autoDisableScope, ruleIndex int, rule objects.APIKeyAutoDisableRule) string {
	disableDurationMinutes := 0
	if rule.DisableDurationMinutes != nil {
		disableDurationMinutes = *rule.DisableDurationMinutes
	}

	return fmt.Sprintf(
		"%s:%s:rule:%d:%v:%v:%d:%s:%d",
		identity,
		scope,
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

// nextAPIKeyRuleCronOccurrence resolves when a disable_until_cron rule lets the
// credential back in: the first cron occurrence strictly after the failure. The
// absolute instant is stored on the disable record, so recovery does not depend
// on any instance re-evaluating the expression later.
func nextAPIKeyRuleCronOccurrence(rule objects.APIKeyAutoDisableRule, now time.Time) (time.Time, error) {
	loc := time.UTC

	if rule.DisableUntilTimezone != "" {
		parsed, err := time.LoadLocation(rule.DisableUntilTimezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid timezone %q: %w", rule.DisableUntilTimezone, err)
		}

		loc = parsed
	}

	expr, err := cronexpr.Parse(rule.DisableUntilCron)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", rule.DisableUntilCron, err)
	}

	next := expr.Next(now.In(loc))
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("cron expression %q never fires again", rule.DisableUntilCron)
	}

	return next, nil
}

func (svc *ChannelService) executeMatchedRuleAction(
	ctx context.Context,
	perf *PerformanceRecord,
	rule objects.APIKeyAutoDisableRule,
	count int,
) bool {
	if perf.APIKey == "" {
		return svc.executeChannelRuleAction(ctx, perf, rule, count)
	}

	return svc.executeAPIKeyRuleAction(ctx, perf, rule, count)
}

func (svc *ChannelService) executeAPIKeyRuleAction(
	ctx context.Context,
	perf *PerformanceRecord,
	rule objects.APIKeyAutoDisableRule,
	count int,
) bool {
	reason := fmt.Sprintf("Disabled by auto-disable rule after %d consecutive errors", count)

	action := rule.Action

	// An OAuth channel's credential cannot be deleted: DeleteDisabledAPIKeys
	// rejects OAuth channels outright, so running the delete step would report the
	// action as failed, restore the error counter and retry on every subsequent
	// failure. Fall back to keeping it disabled, which is the same observable
	// outcome for a channel holding a single credential.
	if action == objects.APIKeyAutoDisableActionPermanentDelete && perf.APIKey == objects.OAuthCredentialRef {
		action = objects.APIKeyAutoDisableActionPermanent
	}

	switch action {
	case objects.APIKeyAutoDisableActionUntilCron:
		expiresAt, err := nextAPIKeyRuleCronOccurrence(rule, time.Now())
		if err != nil {
			log.Error(ctx, "Failed to resolve api key rule cron schedule",
				log.Int("channel_id", perf.ChannelID),
				log.String("cron", rule.DisableUntilCron),
				log.Cause(err),
			)

			return false
		}

		reason = fmt.Sprintf("Disabled until %s by auto-disable rule after %d consecutive errors",
			expiresAt.Format(time.RFC3339), count)

		if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason, &expiresAt); err != nil {
			log.Error(ctx, "Failed to disable API key until cron occurrence",
				log.Int("channel_id", perf.ChannelID),
				log.Cause(err),
			)

			return false
		}

		return true

	case objects.APIKeyAutoDisableActionPermanent:
		// No expiry, so the cleanup task never revives it: the credential stays on
		// the channel until an operator re-enables it.
		if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason); err != nil {
			log.Error(ctx, "Failed to permanently disable API key by auto-disable rule",
				log.Int("channel_id", perf.ChannelID),
				log.Cause(err),
			)

			return false
		}

		return true

	case objects.APIKeyAutoDisableActionPermanentDelete:
		if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason); err != nil {
			log.Error(ctx, "Failed to permanently disable API key by auto-disable rule",
				log.Int("channel_id", perf.ChannelID),
				log.Cause(err),
			)

			return false
		}

		result, err := svc.DeleteDisabledAPIKeys(ctx, perf.ChannelID, []string{perf.APIKey})
		if err != nil {
			log.Error(ctx, "Failed to delete API key disabled by auto-disable rule",
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
				log.Error(ctx, "Failed to keep preserved API key disabled by auto-disable rule",
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
		reason = fmt.Sprintf("Temporarily disabled for %d minutes by auto-disable rule after %d consecutive errors", *rule.DisableDurationMinutes, count)
	}

	if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, perf.ResponseStatusCode, reason, expiresAt); err != nil {
		log.Error(ctx, "Failed to temporarily disable API key by auto-disable rule",
			log.Int("channel_id", perf.ChannelID),
			log.Cause(err),
		)
		return false
	}

	return true
}

func (svc *ChannelService) executeChannelRuleAction(
	ctx context.Context,
	perf *PerformanceRecord,
	rule objects.APIKeyAutoDisableRule,
	count int,
) bool {
	reason := fmt.Sprintf("Disabled by auto-disable rule after %d consecutive errors", count)
	action := rule.Action
	if action == objects.APIKeyAutoDisableActionPermanentDelete {
		log.Warn(ctx, "permanent_disable_delete degraded to permanent_disable for keyless channel",
			log.Int("channel_id", perf.ChannelID),
		)
		action = objects.APIKeyAutoDisableActionPermanent
	}

	var expiresAt *time.Time
	switch action {
	case objects.APIKeyAutoDisableActionUntilCron:
		next, err := nextAPIKeyRuleCronOccurrence(rule, time.Now())
		if err != nil {
			log.Error(ctx, "Failed to resolve channel auto-disable cron schedule",
				log.Int("channel_id", perf.ChannelID),
				log.String("cron", rule.DisableUntilCron),
				log.Cause(err),
			)
			return false
		}
		expiresAt = &next
		reason = fmt.Sprintf("Disabled until %s by auto-disable rule after %d consecutive errors",
			next.Format(time.RFC3339), count)
	case objects.APIKeyAutoDisableActionTemporary:
		if rule.DisableDurationMinutes != nil {
			disabledUntil := time.Now().Add(time.Duration(*rule.DisableDurationMinutes) * time.Minute)
			expiresAt = &disabledUntil
			reason = fmt.Sprintf("Temporarily disabled for %d minutes by auto-disable rule after %d consecutive errors", *rule.DisableDurationMinutes, count)
		}
	}

	svc.markChannelUnavailable(ctx, perf.ChannelID, perf.ResponseStatusCode, max(rule.Times, 1), count, expiresAt, reason)
	return true
}
