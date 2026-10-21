package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/looplj/axonhub/internal/log"
)

// evaluateClientCancellationForFailure increments the independent consecutive
// cancellation counter and applies the configured cooldown when the threshold
// is reached. It never participates in HTTP status-code retry decisions.
func (svc *ChannelService) evaluateClientCancellationForFailure(ctx context.Context, perf *PerformanceRecord) bool {
	if perf == nil || perf.ChannelID <= 0 || !perf.Canceled {
		return false
	}

	retryPolicy := svc.SystemService.RetryPolicyOrDefault(ctx)
	policy, _ := retryPolicy.ClientCancellationPolicy.EffectiveClientCancellationPolicy(perf.ProjectID)
	if !policy.Enabled {
		return false
	}

	threshold := max(policy.Threshold, 1)
	cooldownMinutes := max(policy.CooldownMinutes, 1)
	identity := cancellationCounterIdentity(perf)
	counterKey := fmt.Sprintf(
		"%s:project:%d:threshold:%d:cooldown:%d:action:%s",
		identity,
		perf.ProjectID,
		threshold,
		cooldownMinutes,
		policy.Action,
	)
	// Model-scoped cooldowns count per (channel, model): cancellations of one
	// model must not push another model on the same channel into cooldown.
	if policy.Action == ClientCancellationActionCooldownModel && perf.Model != "" {
		counterKey += ":model:" + perf.Model
	}

	svc.apiKeyErrorCountsLock.Lock()
	if svc.cancellationCounts[perf.ChannelID] == nil {
		svc.cancellationCounts[perf.ChannelID] = make(map[string]int)
	}
	if svc.cancellationActionsInFlight[perf.ChannelID] == nil {
		svc.cancellationActionsInFlight[perf.ChannelID] = make(map[string]bool)
	}
	svc.cancellationCounts[perf.ChannelID][counterKey]++
	count := svc.cancellationCounts[perf.ChannelID][counterKey]
	_, actionInFlight := svc.cancellationActionsInFlight[perf.ChannelID][counterKey]
	shouldAct := count >= threshold && !actionInFlight
	if shouldAct {
		svc.cancellationCounts[perf.ChannelID][counterKey] = 0
		svc.cancellationActionsInFlight[perf.ChannelID][counterKey] = true
	}
	svc.apiKeyErrorCountsLock.Unlock()

	if !shouldAct {
		return false
	}

	acted := svc.executeClientCancellationCooldown(ctx, perf, count, cooldownMinutes, policy.Action)

	svc.apiKeyErrorCountsLock.Lock()
	if svc.cancellationActionsInFlight[perf.ChannelID] != nil {
		delete(svc.cancellationActionsInFlight[perf.ChannelID], counterKey)
		if len(svc.cancellationActionsInFlight[perf.ChannelID]) == 0 {
			delete(svc.cancellationActionsInFlight, perf.ChannelID)
		}
	}
	if !acted && svc.cancellationCounts[perf.ChannelID] != nil {
		svc.cancellationCounts[perf.ChannelID][counterKey] = threshold
	}
	svc.apiKeyErrorCountsLock.Unlock()

	if acted {
		log.Warn(ctx, "channel credential entered cancellation cooldown",
			log.Int("channel_id", perf.ChannelID),
			log.Int("project_id", perf.ProjectID),
			log.Int("threshold", threshold),
			log.Int("cooldown_minutes", cooldownMinutes),
			log.String("action", policy.Action),
		)
	}

	return acted
}

func (svc *ChannelService) executeClientCancellationCooldown(
	ctx context.Context,
	perf *PerformanceRecord,
	count int,
	cooldownMinutes int,
	action string,
) bool {
	expiresAt := time.Now().Add(time.Duration(cooldownMinutes) * time.Minute)
	reason := fmt.Sprintf(
		"Client cancellation cooldown: %d consecutive cancellations, auto-recover after %d minutes",
		max(count, 1),
		cooldownMinutes,
	)

	// cooldown_model freezes only the (channel, model) pair that produced the
	// cancellations: the channel stays enabled and keeps serving its other models,
	// and the frozen model recovers automatically or via manual unfreeze.
	if action == ClientCancellationActionCooldownModel && perf.Model != "" {
		return svc.markChannelModelCooldown(ctx, perf.ChannelID, perf.Model, count, cooldownMinutes)
	}

	// switch_and_cooldown takes the whole channel out of the candidate pool so
	// the next request is routed to a different channel during the cooldown.
	//
	// cooldown_only keeps the rest of a multi-credential channel usable and only
	// cools down the credential that served the canceled request; the channel is
	// disabled by DisableAPIKey once no credential is left.
	if action == ClientCancellationActionSwitchAndCooldown || perf.APIKey == "" {
		svc.markChannelUnavailable(ctx, perf.ChannelID, 0, max(count, 1), count, &expiresAt, reason)
		return true
	}

	if err := svc.DisableAPIKey(ctx, perf.ChannelID, perf.APIKey, 0, reason, &expiresAt); err != nil {
		log.Error(ctx, "Failed to temporarily disable API key after client cancellations",
			log.Int("channel_id", perf.ChannelID),
			log.Cause(err),
		)

		return false
	}

	return true
}

func (svc *ChannelService) clearCancellationCountsOnSuccess(perf *PerformanceRecord) {
	if perf == nil || perf.ChannelID <= 0 {
		return
	}

	identity := cancellationCounterIdentity(perf)
	prefix := fmt.Sprintf("%s:project:%d:", identity, perf.ProjectID)

	svc.apiKeyErrorCountsLock.Lock()
	defer svc.apiKeyErrorCountsLock.Unlock()

	for key := range svc.cancellationCounts[perf.ChannelID] {
		if strings.HasPrefix(key, prefix) {
			delete(svc.cancellationCounts[perf.ChannelID], key)
		}
	}
	if len(svc.cancellationCounts[perf.ChannelID]) == 0 {
		delete(svc.cancellationCounts, perf.ChannelID)
	}
}

func cancellationCounterIdentity(perf *PerformanceRecord) string {
	if perf.APIKey != "" {
		return perf.APIKey
	}
	return fmt.Sprintf("channel:%d", perf.ChannelID)
}
