package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelCircuitBreaker_RecordError(t *testing.T) {
	ctx := context.Background()
	manager := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-4"

	// Test initial state
	stats := manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 0, stats.ConsecutiveFailures)

	// Record first error - should remain closed
	manager.RecordError(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 1, stats.ConsecutiveFailures)

	// Record second error - should remain closed
	manager.RecordError(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 2, stats.ConsecutiveFailures)

	// Record third error - should become half-open
	manager.RecordError(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateHalfOpen, stats.State)
	assert.Equal(t, 3, stats.ConsecutiveFailures)

	// Record fourth error - should remain half-open
	manager.RecordError(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateHalfOpen, stats.State)
	assert.Equal(t, 4, stats.ConsecutiveFailures)

	// Record fifth error - should become open
	manager.RecordError(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateOpen, stats.State)
	assert.Equal(t, 5, stats.ConsecutiveFailures)
	assert.False(t, stats.NextProbeAt.IsZero())
}

func TestModelCircuitBreaker_RecordSuccess(t *testing.T) {
	ctx := context.Background()
	manager := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-4"

	// Record errors to make it half-open
	for i := 0; i < 3; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	stats := manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateHalfOpen, stats.State)
	assert.Equal(t, 3, stats.ConsecutiveFailures)

	// Record success - should become closed immediately
	manager.RecordSuccess(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 0, stats.ConsecutiveFailures)
	assert.True(t, stats.NextProbeAt.IsZero())
}

func TestModelCircuitBreaker_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	manager := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-4"

	// Record 2 errors
	manager.RecordError(ctx, channelID, modelID)
	manager.RecordError(ctx, channelID, modelID)

	stats := manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 2, stats.ConsecutiveFailures)

	// Manually set last failure time to simulate TTL expiry
	s := manager.getStats(channelID, modelID)
	s.Lock()
	s.LastFailureAt = time.Now().Add(-31 * time.Minute) // Older than TTL
	s.Unlock()

	// Record another error - should reset counter due to TTL
	manager.RecordError(ctx, channelID, modelID)
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 1, stats.ConsecutiveFailures) // Reset to 1
}

func TestModelCircuitBreaker_GetEffectiveWeight(t *testing.T) {
	ctx := context.Background()
	manager := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-4"
	baseWeight := 1.0

	// Test closed state
	weight := manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	assert.Equal(t, baseWeight, weight)

	// Make it half-open
	for i := 0; i < 3; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	// Test half-open state
	weight = manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	policy := manager.GetPolicy(ctx)
	expectedWeight := baseWeight * policy.HalfOpenWeight
	assert.Equal(t, expectedWeight, weight)

	// Make it open
	for i := 0; i < 2; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	// Test open state (should be 0 before probe time)
	weight = manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	assert.Equal(t, 0.0, weight)

	// Simulate probe time passed
	s := manager.getStats(channelID, modelID)
	s.Lock()
	s.NextProbeAt = time.Now().Add(-1 * time.Minute) // In the past
	s.Unlock()

	// Test open state (should allow probe)
	weight = manager.GetEffectiveWeight(ctx, channelID, modelID, baseWeight)
	assert.Equal(t, 0.01, weight) // Probe weight
}

func TestModelCircuitBreaker_GetAllNonClosedModels(t *testing.T) {
	ctx := context.Background()
	manager := NewModelCircuitBreaker()

	// Initially no non-closed models
	nonClosed := manager.GetAllNonClosedModels(ctx)
	assert.Empty(t, nonClosed)

	// Make one model half-open
	manager.RecordError(ctx, 1, "gpt-4")
	manager.RecordError(ctx, 1, "gpt-4")
	manager.RecordError(ctx, 1, "gpt-4")

	// Make another model open
	for i := 0; i < 5; i++ {
		manager.RecordError(ctx, 2, "claude-3")
	}

	// Should return both non-closed models
	nonClosed = manager.GetAllNonClosedModels(ctx)
	assert.Len(t, nonClosed, 2)

	// Check the models are correctly identified
	modelStates := make(map[string]CircuitBreakerState)
	for _, m := range nonClosed {
		key := m.ModelID
		modelStates[key] = m.State
	}

	assert.Equal(t, StateHalfOpen, modelStates["gpt-4"])
	assert.Equal(t, StateOpen, modelStates["claude-3"])
}

func TestModelCircuitBreaker_ResetModelStatus(t *testing.T) {
	ctx := context.Background()
	manager := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-4"

	// Make model open
	for i := 0; i < 5; i++ {
		manager.RecordError(ctx, channelID, modelID)
	}

	stats := manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateOpen, stats.State)

	// Reset status
	err := manager.ResetModelStatus(ctx, channelID, modelID)
	require.NoError(t, err)

	// Should be closed now
	stats = manager.GetModelCircuitBreakerStats(ctx, channelID, modelID)
	assert.Equal(t, StateClosed, stats.State)
	assert.Equal(t, 0, stats.ConsecutiveFailures)
	assert.True(t, stats.NextProbeAt.IsZero())
}

func TestModelCircuitBreakerPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  ModelCircuitBreakerPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: ModelCircuitBreakerPolicy{
				HalfOpenThreshold: 3,
				OpenThreshold:     5,
				HalfOpenWeight:    0.3,
			},
			wantErr: false,
		},
		{
			name: "half-open threshold >= open threshold",
			policy: ModelCircuitBreakerPolicy{
				HalfOpenThreshold: 5,
				OpenThreshold:     5,
				HalfOpenWeight:    0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid half-open weight - negative",
			policy: ModelCircuitBreakerPolicy{
				HalfOpenThreshold: 3,
				OpenThreshold:     5,
				HalfOpenWeight:    -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid half-open weight - greater than 1",
			policy: ModelCircuitBreakerPolicy{
				HalfOpenThreshold: 3,
				OpenThreshold:     5,
				HalfOpenWeight:    1.1,
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
