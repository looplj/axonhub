package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/looplj/axonhub/axon/task"
)

// ActionType 定义 action 类型常量
type ActionType string

const (
	ActionTypeSendAgentMessage ActionType = "send_agent_message"
)

// Action 接口，所有 action 类型都需要实现
// 这个接口用于标识不同的 action 类型

type Action interface {
	GetType() ActionType
}

// SendAgentMessageAction 发送消息给 agent 的 action
type SendAgentMessageAction struct {
	Type    ActionType `json:"type"`
	Message string     `json:"message"`
}

func (a SendAgentMessageAction) GetType() ActionType {
	return ActionTypeSendAgentMessage
}

// Validate 验证 action 参数
func (a SendAgentMessageAction) Validate() error {
	if strings.TrimSpace(a.Message) == "" {
		return fmt.Errorf("send_agent_message requires message")
	}
	return nil
}

type AxonClawTaskHandler struct {
	logger    *slog.Logger
	workspace string
	runner    *Runner
}

func NewAxonClawTaskHandler(logger *slog.Logger, workspace string, runner *Runner) *AxonClawTaskHandler {
	return &AxonClawTaskHandler{
		logger:    logger,
		workspace: workspace,
		runner:    runner,
	}
}

func (h *AxonClawTaskHandler) HandleTask(ctx context.Context, t task.Task) error {
	typ, _ := getStringField(t.Action, "type")
	if typ == "" {
		return fmt.Errorf("task action.type is required")
	}

	switch ActionType(typ) {
	case ActionTypeSendAgentMessage:
		return h.handleSendAgentMessage(ctx, t)
	default:
		return fmt.Errorf("unsupported action.type %q", typ)
	}
}

func (h *AxonClawTaskHandler) handleSendAgentMessage(ctx context.Context, t task.Task) error {
	action, err := parseAction[SendAgentMessageAction](t.Action)
	if err != nil {
		return fmt.Errorf("failed to parse send_agent_message action: %w", err)
	}

	if err := action.Validate(); err != nil {
		return err
	}

	h.logger.Info("execute task send_agent_message to agent", "task_id", t.ID, "task_name", t.Name)
	return h.runner.ProcessScheduledMessage(ctx, action.Message)
}

// parseAction 将 map[string]any 解析为指定的 action 结构体
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
