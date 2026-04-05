package claw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/looplj/axonhub/axon/task"
)

const (
	PromptModeMain     = "main"
	PromptModeIsolated = "isolated"
)

// PromptAction sends a prompt to the agent when the task is triggered.
type PromptAction struct {
	Message string `json:"message"`
	Mode    string `json:"mode,omitempty"`
	Model   string `json:"model,omitempty"`
}

func (a PromptAction) Validate() error {
	if strings.TrimSpace(a.Message) == "" {
		return fmt.Errorf("prompt requires message")
	}

	mode := a.NormalizedMode()
	if mode != PromptModeMain && mode != PromptModeIsolated {
		return fmt.Errorf("prompt mode must be %q or %q", PromptModeMain, PromptModeIsolated)
	}

	if strings.TrimSpace(a.Model) != "" && mode == PromptModeMain {
		return fmt.Errorf("model is only supported in %q mode", PromptModeIsolated)
	}

	return nil
}

func (a PromptAction) NormalizedMode() string {
	mode := strings.ToLower(strings.TrimSpace(a.Mode))
	if mode == "" {
		return PromptModeIsolated
	}

	return mode
}

func (h *TaskHandler) handlePrompt(ctx context.Context, t task.Task) error {
	action, err := parseAction[PromptAction](t.Action)
	if err != nil {
		return fmt.Errorf("failed to parse prompt action: %w", err)
	}

	if err := action.Validate(); err != nil {
		return err
	}

	mode := action.NormalizedMode()
	h.logger.Info("execute task prompt to agent", "task_id", t.ID, "task_name", t.Name, "mode", mode)

	if mode == PromptModeMain {
		h.runner.FollowUP(ctx, action.Message)
		return nil
	}

	_, err = h.runner.ProcessIsolated(ctx, action.Message, h.runner.Agent.Config().SystemPrompts, action.Model)
	if err != nil {
		return fmt.Errorf("process isolated prompt task: %w", err)
	}

	return nil
}

func parseAction[T any](m map[string]any) (T, error) {
	var result T

	data, err := json.Marshal(m)
	if err != nil {
		return result, fmt.Errorf("failed to marshal action: %w", err)
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal action: %w", err)
	}

	return result, nil
}

func getStringField(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}

	s, ok := v.(string)
	if !ok {
		return "", false
	}

	return s, true
}

func actionString(action map[string]any, key string) string {
	v, ok := getStringField(action, key)
	if !ok {
		return ""
	}

	return strings.TrimSpace(v)
}
