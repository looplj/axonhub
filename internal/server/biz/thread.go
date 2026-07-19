package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/thread"
	"github.com/looplj/axonhub/internal/ent/trace"
	"github.com/looplj/axonhub/internal/ent/usagelog"
	"github.com/looplj/axonhub/internal/log"
)

type ThreadService struct {
	*AbstractService

	traceService *TraceService
}

func NewThreadService(ent *ent.Client, traceService *TraceService) *ThreadService {
	return &ThreadService{
		AbstractService: &AbstractService{
			db: ent,
		},
		traceService: traceService,
	}
}

// GetOrCreateThread retrieves an existing thread by thread_id and project_id,
// or creates a new one if it doesn't exist.
func (s *ThreadService) GetOrCreateThread(ctx context.Context, projectID int, threadID string) (*ent.Thread, error) {
	client := s.entFromContext(ctx)
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
		if ent.IsConstraintError(err) {
			return client.Thread.Query().
				Where(
					thread.ThreadIDEQ(threadID),
					thread.ProjectIDEQ(projectID),
				).
				Only(ctx)
		}

		return nil, fmt.Errorf("failed to create thread: %w", err)
	}

	log.Debug(ctx, "created new thread", log.String("thread_id", threadID), log.Int("project_id", projectID))

	return newThread, nil
}

// GetThreadByID retrieves a thread by its thread_id and project_id.
func (s *ThreadService) GetThreadByID(ctx context.Context, threadID string, projectID int) (*ent.Thread, error) {
	client := s.entFromContext(ctx)
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

// FirstUserQuery is the resolver for the firstUserQuery field.
func (s *ThreadService) FirstUserQuery(ctx context.Context, id int) (*string, error) {
	trace, err := s.traceService.GetThreadFirstTrace(ctx, id)
	if err != nil {
		return nil, err
	}

	if trace == nil {
		return nil, nil
	}

	return s.traceService.GetFirstUserQuery(ctx, trace.ID)
}

// FirstText is the resolver for the firstText field.
func (s *ThreadService) FirstText(ctx context.Context, id int) (*string, error) {
	trace, err := s.traceService.GetThreadFirstTrace(ctx, id)
	if err != nil {
		return nil, err
	}

	if trace == nil {
		return nil, nil
	}

	return s.traceService.GetFirstText(ctx, trace.ID)
}

// Archive sets the thread status to archived and cascades to all its traces.
func (s *ThreadService) Archive(ctx context.Context, id int) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		tx := ent.FromContext(txCtx)
		_, err := tx.Thread.UpdateOneID(id).
			Where(thread.StatusEQ(thread.StatusActive)).
			SetStatus(thread.StatusArchived).
			Save(txCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("cannot archive thread: current status is not active or thread not found")
			}
			return fmt.Errorf("failed to archive thread: %w", err)
		}

		_, err = tx.Trace.Update().Where(trace.ThreadIDEQ(id), trace.StatusEQ(trace.StatusActive)).SetStatus(trace.StatusArchived).Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to cascade archive traces: %w", err)
		}

		return nil
	})
}

// Unarchive sets the thread status to active and cascades to all its traces.
func (s *ThreadService) Unarchive(ctx context.Context, id int) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		tx := ent.FromContext(txCtx)
		_, err := tx.Thread.UpdateOneID(id).
			Where(thread.StatusEQ(thread.StatusArchived)).
			SetStatus(thread.StatusActive).
			Save(txCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("cannot unarchive thread: current status is not archived or thread not found")
			}
			return fmt.Errorf("failed to unarchive thread: %w", err)
		}

		_, err = tx.Trace.Update().Where(trace.ThreadIDEQ(id), trace.StatusEQ(trace.StatusArchived)).SetStatus(trace.StatusActive).Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to cascade unarchive traces: %w", err)
		}

		return nil
	})
}

// Retain sets the thread status to retained and cascades to all its traces.
func (s *ThreadService) Retain(ctx context.Context, id int) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		tx := ent.FromContext(txCtx)
		_, err := tx.Thread.UpdateOneID(id).
			Where(thread.StatusEQ(thread.StatusActive)).
			SetStatus(thread.StatusRetained).
			Save(txCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("cannot retain thread: current status is not active or thread not found")
			}
			return fmt.Errorf("failed to retain thread: %w", err)
		}

		_, err = tx.Trace.Update().Where(trace.ThreadIDEQ(id), trace.StatusEQ(trace.StatusActive)).SetStatus(trace.StatusRetained).Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to cascade retain traces: %w", err)
		}

		return nil
	})
}

// Unretain sets the thread status to active and cascades to all its traces.
func (s *ThreadService) Unretain(ctx context.Context, id int) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		tx := ent.FromContext(txCtx)
		_, err := tx.Thread.UpdateOneID(id).
			Where(thread.StatusEQ(thread.StatusRetained)).
			SetStatus(thread.StatusActive).
			Save(txCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("cannot unretain thread: current status is not retained or thread not found")
			}
			return fmt.Errorf("failed to unretain thread: %w", err)
		}

		// Unretain all traces that were retained (including individually retained ones).
		// This is a deliberate trade-off: users can re-retain specific traces afterward.
		_, err = tx.Trace.Update().Where(trace.ThreadIDEQ(id), trace.StatusEQ(trace.StatusRetained)).SetStatus(trace.StatusActive).Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to cascade unretain traces: %w", err)
		}

		return nil
	})
}

func (s *ThreadService) UsageMetadata(ctx context.Context, threadID int) (*UsageMetadata, error) {
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	q := client.UsageLog.Query().
		Where(usagelog.HasRequestWith(
			request.HasTraceWith(trace.ThreadIDEQ(threadID)),
			request.StatusEQ(request.StatusCompleted),
		))

	return aggregateUsageMetadata(ctx, q)
}
