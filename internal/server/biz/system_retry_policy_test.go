package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

// TestSystemService_RetryPolicyOrDefault_WithAPIKeyPrincipal reproduces
// https://github.com/looplj/axonhub/issues/2080: internal retry-policy reads
// used to run with the API-key request context, which carries no user
// principal, so the Ent privacy layer denied the system-settings query with
// "no user in context" and RetryPolicyOrDefault silently fell back to the
// default policy.
func TestSystemService_RetryPolicyOrDefault_WithAPIKeyPrincipal(t *testing.T) {
	service, client := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer client.Close()

	// Persist a distinctive non-default policy.
	adminCtx := authz.WithTestBypass(ent.NewContext(t.Context(), client))
	require.NoError(t, service.SetRetryPolicy(adminCtx, &RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       7,
		MaxSingleChannelRetries: 5,
		RetryDelayMs:            4242,
		LoadBalancerStrategy:    "round-robin",
	}))

	// Read it back the way an API-key-authenticated streaming path does: ent
	// client in context, API-key principal, and no privacy bypass.
	apiKeyCtx := authz.NewAPIKeyContext(ent.NewContext(t.Context(), client), 1, 1)

	got := service.RetryPolicyOrDefault(apiKeyCtx)
	require.Equal(t, 7, got.MaxChannelRetries, "configured retry policy must survive an API-key request context")
	require.Equal(t, 4242, got.RetryDelayMs)
}
