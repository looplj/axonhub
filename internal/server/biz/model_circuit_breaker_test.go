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
		cb.RecordError(ctx, channelID, modelID)
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
