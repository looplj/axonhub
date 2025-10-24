package biz

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/trace"
)

type TraceService struct{}

func NewTraceService() *TraceService {
	return &TraceService{}
}

// GetOrCreateTrace retrieves an existing trace by trace_id and project_id,
// or creates a new one if it doesn't exist.
func (s *TraceService) GetOrCreateTrace(ctx context.Context, projectID int, traceID string, threadID *int) (*ent.Trace, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	// Try to find existing trace
	trace, err := client.Trace.Query().
		Where(
			trace.TraceIDEQ(traceID),
			trace.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err == nil {
		// Trace found
		return trace, nil
	}

	// If error is not "not found", return the error
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query trace: %w", err)
	}

	// Trace not found, create new one
	createTrace := client.Trace.Create().
		SetTraceID(traceID).
		SetProjectID(projectID).
		SetNillableThreadID(threadID)

	return createTrace.Save(ctx)
}

// GetTraceByID retrieves a trace by its trace_id and project_id.
func (s *TraceService) GetTraceByID(ctx context.Context, traceID string, projectID int) (*ent.Trace, error) {
	client := ent.FromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	trace, err := client.Trace.Query().
		Where(
			trace.TraceIDEQ(traceID),
			trace.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get trace: %w", err)
	}

	return trace, nil
}
