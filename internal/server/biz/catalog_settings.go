package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
)

const (
	SystemKeyCatalogSettings = "catalog_settings"

	DefaultCatalogUpstreamURL    = "https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json"
	DefaultCatalogRefreshSeconds = 3600
	minCatalogRefreshSeconds     = 60
	maxCatalogRefreshSeconds     = 7 * 24 * 3600
)

// CatalogSettings controls how the built-in model catalog is refreshed.
type CatalogSettings struct {
	UpstreamURL    string `json:"upstream_url"`
	RefreshSeconds int    `json:"refresh_seconds"`
}

func normalizeCatalogSettings(settings *CatalogSettings) {
	settings.UpstreamURL = strings.TrimSpace(settings.UpstreamURL)
	if settings.RefreshSeconds <= 0 {
		settings.RefreshSeconds = DefaultCatalogRefreshSeconds
	}
	if settings.RefreshSeconds < minCatalogRefreshSeconds {
		settings.RefreshSeconds = minCatalogRefreshSeconds
	}
	if settings.RefreshSeconds > maxCatalogRefreshSeconds {
		settings.RefreshSeconds = maxCatalogRefreshSeconds
	}
}

func validateCatalogSettings(settings CatalogSettings) error {
	if settings.UpstreamURL == "" {
		return nil
	}

	parsed, err := url.Parse(settings.UpstreamURL)
	if err != nil {
		return fmt.Errorf("invalid catalog upstream url: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("catalog upstream url must be http or https")
	}

	if parsed.Host == "" {
		return fmt.Errorf("catalog upstream url is missing a host")
	}

	return nil
}

func (settings CatalogSettings) EffectiveUpstreamURL() string {
	if strings.TrimSpace(settings.UpstreamURL) == "" {
		return DefaultCatalogUpstreamURL
	}

	return settings.UpstreamURL
}

func defaultCatalogSettings() CatalogSettings {
	return CatalogSettings{
		UpstreamURL:    "",
		RefreshSeconds: DefaultCatalogRefreshSeconds,
	}
}

func (s *SystemService) CatalogSettings(ctx context.Context) (*CatalogSettings, error) {
	value, err := authz.RunWithSystemBypass(ctx, "catalog-settings", func(bypassCtx context.Context) (string, error) {
		return s.getSystemValue(bypassCtx, SystemKeyCatalogSettings)
	})
	if err != nil {
		if ent.IsNotFound(err) {
			return lo.ToPtr(defaultCatalogSettings()), nil
		}

		return nil, fmt.Errorf("failed to get catalog settings: %w", err)
	}

	settings := defaultCatalogSettings()
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal catalog settings: %w", err)
	}

	normalizeCatalogSettings(&settings)

	return &settings, nil
}

func (s *SystemService) CatalogSettingsOrDefault(ctx context.Context) CatalogSettings {
	settings, err := s.CatalogSettings(ctx)
	if err != nil {
		return defaultCatalogSettings()
	}

	return *settings
}

func (s *SystemService) SetCatalogSettings(ctx context.Context, settings CatalogSettings) error {
	normalizeCatalogSettings(&settings)
	if err := validateCatalogSettings(settings); err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal catalog settings: %w", err)
	}

	return s.setSystemValue(ctx, SystemKeyCatalogSettings, string(jsonBytes))
}
