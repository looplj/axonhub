package objects

import (
	"encoding/json"
	"fmt"
)

const (
	RoutingPolicyDefault = "default"

	LoadBalancerStrategyAdaptive       = "adaptive"
	LoadBalancerStrategyFailover       = "failover"
	LoadBalancerStrategyCircuitBreaker = "circuit-breaker"
	LoadBalancerStrategyRoundRobin     = "round-robin"

	TraceStickyModeDisabled              = "disabled"
	TraceStickyModePreferPreviousChannel = "prefer_previous_channel"
)

func NormalizeRoutingPolicyValue(value string) string {
	switch value {
	case "", "system_default":
		return RoutingPolicyDefault
	default:
		return value
	}
}

func IsValidLoadBalancerStrategy(value string) bool {
	switch NormalizeRoutingPolicyValue(value) {
	case RoutingPolicyDefault,
		LoadBalancerStrategyAdaptive,
		LoadBalancerStrategyFailover,
		LoadBalancerStrategyCircuitBreaker,
		LoadBalancerStrategyRoundRobin:
		return true
	default:
		return false
	}
}

func IsValidTraceStickyMode(value string) bool {
	switch NormalizeRoutingPolicyValue(value) {
	case RoutingPolicyDefault,
		TraceStickyModeDisabled,
		TraceStickyModePreferPreviousChannel:
		return true
	default:
		return false
	}
}

func (s *ModelSettings) NormalizeRoutingPolicy() {
	if s == nil {
		return
	}

	s.LoadBalancerStrategy = NormalizeRoutingPolicyValue(s.LoadBalancerStrategy)
	s.TraceStickyMode = NormalizeRoutingPolicyValue(s.TraceStickyMode)
}

func (s *ModelSettings) UnmarshalJSON(data []byte) error {
	type modelSettingsAlias ModelSettings

	settings := modelSettingsAlias{
		LoadBalancerStrategy: RoutingPolicyDefault,
		TraceStickyMode:      RoutingPolicyDefault,
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("failed to unmarshal model settings: %w", err)
	}

	*s = ModelSettings(settings)
	s.NormalizeRoutingPolicy()

	return nil
}
