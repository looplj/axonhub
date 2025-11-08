package gc

import (
	"context"
	"testing"
)

func TestWorker_getBatchSize(t *testing.T) {
	worker := &Worker{
		Ent:    nil,
		Config: Config{CRON: "0 0 * * *"},
	}

	// Test default batch size
	batchSize := worker.getBatchSize()
	if batchSize != defaultBatchSize {
		t.Errorf("Expected batch size %d, got %d", defaultBatchSize, batchSize)
	}

	// Test with overridden batch size
	originalBatchSize := defaultBatchSize
	defaultBatchSize = 20

	defer func() { defaultBatchSize = originalBatchSize }()

	batchSize = worker.getBatchSize()
	if batchSize != 20 {
		t.Errorf("Expected batch size 20, got %d", batchSize)
	}
}

func TestWorker_deleteInBatches(t *testing.T) {
	// Test that the deleteInBatches method works correctly
	// This test verifies the loop logic without needing a real database
	worker := &Worker{
		Ent:    nil,
		Config: Config{CRON: "0 0 * * *"},
	}

	// Simulate batch deletion - delete 3 times, with decreasing counts
	callCount := 0
	deleteFunc := func() (int, error) {
		callCount++
		if callCount == 1 {
			return 30, nil
		} else if callCount == 2 {
			return 15, nil
		} else {
			return 0, nil
		}
	}

	deleted, err := worker.deleteInBatches(context.Background(), deleteFunc)
	if err != nil {
		t.Fatalf("deleteInBatches failed: %v", err)
	}

	// Verify total deleted
	if deleted != 45 {
		t.Errorf("Expected to delete 45 records total, got %d", deleted)
	}

	// Verify it stopped after third call (when 0 was returned)
	if callCount != 3 {
		t.Errorf("Expected 3 delete calls, got %d", callCount)
	}
}

func TestWorker_cleanupWithZeroDays(t *testing.T) {
	worker := &Worker{
		Ent:    nil,
		Config: Config{CRON: "0 0 * * *"},
	}

	ctx := context.Background()

	// Test with 0 days - should not error
	err := worker.cleanupRequests(ctx, 0)
	if err != nil {
		t.Fatalf("cleanupRequests with 0 days failed: %v", err)
	}

	// Test with negative days - should not error
	err = worker.cleanupUsageLogs(ctx, -1)
	if err != nil {
		t.Fatalf("cleanupUsageLogs with negative days failed: %v", err)
	}
}
