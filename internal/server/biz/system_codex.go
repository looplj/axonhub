package biz

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/scopes"
)

const SystemKeyCodexCompatibility = "codex_compatibility"

type CodexCompatibilitySettings struct {
	Enabled   bool `json:"enabled"`
	ChannelID *int `json:"channelID"`
}

func (s *SystemService) CodexCompatibilitySettings(ctx context.Context) (*CodexCompatibilitySettings, error) {
	if err := authz.RequireScope(ctx, scopes.ScopeReadSettings); err != nil {
		return nil, err
	}
	value, err := s.getSystemValue(ctx, SystemKeyCodexCompatibility)
	if ent.IsNotFound(err) {
		return &CodexCompatibilitySettings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Codex compatibility settings: %w", err)
	}
	var settings CodexCompatibilitySettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("decode Codex compatibility settings: %w", err)
	}
	return &settings, nil
}

func (s *SystemService) SetCodexCompatibilitySettings(ctx context.Context, settings CodexCompatibilitySettings) error {
	if err := authz.RequireScope(ctx, scopes.ScopeWriteSettings); err != nil {
		return err
	}
	if settings.Enabled && settings.ChannelID == nil {
		return xerrors.ValidationError("a Codex channel is required when enabled")
	}
	if settings.ChannelID != nil {
		if err := authz.RequireScope(ctx, scopes.ScopeReadChannels); err != nil {
			return err
		}
		entity, err := s.entFromContext(ctx).Channel.Get(ctx, *settings.ChannelID)
		if err != nil {
			return xerrors.ValidationError("Codex catalog channel is unavailable or inaccessible")
		}
		if entity.Type != channel.TypeCodex {
			return xerrors.ValidationError("catalog channel must be Codex")
		}
	}
	value, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("encode Codex compatibility settings: %w", err)
	}
	return s.setSystemValue(ctx, SystemKeyCodexCompatibility, string(value))
}
