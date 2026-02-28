package approval

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/permission/grant"
)

type remoteApprover struct {
	logger *slog.Logger
	client graphql.Client

	pollInterval time.Duration
}

func NewRemoteApprover(logger *slog.Logger, client graphql.Client, pollInterval time.Duration) Service {
	if logger == nil {
		logger = slog.Default()
	}
	if pollInterval <= 0 {
		pollInterval = 1200 * time.Millisecond
	}
	return &remoteApprover{
		logger:       logger,
		client:       client,
		pollInterval: pollInterval,
	}
}

func (a *remoteApprover) Subscribe(ctx context.Context) <-chan Request {
	ch := make(chan Request)
	close(ch)
	return ch
}

func (a *remoteApprover) Active() (Request, bool) {
	return Request{}, false
}

func (a *remoteApprover) Grant(req Request, scope grant.Scope) error {
	return errors.New("remote approver: Grant not supported (operator must call resolveApproval via admin GraphQL)")
}

func (a *remoteApprover) Deny(req Request) error {
	return errors.New("remote approver: Deny not supported (operator must call resolveApproval via admin GraphQL)")
}

func (a *remoteApprover) Request(ctx context.Context, req Request) (Response, error) {
	if req.ID == "" {
		req.ID = uuid.NewString()
	}

	exp := time.Now().Add(2 * time.Minute).UTC()

	payload := map[string]any{
		"type":      "approval_request",
		"id":        req.ID,
		"thread_id": req.ThreadID,
		"tool_call_id": func() string {
			if req.ToolCallID == "" {
				return ""
			}
			return req.ToolCallID
		}(),
		"tool_name":  req.ToolName,
		"risk_level": req.RiskLevel,
		"summary":    req.Summary,
		"reason":     req.Reason,
		"expires_at": exp.Format(time.RFC3339),
		"resources": func() any {
			if len(req.Resources) == 0 {
				return []any{}
			}
			var v any
			if err := json.Unmarshal(req.Resources, &v); err == nil {
				return v
			}
			return []any{}
		}(),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal approval_request: %w", err)
	}

	text := req.Summary
	if text == "" {
		text = fmt.Sprintf("approval required: %s", req.ToolName)
	}

	kind := api.AgentMessageKindApprovalRequest
	correlationID := req.ID
	if _, err := api.ReplyMessage(ctx, a.client, &api.ReplyMessageInput{
		Text:          text,
		Content:       (*json.RawMessage)(&raw),
		Kind:          &kind,
		CorrelationID: &correlationID,
	}); err != nil {
		return Response{}, fmt.Errorf("push approval_request: %w", err)
	}

	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-ticker.C:
			limit := 10
			kindIn := []api.AgentMessageKind{api.AgentMessageKindApprovalResult}
			resp, err := api.PullAgentMessages(ctx, a.client, &api.PullAgentMessagesInput{
				Limit:         &limit,
				KindIn:        kindIn,
				CorrelationID: &correlationID,
			})
			if err != nil {
				a.logger.Warn("approval: pullAgentMessages failed", "error", err, "request_id", req.ID)
				continue
			}

			var found *api.PullAgentMessagesPullAgentMessagesAgentMessage
			for _, m := range resp.PullAgentMessages {
				if m.CorrelationID == correlationID && m.Kind == api.AgentMessageKindApprovalResult {
					found = m
					break
				}
			}
			if found == nil {
				continue
			}

			var result struct {
				RequestID string `json:"request_id"`
				Granted   bool   `json:"granted"`
				Scope     string `json:"scope"`
				Reason    string `json:"reason"`
			}
			_ = json.Unmarshal(found.Content, &result)

			respScope := grant.ScopeOnce
			switch result.Scope {
			case "thread":
				respScope = grant.ScopeThread
			case "workspace":
				respScope = grant.ScopeWorkspace
			case "global":
				respScope = grant.ScopeGlobal
			default:
				respScope = grant.ScopeOnce
			}

			if _, err := api.AckAgentMessages(ctx, a.client, &api.AckAgentMessagesInput{
				MessageIDs: []string{found.Id},
			}); err != nil {
				a.logger.Warn("approval: ackAgentMessages failed", "error", err, "message_id", found.Id, "request_id", req.ID)
			}

			return Response{
				Granted: result.Granted,
				Scope:   respScope,
				Reason:  result.Reason,
			}, nil
		}
	}
}
