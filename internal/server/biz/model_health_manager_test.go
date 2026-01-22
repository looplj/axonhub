package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelHealthManager_RecordError(t *testing.T) {
	ctx := context.Background()
	// Create a mock system service - in real implementation this would be properly initialized
	manager := NewModelHealthManager(nil) // Pass nil for now since we're not using system service in tests

	channelID := 1
	modelID := "gpt-4"

	// Test initial state
	health := manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 0, health.ConsecutiveFailures)

	// Record first error - should remain healthy
	manager.RecordError(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 1, health.ConsecutiveFailures)

	// Record second error - should remain healthy
	manager.RecordError(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 2, health.ConsecutiveFailures)

	// Record third error - should become degraded
	manager.RecordError(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusDegraded, health.Status)
	assert.Equal(t, 3, health.ConsecutiveFailures)

	// Record fourth error - should remain degraded
	manager.RecordError(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusDegraded, health.Status)
	assert.Equal(t, 4, health.ConsecutiveFailures)

	// Record fifth error - should become disabled
	manager.RecordError(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusDisabled, health.Status)
	assert.Equal(t, 5, health.ConsecutiveFailures)
	assert.False(t, health.NextProbeAt.IsZero())
}

func TestModelHealthManager_RecordSuccess(t *testing.T) {
	ctx := context.Background()
	manager := NewModelHealthManager(nil) // Pass nil for now since we're not using system service in tests

	channelID := 1
	modelID := "gpt-4"

	// Record errors to make it degraded
	for i := 0; i < 3; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	health := manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusDegraded, health.Status)
	assert.Equal(t, 3, health.ConsecutiveFailures)

	// Record success - should become healthy immediately
	manager.RecordSuccess(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 0, health.ConsecutiveFailures)
	assert.True(t, health.NextProbeAt.IsZero())
}

func TestModelHealthManager_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	manager := NewModelHealthManager(nil) // Pass nil for now since we're not using system service in tests

	channelID := 1
	modelID := "gpt-4"

	// Record 2 errors
	manager.RecordError(ctx, channelID, modelID)
	manager.RecordError(ctx, channelID, modelID)

	health := manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 2, health.ConsecutiveFailures)

	// Manually set last failure time to simulate TTL expiry
	stats := manager.getStats(channelID, modelID)
	stats.Lock()
	stats.LastFailureAt = time.Now().Add(-31 * time.Minute) // Older than TTL
	stats.Unlock()

	// Record another error - should reset counter due to TTL
	manager.RecordError(ctx, channelID, modelID)
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 1, health.ConsecutiveFailures) // Reset to 1
}

func TestModelHealthManager_GetEffectiveWeight(t *testing.T) {
	ctx := context.Background()
	manager := NewModelHealthManager(nil) // Pass nil for now since we're not using system service in tests

	channelID := 1
	modelID := "gpt-4"
	baseWeight := 1.0

	// Test healthy status
	weight := manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	assert.Equal(t, baseWeight, weight)

	// Make it degraded
	for i := 0; i < 3; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	// Test degraded status
	weight = manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	policy := manager.GetPolicy(ctx)
	expectedWeight := baseWeight * policy.DegradedWeight
	assert.Equal(t, expectedWeight, weight)

	// Make it disabled
	for i := 0; i < 2; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	// Test disabled status (should be 0 before probe time)
	weight = manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	assert.Equal(t, 0.0, weight)

	// Simulate probe time passed
	stats := manager.getStats(channelID, modelID)
	stats.Lock()
	stats.NextProbeAt = time.Now().Add(-1 * time.Minute) // In the past
	stats.Unlock()

	// Test disabled status (should allow probe)
	weight = manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	assert.Equal(t, 0.01, weight) // Probe weight
}

func TestModelHealthManager_GetAllUnhealthyModels(t *testing.T) {
	ctx := context.Background()
	manager := NewModelHealthManager(nil) // Pass nil for now since we're not using system service in tests

	// Initially no unhealthy models
	unhealthy := manager.GetAllUnhealthyModels(ctx)
	assert.Empty(t, unhealthy)

	// Make one model degraded
	manager.RecordError(ctx, 1, "gpt-4")
	manager.RecordError(ctx, 1, "gpt-4")
	manager.RecordError(ctx, 1, "gpt-4")

	// Make another model disabled
	for i := 0; i < 5; i++ {
		manager.RecordError(ctx, 2, "claude-3")
	}

	// Should return both unhealthy models
	unhealthy = manager.GetAllUnhealthyModels(ctx)
	assert.Len(t, unhealthy, 2)

	// Check the models are correctly identified
	modelStatuses := make(map[string]HealthStatus)
	for _, model := range unhealthy {
		key := model.ModelID
		modelStatuses[key] = model.Status
	}

	assert.Equal(t, StatusDegraded, modelStatuses["gpt-4"])
	assert.Equal(t, StatusDisabled, modelStatuses["claude-3"])
}

func TestModelHealthManager_ResetModelStatus(t *testing.T) {
	ctx := context.Background()
	manager := NewModelHealthManager(nil) // Pass nil for now since we're not using system service in tests

	channelID := 1
	modelID := "gpt-4"

	// Make model disabled
	for i := 0; i < 5; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	health := manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusDisabled, health.Status)

	// Reset status
	err := manager.ResetModelStatus(ctx, channelID, modelID)
	require.NoError(t, err)

	// Should be healthy now
	health = manager.GetModelHealth(ctx, channelID, modelID)
	assert.Equal(t, StatusHealthy, health.Status)
	assert.Equal(t, 0, health.ConsecutiveFailures)
	assert.True(t, health.NextProbeAt.IsZero())
}

func TestModelHealthPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  ModelHealthPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: ModelHealthPolicy{
				DegradeThreshold: 3,
				DisableThreshold: 5,
				DegradedWeight:   0.3,
			},
			wantErr: false,
		},
		{
			name: "degrade threshold >= disable threshold",
			policy: ModelHealthPolicy{
				DegradeThreshold: 5,
				DisableThreshold: 5,
				DegradedWeight:   0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid degraded weight - negative",
			policy: ModelHealthPolicy{
				DegradeThreshold: 3,
				DisableThreshold: 5,
				DegradedWeight:   -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid degraded weight - greater than 1",
			policy: ModelHealthPolicy{
				DegradeThreshold: 3,
				DisableThreshold: 5,
				DegradedWeight:   1.1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
