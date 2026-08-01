package biz

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/objects"
)

func TestValidateProfileRoutingPolicies(t *testing.T) {
	profiles := []objects.APIKeyProfile{{Name: "default"}}
	require.NoError(t, validateProfileRoutingPolicies(profiles))
	require.Equal(t, objects.RoutingPolicyDefault, *profiles[0].LoadBalanceStrategy)
	require.Equal(t, objects.RoutingPolicyDefault, *profiles[0].TraceStickyMode)

	profiles = []objects.APIKeyProfile{{
		Name:                "legacy",
		LoadBalanceStrategy: lo.ToPtr("system_default"),
		TraceStickyMode:     lo.ToPtr(""),
	}}
	require.NoError(t, validateProfileRoutingPolicies(profiles))
	require.Equal(t, objects.RoutingPolicyDefault, *profiles[0].LoadBalanceStrategy)
	require.Equal(t, objects.RoutingPolicyDefault, *profiles[0].TraceStickyMode)

	require.Error(t, validateProfileRoutingPolicies([]objects.APIKeyProfile{{
		Name:                "invalid",
		LoadBalanceStrategy: lo.ToPtr("unknown"),
	}}))
	require.Error(t, validateProfileRoutingPolicies([]objects.APIKeyProfile{{
		Name:            "invalid",
		TraceStickyMode: lo.ToPtr("unknown"),
	}}))
}
