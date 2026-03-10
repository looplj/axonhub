package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/agent"
	"github.com/looplj/axonhub/internal/ent/agentinstance"
	"github.com/looplj/axonhub/internal/ent/agentmessage"
	"github.com/looplj/axonhub/internal/ent/agentskill"
	"github.com/looplj/axonhub/internal/ent/agentthread"
	"github.com/looplj/axonhub/internal/ent/agenttool"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/prompt"
	"github.com/looplj/axonhub/internal/ent/thread"
	"github.com/looplj/axonhub/internal/objects"
)

type AgentBootstrapServiceParams struct {
	fx.In

	Ent *ent.Client
}

// AgentBootstrapService provides APIs for the runtime agent endpoint (/agent/v1/graphql).
// This service enforces agent API key ownership checks at the application layer and
// uses system bypass for DB access to avoid coupling to Ent privacy rules.
type AgentBootstrapService struct {
	*AbstractService
}

func NewAgentBootstrapService(params AgentBootstrapServiceParams) *AgentBootstrapService {
	return &AgentBootstrapService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
	}
}

type AgentToolDefinition struct {
	Name        string
	Description string
	Parameters  objects.JSONRawMessage
	Config      *objects.JSONRawMessage
}

type AgentSkillDefinition struct {
	Name       string
	Content    *string
	Entrypoint *string
	Args       *string
}

type AgentBootstrap struct {
	AgentID         int
	AgentName       string
	Model           *string
	ReasoningEffort string
	SystemPrompt    string

	Tools        []AgentToolDefinition
	Skills       []AgentSkillDefinition
	BuiltinTools []objects.AgentBuiltinTool
	SkillsPolicy objects.AgentSkillsPolicy

	MemoryPolicy *objects.JSONRawMessage
}

type AgentMessageView struct {
	ID                int
	AgentID           int
	AgentInstanceID   int
	Direction         agentmessage.Direction
	SenderType        agentmessage.SenderType
	SenderID          *int
	Type              agentmessage.Type
	CorrelationID     string
	Content           objects.JSONRawMessage
	Text              string
	Sequence          int64
	Status            agentmessage.Status
	CreatedAt         time.Time
	ExternalMessageID *string
	ReplyToMessageID  *int
}

type RegisterAgentInstanceInput struct {
	AgentID     int
	Name        *string
	Platform    *string
	Description *string
	ThreadID    *string
}

type SendPeerMessageInput struct {
	SenderAgentID         int
	TargetAgentID         int
	TargetAgentInstanceID *int
	Text                  string
	Content               *objects.JSONRawMessage
	CorrelationID         *string
}

type PeerAgentView struct {
	AgentID         int
	AgentInstanceID int
	Name            string
	Description     string
	Status          string
}

// SendAgentMessageInput is used by the admin GraphQL API (Web UI) to send a user message to an agent.
type SendAgentMessageInput struct {
	AgentID         int
	AgentInstanceID *int
	Text            string
}

type PushAgentMessageInput struct {
	Text             string
	Content          *objects.JSONRawMessage
	Type             *agentmessage.Type
	CorrelationID    *string
	ReplyToMessageID *int
}

type PullAgentMessagesInput struct {
	AfterSequence *int64
	Limit         int
	TypeIn        []agentmessage.Type
	CorrelationID *string
}

type AckAgentMessagesInput struct {
	AgentID         int
	AgentInstanceID *int
	MessageIDs      []int
}

type ResolveApprovalCommandInput struct {
	AgentID         int
	AgentInstanceID *int
	RequestID       string
	Granted         bool
	Scope           string // once|thread|workspace|global
	Reason          *string
	ResourceIndices []int
}

type AgentApprovalRequestView struct {
	ID              int
	AgentID         int
	AgentInstanceID int
	CorrelationID   string
	Content         objects.JSONRawMessage
	Sequence        int64
	CreatedAt       time.Time
}

func (s *AgentBootstrapService) GetAgentInstanceFromAPIKey(ctx context.Context) (*ent.AgentInstance, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	return authz.RunWithSystemBypass(ctx, "agent-runtime-get-agent-instance", func(bypassCtx context.Context) (*ent.AgentInstance, error) {
		client := s.entFromContext(bypassCtx)

		inst, err := client.AgentInstance.Query().
			Where(agentinstance.APIKeyIDEQ(apiKey.ID)).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent instance: %w", err)
		}

		return inst, nil
	})
}

func (s *AgentBootstrapService) AgentBootstrap(ctx context.Context, inst *ent.AgentInstance) (*AgentBootstrap, error) {
	return authz.RunWithSystemBypass(ctx, "agent-runtime-bootstrap", func(bypassCtx context.Context) (*AgentBootstrap, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(inst.AgentID),
				agent.ProjectIDEQ(inst.ProjectID),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		p, err := client.Prompt.Query().
			Where(
				prompt.IDEQ(a.PromptID),
				prompt.ProjectIDEQ(inst.ProjectID),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent system prompt: %w", err)
		}

		builtinTools := a.AgentBuiltinTools
		if builtinTools == nil {
			builtinTools = []objects.AgentBuiltinTool{}
		}

		skillsPolicy := a.SkillsPolicy
		if skillsPolicy.Add == "" {
			skillsPolicy.Add = "open"
		}

		toolBindings, err := client.AgentTool.Query().
			Where(
				agenttool.AgentIDEQ(a.ID),
				agenttool.ProjectIDEQ(inst.ProjectID),
			).
			Order(agenttool.ByEnabled(sql.OrderDesc()), agenttool.ByOrder()).
			WithTool().
			All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent tool bindings: %w", err)
		}

		tools := make([]AgentToolDefinition, 0, len(toolBindings))
		for _, b := range toolBindings {
			if b.Edges.Tool == nil {
				continue
			}
			t := b.Edges.Tool
			def := AgentToolDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Schema,
			}
			if len(b.Config) > 0 && string(b.Config) != "{}" {
				def.Config = &b.Config
			}
			tools = append(tools, def)
		}

		skillBindings, err := client.AgentSkill.Query().
			Where(
				agentskill.AgentIDEQ(a.ID),
				agentskill.ProjectIDEQ(inst.ProjectID),
			).
			Order(agentskill.ByEnabled(sql.OrderDesc()), agentskill.ByOrder()).
			WithSkill().
			All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent skill bindings: %w", err)
		}

		skills := make([]AgentSkillDefinition, 0, len(skillBindings))
		for _, b := range skillBindings {
			if b.Edges.Skill == nil {
				continue
			}
			sk := b.Edges.Skill

			var entrypoint *string
			if sk.Entrypoint != "" {
				entrypoint = &sk.Entrypoint
			}

			var args *string
			if b.Args != "" {
				args = &b.Args
			}

			skills = append(skills, AgentSkillDefinition{
				Name:       sk.Name,
				Content:    sk.Content,
				Entrypoint: entrypoint,
				Args:       args,
			})
		}

		var model *string
		if a.Model != "" {
			model = &a.Model
		}

		return &AgentBootstrap{
			AgentID:         a.ID,
			AgentName:       a.Name,
			Model:           model,
			ReasoningEffort: string(a.ReasoningEffort),
			SystemPrompt:    p.Content,
			Tools:           tools,
			Skills:          skills,
			BuiltinTools:    builtinTools,
			SkillsPolicy:    skillsPolicy,
			MemoryPolicy:    nil,
		}, nil
	})
}

func (s *AgentBootstrapService) SendAgentMessageAsUser(ctx context.Context, userID int, input SendAgentMessageInput) (*AgentMessageView, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	return authz.RunWithSystemBypass(ctx, "agent-admin-send-message", func(bypassCtx context.Context) (*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		var inst *ent.AgentInstance
		if input.AgentInstanceID != nil {
			inst, err = client.AgentInstance.Query().
				Where(
					agentinstance.IDEQ(*input.AgentInstanceID),
					agentinstance.AgentIDEQ(a.ID),
					agentinstance.DeletedAtEQ(0),
				).
				Only(bypassCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to load agent instance: %w", err)
			}
		} else {
			inst, err = client.AgentInstance.Query().
				Where(
					agentinstance.AgentIDEQ(a.ID),
					agentinstance.DeletedAtEQ(0),
				).
				First(bypassCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to load agent instance: %w", err)
			}
		}

		raw, err := marshalMessageContent("text", input.Text, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message content: %w", err)
		}

		var msg *ent.AgentMessage
		for attempt := 0; attempt < 3; attempt++ {
			nextSeq, err := s.nextSequence(bypassCtx, a.ID)
			if err != nil {
				return nil, err
			}

			created, err := client.AgentMessage.Create().
				SetProjectID(projectID).
				SetAgentID(a.ID).
				SetAgentInstanceID(inst.ID).
				SetDirection(agentmessage.DirectionToAgent).
				SetSenderType(agentmessage.SenderTypeUser).
				SetSenderID(userID).
				SetType(agentmessage.TypeChat).
				SetCorrelationID("").
				SetContent(raw).
				SetStatus(agentmessage.StatusPending).
				SetSequence(nextSeq).
				Save(bypassCtx)
			if err == nil {
				msg = created
				break
			}

			if ent.IsConstraintError(err) && attempt < 2 {
				continue
			}

			return nil, fmt.Errorf("failed to create message: %w", err)
		}
		if msg == nil {
			return nil, fmt.Errorf("failed to create message: no message created")
		}

		return &AgentMessageView{
			ID:                msg.ID,
			AgentID:           a.ID,
			AgentInstanceID:   inst.ID,
			Direction:         msg.Direction,
			SenderType:        msg.SenderType,
			SenderID:          new(userID),
			Type:              msg.Type,
			CorrelationID:     msg.CorrelationID,
			Content:           msg.Content,
			Text:              input.Text,
			Sequence:          msg.Sequence,
			Status:            msg.Status,
			CreatedAt:         msg.CreatedAt,
			ExternalMessageID: msg.ExternalMessageID,
			ReplyToMessageID:  msg.ReplyToMessageID,
		}, nil
	})
}

func (s *AgentBootstrapService) PullAgentMessagesToUserAsAdmin(ctx context.Context, agentID int, agentInstanceID *int, afterSequence *int64, limit int) ([]*AgentMessageView, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	return authz.RunWithSystemBypass(ctx, "agent-admin-pull-messages-to-user", func(bypassCtx context.Context) ([]*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.StatusEQ(agentmessage.StatusPending),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			Where(func(s *sql.Selector) {})

		if agentInstanceID != nil {
			q = q.Where(agentmessage.AgentInstanceIDEQ(*agentInstanceID))
		}

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			text := extractTextFromMessageContent(m.Content)

			out = append(out, &AgentMessageView{
				ID:                m.ID,
				AgentID:           a.ID,
				AgentInstanceID:   m.AgentInstanceID,
				Direction:         m.Direction,
				SenderType:        m.SenderType,
				SenderID:          m.SenderID,
				Type:              m.Type,
				CorrelationID:     m.CorrelationID,
				Content:           m.Content,
				Text:              text,
				Sequence:          m.Sequence,
				Status:            m.Status,
				CreatedAt:         m.CreatedAt,
				ExternalMessageID: m.ExternalMessageID,
				ReplyToMessageID:  m.ReplyToMessageID,
			})
		}

		return out, nil
	})
}

func (s *AgentBootstrapService) PullAgentApprovalRequestsAsAdmin(ctx context.Context, agentID int, agentInstanceID *int, afterSequence *int64, limit int) ([]*AgentApprovalRequestView, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	if limit <= 0 {
		limit = 50
	}

	if limit > 200 {
		limit = 200
	}

	return authz.RunWithSystemBypass(ctx, "agent-admin-pull-approval-requests", func(bypassCtx context.Context) ([]*AgentApprovalRequestView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.TypeEQ(agentmessage.TypeApprovalRequest),
				agentmessage.StatusEQ(agentmessage.StatusPending),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			Where(func(s *sql.Selector) {})

		if agentInstanceID != nil {
			q = q.Where(agentmessage.AgentInstanceIDEQ(*agentInstanceID))
		}

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query approval requests: %w", err)
		}

		out := make([]*AgentApprovalRequestView, 0, len(items))
		for _, m := range items {
			out = append(out, &AgentApprovalRequestView{
				ID:              m.ID,
				AgentID:         a.ID,
				AgentInstanceID: m.AgentInstanceID,
				CorrelationID:   m.CorrelationID,
				Content:         m.Content,
				Sequence:        m.Sequence,
				CreatedAt:       m.CreatedAt,
			})
		}

		return out, nil
	})
}

func (s *AgentBootstrapService) ListAgentMessagesAsAdmin(ctx context.Context, agentID int, agentInstanceID *int, afterSequence *int64, limit int) ([]*AgentMessageView, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	if limit <= 0 {
		limit = 200
	}

	if limit > 500 {
		limit = 500
	}

	return authz.RunWithSystemBypass(ctx, "agent-admin-list-thread-messages", func(bypassCtx context.Context) ([]*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			Where(func(s *sql.Selector) {})

		if agentInstanceID != nil {
			q = q.Where(agentmessage.AgentInstanceIDEQ(*agentInstanceID))
		}

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			text := extractTextFromMessageContent(m.Content)

			out = append(out, &AgentMessageView{
				ID:                m.ID,
				AgentID:           a.ID,
				AgentInstanceID:   m.AgentInstanceID,
				Direction:         m.Direction,
				SenderType:        m.SenderType,
				SenderID:          m.SenderID,
				Type:              m.Type,
				CorrelationID:     m.CorrelationID,
				Content:           m.Content,
				Text:              text,
				Sequence:          m.Sequence,
				Status:            m.Status,
				CreatedAt:         m.CreatedAt,
				ExternalMessageID: m.ExternalMessageID,
				ReplyToMessageID:  m.ReplyToMessageID,
			})
		}

		return out, nil
	})
}

func (s *AgentBootstrapService) ResolveApprovalAsUser(ctx context.Context, userID int, input ResolveApprovalCommandInput) (bool, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return false, fmt.Errorf("project id not found in context")
	}

	// Normalize scope value.
	scope := input.Scope
	if scope == "" {
		scope = "once"
	}

	switch scope {
	case "once", "thread", "workspace", "global":
	default:
		return false, fmt.Errorf("invalid approval scope: %q", scope)
	}

	return authz.RunWithSystemBypass(ctx, "agent-admin-resolve-approval", func(bypassCtx context.Context) (bool, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
			).
			Only(bypassCtx)
		if err != nil {
			return false, fmt.Errorf("failed to load agent: %w", err)
		}

		// Ensure there is a pending approval_request to resolve (prevents arbitrary injection).
		approvalReq, err := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.TypeEQ(agentmessage.TypeApprovalRequest),
				agentmessage.CorrelationIDEQ(input.RequestID),
			).
			Only(bypassCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return false, fmt.Errorf("approval request not found or already resolved")
			}

			return false, fmt.Errorf("failed to query approval request: %w", err)
		}

		payload := map[string]any{
			"type":       "approval_result",
			"request_id": input.RequestID,
			"granted":    input.Granted,
			"scope":      scope,
		}
		if input.Reason != nil && *input.Reason != "" {
			payload["reason"] = *input.Reason
		}

		if len(input.ResourceIndices) > 0 && input.Granted {
			var reqContent struct {
				Resources []json.RawMessage `json:"resources"`
			}
			if err := json.Unmarshal(approvalReq.Content, &reqContent); err == nil && len(reqContent.Resources) > 0 {
				var selectedResources []json.RawMessage

				for _, idx := range input.ResourceIndices {
					if idx >= 0 && idx < len(reqContent.Resources) {
						selectedResources = append(selectedResources, reqContent.Resources[idx])
					}
				}

				if len(selectedResources) > 0 {
					payload["resources"] = selectedResources
				}
			}
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			return false, fmt.Errorf("marshal approval result: %w", err)
		}

		var resultMsg *ent.AgentMessage

		for attempt := range 3 {
			nextSeq, err := s.nextSequence(bypassCtx, a.ID)
			if err != nil {
				return false, err
			}

			created, err := client.AgentMessage.Create().
				SetProjectID(projectID).
				SetAgentID(a.ID).
				SetAgentInstanceID(approvalReq.AgentInstanceID).
				SetDirection(agentmessage.DirectionToAgent).
				SetSenderType(agentmessage.SenderTypeUser).
				SetSenderID(userID).
				SetType(agentmessage.TypeApprovalResult).
				SetCorrelationID(input.RequestID).
				SetContent(raw).
				SetStatus(agentmessage.StatusPending).
				SetSequence(nextSeq).
				Save(bypassCtx)
			if err == nil {
				resultMsg = created
				break
			}

			if ent.IsConstraintError(err) && attempt < 2 {
				continue
			}

			return false, fmt.Errorf("create approval result: %w", err)
		}

		if resultMsg == nil {
			return false, fmt.Errorf("create approval result: no message created")
		}

		// Mark request as resolved to avoid repeated approvals.
		_, _ = client.AgentMessage.Update().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ProjectIDEQ(projectID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.TypeEQ(agentmessage.TypeApprovalRequest),
				agentmessage.StatusEQ(agentmessage.StatusPending),
				agentmessage.CorrelationIDEQ(input.RequestID),
			).
			SetStatus(agentmessage.StatusAcked).
			Save(bypassCtx)

		return true, nil
	})
}

func (s *AgentBootstrapService) RegisterAgentInstance(ctx context.Context, input RegisterAgentInstanceInput) (*ent.AgentInstance, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID
	now := time.Now()

	return authz.RunWithSystemBypass(ctx, "agent-runtime-register-instance", func(bypassCtx context.Context) (*ent.AgentInstance, error) {
		client := s.entFromContext(bypassCtx)

		// Verify agent exists and belongs to the project
		_, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		// Check if an instance already exists for this agent and API key
		existing, err := client.AgentInstance.Query().
			Where(
				agentinstance.AgentIDEQ(input.AgentID),
				agentinstance.APIKeyIDEQ(apiKey.ID),
				agentinstance.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err == nil {
			upd := client.AgentInstance.UpdateOneID(existing.ID).
				SetLastHeartbeatAt(now)

			if input.Name != nil {
				upd.SetName(*input.Name)
			}
			if input.Platform != nil {
				upd.SetPlatform(*input.Platform)
			}

			if input.Description != nil {
				upd.SetDescription(*input.Description)
			}

			inst, err := upd.Save(bypassCtx)
			if err != nil {
				return nil, err
			}

			if input.ThreadID != nil && *input.ThreadID != "" {
				if err := s.createAgentThread(bypassCtx, client, projectID, input.AgentID, *input.ThreadID); err != nil {
					return nil, err
				}
			}

			return inst, nil
		}
		if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("failed to query agent instance: %w", err)
		}

		create := client.AgentInstance.Create().
			SetProjectID(projectID).
			SetAgentID(input.AgentID).
			SetAPIKeyID(apiKey.ID).
			SetLastHeartbeatAt(now)

		if input.Name != nil {
			create.SetName(*input.Name)
		}
		if input.Platform != nil {
			create.SetPlatform(*input.Platform)
		}

		if input.Description != nil {
			create.SetDescription(*input.Description)
		}

		created, err := create.Save(bypassCtx)
		if err != nil {
			if ent.IsConstraintError(err) {
				existing, getErr := client.AgentInstance.Query().
					Where(
						agentinstance.AgentIDEQ(input.AgentID),
						agentinstance.APIKeyIDEQ(apiKey.ID),
						agentinstance.DeletedAtEQ(0),
					).
					Only(bypassCtx)
				if getErr != nil {
					return nil, fmt.Errorf("failed to reload agent instance after conflict: %w", getErr)
				}

				inst, err := client.AgentInstance.UpdateOneID(existing.ID).
					SetLastHeartbeatAt(now).
					Save(bypassCtx)
				if err != nil {
					return nil, err
				}

				if input.ThreadID != nil && *input.ThreadID != "" {
					if err := s.createAgentThread(bypassCtx, client, projectID, input.AgentID, *input.ThreadID); err != nil {
						return nil, err
					}
				}

				return inst, nil
			}
			return nil, fmt.Errorf("failed to create agent instance: %w", err)
		}

		if input.ThreadID != nil && *input.ThreadID != "" {
			if err := s.createAgentThread(bypassCtx, client, projectID, input.AgentID, *input.ThreadID); err != nil {
				return nil, err
			}
		}

		return created, nil
	})
}

func (s *AgentBootstrapService) createAgentThread(ctx context.Context, client *ent.Client, projectID int, agentID int, threadID string) error {
	t, err := client.Thread.Query().
		Where(
			thread.ThreadIDEQ(threadID),
			thread.ProjectIDEQ(projectID),
		).
		Only(ctx)
	if err != nil {
		if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to query thread: %w", err)
		}

		t, err = client.Thread.Create().
			SetThreadID(threadID).
			SetProjectID(projectID).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				t, err = client.Thread.Query().
					Where(
						thread.ThreadIDEQ(threadID),
						thread.ProjectIDEQ(projectID),
					).
					Only(ctx)
				if err != nil {
					return fmt.Errorf("failed to reload thread after conflict: %w", err)
				}
			} else {
				return fmt.Errorf("failed to create thread: %w", err)
			}
		}
	}

	exists, err := client.AgentThread.Query().
		Where(
			agentthread.AgentIDEQ(agentID),
			agentthread.ThreadIDEQ(t.ID),
		).
		Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to query agent thread: %w", err)
	}

	if exists {
		return nil
	}

	_, err = client.AgentThread.Create().
		SetProjectID(projectID).
		SetAgentID(agentID).
		SetThreadID(t.ID).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil
		}

		return fmt.Errorf("failed to create agent thread: %w", err)
	}

	return nil
}

func (s *AgentBootstrapService) HeartbeatAgentInstance(ctx context.Context, inst *ent.AgentInstance) error {
	now := time.Now()

	_, err := authz.RunWithSystemBypass(ctx, "agent-runtime-heartbeat-instance", func(bypassCtx context.Context) (*ent.AgentInstance, error) {
		client := s.entFromContext(bypassCtx)

		return client.AgentInstance.UpdateOneID(inst.ID).
			SetLastHeartbeatAt(now).
			SetStatus(agentinstance.StatusRunning).
			Save(bypassCtx)
	})

	return err
}

type agentTextMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func marshalMessageContent(typ string, text string, payload any) (objects.JSONRawMessage, error) {
	type base struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}

	if typ == "" {
		typ = "text"
	}

	if typ == "text" {
		raw, err := json.Marshal(agentTextMessageContent{Type: typ, Text: text})
		return objects.JSONRawMessage(raw), err
	}
	// For non-text messages, allow payload to carry structured fields.
	// The caller must ensure payload is JSON-safe and redacted.
	if payload == nil {
		raw, err := json.Marshal(base{Type: typ, Text: text})
		return objects.JSONRawMessage(raw), err
	}

	raw, err := json.Marshal(payload)

	return objects.JSONRawMessage(raw), err
}

func extractTextFromMessageContent(raw objects.JSONRawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var content agentTextMessageContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return ""
	}

	return content.Text
}

func (s *AgentBootstrapService) ListPeerAgents(ctx context.Context, inst *ent.AgentInstance) ([]*PeerAgentView, error) {
	return authz.RunWithSystemBypass(ctx, "agent-runtime-list-peer-agents", func(bypassCtx context.Context) ([]*PeerAgentView, error) {
		client := s.entFromContext(bypassCtx)

		var out []*PeerAgentView

		instances, err := client.AgentInstance.Query().
			Where(
				agentinstance.ProjectIDEQ(inst.ProjectID),
				agentinstance.AgentIDEQ(inst.AgentID),
			).
			Order(agentinstance.ByLastHeartbeatAt(sql.OrderDesc())).
			All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query instances for agent %d: %w", inst.AgentID, err)
		}

		for _, i := range instances {
			out = append(out, &PeerAgentView{
				AgentID:         i.AgentID,
				AgentInstanceID: i.ID,
				Name:            i.Name,
				Description:     i.Description,
				Status:          string(i.Status),
			})
		}

		return out, nil
	})
}

// sendPeerMessageToInstance sends a peer message to a specific agent instance with retry logic.
func (s *AgentBootstrapService) sendPeerMessageToInstance(
	bypassCtx context.Context,
	senderInst *ent.AgentInstance,
	targetInst *ent.AgentInstance,
	raw objects.JSONRawMessage,
) (*ent.AgentMessage, error) {
	for attempt := range 3 {
		nextSeq, err := s.nextSequence(bypassCtx, targetInst.AgentID)
		if err != nil {
			return nil, err
		}

		created, err := s.entFromContext(bypassCtx).AgentMessage.Create().
			SetProjectID(targetInst.ProjectID).
			SetAgentID(targetInst.AgentID).
			SetAgentInstanceID(targetInst.ID).
			SetDirection(agentmessage.DirectionToAgent).
			SetSenderType(agentmessage.SenderTypeAgent).
			SetSenderID(senderInst.AgentID).
			SetType(agentmessage.TypeChat).
			SetContent(raw).
			SetStatus(agentmessage.StatusPending).
			SetSequence(nextSeq).
			Save(bypassCtx)
		if err == nil {
			return created, nil
		}

		if ent.IsConstraintError(err) && attempt < 2 {
			continue
		}

		return nil, fmt.Errorf("failed to create peer message: %w", err)
	}

	return nil, fmt.Errorf("failed to create peer message: max retries exceeded")
}

func (s *AgentBootstrapService) SendPeerMessage(ctx context.Context, input SendPeerMessageInput) (*AgentMessageView, error) {
	senderInst, err := s.GetAgentInstanceFromAPIKey(ctx)
	if err != nil {
		return nil, err
	}

	projectID := senderInst.ProjectID

	return authz.RunWithSystemBypass(ctx, "agent-runtime-send-peer-message", func(bypassCtx context.Context) (*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		// Verify target agent exists in the same project.
		targetAgent, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.TargetAgentID),
				agent.ProjectIDEQ(projectID),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load target agent: %w", err)
		}

		// Get the target agent instances - either specified one or all active ones (broadcast)
		var targetInsts []*ent.AgentInstance

		if input.TargetAgentInstanceID != nil {
			targetInst, err := client.AgentInstance.Query().
				Where(
					agentinstance.IDEQ(*input.TargetAgentInstanceID),
					agentinstance.AgentIDEQ(targetAgent.ID),
				).
				Only(bypassCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to load target agent instance: %w", err)
			}

			targetInsts = []*ent.AgentInstance{targetInst}
		} else {
			targetInsts, err = client.AgentInstance.Query().
				Where(
					agentinstance.AgentIDEQ(targetAgent.ID),
					agentinstance.DeletedAtEQ(0),
				).
				All(bypassCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to load target agent instances: %w", err)
			}

			if len(targetInsts) == 0 {
				return nil, fmt.Errorf("no active agent instances found for agent %d", targetAgent.ID)
			}
		}

		raw := objects.JSONRawMessage(nil)
		if input.Content != nil && len(*input.Content) > 0 && string(*input.Content) != "null" {
			raw = *input.Content
		} else {
			b, err := marshalMessageContent("text", input.Text, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal message content: %w", err)
			}

			raw = b
		}

		var msgs []*ent.AgentMessage

		for _, targetInst := range targetInsts {
			msg, err := s.sendPeerMessageToInstance(bypassCtx, senderInst, targetInst, raw)
			if err != nil {
				return nil, err
			}

			msgs = append(msgs, msg)
		}

		if len(msgs) == 0 {
			return nil, fmt.Errorf("failed to create peer message: no message created")
		}

		// Return the first message as the representative
		msg := msgs[0]

		viewText := extractTextFromMessageContent(msg.Content)
		if viewText == "" {
			viewText = input.Text
		}

		return &AgentMessageView{
			ID:                msg.ID,
			AgentID:           targetAgent.ID,
			Direction:         msg.Direction,
			SenderType:        msg.SenderType,
			Type:              msg.Type,
			CorrelationID:     msg.CorrelationID,
			Content:           msg.Content,
			Text:              viewText,
			Sequence:          msg.Sequence,
			Status:            msg.Status,
			CreatedAt:         msg.CreatedAt,
			ExternalMessageID: msg.ExternalMessageID,
			ReplyToMessageID:  msg.ReplyToMessageID,
		}, nil
	})
}

func (s *AgentBootstrapService) PushAgentMessage(ctx context.Context, inst *ent.AgentInstance, input PushAgentMessageInput) (*AgentMessageView, error) {
	msgType := agentmessage.TypeChat
	if input.Type != nil && *input.Type != "" {
		msgType = *input.Type
	}

	corr := ""
	if input.CorrelationID != nil {
		corr = *input.CorrelationID
	}

	return s.createMessage(ctx, inst, input.Text, input.Content, agentmessage.DirectionToUser, agentmessage.SenderTypeAgent, msgType, corr, input.ReplyToMessageID)
}

func (s *AgentBootstrapService) createMessage(
	ctx context.Context,
	inst *ent.AgentInstance,
	text string,
	content *objects.JSONRawMessage,
	direction agentmessage.Direction,
	senderType agentmessage.SenderType,
	msgType agentmessage.Type,
	correlationID string,
	replyToMessageID *int,
) (*AgentMessageView, error) {
	return authz.RunWithSystemBypass(ctx, "agent-runtime-create-message", func(bypassCtx context.Context) (*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(inst.AgentID),
				agent.ProjectIDEQ(inst.ProjectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		senderID := &inst.ID

		raw := objects.JSONRawMessage(nil)
		if content != nil && len(*content) > 0 && string(*content) != "null" {
			raw = *content
		} else {
			b, err := marshalMessageContent("text", text, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal message content: %w", err)
			}

			raw = b
		}

		var msg *ent.AgentMessage
		for attempt := 0; attempt < 3; attempt++ {
			nextSeq, err := s.nextSequence(bypassCtx, a.ID)
			if err != nil {
				return nil, err
			}

			created, err := client.AgentMessage.Create().
				SetProjectID(inst.ProjectID).
				SetAgentID(a.ID).
				SetAgentInstanceID(*senderID).
				SetDirection(direction).
				SetSenderType(senderType).
				SetNillableSenderID(senderID).
				SetType(msgType).
				SetCorrelationID(correlationID).
				SetContent(raw).
				SetStatus(agentmessage.StatusPending).
				SetSequence(nextSeq).
				SetNillableReplyToMessageID(replyToMessageID).
				Save(bypassCtx)
			if err == nil {
				msg = created
				break
			}

			if ent.IsConstraintError(err) && attempt < 2 {
				continue
			}

			return nil, fmt.Errorf("failed to create message: %w", err)
		}
		if msg == nil {
			return nil, fmt.Errorf("failed to create message: no message created")
		}

		viewText := extractTextFromMessageContent(msg.Content)
		if viewText == "" {
			viewText = text
		}

		return &AgentMessageView{
			ID:                msg.ID,
			AgentID:           a.ID,
			Direction:         msg.Direction,
			SenderType:        msg.SenderType,
			SenderID:          senderID,
			Type:              msg.Type,
			CorrelationID:     msg.CorrelationID,
			Content:           msg.Content,
			Text:              viewText,
			Sequence:          msg.Sequence,
			Status:            msg.Status,
			CreatedAt:         msg.CreatedAt,
			ExternalMessageID: msg.ExternalMessageID,
			ReplyToMessageID:  msg.ReplyToMessageID,
		}, nil
	})
}

func (s *AgentBootstrapService) nextSequence(ctx context.Context, agentID int) (int64, error) {
	client := s.entFromContext(ctx)

	last, err := client.AgentMessage.Query().
		Where(
			agentmessage.AgentIDEQ(agentID),
		).
		Order(agentmessage.BySequence(sql.OrderDesc())).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("failed to query last sequence: %w", err)
	}

	return last.Sequence + 1, nil
}

func (s *AgentBootstrapService) PullAgentMessages(ctx context.Context, inst *ent.AgentInstance, input PullAgentMessagesInput) ([]*AgentMessageView, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	return authz.RunWithSystemBypass(ctx, "agent-runtime-pull-messages", func(bypassCtx context.Context) ([]*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(inst.AgentID),
				agent.ProjectIDEQ(inst.ProjectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToAgent),
				agentmessage.StatusEQ(agentmessage.StatusPending),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			Where(func(s *sql.Selector) {})

		if len(input.TypeIn) > 0 {
			q = q.Where(agentmessage.TypeIn(input.TypeIn...))
		}

		if input.CorrelationID != nil && *input.CorrelationID != "" {
			q = q.Where(agentmessage.CorrelationIDEQ(*input.CorrelationID))
		}
		if input.AfterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*input.AfterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			text := extractTextFromMessageContent(m.Content)

			out = append(out, &AgentMessageView{
				ID:                m.ID,
				AgentID:           a.ID,
				Direction:         m.Direction,
				SenderType:        m.SenderType,
				SenderID:          m.SenderID,
				Type:              m.Type,
				CorrelationID:     m.CorrelationID,
				Content:           m.Content,
				Text:              text,
				Sequence:          m.Sequence,
				Status:            m.Status,
				CreatedAt:         m.CreatedAt,
				ExternalMessageID: m.ExternalMessageID,
				ReplyToMessageID:  m.ReplyToMessageID,
			})
		}

		return out, nil
	})
}

func (s *AgentBootstrapService) PullAgentMessagesToUser(ctx context.Context, inst *ent.AgentInstance, afterSequence *int64, limit int) ([]*AgentMessageView, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	return authz.RunWithSystemBypass(ctx, "agent-runtime-pull-messages-to-user", func(bypassCtx context.Context) ([]*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(inst.AgentID),
				agent.ProjectIDEQ(inst.ProjectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.TypeEQ(agentmessage.TypeChat),
				agentmessage.StatusEQ(agentmessage.StatusPending),
			).
			Order(agentmessage.BySequence()).
			Limit(limit)

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			text := extractTextFromMessageContent(m.Content)

			out = append(out, &AgentMessageView{
				ID:                m.ID,
				AgentID:           a.ID,
				Direction:         m.Direction,
				SenderType:        m.SenderType,
				SenderID:          m.SenderID,
				Type:              m.Type,
				CorrelationID:     m.CorrelationID,
				Content:           m.Content,
				Text:              text,
				Sequence:          m.Sequence,
				Status:            m.Status,
				CreatedAt:         m.CreatedAt,
				ExternalMessageID: m.ExternalMessageID,
			})
		}

		return out, nil
	})
}

func (s *AgentBootstrapService) AckAgentMessages(ctx context.Context, inst *ent.AgentInstance, messageIDs []int) error {
	if len(messageIDs) == 0 {
		return nil
	}

	_, err := authz.RunWithSystemBypass(ctx, "agent-runtime-ack-messages", func(bypassCtx context.Context) (int, error) {
		client := s.entFromContext(bypassCtx)

		affected, err := client.AgentMessage.Update().
			Where(
				agentmessage.IDIn(messageIDs...),
				agentmessage.AgentIDEQ(inst.AgentID),
				agentmessage.ProjectIDEQ(inst.ProjectID),
				agentmessage.StatusEQ(agentmessage.StatusPending),
			).
			SetStatus(agentmessage.StatusAcked).
			Save(bypassCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to ack messages: %w", err)
		}

		return affected, nil
	})

	return err
}

// AckAgentMessagesAsUser acknowledges messages as a user (for Web UI).
// Unlike AckAgentMessages which requires agent API key, this method uses user authentication.
func (s *AgentBootstrapService) AckAgentMessagesAsUser(ctx context.Context, userID int, input AckAgentMessagesInput) (bool, error) {
	if len(input.MessageIDs) == 0 {
		return true, nil
	}

	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return false, fmt.Errorf("project id not found in context")
	}

	_, err := authz.RunWithSystemBypass(ctx, "agent-user-ack-messages", func(bypassCtx context.Context) (int, error) {
		client := s.entFromContext(bypassCtx)

		// Verify agent exists and belongs to project
		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to load agent: %w", err)
		}

		if input.AgentInstanceID != nil {
			exists, err := client.AgentInstance.Query().
				Where(
					agentinstance.IDEQ(*input.AgentInstanceID),
					agentinstance.AgentIDEQ(a.ID),
					agentinstance.DeletedAtEQ(0),
				).
				Exist(bypassCtx)
			if err != nil {
				return 0, fmt.Errorf("failed to check agent instance: %w", err)
			}

			if !exists {
				return 0, fmt.Errorf("agent instance not found")
			}
		}

		q := client.AgentMessage.Update().
			Where(
				agentmessage.IDIn(input.MessageIDs...),
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ProjectIDEQ(projectID),
				agentmessage.StatusEQ(agentmessage.StatusPending),
			)

		if input.AgentInstanceID != nil {
			q = q.Where(agentmessage.AgentInstanceIDEQ(*input.AgentInstanceID))
		}

		affected, err := q.SetStatus(agentmessage.StatusAcked).Save(bypassCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to ack messages: %w", err)
		}

		return affected, nil
	})
	if err != nil {
		return false, err
	}

	return true, nil
}
