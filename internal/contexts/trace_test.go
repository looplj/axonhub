package contexts

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/internal/ent"
)

func TestWithTrace(t *testing.T) {
	ctx := context.Background()
	trace := &ent.Trace{
		ID:      1,
		TraceID: "trace-123",
	}

	// Test storing trace entity
	newCtx := WithTrace(ctx, trace)
	if newCtx == ctx {
		t.Error("WithTrace should return a new context")
	}

	// Test retrieving trace entity
	retrievedTrace, ok := GetTrace(newCtx)
	if !ok {
		t.Error("GetTrace should return true for existing trace")
	}

	if retrievedTrace == nil {
		t.Error("GetTrace should return non-nil trace")
	}

	if retrievedTrace.ID != trace.ID {
		t.Errorf("expected ID %d, got %d", trace.ID, retrievedTrace.ID)
	}

	if retrievedTrace.TraceID != trace.TraceID {
		t.Errorf("expected TraceID %s, got %s", trace.TraceID, retrievedTrace.TraceID)
	}
}

func TestGetTrace(t *testing.T) {
	ctx := context.Background()

	// Test retrieving trace from empty context
	trace, ok := GetTrace(ctx)
	if ok {
		t.Error("GetTrace should return false for empty context")
	}

	if trace != nil {
		t.Error("GetTrace should return nil for empty context")
	}

	// Test retrieving trace from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	trace, ok = GetTrace(ctxWithOtherValue)
	if ok {
		t.Error("GetTrace should return false for context without trace")
	}

	if trace != nil {
		t.Error("GetTrace should return nil for context without trace")
	}
}

func TestTraceWithMultipleValues(t *testing.T) {
	ctx := context.Background()

	// Test storing trace along with other values
	ctx = WithAPIKey(ctx, &ent.APIKey{ID: 1, Key: "test-key"})
	ctx = WithUser(ctx, &ent.User{ID: 123, Email: "test@example.com"})
	ctx = WithThread(ctx, &ent.Thread{ID: 1, ThreadID: "thread-123"})
	ctx = WithTrace(ctx, &ent.Trace{ID: 2, TraceID: "trace-456"})
	ctx = WithProjectID(ctx, 789)

	// Test retrieving all values
	apiKey, ok := GetAPIKey(ctx)
	if !ok || apiKey.ID != 1 {
		t.Error("API key should be stored and retrievable")
	}

	user, ok := GetUser(ctx)
	if !ok || user.ID != 123 {
		t.Error("User should be stored and retrievable")
	}

	thread, ok := GetThread(ctx)
	if !ok || thread.ID != 1 {
		t.Error("Thread should be stored and retrievable")
	}

	trace, ok := GetTrace(ctx)
	if !ok || trace.ID != 2 {
		t.Error("Trace should be stored and retrievable")
	}

	projectID, ok := GetProjectID(ctx)
	if !ok || projectID != 789 {
		t.Error("Project ID should be stored and retrievable")
	}
}

func TestTraceOverwrite(t *testing.T) {
	ctx := context.Background()

	// Test overwriting existing trace
	ctx = WithTrace(ctx, &ent.Trace{ID: 1, TraceID: "trace-1"})
	ctx = WithTrace(ctx, &ent.Trace{ID: 2, TraceID: "trace-2"})

	trace, ok := GetTrace(ctx)
	if !ok {
		t.Error("Trace should exist")
	}

	if trace.ID != 2 || trace.TraceID != "trace-2" {
		t.Error("Trace should be the overwritten value")
	}
}

func TestThreadAndTraceRelationship(t *testing.T) {
	ctx := context.Background()

	// Test storing thread and trace together
	thread := &ent.Thread{ID: 1, ThreadID: "thread-123"}
	trace := &ent.Trace{ID: 2, TraceID: "trace-456", ThreadID: thread.ID}

	ctx = WithThread(ctx, thread)
	ctx = WithTrace(ctx, trace)

	// Verify both are stored correctly
	retrievedThread, ok := GetThread(ctx)
	if !ok || retrievedThread.ID != thread.ID {
		t.Error("Thread should be stored and retrievable")
	}

	retrievedTrace, ok := GetTrace(ctx)
	if !ok || retrievedTrace.ID != trace.ID {
		t.Error("Trace should be stored and retrievable")
	}

	if retrievedTrace.ThreadID != trace.ThreadID {
		t.Error("Trace should maintain thread relationship")
	}
}
