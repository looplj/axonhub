// 职责: 把逐 key 的提供商额度观测结果同步为渠道级 key 禁用记录。
// 关联文件:
//
//	internal/server/biz/channel_apikey.go
//	internal/server/biz/channel_events.go
//	internal/server/biz/provider_quota.go
//	internal/server/biz/channel_provider_quota_hook.go
//	internal/server/biz/provider_quota/apikey_quota.go
//
// 遵循的 Rules:
//
//	.agents/rules/biz-services.md
//	.agents/rules/go-general.md
//
// 注意事项:
//
//	只处理 QuotaDisabled 记录；手工、插件和 HTTP 错误规则产生的禁用记录一律不碰。
//	本文件不得回调 ProviderQuotaService（调用方持有该服务的互斥锁）。
package biz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
	"github.com/looplj/axonhub/internal/server/biz/provider_quota"
)

// quotaKeyDisabledReason is the reason stored on a disable owned by a provider
// quota observation. Ownership itself is the QuotaDisabled flag, never this
// text.
const quotaKeyDisabledReason = "Provider quota exhausted"

// errQuotaKeySnapshotStale reports that the channel identity changed while the
// quota request was in flight. The result of that poll describes the previous
// credentials, so it is discarded and the next poll checks the new identity.
var errQuotaKeySnapshotStale = errors.New("channel identity changed while the quota check was in flight")

// quotaKeyProviderType maps a channel onto the provider quota key it is
// collected under, restricted to the providers whose keys are independent quota
// accounts.
func quotaKeyProviderType(ch *ent.Channel) (string, bool) {
	switch ch.Type { //nolint:exhaustive // Every other channel type is not a per-key quota provider.
	case channel.TypeOpencodeGo, channel.TypeOpencodeGoAnthropic:
		return "opencode_go", true
	case channel.TypeZhipu, channel.TypeZhipuAnthropic:
		return "zhipu", true
	case channel.TypeZai, channel.TypeZaiAnthropic:
		return "zai", true
	case channel.TypeDeepseek, channel.TypeDeepseekAnthropic:
		return "deepseek", true
	case channel.TypeOpenai, channel.TypeOpenaiResponses:
		if provider_quota.DetectProviderFromURL(ch.BaseURL) == "deepseek" {
			return "deepseek", true
		}

		return "", false
	default:
		return "", false
	}
}

// hasActiveQuotaOwnedRecord reports whether any key of the channel is disabled
// because of a provider quota observation. Expired records own nothing.
func hasActiveQuotaOwnedRecord(keys []objects.DisabledAPIKey) bool {
	for _, key := range keys {
		if key.QuotaDisabled && !key.IsExpired() {
			return true
		}
	}

	return false
}

// quotaKeyReconcileResult is one replacement disabled-key list together with
// the credential events it implies.
type quotaKeyReconcileResult struct {
	keys        []objects.DisabledAPIKey
	quotaAdded  []string
	quotaFreed  []string
	changed     bool
	leavesEmpty bool
}

// ReconcileQuotaKeys aligns the channel's quota-owned key disables with the
// per-key quota observation. It is called by the provider quota service while
// that service holds its own mutex; it never calls back into it.
//
// A nil accounts slice is a cleanup-only pass: no key is newly disabled, and
// quota-owned records whose key is still configured survive so a poll that
// carried no observation cannot erase them.
func (svc *ChannelService) ReconcileQuotaKeys(
	ctx context.Context,
	snapshot *ent.Channel,
	accounts []provider_quota.APIKeyQuota,
) (*ent.Channel, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("quota key reconciliation requires a channel snapshot")
	}

	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	entClient := svc.entFromContext(ctx)

	ch, err := entClient.Channel.Get(ctx, snapshot.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload channel for quota key reconciliation: %w", err)
	}

	if !quotaKeyIdentityUnchanged(snapshot, ch) {
		return ch, fmt.Errorf("%w (channel %d)", errQuotaKeySnapshotStale, ch.ID)
	}

	collecting, err := svc.quotaKeyCollectionEnabled(ctx, ch)
	if err != nil {
		return ch, err
	}

	now := time.Now()
	result := reconcileQuotaKeyDisables(ch, accounts, collecting, now)
	if !result.changed {
		return ch, nil
	}

	update := entClient.Channel.Update().
		Where(channel.IDEQ(ch.ID), channel.UpdatedAtEQ(ch.UpdatedAt)).
		SetDisabledAPIKeys(result.keys)

	autoDisabled := false
	recovered := false

	switch {
	case result.leavesEmpty:
		// Zero enabled credentials reuses the existing all-keys bookkeeping, so
		// the channel disappears from the serving cache exactly as it does when
		// the last key is disabled by hand — unless that state is already set.
		if !channelIsAllKeysDisabled(ch) {
			autoDisabled = true
			update = update.
				SetStatus(channel.StatusDisabled).
				SetErrorMessage(fmt.Sprintf("%s (last error: %d)", allKeysDisabledErrorPrefix, 0)).
				SetAutoDisabledAt(now).
				ClearAutoDisableExpiresAt()
		}
	case channelIsAllKeysDisabled(ch):
		recovered = true
		update = update.
			SetStatus(channel.StatusEnabled).
			ClearErrorMessage().
			ClearAutoDisabledAt().
			ClearAutoDisableExpiresAt()
	}

	// The compare-and-set predicate keeps this writer from overwriting a
	// concurrent manual disable or channel edit: a lost race writes nothing and
	// the next poll reconciles from the current state.
	affected, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile quota key disables: %w", err)
	}
	if affected == 0 {
		// A concurrent edit won the row: this observation no longer describes
		// the stored channel, so the caller must not persist it either. The
		// next poll reconciles from the current state.
		return ch, fmt.Errorf("%w (channel %d): the channel changed during reconciliation", errQuotaKeySnapshotStale, ch.ID)
	}

	eventStatus := ch.Status
	if autoDisabled {
		eventStatus = channel.StatusDisabled
	}

	for _, key := range result.quotaAdded {
		svc.publishCredentialDisabled(ctx, ch, key, 0, quotaKeyDisabledReason, nil, eventStatus)
	}
	svc.publishCredentialEnabled(ctx, ch, result.quotaFreed, result.keys, credentialTriggerQuota)

	if autoDisabled {
		reason := fmt.Sprintf("%s (last error: %d)", allKeysDisabledErrorPrefix, 0)
		log.Warn(ctx, "Channel disabled because provider quota exhausted all API keys",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
		)
		svc.asyncNotifyChannelAutoDisabled(ctx, ChannelAutoDisabledEvent{
			ChannelID:       ch.ID,
			ChannelName:     ch.Name,
			ChannelProvider: ch.Type.String(),
			ChannelBaseURL:  ch.BaseURL,
			ChannelStatus:   channel.StatusDisabled.String(),
			StatusCode:      0,
			Reason:          reason,
			OccurredAt:      now,
		})
	}
	if recovered {
		log.Info(ctx, "Re-enabled channel after provider quota recovered for an API key",
			log.Int("channel_id", ch.ID),
			log.String("channel_name", ch.Name),
		)
	}

	reloaded, err := entClient.Channel.Get(ctx, ch.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload channel after quota key reconciliation: %w", err)
	}

	// The local serving cache must reflect the new eligibility before the next
	// request is routed; other instances are notified through the watcher.
	if svc.enabledChannelsCache != nil {
		reloadCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := svc.enabledChannelsCache.Load(reloadCtx, true); err != nil {
			log.Warn(ctx, "Failed to synchronously reload channels after quota key reconciliation",
				log.Int("channel_id", ch.ID),
				log.Cause(err),
			)
		}
	}
	svc.asyncReloadChannels()

	return reloaded, nil
}

// refreshAPIKeyQuotaOwnership stamps the observed accounts with the disable
// ownership the channel holds after reconciliation, so the persisted per-key
// rows explain what the UI shows.
func refreshAPIKeyQuotaOwnership(ch *ent.Channel, accounts []provider_quota.APIKeyQuota) []provider_quota.APIKeyQuota {
	records := make(map[string]objects.DisabledAPIKey, len(ch.DisabledAPIKeys))
	for _, record := range ch.DisabledAPIKeys {
		if record.Key == "" || record.IsExpired() {
			continue
		}

		records[record.Key] = record
	}

	refreshed := make([]provider_quota.APIKeyQuota, len(accounts))
	for i, account := range accounts {
		clone := account
		if record, ok := records[account.Key]; ok {
			clone.Disabled = true
			clone.QuotaDisabled = record.QuotaDisabled
		} else {
			clone.Disabled = false
			clone.QuotaDisabled = false
		}

		refreshed[i] = clone
	}

	return refreshed
}

// quotaKeyCollectionEnabled reports whether quota observations may change this
// channel's keys: the provider must be collected and the effective routing mode
// must not opt out.
func (svc *ChannelService) quotaKeyCollectionEnabled(ctx context.Context, ch *ent.Channel) (bool, error) {
	providerType, ok := quotaKeyProviderType(ch)
	if !ok {
		return false, nil
	}

	if svc.SystemService != nil {
		enabled, err := svc.SystemService.IsProviderQuotaCollectionEnabled(ctx, providerType)
		if err != nil {
			return false, fmt.Errorf("failed to read provider quota collection settings: %w", err)
		}
		if !enabled {
			return false, nil
		}
	}

	mode := objects.QuotaRoutingModeRemoveOnExhausted
	if svc.SystemService != nil {
		mode = svc.SystemService.QuotaRoutingSettingsOrDefault(ctx).DefaultMode
	}
	if ch.Settings != nil && ch.Settings.QuotaRoutingMode != "" {
		mode = ch.Settings.QuotaRoutingMode
	}
	// Resolve an unset or unknown override exactly like the routing gate does.
	switch mode {
	case objects.QuotaRoutingModeRemoveOnExhausted,
		objects.QuotaRoutingModeBackpressure,
		objects.QuotaRoutingModeIgnoreQuota:
	default:
		mode = objects.QuotaRoutingModeRemoveOnExhausted
	}

	return mode != objects.QuotaRoutingModeIgnoreQuota, nil
}

// quotaKeyIdentityUnchanged reports whether the requester's snapshot still
// describes the stored channel.
func quotaKeyIdentityUnchanged(snapshot, current *ent.Channel) bool {
	if snapshot.ID != current.ID || snapshot.Type != current.Type || snapshot.BaseURL != current.BaseURL {
		return false
	}

	return slices.Equal(
		snapshot.Credentials.GetAllCredentialRefs(),
		current.Credentials.GetAllCredentialRefs(),
	)
}

// reconcileQuotaKeyDisables builds the replacement disabled-key list. Non-quota
// records always survive untouched; only records this service owns are added or
// removed.
func reconcileQuotaKeyDisables(
	ch *ent.Channel,
	accounts []provider_quota.APIKeyQuota,
	collecting bool,
	now time.Time,
) quotaKeyReconcileResult {
	configured := make(map[string]struct{})
	for _, key := range ch.Credentials.GetAllCredentialRefs() {
		configured[key] = struct{}{}
	}

	observed := make(map[string]provider_quota.APIKeyQuota, len(accounts))
	for _, account := range accounts {
		observed[account.Key] = account
	}

	result := quotaKeyReconcileResult{
		keys: make([]objects.DisabledAPIKey, 0, len(ch.DisabledAPIKeys)+1),
	}
	kept := make(map[string]struct{}, len(ch.DisabledAPIKeys))

	for _, record := range ch.DisabledAPIKeys {
		if !record.QuotaDisabled {
			// A manual, plugin or HTTP-rule disable owns this record for good.
			// An expired record owns nothing (the expiry cleanup removes it), so
			// it must not stop quota from parking the same key.
			result.keys = append(result.keys, record)
			if !record.IsExpired() {
				kept[record.Key] = struct{}{}
			}

			continue
		}

		if _, ok := configured[record.Key]; !ok {
			result.changed = true
			result.quotaFreed = append(result.quotaFreed, record.Key)

			continue
		}

		if !collecting {
			result.changed = true
			result.quotaFreed = append(result.quotaFreed, record.Key)

			continue
		}

		if accounts != nil {
			account, observedKey := observed[record.Key]
			unknown := !observedKey || account.ErrorCode != "" ||
				account.Data.Status == "unknown" || account.Data.Status == ""
			if !unknown && account.Data.Status != "exhausted" {
				// Only an observed recovery releases the key: an outage, a
				// removed observation or a stale row keeps it disabled.
				result.changed = true
				result.quotaFreed = append(result.quotaFreed, record.Key)

				continue
			}
		}

		result.keys = append(result.keys, record)
		kept[record.Key] = struct{}{}
	}

	if collecting {
		for _, account := range accounts {
			if _, ok := kept[account.Key]; ok {
				continue
			}
			if account.ErrorCode != "" || account.Data.Status != "exhausted" {
				continue
			}

			result.keys = append(result.keys, objects.DisabledAPIKey{
				Key:           account.Key,
				DisabledAt:    now,
				ErrorCode:     0,
				Reason:        quotaKeyDisabledReason,
				QuotaDisabled: true,
			})
			result.quotaAdded = append(result.quotaAdded, account.Key)
			result.changed = true
		}
	}

	result.leavesEmpty = len(ch.Credentials.GetEnabledCredentialRefs(result.keys)) == 0

	return result
}
