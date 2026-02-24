package runner

import (
	"context"
	"encoding/json"

	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/permission"
)

type permissionMiddleware struct {
	evaluator *permission.Evaluator
}

func NewPermissionMiddleware(evaluator *permission.Evaluator) agent.Middleware {
	return &permissionMiddleware{evaluator: evaluator}
}

func (m *permissionMiddleware) BeforeTool(ctx context.Context, req agent.ToolRequest) error {
	permReq := permission.ToolRequest{
		ThreadID:   req.ThreadID,
		Workspace:  req.Workspace,
		ToolCallID: req.ToolCallID,
		ToolName:   req.ToolName,
		ToolInput:  json.RawMessage(req.ToolInput),
		StartedAt:  req.StartedAt,
	}
	return m.evaluator.Evaluate(ctx, permReq)
}

func (m *permissionMiddleware) AfterTool(ctx context.Context, req agent.ToolRequest, toolErr error) error {
	permReq := permission.ToolRequest{
		ThreadID:   req.ThreadID,
		Workspace:  req.Workspace,
		ToolCallID: req.ToolCallID,
		ToolName:   req.ToolName,
		ToolInput:  json.RawMessage(req.ToolInput),
		StartedAt:  req.StartedAt,
	}
	m.evaluator.LogToolResult(permReq, toolErr)
	return nil
}
