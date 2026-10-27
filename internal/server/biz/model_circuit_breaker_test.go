package biz

import (
	"context"
	"testing"
	"time"
)

func TestModelCircuitBreakerProbeLock_SingleBeginAndExplicitEnd(t *testing.T) {
	ctx := context.Background()
	cb := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-test"

	for range 5 {
		cb.RecordError(ctx, channelID, modelID, false)
	}

	stats := cb.getStats(channelID, modelID)
	stats.Lock()
	stats.State = StateOpen
	stats.NextProbeAt = time.Now().Add(-time.Second)
	stats.Unlock()

	if got := cb.GetEffectiveWeight(ctx, channelID, modelID, 1.0); got <= 0 {
		t.Fatalf("expected positive probe weight, got %v", got)
	}

	ok := cb.TryBeginProbe(ctx, channelID, modelID)
	if !ok {
		t.Fatalf("expected to begin probe")
	}

	if got := cb.GetEffectiveWeight(ctx, channelID, modelID, 1.0); got != 0.0 {
		t.Fatalf("expected zero weight while probe in progress, got %v", got)
	}

	ok2 := cb.TryBeginProbe(ctx, channelID, modelID)
	if ok2 {
		t.Fatalf("expected second begin probe to fail")
	}

	cb.EndProbe(channelID, modelID)

	if got := cb.GetEffectiveWeight(ctx, channelID, modelID, 1.0); got <= 0 {
		t.Fatalf("expected positive probe weight after end, got %v", got)
	}
}

// TestRecordError_OpenState_BackoffOnlyOnProbe verifies that when the circuit
// breaker is in Open state, exponential backoff on NextProbeAt is only applied
// when the error came from an actual probe request (wasProbe=true). Non-probe
// errors must not push NextProbeAt further away, fixing the auto-recovery bug.
func TestRecordError_OpenState_BackoffOnlyOnProbe(t *testing.T) {
	ctx := context.Background()
	cb := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-test"

	// Push to Open state.
	for range 5 {
		cb.RecordError(ctx, channelID, modelID, false)
	}

	stats := cb.getStats(channelID, modelID)
	stats.Lock()
	if stats.State != StateOpen {
		t.Fatalf("expected Open state, got %s", stats.State)
	}
	// Pin NextProbeAt to a known time that is before what a fresh backoff would produce.
	// The backoff formula starts at 2x ProbeInterval (10min) for the first probe failure,
	// so pinning to now-1s ensures the new value will be strictly after the pin.
	pinnedProbeAt := time.Now().Add(-time.Second)
	stats.NextProbeAt = pinnedProbeAt
	stats.Unlock()

	// Simulate a non-probe error (e.g. request rejected by CB).
	cb.RecordError(ctx, channelID, modelID, false)

	stats.Lock()
	if !stats.NextProbeAt.Equal(pinnedProbeAt) {
		t.Fatalf("NextProbeAt should not change for non-probe errors in Open state: got %v, want %v",
			stats.NextProbeAt, pinnedProbeAt)
	}
	stats.Unlock()

	// Now simulate an actual probe failure.
	cb.RecordError(ctx, channelID, modelID, true)

	stats.Lock()
	if stats.NextProbeAt.Equal(pinnedProbeAt) {
		t.Fatal("NextProbeAt should have been updated with exponential backoff for probe failures")
	}
	if !stats.NextProbeAt.After(pinnedProbeAt) {
		t.Fatalf("NextProbeAt should be pushed further into the future after probe failure: got %v, pinned %v",
			stats.NextProbeAt, pinnedProbeAt)
	}
	stats.Unlock()
}

// TestGetEffectiveWeight_TTLAutoRecovery verifies that when no failures occur
// within the FailureStatsTTL window, GetEffectiveWeight lazily resets the
// circuit breaker to Closed, allowing the channel to recover automatically.
func TestGetEffectiveWeight_TTLAutoRecovery(t *testing.T) {
	ctx := context.Background()
	cb := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-test"

	// Push to Open state.
	for range 5 {
		cb.RecordError(ctx, channelID, modelID, false)
	}

	stats := cb.getStats(channelID, modelID)
	stats.Lock()
	if stats.State != StateOpen {
		t.Fatalf("expected Open state, got %s", stats.State)
	}
	// Set LastFailureAt to beyond the TTL so auto-recovery kicks in.
	policy := DefaultModelCircuitBreakerPolicy()
	stats.LastFailureAt = time.Now().Add(-(policy.FailureStatsTTL + time.Minute))
	stats.Unlock()

	// GetEffectiveWeight should auto-recover to Closed and return full weight.
	got := cb.GetEffectiveWeight(ctx, channelID, modelID, 1.0)
	if got != 1.0 {
		t.Fatalf("expected full weight after TTL auto-recovery, got %v", got)
	}

	stats.Lock()
	if stats.State != StateClosed {
		t.Fatalf("expected Closed state after TTL auto-recovery, got %s", stats.State)
	}
	if stats.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0 consecutive failures after auto-recovery, got %d", stats.ConsecutiveFailures)
	}
	stats.Unlock()
}

// TestRecordError_ProbeBackoffSequence verifies the exponential backoff sequence
// after consecutive probe failures: 10m, 20m, 40m and then capped at 40m.
func TestRecordError_ProbeBackoffSequence(t *testing.T) {
	ctx := context.Background()
	cb := NewModelCircuitBreaker()

	channelID := 1
	modelID := "gpt-test"

	for range 5 {
		cb.RecordError(ctx, channelID, modelID, false)
	}

	policy := DefaultModelCircuitBreakerPolicy()
	wantIntervals := []time.Duration{
		2 * policy.ProbeInterval, // first probe failure: 10m
		4 * policy.ProbeInterval, // 20m
		8 * policy.ProbeInterval, // 40m (cap)
		8 * policy.ProbeInterval, // stays at the cap
	}

	stats := cb.getStats(channelID, modelID)
	for i, want := range wantIntervals {
		// Open the probe window so the failure is treated as a real probe.
		stats.Lock()
		stats.NextProbeAt = time.Now().Add(-time.Second)
		stats.Unlock()

		before := time.Now()
		cb.RecordError(ctx, channelID, modelID, true)

		stats.Lock()
		got := stats.NextProbeAt.Sub(before)
		stats.Unlock()

		if got < want || got > want+2*time.Second {
			t.Fatalf("probe #%d: expected backoff around %v, got %v", i+1, want, got)
		}
	}
}
