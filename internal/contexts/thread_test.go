package contexts

import (
	"context"
	"testing"

	"github.com/looplj/axonhub/internal/ent"
)

func TestWithThread(t *testing.T) {
	ctx := context.Background()
	thread := &ent.Thread{
		ID:       1,
		ThreadID: "thread-123",
	}

	// Test storing thread entity
	newCtx := WithThread(ctx, thread)
	if newCtx == ctx {
		t.Error("WithThread should return a new context")
	}

	// Test retrieving thread entity
	retrievedThread, ok := GetThread(newCtx)
	if !ok {
		t.Error("GetThread should return true for existing thread")
	}

	if retrievedThread == nil {
		t.Error("GetThread should return non-nil thread")
	}

	if retrievedThread.ID != thread.ID {
		t.Errorf("expected ID %d, got %d", thread.ID, retrievedThread.ID)
	}

	if retrievedThread.ThreadID != thread.ThreadID {
		t.Errorf("expected ThreadID %s, got %s", thread.ThreadID, retrievedThread.ThreadID)
	}
}

func TestGetThread(t *testing.T) {
	ctx := context.Background()

	// Test retrieving thread from empty context
	thread, ok := GetThread(ctx)
	if ok {
		t.Error("GetThread should return false for empty context")
	}

	if thread != nil {
		t.Error("GetThread should return nil for empty context")
	}

	// Test retrieving thread from context with other values
	ctxWithOtherValue := context.WithValue(ctx, "other_key", "other_value")

	thread, ok = GetThread(ctxWithOtherValue)
	if ok {
		t.Error("GetThread should return false for context without thread")
	}

	if thread != nil {
		t.Error("GetThread should return nil for context without thread")
	}
}

func TestThreadWithMultipleValues(t *testing.T) {
	ctx := context.Background()

	// Test storing thread along with other values
	ctx = WithAPIKey(ctx, &ent.APIKey{ID: 1, Key: "test-key"})
	ctx = WithUser(ctx, &ent.User{ID: 123, Email: "test@example.com"})
	ctx = WithThread(ctx, &ent.Thread{ID: 1, ThreadID: "thread-123"})
	ctx = WithProjectID(ctx, 456)

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

	projectID, ok := GetProjectID(ctx)
	if !ok || projectID != 456 {
		t.Error("Project ID should be stored and retrievable")
	}
}

func TestThreadOverwrite(t *testing.T) {
	ctx := context.Background()

	// Test overwriting existing thread
	ctx = WithThread(ctx, &ent.Thread{ID: 1, ThreadID: "thread-1"})
	ctx = WithThread(ctx, &ent.Thread{ID: 2, ThreadID: "thread-2"})

	thread, ok := GetThread(ctx)
	if !ok {
		t.Error("Thread should exist")
	}

	if thread.ID != 2 || thread.ThreadID != "thread-2" {
		t.Error("Thread should be the overwritten value")
	}
}
