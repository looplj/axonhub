package claw

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/looplj/axonhub/axon/task"
)

type TaskType string

const (
	TaskTypePrompt      TaskType = "prompt"
	TaskTypeHeartbeat   TaskType = "heartbeat"
	TaskTypeSelfReflect TaskType = "self-reflect"
	TaskTypeSelfEvolve  TaskType = "self-evolve"
)

type Handler func(ctx context.Context, t task.Task) error

type TaskHandler struct {
	logger    *slog.Logger
	workspace string
	runner    *Runner
	handlers  map[string]Handler
}

func NewTaskHandler(logger *slog.Logger, workspace string, runner *Runner) *TaskHandler {
	h := &TaskHandler{
		logger:    logger,
		workspace: workspace,
		runner:    runner,
		handlers:  make(map[string]Handler),
	}
	h.Register(string(TaskTypePrompt), h.handlePrompt)
	h.Register(string(TaskTypeHeartbeat), h.handleHeartbeat)
	h.Register(string(TaskTypeSelfReflect), h.handleSelfReflect)
	h.Register(string(TaskTypeSelfEvolve), h.handleSelfEvolve)

	return h
}

func (h *TaskHandler) Register(taskType string, handler Handler) {
	if handler == nil {
		return
	}

	h.handlers[strings.TrimSpace(taskType)] = handler
}

func (h *TaskHandler) HandleTask(ctx context.Context, t task.Task) error {
	taskType := resolveTaskType(t)
	if taskType == "" {
		return fmt.Errorf("task type is required")
	}

	handler, ok := h.handlers[taskType]
	if !ok {
		return fmt.Errorf("unsupported task type %q", taskType)
	}

	return handler(ctx, t)
}

func resolveTaskType(t task.Task) string {
	return strings.TrimSpace(t.Type)
}
