package claw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/looplj/axonhub/axon/mcp"
	"github.com/looplj/axonhub/axon/permission"
	"github.com/looplj/axonhub/axon/subagent"
	"github.com/looplj/axonhub/axon/task"
	"github.com/looplj/axonhub/axon/tools"

	axoncontext "github.com/looplj/axonhub/axon/context"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

const defaultMaxIterations = 30

type Runner struct {
	Client        graphql.Client
	Agent         *agent.Agent
	Provider      agent.Provider
	Logger        *slog.Logger
	Workspace     string
	Config        Config
	ThreadID      string
	Boot          *bootstrap.Bootstrap
	lastSequence  int
	TaskScheduler *task.Scheduler
	TaskStore     *task.Store
	processMu     sync.Mutex
	processing    atomic.Bool
	processCancel context.CancelFunc
	mcpManager    *mcp.Manager
	toolSource    subagent.ToolSource
	slashCommands *SlashCommandRegistry
	subagentMgr   *subagent.Manager
	skillMgr      *tools.SkillManager
	bus           bus.EventBus
}

type NewOptions struct {
	Logger         *slog.Logger
	Client         graphql.Client
	Provider       agent.Provider
	ContextManager agent.ContextManager
	Config         Config
	Workspace      string
	Boot           *bootstrap.Bootstrap
	PermEvaluator  *permission.Evaluator
	Bus            bus.EventBus
	TaskScheduler  *task.Scheduler
	TaskStore      *task.Store
	SubagentMgr    *subagent.Manager
	SkillMgr       *tools.SkillManager
}

func New(opts NewOptions) *Runner {
	permMw := NewPermissionMiddleware(opts.PermEvaluator)

	env := buildPromptEnv(opts.Boot, opts.Workspace)

	systemPrompts := prompts.BuildSystemPrompts(env, opts.Boot.Prompts)

	a := agent.New(agent.Config{
		Model:         opts.Boot.Model,
		MaxIterations: defaultMaxIterations,
		SystemPrompts: systemPrompts,
	}, opts.Provider,
		agent.WithBus(opts.Bus),
		agent.WithContextManager(opts.ContextManager),
		agent.WithMiddlewares(permMw),
	)

	mcpMgr := mcp.NewManager(mcp.ManagerOptions{
		Logger:    opts.Logger,
		ConfigDir: opts.Boot.RuntimeDir,
	})

	subagentMgr := opts.SubagentMgr
	skillMgr := opts.SkillMgr

	toolSource := &agentToolSource{agent: a}
	registerTools(a, opts.Workspace, opts.Boot, opts.Logger, opts.Client, opts.Provider, opts.Bus, mcpMgr, subagentMgr, skillMgr)

	r := &Runner{
		Client:        opts.Client,
		Agent:         a,
		Provider:      opts.Provider,
		Logger:        opts.Logger,
		Workspace:     opts.Workspace,
		Config:        opts.Config,
		ThreadID:      opts.Boot.ThreadID,
		Boot:          opts.Boot,
		TaskScheduler: opts.TaskScheduler,
		TaskStore:     opts.TaskStore,
		mcpManager:    mcpMgr,
		toolSource:    toolSource,
		slashCommands: NewDefaultSlashCommands(opts.Client),
		subagentMgr:   subagentMgr,
		skillMgr:      skillMgr,
		bus:           opts.Bus,
	}

	return r
}

func buildPromptEnv(boot *bootstrap.Bootstrap, workspace string) prompts.PromptEnv {
	return prompts.PromptEnv{
		Date:         boot.Date,
		Timezone:     boot.Timezone,
		OS:           boot.OS,
		Workspace:    workspace,
		ThreadID:     boot.ThreadID,
		AxonClawPath: boot.AxonClawPath,
		SkillsRoot:   boot.SkillsRoot,
		AgentID:      boot.AgentID,
		AgentName:    boot.AgentName,
	}
}

type IncomingMessage struct {
	Id                string
	Text              string
	Type              api.AgentMessageType
	CorrelationID     string
	Content           json.RawMessage
	ExternalMessageID *string
	Sequence          int
}

func (r *Runner) Run(ctx context.Context) error {
	ctx = axoncontext.WithThreadID(ctx, r.ThreadID)
	ctx = axoncontext.WithWorkspace(ctx, r.Workspace)

	msgCh := make(chan IncomingMessage, 64)

	go r.pollMessages(ctx, msgCh)

	hbTicker := time.NewTicker(r.Config.HeartbeatInterval)
	defer hbTicker.Stop()

	var autoUpdateTicker *time.Ticker
	var autoUpdateCh <-chan time.Time
	if r.Config.AutoSyncConfig {
		autoUpdateTicker = time.NewTicker(r.Config.AutoSyncConfigInterval)
		autoUpdateCh = autoUpdateTicker.C
		defer autoUpdateTicker.Stop()
		r.Logger.Info("auto-sync-config enabled", "interval", r.Config.AutoSyncConfigInterval)
	}

	for {
		select {
		case <-ctx.Done():
			r.Logger.Info("axonclaw stopping", "reason", ctx.Err())
			return ctx.Err()

		case <-autoUpdateCh:
			r.autoUpdateConfig(ctx)

		case <-hbTicker.C:
			_, err := api.HeartbeatAgentInstance(ctx, r.Client, &api.HeartbeatAgentInstanceInput{})
			if err != nil {
				r.Logger.Warn("heartbeat failed", "error", err)
				continue
			}

		case msg := <-msgCh:
			// Slash commands are handled immediately, regardless of
			// whether the agent is currently processing.
			if cmd, args, ok := r.slashCommands.Match(msg.Text); ok {
				result, err := cmd.Execute(ctx, r, args)
				if err != nil {
					r.Logger.Warn("slash command failed", "command", cmd.Name, "error", err)
					result = fmt.Sprintf("Error executing %s: %v", cmd.Name, err)
				}

				r.sendSlashCommandResult(ctx, result, msg.Id)

				continue
			}

			if r.processing.Load() {
				r.Logger.Info("agent busy, delivering as steering", "text_len", len(msg.Text))
				t := r.formatMessageForLLM(msg)
				r.Agent.Steer(agent.Message{
					Role:    agent.RoleUser,
					Content: &agent.Content{Text: &t},
				})
			} else {
				go func() {
					if err := r.processMessage(ctx, msg); err != nil {
						r.Logger.Warn("agent process failed", "error", err)
					}
				}()
			}
		}
	}
}

func (r *Runner) pollMessages(ctx context.Context, out chan<- IncomingMessage) {
	ticker := time.NewTicker(r.Config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			limit := 50
			afterSeq := r.lastSequence
			typeIn := []api.AgentMessageType{api.AgentMessageTypeChat}
			resp, err := api.PullAgentMessages(ctx, r.Client, &api.PullAgentMessagesInput{
				AfterSequence: &afterSeq,
				Limit:         &limit,
				TypeIn:        typeIn,
			})
			if err != nil {
				r.Logger.Warn("pullAgentMessages failed", "error", err)
				continue
			}

			msgs := resp.PullAgentMessages
			if len(msgs) == 0 {
				continue
			}

			var ackedIDs []string
			for _, msg := range msgs {
				if msg.Sequence > r.lastSequence {
					r.lastSequence = msg.Sequence
				}

				if msg.Text == "" {
					r.Logger.Debug("skip empty message", "message_id", msg.Id, "sequence", msg.Sequence)
					ackedIDs = append(ackedIDs, msg.Id)
					continue
				}

				select {
				case out <- IncomingMessage{
					Id:                msg.Id,
					Text:              msg.Text,
					Type:              msg.Type,
					CorrelationID:     msg.CorrelationID,
					Content:           msg.Content,
					ExternalMessageID: msg.ExternalMessageID,
					Sequence:          msg.Sequence,
				}:
					ackedIDs = append(ackedIDs, msg.Id)
				case <-ctx.Done():
					return
				}
			}

			if len(ackedIDs) > 0 {
				if _, err := api.AckAgentMessages(ctx, r.Client, &api.AckAgentMessagesInput{
					MessageIDs: ackedIDs,
				}); err != nil {
					r.Logger.Warn("ackAgentMessages failed", "error", err, "count", len(ackedIDs))
				}
			}
		}
	}
}

func (r *Runner) formatMessageForLLM(msg IncomingMessage) string {
	if msg.ExternalMessageID != nil && *msg.ExternalMessageID != "" {
		payload := map[string]any{
			"content":    msg.Text,
			"message_id": msg.Id,
			"reply_instruction": map[string]any{
				"tool":                "SendMessage",
				"target":              "user",
				"reply_to_message_id": msg.Id,
			},
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			r.Logger.Warn("failed to marshal message payload for LLM", "error", err, "message_id", msg.Id)
			return msg.Text
		}

		return string(raw)
	}

	return msg.Text
}

func (r *Runner) processMessage(ctx context.Context, msg IncomingMessage) error {
	r.processMu.Lock()
	defer r.processMu.Unlock()

	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	r.processCancel = cancel
	r.processing.Store(true)

	defer func() {
		r.processing.Store(false)
		r.processCancel = nil
	}()

	traceID := uuid.New().String()
	processCtx = axoncontext.WithThreadID(processCtx, r.ThreadID)
	processCtx = axoncontext.WithTraceID(processCtx, traceID)
	formatted := r.formatMessageForLLM(msg)
	_, err := r.Agent.Process(processCtx, agent.Content{Text: &formatted})
	if err != nil {
		var providerErr *agent.ProviderError
		if errors.As(err, &providerErr) {
			r.Logger.Warn("provider error",
				"status", providerErr.StatusCode, "error", providerErr.Message)
			r.replyError(ctx, fmt.Sprintf("⚠️ Provider error (HTTP %d): %s", providerErr.StatusCode, providerErr.Message))

			return nil
		}

		r.Logger.Warn("agent process failed", "error", err)
		r.replyError(ctx, fmt.Sprintf("⚠️ Agent error: %v", err))

		return nil
	}

	return nil
}

// stopProcessing cancels the active agent processing if running.
// Returns true if processing was active and has been canceled.
func (r *Runner) stopProcessing() bool {
	if !r.processing.Load() {
		return false
	}

	r.Logger.Info("stopping current processing")

	if r.processCancel != nil {
		r.processCancel()
	}

	return true
}

func (r *Runner) autoUpdateConfig(ctx context.Context) {
	r.processMu.Lock()
	defer r.processMu.Unlock()

	newBoot, err := bootstrap.Do(ctx, r.Client, bootstrap.Params{
		Workspace:  r.Workspace,
		SkillsRoot: r.Boot.SkillsRoot,
		PromptDir:  r.Boot.PromptDir,
		RuntimeDir: r.Boot.RuntimeDir,
	})
	if err != nil {
		r.Logger.Warn("auto-update config failed", "error", err)
		return
	}

	r.Boot.AgentID = newBoot.AgentID
	r.Boot.AgentName = newBoot.AgentName
	r.Boot.Model = newBoot.Model
	r.Boot.Tools = newBoot.Tools
	r.Boot.Skills = newBoot.Skills
	r.Boot.BuiltinTools = newBoot.BuiltinTools
	r.Boot.Prompts = newBoot.Prompts
	r.Boot.AxonClawPath = newBoot.AxonClawPath
	r.Boot.Date = newBoot.Date
	r.Boot.Timezone = newBoot.Timezone
	r.Boot.OS = newBoot.OS

	r.ReloadSystemPrompts()

	r.Logger.Info("auto-update config completed", "agent_name", newBoot.AgentName, "model", newBoot.Model)
}

func (r *Runner) ReloadSystemPrompts() {
	env := buildPromptEnv(r.Boot, r.Workspace)
	systemPrompts := prompts.BuildSystemPrompts(env, r.Boot.Prompts)
	r.Agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.Model = r.Boot.Model
		cfg.SystemPrompts = systemPrompts
		return cfg
	})
}

func (r *Runner) FollowUP(ctx context.Context, text string) {
	r.Agent.FollowUp(agent.Message{
		Role:    agent.RoleUser,
		Content: &agent.Content{Text: &text},
	})
}

func (r *Runner) ProcessIsolated(ctx context.Context, text string, systemPrompts []string) (*agent.Result, error) {
	cfg := r.Agent.Config()

	ctx = newIsolatedContext(ctx)

	// Each isolated run gets its own context manager to ensure complete
	// separation from the main agent's message history. The parent bus is
	// shared so that events (e.g. archive writing) are still propagated.
	cm := agent.NewSimpleContextManager(nil)

	return subagent.Run(ctx, subagent.Config{
		Model:          cfg.Model,
		SystemPrompts:  systemPrompts,
		Provider:       r.Provider,
		ContextManager: cm,
		Bus:            r.bus,
		Middlewares:    r.Agent.Middlewares(),
		Logger:         r.Logger.With("component", "isolated_prompt"),
	}, text, r.toolSource)
}

func newIsolatedContext(ctx context.Context) context.Context {
	threadID := fmt.Sprintf("th-%s", uuid.New().String())
	traceID := uuid.New().String()
	ctx = axoncontext.WithThreadID(ctx, threadID)
	ctx = axoncontext.WithTraceID(ctx, traceID)

	return ctx
}

func (r *Runner) replyError(ctx context.Context, text string) {
	_, err := api.ReplyMessage(ctx, r.Client, &api.ReplyMessageInput{
		Text: text,
	})
	if err != nil {
		r.Logger.Warn("failed to send error reply", "error", err)
	}
}

func (r *Runner) Close() error {
	if r.mcpManager == nil {
		return nil
	}

	return r.mcpManager.Close()
}
