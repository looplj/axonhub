package claw

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/axon/task"
)

func (h *TaskHandler) handleSelfEvolve(ctx context.Context, t task.Task) error {
	h.logger.Info("execute self evolution task", "task_id", t.ID, "task_name", t.Name)

	_, err := h.runner.ProcessIsolated(ctx, "Run the self-evolution task now.", h.runner.Agent.Config().SystemPrompts)
	if err != nil {
		return fmt.Errorf("process self evolution task: %w", err)
	}

	return nil
}
