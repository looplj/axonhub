package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/thread"
)

type ThreadService struct{}

func NewThreadService() *ThreadService {
	return &ThreadService{}
}

// GetOrCreateThread retrieves an existing thread by thread_id and project_id,
// or creates a new one if it doesn't exist.
func (s *ThreadService) GetOrCreateThread(ctx context.Context, projectID int, threadID string) (*ent.Thread, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	// Try to find existing thread
	existingThread, err := client.Thread.Query().
		Where(
			thread.ThreadIDEQ(threadID),
			thread.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err == nil {
		// Thread found
		return existingThread, nil
	}

	// If error is not "not found", return the error
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query thread: %w", err)
	}

	// Thread not found, create new one
	newThread, err := client.Thread.Create().
		SetThreadID(threadID).
		SetProjectID(projectID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	return newThread, nil
}

// GetThreadByID retrieves a thread by its thread_id and project_id.
func (s *ThreadService) GetThreadByID(ctx context.Context, threadID string, projectID int) (*ent.Thread, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	thread, err := client.Thread.Query().
		Where(
			thread.ThreadIDEQ(threadID),
			thread.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	return thread, nil
}
