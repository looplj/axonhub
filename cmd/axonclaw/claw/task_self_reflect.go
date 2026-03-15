package claw

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/axon/task"

	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

func (h *TaskHandler) handleSelfReflect(ctx context.Context, t task.Task) error {
	h.logger.Info("execute self reflection task", "task_id", t.ID, "task_name", t.Name)

	_, err := h.runner.ProcessIsolated(ctx, "Run the self-reflection task now.", prompts.BuildSelfReflectTaskSystemPrompts())
	if err != nil {
		return fmt.Errorf("process self reflection task: %w", err)
	}

	return nil
}
