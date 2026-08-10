package biz

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcontext"
)

const allKeysDisabledErrorPrefix = "All API keys disabled"

// DisableAPIKey 禁用指定 key；若所有 key 都不可用则禁用 channel.
func (svc *ChannelService) DisableAPIKey(
	ctx context.Context,
	channelID int,
	key string,
	errorCode int,
	reason string,
	expiresAt ...*time.Time,
) error {
	if key == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	// 读取 channel
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	// 检查 key 是否在 credentials 中。OAuth 渠道用固定的 OAuthCredentialRef 作为
	// 唯一凭证标识，所以这里按凭证引用而非明文 key 匹配。
	allKeys := ch.Credentials.GetAllCredentialRefs()

	found := slices.Contains(allKeys, key)
	if !found {
		// key 不在 credentials 中，忽略
		return nil
	}

	activeDisabledKeys := lo.Filter(ch.DisabledAPIKeys, func(dk objects.DisabledAPIKey, _ int) bool {
		return !dk.IsExpired()
	})

	disabled := lo.ContainsBy(activeDisabledKeys, func(dk objects.DisabledAPIKey) bool {
		return dk.Key == key
	})

	if disabled {
		// 已禁用，忽略
		return nil
	}

	// 追加到 disabled_api_keys
	disabledKey := objects.DisabledAPIKey{
		Key:        key,
		DisabledAt: time.Now(),
		ErrorCode:  errorCode,
		Reason:     reason,
	}
	if len(expiresAt) > 0 {
		disabledKey.ExpiresAt = expiresAt[0]
	}

	newDisabledKeys := append(activeDisabledKeys, disabledKey)

	// 计算 enabled 凭证
	enabledKeys := ch.Credentials.GetEnabledCredentialRefs(newDisabledKeys)

	// 更新 channel
	update := svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetDisabledAPIKeys(newDisabledKeys)

	// 如果没有可用 key 了，禁用整个 channel
	channelDisabled := len(enabledKeys) == 0
	if channelDisabled {
		update.SetStatus(channel.StatusDisabled)
		update.SetErrorMessage(fmt.Sprintf("%s (last error: %d)", allKeysDisabledErrorPrefix, errorCode))
		update.SetAutoDisabledAt(time.Now())
		log.Warn(ctx, "Channel disabled because all API keys are disabled",
			log.Int("channel_id", channelID),
			log.String("channel_name", ch.Name),
		)
	}

	if _, err := update.Save(ctx); err != nil {
		return fmt.Errorf("failed to disable api key: %w", err)
	}

	log.Info(ctx, "API key disabled",
		log.Int("channel_id", channelID),
		log.Int("error_code", errorCode),
	)

	if channelDisabled {
		// Synchronously reload the local cache to immediately stop selecting this channel.
		// This matches the behavior of markChannelUnavailable.
		reloadCtx, cancel := xcontext.DetachWithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := svc.enabledChannelsCache.Load(reloadCtx, true); err != nil {
			log.Warn(ctx, "Failed to synchronously reload channels after API key exhaustion",
				log.Int("channel_id", channelID),
				log.Cause(err),
			)
		}
	}

	// Also notify other instances via the watcher for cross-instance cache invalidation.
	svc.asyncReloadChannels()

	return nil
}

// EnableAPIKey 重新启用指定 key（从 disabled_api_keys 中移除）.
func (svc *ChannelService) EnableAPIKey(ctx context.Context, channelID int, key string) error {
	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	// 读取 channel
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	if len(ch.DisabledAPIKeys) == 0 {
		// 没有禁用的 key，忽略
		return nil
	}

	// 从 disabled_api_keys 中移除指定 key
	newDisabledKeys := make([]objects.DisabledAPIKey, 0, len(ch.DisabledAPIKeys))
	found := false

	for _, dk := range ch.DisabledAPIKeys {
		if dk.Key == key {
			found = true
			continue
		}

		newDisabledKeys = append(newDisabledKeys, dk)
	}

	if !found {
		// key 不在禁用列表中，忽略
		return nil
	}

	// 更新 channel
	update := svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetDisabledAPIKeys(newDisabledKeys)
	update = applyRecoveredChannelStatus(ctx, update, ch, ch.Credentials, newDisabledKeys)

	if _, err := update.Save(ctx); err != nil {
		return fmt.Errorf("failed to enable api key: %w", err)
	}

	svc.asyncReloadChannels()

	return nil
}

// EnableAllAPIKeys 清空 disabled_api_keys.
func (svc *ChannelService) EnableAllAPIKeys(ctx context.Context, channelID int) error {
	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	// 读取 channel
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	if len(ch.DisabledAPIKeys) == 0 {
		// 没有禁用的 key，忽略
		return nil
	}

	// 更新 channel，清空 disabled_api_keys
	update := svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{})
	update = applyRecoveredChannelStatus(ctx, update, ch, ch.Credentials, nil)

	if _, err := update.Save(ctx); err != nil {
		return fmt.Errorf("failed to enable all api keys: %w", err)
	}

	log.Info(ctx, "All API keys enabled",
		log.Int("channel_id", channelID),
	)

	svc.asyncReloadChannels()

	return nil
}

// EnableSelectedAPIKeys re-enables multiple specific keys from disabled_api_keys.
func (svc *ChannelService) EnableSelectedAPIKeys(ctx context.Context, channelID int, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	if len(ch.DisabledAPIKeys) == 0 {
		return nil
	}

	keysToEnable := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keysToEnable[k] = struct{}{}
	}

	newDisabledKeys := make([]objects.DisabledAPIKey, 0, len(ch.DisabledAPIKeys))
	for _, dk := range ch.DisabledAPIKeys {
		if _, found := keysToEnable[dk.Key]; !found {
			newDisabledKeys = append(newDisabledKeys, dk)
		}
	}

	if len(newDisabledKeys) == len(ch.DisabledAPIKeys) {
		return nil
	}

	update := svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetDisabledAPIKeys(newDisabledKeys)
	update = applyRecoveredChannelStatus(ctx, update, ch, ch.Credentials, newDisabledKeys)

	if _, err := update.Save(ctx); err != nil {
		return fmt.Errorf("failed to enable selected api keys: %w", err)
	}

	log.Info(ctx, "Selected API keys enabled",
		log.Int("channel_id", channelID),
		log.Int("count", len(keys)),
	)

	svc.asyncReloadChannels()

	return nil
}

// DeleteDisabledAPIKeysResult is the result of deleting disabled API keys.
type DeleteDisabledAPIKeysResult struct {
	Success bool
	Message string
}

// DeleteDisabledAPIKeys removes disabled API keys from both disabled_api_keys list and credentials.
// It ensures at least one API key remains and prevents deletion for OAuth channels.
func (svc *ChannelService) DeleteDisabledAPIKeys(ctx context.Context, channelID int, keys []string) (*DeleteDisabledAPIKeysResult, error) {
	if len(keys) == 0 {
		return &DeleteDisabledAPIKeysResult{Success: true}, nil
	}

	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	// Check if channel uses OAuth - cannot delete keys for OAuth channels
	if ch.Credentials.IsOAuth() {
		return nil, fmt.Errorf("cannot delete API keys for OAuth channels")
	}

	keysToDelete := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keysToDelete[k] = struct{}{}
	}

	// Remove from disabled_api_keys
	newDisabledKeys := make([]objects.DisabledAPIKey, 0, len(ch.DisabledAPIKeys))
	for _, dk := range ch.DisabledAPIKeys {
		if _, found := keysToDelete[dk.Key]; !found {
			newDisabledKeys = append(newDisabledKeys, dk)
		}
	}

	// Remove from credentials
	newCredentials := ch.Credentials
	if len(newCredentials.APIKeys) > 0 {
		filteredKeys := make([]string, 0, len(newCredentials.APIKeys))
		for _, k := range newCredentials.APIKeys {
			if _, found := keysToDelete[k]; !found {
				filteredKeys = append(filteredKeys, k)
			}
		}

		newCredentials.APIKeys = filteredKeys
	}

	if newCredentials.APIKey != "" {
		if _, found := keysToDelete[newCredentials.APIKey]; found {
			newCredentials.APIKey = ""
		}
	}

	// Ensure at least one API key remains
	allKeys := newCredentials.GetAllAPIKeys()
	if len(allKeys) == 0 {
		// Restore at least one key from the keys being deleted
		// Prefer the first key that was supposed to be deleted
		restoredKey := keys[0]
		newCredentials.APIKeys = []string{restoredKey}
	}

	update := svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetDisabledAPIKeys(newDisabledKeys).
		SetCredentials(newCredentials)
	update = applyRecoveredChannelStatus(ctx, update, ch, newCredentials, newDisabledKeys)

	if _, err := update.Save(ctx); err != nil {
		return nil, fmt.Errorf("failed to delete disabled api keys: %w", err)
	}

	log.Info(ctx, "Disabled API keys deleted",
		log.Int("channel_id", channelID),
		log.Int("count", len(keys)),
	)

	// Check if we had to preserve a key
	result := &DeleteDisabledAPIKeysResult{Success: true}
	if len(allKeys) == 0 {
		result.Message = "ONE_KEY_PRESERVED"
	}

	svc.asyncReloadChannels()

	return result, nil
}

func applyRecoveredChannelStatus(
	ctx context.Context,
	update *ent.ChannelUpdateOne,
	ch *ent.Channel,
	credentials objects.ChannelCredentials,
	disabledKeys []objects.DisabledAPIKey,
) *ent.ChannelUpdateOne {
	if ch.Status != channel.StatusDisabled || ch.ErrorMessage == nil ||
		!strings.HasPrefix(*ch.ErrorMessage, allKeysDisabledErrorPrefix) ||
		len(credentials.GetEnabledCredentialRefs(disabledKeys)) == 0 {
		return update
	}

	log.Info(ctx, "Re-enabled channel after API key availability recovered",
		log.Int("channel_id", ch.ID),
		log.String("channel_name", ch.Name),
	)

	return update.SetStatus(channel.StatusEnabled).ClearErrorMessage().ClearAutoDisabledAt()
}

// cleanupExpiredDisabledAPIKeys prunes elapsed temporary disables and restores
// channels that were disabled only because all of their keys were unavailable.
func (svc *ChannelService) cleanupExpiredDisabledAPIKeys(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ctx = authz.WithSystemBypass(ctx, "channel-cleanup-expired-disabled-api-keys")

	channelIDs, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.StatusIn(channel.StatusEnabled, channel.StatusDisabled)).
		IDs(ctx)
	if err != nil {
		log.Error(ctx, "Failed to query channels for expired API key cleanup", log.Cause(err))
		return
	}

	needsReload := false
	for _, channelID := range channelIDs {
		cleaned, removed, err := svc.cleanupChannelExpiredDisabledAPIKeys(ctx, channelID)
		if err != nil {
			log.Error(ctx, "Failed to cleanup expired disabled API keys",
				log.Int("channel_id", channelID),
				log.Cause(err),
			)
			continue
		}
		if !cleaned {
			continue
		}

		log.Info(ctx, "Cleaned up expired disabled API keys",
			log.Int("channel_id", channelID),
			log.Int("removed", removed),
		)
		needsReload = true
	}

	if needsReload {
		svc.asyncReloadChannels()
	}
}

func (svc *ChannelService) cleanupChannelExpiredDisabledAPIKeys(ctx context.Context, channelID int) (bool, int, error) {
	svc.apiKeyOpsLock.Lock()
	defer svc.apiKeyOpsLock.Unlock()

	entClient := svc.entFromContext(ctx)
	ch, err := entClient.Channel.Get(ctx, channelID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get channel: %w", err)
	}

	active := lo.Filter(ch.DisabledAPIKeys, func(dk objects.DisabledAPIKey, _ int) bool {
		return !dk.IsExpired()
	})
	removed := len(ch.DisabledAPIKeys) - len(active)
	if removed == 0 {
		return false, 0, nil
	}

	update := entClient.Channel.UpdateOneID(ch.ID).SetDisabledAPIKeys(active)
	update = applyRecoveredChannelStatus(ctx, update, ch, ch.Credentials, active)
	if _, err := update.Save(ctx); err != nil {
		return false, 0, fmt.Errorf("failed to update channel: %w", err)
	}

	return true, removed, nil
}
