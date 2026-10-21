package biz

import (
	"slices"
	"strings"
)

const (
	ClientCancellationActionSwitchAndCooldown = "switch_and_cooldown"
	ClientCancellationActionCooldownOnly      = "cooldown_only"
	// ClientCancellationActionCooldownModel cools down only the (channel, model)
	// pair that served the canceled requests. The channel stays enabled and keeps
	// serving its other models; the model recovers when the cooldown elapses or an
	// operator clears it manually.
	ClientCancellationActionCooldownModel = "cooldown_model"

	ClientCancellationScopeGlobal       = "global"
	ClientCancellationScopeProjectFirst = "project_first"

	defaultClientCancellationThreshold       = 2
	defaultClientCancellationCooldownMinutes = 30
	maxClientCancellationCooldownMinutes     = 7 * 24 * 60
)

func normalizeClientCancellationPolicy(policy *ClientCancellationPolicy) error {
	if policy == nil {
		return nil
	}

	// A stored policy written before this feature existed carries only zero
	// values. Treat it as the documented default so upgrading does not silently
	// turn the cooldown off; an explicit disable always ships the other fields
	// that normalization fills in below.
	if !policy.Enabled && policy.Threshold <= 0 && policy.CooldownMinutes <= 0 &&
		policy.Action == "" && policy.Scope == "" && len(policy.ProjectOverrides) == 0 {
		policy.Enabled = true
	}

	if policy.Threshold <= 0 {
		policy.Threshold = defaultClientCancellationThreshold
	}
	if policy.CooldownMinutes <= 0 {
		policy.CooldownMinutes = defaultClientCancellationCooldownMinutes
	}
	if policy.CooldownMinutes > maxClientCancellationCooldownMinutes {
		policy.CooldownMinutes = maxClientCancellationCooldownMinutes
	}

	switch policy.Action {
	case ClientCancellationActionSwitchAndCooldown, ClientCancellationActionCooldownOnly, ClientCancellationActionCooldownModel:
	default:
		policy.Action = ClientCancellationActionSwitchAndCooldown
	}

	switch policy.Scope {
	case ClientCancellationScopeGlobal, ClientCancellationScopeProjectFirst:
	default:
		policy.Scope = ClientCancellationScopeGlobal
	}

	if policy.ProjectOverrides == nil {
		policy.ProjectOverrides = []ClientCancellationProjectPolicy{}
	}

	seen := make(map[int]struct{}, len(policy.ProjectOverrides))
	overrides := make([]ClientCancellationProjectPolicy, 0, len(policy.ProjectOverrides))
	for _, override := range policy.ProjectOverrides {
		if override.ProjectID <= 0 {
			continue
		}
		if _, ok := seen[override.ProjectID]; ok {
			continue
		}
		if override.Threshold <= 0 {
			override.Threshold = policy.Threshold
		}
		if override.CooldownMinutes <= 0 {
			override.CooldownMinutes = policy.CooldownMinutes
		}
		if override.CooldownMinutes > maxClientCancellationCooldownMinutes {
			override.CooldownMinutes = maxClientCancellationCooldownMinutes
		}
		switch override.Action {
		case ClientCancellationActionSwitchAndCooldown, ClientCancellationActionCooldownOnly, ClientCancellationActionCooldownModel:
		default:
			override.Action = policy.Action
		}
		seen[override.ProjectID] = struct{}{}
		overrides = append(overrides, override)
	}
	policy.ProjectOverrides = overrides

	return nil
}

// EffectiveClientCancellationPolicy resolves the policy for a project. Global
// settings are used unless project_first is enabled and an enabled override
// exists for the project.
func (p ClientCancellationPolicy) EffectiveClientCancellationPolicy(projectID int) (ClientCancellationPolicy, string) {
	if p.Scope == ClientCancellationScopeProjectFirst && projectID > 0 {
		for _, override := range p.ProjectOverrides {
			if override.ProjectID != projectID || !override.Enabled {
				continue
			}
			return ClientCancellationPolicy{
				Enabled:         true,
				Threshold:       override.Threshold,
				CooldownMinutes: override.CooldownMinutes,
				Action:          override.Action,
			}, "project"
		}
	}

	return ClientCancellationPolicy{
		Enabled:          p.Enabled,
		Threshold:        p.Threshold,
		CooldownMinutes:  p.CooldownMinutes,
		Action:           p.Action,
		Scope:            p.Scope,
		ProjectOverrides: slices.Clone(p.ProjectOverrides),
	}, "global"
}

func normalizeClientCancellationAction(action string) string {
	action = strings.TrimSpace(action)
	switch action {
	case ClientCancellationActionSwitchAndCooldown, ClientCancellationActionCooldownOnly, ClientCancellationActionCooldownModel:
		return action
	default:
		return ClientCancellationActionSwitchAndCooldown
	}
}
