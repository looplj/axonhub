package objects

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelSettingsUnmarshalJSONRoutingDefaults(t *testing.T) {
	var settings ModelSettings
	require.NoError(t, json.Unmarshal([]byte(`{"associations":[]}`), &settings))
	require.Equal(t, RoutingPolicyDefault, settings.LoadBalancerStrategy)
	require.Equal(t, RoutingPolicyDefault, settings.TraceStickyMode)
}

func TestModelSettingsUnmarshalJSONNormalizesLegacyDefault(t *testing.T) {
	var settings ModelSettings
	require.NoError(t, json.Unmarshal([]byte(`{
		"associations":[],
		"loadBalancerStrategy":"system_default",
		"traceStickyMode":""
	}`), &settings))
	require.Equal(t, RoutingPolicyDefault, settings.LoadBalancerStrategy)
	require.Equal(t, RoutingPolicyDefault, settings.TraceStickyMode)
}

func TestRoutingPolicyValidation(t *testing.T) {
	require.True(t, IsValidLoadBalancerStrategy(RoutingPolicyDefault))
	require.True(t, IsValidLoadBalancerStrategy(LoadBalancerStrategyRoundRobin))
	require.False(t, IsValidLoadBalancerStrategy("unknown"))
	require.True(t, IsValidTraceStickyMode(RoutingPolicyDefault))
	require.True(t, IsValidTraceStickyMode(TraceStickyModeDisabled))
	require.False(t, IsValidTraceStickyMode("unknown"))
}
