package agent

import (
	"context"
	"time"
)

// ContextManager manages message history and optional context preparation policies.
// Implementations can be composed via decorators.
type ContextManager interface {
	// AddMessages appends messages into history.
	AddMessages(ctx context.Context, msgs ...Message)

	// SetMessages replaces current history with the provided messages.
	SetMessages(ctx context.Context, msgs []Message)

	// Messages returns a safe copy of current history.
	Messages(ctx context.Context) []Message

	// ClearMessages clears current history.
	ClearMessages(ctx context.Context)

	// BuildMessages returns model input messages.
	// Implementations may inject summaries and compact/write history internally.
	BuildMessages(ctx context.Context) []Message

	// Snapshot returns a copy of manager runtime state for persistence/inspection.
	Snapshot() ContextManagerState
}

// ContextManagerState is the persisted/observable state of context policies.
type ContextManagerState struct {
	Summary         string    `json:"summary,omitempty"`
	CompactionCount int64     `json:"compaction_count"`
	RoundIndex      int64     `json:"round_index"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func emptyContextState() ContextManagerState {
	return ContextManagerState{}
}

func copyContextState(state ContextManagerState) ContextManagerState {
	out := ContextManagerState{
		Summary:         state.Summary,
		CompactionCount: state.CompactionCount,
		RoundIndex:      state.RoundIndex,
		UpdatedAt:       state.UpdatedAt,
	}
	return out
}
