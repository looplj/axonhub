package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"
	axoncontext "github.com/looplj/axonhub/axon/context"
	"github.com/looplj/axonhub/axon/tools"
	"github.com/looplj/axonhub/cmd/axonclaw/api"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

type Runner struct {
	Client     graphql.Client
	Agent      *agent.Agent
	Logger     *slog.Logger
	InstanceID string
	ThreadID   string
	Config     conf.Config
}

func New(
	logger *slog.Logger,
	client graphql.Client,
	agent *agent.Agent,
	cfg conf.Config,
	threadID string,
) *Runner {
	return &Runner{
		Client:     client,
		Agent:      agent,
		Logger:     logger,
		InstanceID: cfg.InstanceID,
		ThreadID:   threadID,
		Config:     cfg,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	pollTicker := time.NewTicker(r.Config.PollInterval)
	defer pollTicker.Stop()
	hbTicker := time.NewTicker(r.Config.HeartbeatInterval)
	defer hbTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.Logger.Info("axonclaw stopping", "reason", ctx.Err())
			return ctx.Err()

		case <-hbTicker.C:
			_, err := api.HeartbeatAgentInstance(ctx, r.Client, &api.HeartbeatAgentInstanceInput{
				InstanceID: r.InstanceID,
			})
			if err != nil {
				r.Logger.Warn("heartbeat failed", "error", err)
				continue
			}

		case <-pollTicker.C:
			limit := 50
			resp, err := api.PullAgentMessages(ctx, r.Client, &api.PullAgentMessagesInput{
				InstanceID: r.InstanceID,
				ThreadID:   r.ThreadID,
				Limit:      &limit,
			})
			if err != nil {
				r.Logger.Warn("pullAgentMessages failed", "error", err)
				continue
			}

			msgs := resp.PullAgentMessages
			if len(msgs) == 0 {
				continue
			}

			for _, msg := range msgs {
				if msg.Text == "" {
					r.Logger.Debug("skip empty message", "message_id", msg.Id, "sequence", msg.Sequence)
					continue
				}

				reqCtx := axoncontext.WithThreadID(ctx, r.ThreadID)
				reqCtx = axoncontext.WithTraceID(reqCtx, uuid.New().String())
				response, err := r.processAndCollect(reqCtx, msg.Text)
				if err != nil {
					r.Logger.Warn("agent process failed", "error", err, "message_id", msg.Id, "sequence", msg.Sequence)
					continue
				}
				if strings.TrimSpace(response) == "" {
					response = "(no response)"
				}

				if _, err := api.PushAgentMessage(ctx, r.Client, &api.PushAgentMessageInput{
					InstanceID: r.InstanceID,
					ThreadID:   r.ThreadID,
					Text:       response,
				}); err != nil {
					r.Logger.Warn("pushAgentMessage failed", "error", err, "message_id", msg.Id, "sequence", msg.Sequence)
					continue
				}

				if _, err := api.AckAgentMessages(ctx, r.Client, &api.AckAgentMessagesInput{
					InstanceID: r.InstanceID,
					MessageIDs: []string{msg.Id},
				}); err != nil {
					r.Logger.Warn("ackAgentMessages failed", "error", err, "message_id", msg.Id, "sequence", msg.Sequence)
				}
			}
		}
	}
}

func (r *Runner) processAndCollect(ctx context.Context, text string) (string, error) {
	before := r.Agent.Messages()
	beforeLen := len(before)

	t := text
	if err := r.Agent.Process(ctx, agent.Content{Text: &t}); err != nil {
		return "", err
	}

	after := r.Agent.Messages()
	if len(after) <= beforeLen {
		return "", nil
	}

	last := ""
	for i := beforeLen; i < len(after); i++ {
		if after[i].Role == agent.RoleAssistant && after[i].Content != nil {
			last = after[i].Content.String()
		}
	}
	return last, nil
}

func RegisterToolsFromBootstrap(
	a *agent.Agent,
	threadWorkspace string,
	workspaceRoot string,
	bootstrapResult *bootstrap.Result,
	logger *slog.Logger,
) {
	enabledBuiltin := map[string]bool{}
	for _, t := range bootstrapResult.BuiltinTools {
		if t.Name == "" {
			continue
		}
		if t.Enabled {
			enabledBuiltin[t.Name] = true
		}
	}

	if len(enabledBuiltin) == 0 {
		for _, name := range []string{"Read", "Write", "Edit", "Bash", "Grep", "Glob", "Skill"} {
			enabledBuiltin[name] = true
		}
	}

	if enabledBuiltin["Read"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewReadTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Write"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewWriteTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Edit"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewEditTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Bash"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewBashTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Grep"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewGrepTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Glob"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewGlobTool(threadWorkspace, true)))
	}
	if enabledBuiltin["Skill"] {
		a.RegisterTool(tools.NewAgentTool(tools.NewSkillTool(filepath.Join(workspaceRoot, "skills"), filepath.Join(workspaceRoot, "skills"))))
	}

	known := map[string]struct{}{}
	for name := range enabledBuiltin {
		known[name] = struct{}{}
	}
	for _, t := range bootstrapResult.Tools {
		if t.Name == "" {
			continue
		}
		if _, ok := known[t.Name]; ok {
			continue
		}

		def, err := convertRemoteToolDefinition(t)
		if err != nil {
			logger.Warn("skip invalid tool schema from bootstrap", "tool", t.Name, "error", err)
			continue
		}
		a.RegisterTool(&unimplementedTool{def: def})
	}
}

type unimplementedTool struct {
	def agent.ToolDefinition
}

func (t *unimplementedTool) Definition() agent.ToolDefinition { return t.def }

func (t *unimplementedTool) Execute(_ context.Context, _ json.RawMessage) agent.ToolResult {
	return agent.ToolResult{Error: fmt.Errorf("tool %q is not implemented in axonclaw", t.def.Name)}
}

func convertRemoteToolDefinition(in *api.AgentBootstrapAgentBootstrapToolsAgentToolDefinition) (agent.ToolDefinition, error) {
	var schema jsonschema.Schema
	if len(in.Parameters) > 0 && string(in.Parameters) != "null" {
		if err := json.Unmarshal(in.Parameters, &schema); err != nil {
			return agent.ToolDefinition{}, err
		}
	}

	if schema.Type == "" {
		schema = jsonschema.Schema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{},
		}
	}

	return agent.ToolDefinition{
		Name:        in.Name,
		Description: in.Description,
		Parameters:  schema,
	}, nil
}
