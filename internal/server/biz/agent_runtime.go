package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/samber/lo"
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

type AgentRuntimeServiceParams struct {
	fx.In

	Ent           *ent.Client
	ThreadService *ThreadService
}

// AgentRuntimeService provides APIs for the runtime agent endpoint (/agent/v1/graphql).
// This service enforces agent API key ownership checks at the application layer and
// uses system bypass for DB access to avoid coupling to Ent privacy rules.
type AgentRuntimeService struct {
	*AbstractService

	threadService *ThreadService
}

func NewAgentRuntimeService(params AgentRuntimeServiceParams) *AgentRuntimeService {
	return &AgentRuntimeService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		threadService: params.ThreadService,
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
	AgentID      int
	AgentName    string
	Model        *string
	SystemPrompt string

	Tools        []AgentToolDefinition
	Skills       []AgentSkillDefinition
	BuiltinTools []objects.AgentBuiltinTool
	SkillsPolicy objects.AgentSkillsPolicy

	MemoryPolicy *objects.JSONRawMessage
}

type AgentMessageView struct {
	ID         int
	AgentID    int
	ThreadID   string
	Direction  agentmessage.Direction
	SenderType agentmessage.SenderType
	SenderID   *int
	Text       string
	Sequence   int64
	Status     agentmessage.Status
	CreatedAt  time.Time
}

type RegisterAgentInstanceInput struct {
	AgentID    int
	InstanceID string
	Name       *string
	Platform   *string
	Version    *string
}

type SendAgentMessageInput struct {
	AgentID  int
	ThreadID string
	Text     string
}

type PushAgentMessageInput struct {
	AgentID    int
	InstanceID string
	ThreadID   string
	Text       string
}

type PullAgentMessagesInput struct {
	AgentID       int
	InstanceID    string
	ThreadID      string
	AfterSequence *int64
	Limit         int
}

type AckAgentMessagesInput struct {
	AgentID    int
	InstanceID string
	MessageIDs []int
}

func (s *AgentRuntimeService) AgentBootstrap(ctx context.Context, agentID int) (*AgentBootstrap, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID

	return authz.RunWithSystemBypass(ctx, "agent-runtime-bootstrap", func(bypassCtx context.Context) (*AgentBootstrap, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		p, err := client.Prompt.Query().
			Where(
				prompt.IDEQ(a.PromptID),
				prompt.ProjectIDEQ(projectID),
				prompt.DeletedAtEQ(0),
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
				agenttool.ProjectIDEQ(projectID),
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
				cfg := objects.JSONRawMessage(b.Config)
				def.Config = &cfg
			}
			tools = append(tools, def)
		}

		skillBindings, err := client.AgentSkill.Query().
			Where(
				agentskill.AgentIDEQ(a.ID),
				agentskill.ProjectIDEQ(projectID),
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
			AgentID:      a.ID,
			AgentName:    a.Name,
			Model:        model,
			SystemPrompt: p.Content,
			Tools:        tools,
			Skills:       skills,
			BuiltinTools: builtinTools,
			SkillsPolicy: skillsPolicy,
			MemoryPolicy: nil,
		}, nil
	})
}

func (s *AgentRuntimeService) SendAgentMessageAsUser(ctx context.Context, userID int, agentID int, threadID string, text string) (*AgentMessageView, error) {
	projectID, ok := contexts.GetProjectID(ctx)
	if !ok {
		return nil, fmt.Errorf("project id not found in context")
	}

	return authz.RunWithSystemBypass(ctx, "agent-admin-send-message", func(bypassCtx context.Context) (*AgentMessageView, error) {
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

		th, err := s.threadService.GetOrCreateThread(bypassCtx, projectID, threadID)
		if err != nil {
			return nil, fmt.Errorf("failed to get or create thread: %w", err)
		}

		if _, err := client.AgentThread.Create().
			SetProjectID(projectID).
			SetAgentID(a.ID).
			SetThreadRowID(th.ID).
			Save(bypassCtx); err != nil {
			if !ent.IsConstraintError(err) {
				return nil, fmt.Errorf("failed to bind agent thread: %w", err)
			}
		}

		raw, err := json.Marshal(agentTextMessageContent{Type: "text", Text: text})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message content: %w", err)
		}

		var msg *ent.AgentMessage
		for attempt := 0; attempt < 3; attempt++ {
			nextSeq, err := s.nextSequence(bypassCtx, a.ID, th.ID)
			if err != nil {
				return nil, err
			}

			created, err := client.AgentMessage.Create().
				SetProjectID(projectID).
				SetAgentID(a.ID).
				SetThreadRowID(th.ID).
				SetDirection(agentmessage.DirectionToRuntime).
				SetSenderType(agentmessage.SenderTypeUser).
				SetSenderID(userID).
				SetContent(objects.JSONRawMessage(raw)).
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
			ID:         msg.ID,
			AgentID:    a.ID,
			ThreadID:   th.ThreadID,
			Direction:  msg.Direction,
			SenderType: msg.SenderType,
			SenderID:   lo.ToPtr(userID),
			Text:       text,
			Sequence:   msg.Sequence,
			Status:     msg.Status,
			CreatedAt:  msg.CreatedAt,
		}, nil
	})
}

func (s *AgentRuntimeService) PullAgentMessagesToUserAsAdmin(ctx context.Context, agentID int, threadID string, afterSequence *int64, limit int) ([]*AgentMessageView, error) {
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

		th, err := client.Thread.Query().
			Where(thread.ProjectIDEQ(projectID), thread.ThreadIDEQ(threadID)).
			Only(bypassCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return []*AgentMessageView{}, nil
			}
			return nil, fmt.Errorf("failed to load thread: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ThreadRowIDEQ(th.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.StatusEQ(agentmessage.StatusPending),
				agentmessage.DeletedAtEQ(0),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			WithThread()

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			if m.Edges.Thread == nil {
				continue
			}

			text := ""
			var content agentTextMessageContent
			if len(m.Content) > 0 && json.Unmarshal(m.Content, &content) == nil {
				text = content.Text
			}

			out = append(out, &AgentMessageView{
				ID:         m.ID,
				AgentID:    a.ID,
				ThreadID:   m.Edges.Thread.ThreadID,
				Direction:  m.Direction,
				SenderType: m.SenderType,
				SenderID:   m.SenderID,
				Text:       text,
				Sequence:   m.Sequence,
				Status:     m.Status,
				CreatedAt:  m.CreatedAt,
			})
		}

		return out, nil
	})
}

func (s *AgentRuntimeService) ListAgentThreadMessagesAsAdmin(ctx context.Context, agentID int, threadID string, afterSequence *int64, limit int) ([]*AgentMessageView, error) {
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

		th, err := client.Thread.Query().
			Where(thread.ProjectIDEQ(projectID), thread.ThreadIDEQ(threadID)).
			Only(bypassCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return []*AgentMessageView{}, nil
			}
			return nil, fmt.Errorf("failed to load thread: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ThreadRowIDEQ(th.ID),
				agentmessage.DeletedAtEQ(0),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			WithThread()

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			if m.Edges.Thread == nil {
				continue
			}

			text := ""
			var content agentTextMessageContent
			if len(m.Content) > 0 && json.Unmarshal(m.Content, &content) == nil {
				text = content.Text
			}

			out = append(out, &AgentMessageView{
				ID:         m.ID,
				AgentID:    a.ID,
				ThreadID:   m.Edges.Thread.ThreadID,
				Direction:  m.Direction,
				SenderType: m.SenderType,
				SenderID:   m.SenderID,
				Text:       text,
				Sequence:   m.Sequence,
				Status:     m.Status,
				CreatedAt:  m.CreatedAt,
			})
		}

		return out, nil
	})
}

type AgentThreadSummaryView struct {
	ThreadID  string
	CreatedAt time.Time
}

func (s *AgentRuntimeService) ListAgentThreadsAsAdmin(ctx context.Context, agentID int, limit int) ([]AgentThreadSummaryView, error) {
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

	return authz.RunWithSystemBypass(ctx, "agent-admin-list-threads", func(bypassCtx context.Context) ([]AgentThreadSummaryView, error) {
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

		items, err := client.AgentThread.Query().
			Where(
				agentthread.AgentIDEQ(a.ID),
				agentthread.ProjectIDEQ(projectID),
			).
			Order(agentthread.ByCreatedAt(sql.OrderDesc())).
			Limit(limit).
			WithThread().
			All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query agent threads: %w", err)
		}

		out := make([]AgentThreadSummaryView, 0, len(items))
		for _, it := range items {
			if it.Edges.Thread == nil {
				continue
			}
			out = append(out, AgentThreadSummaryView{
				ThreadID:  it.Edges.Thread.ThreadID,
				CreatedAt: it.CreatedAt,
			})
		}

		return out, nil
	})
}

func (s *AgentRuntimeService) RegisterAgentInstance(ctx context.Context, input RegisterAgentInstanceInput) (*ent.AgentInstance, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID
	now := time.Now()

	return authz.RunWithSystemBypass(ctx, "agent-runtime-register-instance", func(bypassCtx context.Context) (*ent.AgentInstance, error) {
		client := s.entFromContext(bypassCtx)

		_, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		existing, err := client.AgentInstance.Query().
			Where(
				agentinstance.AgentIDEQ(input.AgentID),
				agentinstance.InstanceIDEQ(input.InstanceID),
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
			if input.Version != nil {
				upd.SetVersion(*input.Version)
			}

			return upd.Save(bypassCtx)
		}
		if !ent.IsNotFound(err) {
			return nil, fmt.Errorf("failed to query agent instance: %w", err)
		}

		create := client.AgentInstance.Create().
			SetProjectID(projectID).
			SetAgentID(input.AgentID).
			SetInstanceID(input.InstanceID).
			SetLastHeartbeatAt(now)

		if input.Name != nil {
			create.SetName(*input.Name)
		}
		if input.Platform != nil {
			create.SetPlatform(*input.Platform)
		}
		if input.Version != nil {
			create.SetVersion(*input.Version)
		}

		created, err := create.Save(bypassCtx)
		if err != nil {
			if ent.IsConstraintError(err) {
				// Retry as update when concurrent register happens.
				existing, getErr := client.AgentInstance.Query().
					Where(
						agentinstance.AgentIDEQ(input.AgentID),
						agentinstance.InstanceIDEQ(input.InstanceID),
						agentinstance.DeletedAtEQ(0),
					).
					Only(bypassCtx)
				if getErr != nil {
					return nil, fmt.Errorf("failed to reload agent instance after conflict: %w", getErr)
				}
				return client.AgentInstance.UpdateOneID(existing.ID).
					SetLastHeartbeatAt(now).
					Save(bypassCtx)
			}
			return nil, fmt.Errorf("failed to create agent instance: %w", err)
		}

		return created, nil
	})
}

func (s *AgentRuntimeService) HeartbeatAgentInstance(ctx context.Context, agentID int, instanceID string) (bool, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return false, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID
	now := time.Now()

	affected, err := authz.RunWithSystemBypass(ctx, "agent-runtime-heartbeat-instance", func(bypassCtx context.Context) (int, error) {
		client := s.entFromContext(bypassCtx)

		_, err := client.Agent.Query().
			Where(
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to load agent: %w", err)
		}

		return client.AgentInstance.Update().
			Where(
				agentinstance.AgentIDEQ(agentID),
				agentinstance.InstanceIDEQ(instanceID),
				agentinstance.DeletedAtEQ(0),
			).
			SetLastHeartbeatAt(now).
			Save(bypassCtx)
	})
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

type agentTextMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *AgentRuntimeService) SendAgentMessage(ctx context.Context, input SendAgentMessageInput) (*AgentMessageView, error) {
	return s.createMessage(ctx, input.AgentID, nil, input.ThreadID, input.Text, agentmessage.DirectionToRuntime, agentmessage.SenderTypeUser)
}

func (s *AgentRuntimeService) PushAgentMessage(ctx context.Context, input PushAgentMessageInput) (*AgentMessageView, error) {
	return s.createMessage(ctx, input.AgentID, &input.InstanceID, input.ThreadID, input.Text, agentmessage.DirectionToUser, agentmessage.SenderTypeRuntime)
}

func (s *AgentRuntimeService) createMessage(
	ctx context.Context,
	agentID int,
	instanceID *string,
	threadID string,
	text string,
	direction agentmessage.Direction,
	senderType agentmessage.SenderType,
) (*AgentMessageView, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID

	return authz.RunWithSystemBypass(ctx, "agent-runtime-create-message", func(bypassCtx context.Context) (*AgentMessageView, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		var senderID *int
		if instanceID != nil {
			inst, err := client.AgentInstance.Query().
				Where(
					agentinstance.AgentIDEQ(a.ID),
					agentinstance.InstanceIDEQ(*instanceID),
					agentinstance.DeletedAtEQ(0),
				).
				Only(bypassCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to load agent instance: %w", err)
			}
			senderID = &inst.ID
		}

		th, err := s.threadService.GetOrCreateThread(bypassCtx, projectID, threadID)
		if err != nil {
			return nil, fmt.Errorf("failed to get or create thread: %w", err)
		}

		// Ensure thread binding exists.
		if _, err := client.AgentThread.Create().
			SetProjectID(projectID).
			SetAgentID(a.ID).
			SetThreadRowID(th.ID).
			Save(bypassCtx); err != nil {
			if !ent.IsConstraintError(err) {
				return nil, fmt.Errorf("failed to bind agent thread: %w", err)
			}
		}

		raw, err := json.Marshal(agentTextMessageContent{Type: "text", Text: text})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal message content: %w", err)
		}

		var msg *ent.AgentMessage
		for attempt := 0; attempt < 3; attempt++ {
			nextSeq, err := s.nextSequence(bypassCtx, a.ID, th.ID)
			if err != nil {
				return nil, err
			}

			created, err := client.AgentMessage.Create().
				SetProjectID(projectID).
				SetAgentID(a.ID).
				SetThreadRowID(th.ID).
				SetDirection(direction).
				SetSenderType(senderType).
				SetNillableSenderID(senderID).
				SetContent(objects.JSONRawMessage(raw)).
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
			ID:         msg.ID,
			AgentID:    a.ID,
			ThreadID:   th.ThreadID,
			Direction:  msg.Direction,
			SenderType: msg.SenderType,
			SenderID:   senderID,
			Text:       text,
			Sequence:   msg.Sequence,
			Status:     msg.Status,
			CreatedAt:  msg.CreatedAt,
		}, nil
	})
}

func (s *AgentRuntimeService) nextSequence(ctx context.Context, agentID int, threadRowID int) (int64, error) {
	client := s.entFromContext(ctx)

	last, err := client.AgentMessage.Query().
		Where(
			agentmessage.AgentIDEQ(agentID),
			agentmessage.ThreadRowIDEQ(threadRowID),
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

func (s *AgentRuntimeService) PullAgentMessages(ctx context.Context, input PullAgentMessagesInput) ([]*AgentMessageView, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID
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
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		_, err = client.AgentInstance.Query().
			Where(
				agentinstance.AgentIDEQ(a.ID),
				agentinstance.InstanceIDEQ(input.InstanceID),
				agentinstance.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent instance: %w", err)
		}

		th, err := client.Thread.Query().
			Where(thread.ProjectIDEQ(projectID), thread.ThreadIDEQ(input.ThreadID)).
			Only(bypassCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return []*AgentMessageView{}, nil
			}
			return nil, fmt.Errorf("failed to load thread: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ThreadRowIDEQ(th.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToRuntime),
				agentmessage.StatusEQ(agentmessage.StatusPending),
				agentmessage.DeletedAtEQ(0),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			WithThread()

		if input.AfterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*input.AfterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			if m.Edges.Thread == nil {
				continue
			}

			text := ""
			var content agentTextMessageContent
			if len(m.Content) > 0 && json.Unmarshal(m.Content, &content) == nil {
				text = content.Text
			}

			out = append(out, &AgentMessageView{
				ID:         m.ID,
				AgentID:    a.ID,
				ThreadID:   m.Edges.Thread.ThreadID,
				Direction:  m.Direction,
				SenderType: m.SenderType,
				SenderID:   m.SenderID,
				Text:       text,
				Sequence:   m.Sequence,
				Status:     m.Status,
				CreatedAt:  m.CreatedAt,
			})
		}

		return out, nil
	})
}

func (s *AgentRuntimeService) PullAgentMessagesToUser(ctx context.Context, agentID int, threadID string, afterSequence *int64, limit int) ([]*AgentMessageView, error) {
	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return nil, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID
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
				agent.IDEQ(agentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to load agent: %w", err)
		}

		th, err := client.Thread.Query().
			Where(thread.ProjectIDEQ(projectID), thread.ThreadIDEQ(threadID)).
			Only(bypassCtx)
		if err != nil {
			if ent.IsNotFound(err) {
				return []*AgentMessageView{}, nil
			}
			return nil, fmt.Errorf("failed to load thread: %w", err)
		}

		q := client.AgentMessage.Query().
			Where(
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ThreadRowIDEQ(th.ID),
				agentmessage.DirectionEQ(agentmessage.DirectionToUser),
				agentmessage.StatusEQ(agentmessage.StatusPending),
				agentmessage.DeletedAtEQ(0),
			).
			Order(agentmessage.BySequence()).
			Limit(limit).
			WithThread()

		if afterSequence != nil {
			q = q.Where(agentmessage.SequenceGT(*afterSequence))
		}

		items, err := q.All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to query messages: %w", err)
		}

		out := make([]*AgentMessageView, 0, len(items))
		for _, m := range items {
			if m.Edges.Thread == nil {
				continue
			}

			text := ""
			var content agentTextMessageContent
			if len(m.Content) > 0 && json.Unmarshal(m.Content, &content) == nil {
				text = content.Text
			}

			out = append(out, &AgentMessageView{
				ID:         m.ID,
				AgentID:    a.ID,
				ThreadID:   m.Edges.Thread.ThreadID,
				Direction:  m.Direction,
				SenderType: m.SenderType,
				SenderID:   m.SenderID,
				Text:       text,
				Sequence:   m.Sequence,
				Status:     m.Status,
				CreatedAt:  m.CreatedAt,
			})
		}

		return out, nil
	})
}

func (s *AgentRuntimeService) AckAgentMessages(ctx context.Context, input AckAgentMessagesInput) (bool, error) {
	if len(input.MessageIDs) == 0 {
		return true, nil
	}

	apiKey, ok := contexts.GetAPIKey(ctx)
	if !ok || apiKey == nil || apiKey.Type != apikey.TypeAgent {
		return false, fmt.Errorf("agent api key not found in context")
	}

	projectID := apiKey.ProjectID

	_, err := authz.RunWithSystemBypass(ctx, "agent-runtime-ack-messages", func(bypassCtx context.Context) (int, error) {
		client := s.entFromContext(bypassCtx)

		a, err := client.Agent.Query().
			Where(
				agent.IDEQ(input.AgentID),
				agent.ProjectIDEQ(projectID),
				agent.APIKeyIDEQ(apiKey.ID),
				agent.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to load agent: %w", err)
		}

		_, err = client.AgentInstance.Query().
			Where(
				agentinstance.AgentIDEQ(a.ID),
				agentinstance.InstanceIDEQ(input.InstanceID),
				agentinstance.DeletedAtEQ(0),
			).
			Only(bypassCtx)
		if err != nil {
			return 0, fmt.Errorf("failed to load agent instance: %w", err)
		}

		affected, err := client.AgentMessage.Update().
			Where(
				agentmessage.IDIn(input.MessageIDs...),
				agentmessage.AgentIDEQ(a.ID),
				agentmessage.ProjectIDEQ(projectID),
				agentmessage.StatusEQ(agentmessage.StatusPending),
				agentmessage.DeletedAtEQ(0),
			).
			SetStatus(agentmessage.StatusAcked).
			Save(bypassCtx)
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
