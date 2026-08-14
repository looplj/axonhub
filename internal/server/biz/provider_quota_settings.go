package biz

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
)

const SystemKeyProviderQuotaCollectionSettings = "provider_quota_collection_settings"

var supportedProviderQuotaTypes = []string{
	"claudecode", "codex", "github_copilot", "nanogpt", "cline",
	"xai_subscription",
	"wafer", "synthetic", "neuralwatt", "apertis", "opencode_go",
	"kimi_code", "minimax", "zhipu", "charm_hyper",
}

var supportedProviderQuotaTypeSet = func() map[string]struct{} {
	result := make(map[string]struct{}, len(supportedProviderQuotaTypes))
	for _, providerType := range supportedProviderQuotaTypes {
		result[providerType] = struct{}{}
	}
	return result
}()

type ProviderQuotaCollectionSettings struct {
	Enabled   bool            `json:"enabled"`
	Providers map[string]bool `json:"providers"`
}

type ProviderQuotaCollectionProvider struct {
	Provider string `json:"provider"`
	Enabled  bool   `json:"enabled"`
}

func defaultProviderQuotaCollectionSettings() *ProviderQuotaCollectionSettings {
	providers := make(map[string]bool, len(supportedProviderQuotaTypes))
	for _, providerType := range supportedProviderQuotaTypes {
		providers[providerType] = true
	}

	return &ProviderQuotaCollectionSettings{Enabled: true, Providers: providers}
}

func normalizeProviderQuotaCollectionSettings(settings *ProviderQuotaCollectionSettings) *ProviderQuotaCollectionSettings {
	normalized := defaultProviderQuotaCollectionSettings()
	normalized.Enabled = settings.Enabled

	// 旧配置缺少后来新增的提供商时保持默认开启，避免升级后静默关闭新能力。
	for providerType, enabled := range settings.Providers {
		if _, ok := supportedProviderQuotaTypeSet[providerType]; ok {
			normalized.Providers[providerType] = enabled
		}
	}

	return normalized
}

func SupportedProviderQuotaTypes() []string {
	result := make([]string, len(supportedProviderQuotaTypes))
	copy(result, supportedProviderQuotaTypes)
	return result
}

func (s *SystemService) ProviderQuotaCollectionSettings(ctx context.Context) (*ProviderQuotaCollectionSettings, error) {
	value, err := s.getSystemValue(ctx, SystemKeyProviderQuotaCollectionSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			settings := defaultProviderQuotaCollectionSettings()
			jsonBytes, marshalErr := json.Marshal(settings)
			if marshalErr != nil {
				return nil, fmt.Errorf("failed to marshal default provider quota collection settings: %w", marshalErr)
			}

			// 缓存规范化默认值，避免未保存设置时在渠道循环中重复查询数据库。
			_ = s.Cache.Set(ctx, "system:"+SystemKeyProviderQuotaCollectionSettings, ent.System{
				Key:   SystemKeyProviderQuotaCollectionSettings,
				Value: string(jsonBytes),
			})

			return settings, nil
		}
		return nil, fmt.Errorf("failed to get provider quota collection settings: %w", err)
	}

	var settings ProviderQuotaCollectionSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal provider quota collection settings: %w", err)
	}

	return normalizeProviderQuotaCollectionSettings(&settings), nil
}

func (s *SystemService) ProviderQuotaCollectionSettingsOrDefault(ctx context.Context) *ProviderQuotaCollectionSettings {
	settings, err := s.ProviderQuotaCollectionSettings(ctx)
	if err != nil {
		log.Warn(ctx, "failed to get provider quota collection settings", log.Cause(err))
		return defaultProviderQuotaCollectionSettings()
	}
	return settings
}

func (s *SystemService) UpdateProviderQuotaCollectionSettings(
	ctx context.Context,
	enabled *bool,
	providers []ProviderQuotaCollectionProvider,
) error {
	settings, err := s.ProviderQuotaCollectionSettings(ctx)
	if err != nil {
		return err
	}
	if enabled != nil {
		settings.Enabled = *enabled
	}

	seen := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		if _, ok := supportedProviderQuotaTypeSet[provider.Provider]; !ok {
			return fmt.Errorf("unsupported provider quota type: %q", provider.Provider)
		}
		if _, ok := seen[provider.Provider]; ok {
			return fmt.Errorf("duplicate provider quota type: %q", provider.Provider)
		}
		seen[provider.Provider] = struct{}{}
		settings.Providers[provider.Provider] = provider.Enabled
	}

	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal provider quota collection settings: %w", err)
	}
	if err := s.setSystemValue(ctx, SystemKeyProviderQuotaCollectionSettings, string(jsonBytes)); err != nil {
		return err
	}

	// Publish the exact primary value only after the surrounding transaction
	// commits. This avoids exposing rolled-back settings while still preventing
	// a lagging read replica from repopulating the cache with an older value.
	runAfterCommit(ctx, func(ctx context.Context) {
		if err := s.Cache.Set(ctx, "system:"+SystemKeyProviderQuotaCollectionSettings, ent.System{
			Key:   SystemKeyProviderQuotaCollectionSettings,
			Value: string(jsonBytes),
		}); err != nil {
			log.Warn(ctx, "failed to cache provider quota collection settings", log.Cause(err))
		}
	})

	return nil
}

func (s *SystemService) IsProviderQuotaCollectionEnabled(ctx context.Context, providerType string) (bool, error) {
	if _, ok := supportedProviderQuotaTypeSet[providerType]; !ok {
		return false, fmt.Errorf("unsupported provider quota type: %q", providerType)
	}
	settings, err := s.ProviderQuotaCollectionSettings(ctx)
	if err != nil {
		return false, err
	}
	return settings.Enabled && settings.Providers[providerType], nil
}
