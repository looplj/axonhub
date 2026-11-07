package objects

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/looplj/axonhub/llm"
)

type PluginRequestHook struct {
	Enabled      bool              `json:"enabled"`
	RequestTypes []llm.RequestType `json:"requestTypes,omitempty"`
}
type PluginAttemptHook struct {
	Enabled    bool            `json:"enabled"`
	APIFormats []llm.APIFormat `json:"apiFormats,omitempty"`
}

func (h *PluginRequestHook) IsEnabled() bool { return h != nil && h.Enabled }
func (h *PluginAttemptHook) IsEnabled() bool { return h != nil && h.Enabled }
func (h *PluginRequestHook) Matches(t llm.RequestType) bool {
	if t == "" {
		t = llm.RequestTypeChat
	}
	return h.IsEnabled() && slices.Contains(h.RequestTypes, t)
}

func (h *PluginAttemptHook) Matches(f llm.APIFormat) bool {
	return h.IsEnabled() && slices.Contains(h.APIFormats, f)
}

func decodeHook[T ~string](data []byte, name, scope string, allowed []T) (bool, []T, error) {
	raw, err := decodeObject(data, name)
	if err != nil {
		return false, nil, err
	}
	if err := rejectUnknownFields(raw, map[string]struct{}{"enabled": {}, scope: {}}, name); err != nil {
		return false, nil, err
	}
	value, ok := raw["enabled"]
	if !ok {
		return false, nil, fmt.Errorf("%s.enabled is required", name)
	}
	enabled, err := decodeJSONBool(value, name+".enabled")
	if err != nil {
		return false, nil, err
	}
	var values []T
	if value, ok := raw[scope]; ok {
		if len(value) == 0 || value[0] != '[' {
			return false, nil, fmt.Errorf("%s.%s must be an array", name, scope)
		}
		if err := json.Unmarshal(value, &values); err != nil {
			return false, nil, fmt.Errorf("%s.%s: %w", name, scope, err)
		}
	}
	if enabled && len(values) == 0 {
		return false, nil, fmt.Errorf("%s.%s must not be empty when enabled", name, scope)
	}
	seen := map[T]bool{}
	for _, v := range values {
		if !slices.Contains(allowed, v) || seen[v] {
			return false, nil, fmt.Errorf("%s.%s: unknown or duplicate value %q", name, scope, v)
		}
		seen[v] = true
	}
	return enabled, values, nil
}

func (h *PluginRequestHook) UnmarshalJSON(data []byte) error {
	enabled, values, err := decodeHook(data, "hooks.request", "requestTypes", llm.AllRequestTypes())
	if err != nil {
		return err
	}
	h.Enabled = enabled
	h.RequestTypes = values
	return nil
}

func (h *PluginAttemptHook) UnmarshalJSON(data []byte) error {
	enabled, values, err := decodeHook(data, "hooks.attempt", "apiFormats", llm.AllAPIFormats())
	if err != nil {
		return err
	}
	h.Enabled = enabled
	h.APIFormats = values
	return nil
}
