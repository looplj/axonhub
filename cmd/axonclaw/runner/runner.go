package runner

import (
	"context"
	"encoding/json"
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
	"github.com/looplj/axonhub/axon/task"

	axoncontext "github.com/looplj/axonhub/axon/context"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

const defaultMaxIterations = 30

type Runner struct {
	Client        graphql.Client
	Agent         *agent.Agent
	Logger        *slog.Logger
	Workspace     string
	Config        conf.Config
	ThreadID      string
	Boot          *bootstrap.Result
	lastSequence  int
	TaskScheduler *task.Scheduler
	processMu     sync.Mutex
	processing    atomic.Bool
	mcpManager    *mcp.Manager
}

type NewOptions struct {
	Logger         *slog.Logger
	Client         graphql.Client
	Provider       agent.Provider
	ContextManager agent.ContextManager
	Config         conf.Config
	Workspace      string
	Boot           *bootstrap.Result
	PermEvaluator  *permission.Evaluator
	Bus            bus.EventBus
	TaskScheduler  *task.Scheduler
}

func New(opts NewOptions) *Runner {
	permMw := NewPermissionMiddleware(opts.PermEvaluator)

	env := buildPromptEnv(opts.Boot, opts.Workspace)
	serverPrompt := buildServerSystemPrompt(opts.Boot.SystemPrompt, env)
	serverPrompt = appendSkillsToPrompt(serverPrompt, opts.Boot.Skills)
	localPrompt := buildLocalSystemPrompt(env)

	a := agent.New(agent.Config{
		Model:         opts.Boot.Model,
		MaxIterations: defaultMaxIterations,
		SystemPrompts: []string{serverPrompt, localPrompt},
	}, opts.Provider,
		agent.WithBus(opts.Bus),
		agent.WithContextManager(opts.ContextManager),
		agent.WithMiddlewares(permMw),
	)

	mcpMgr := registerTools(a, opts.Workspace, opts.Boot, opts.Logger, opts.Client)

	return &Runner{
		Client:        opts.Client,
		Agent:         a,
		Logger:        opts.Logger,
		Workspace:     opts.Workspace,
		Config:        opts.Config,
		ThreadID:      opts.Boot.ThreadID,
		Boot:          opts.Boot,
		TaskScheduler: opts.TaskScheduler,
		mcpManager:    mcpMgr,
	}
}

func buildPromptEnv(boot *bootstrap.Result, workspace string) PromptEnv {
	return PromptEnv{
		Date:         boot.Date,
		Timezone:     boot.Timezone,
		OS:           boot.OS,
		Workspace:    workspace,
		ThreadID:     boot.ThreadID,
		AxonClawPath: boot.AxonClawPath,
		SkillsRoot:   boot.SkillsRoot,
		ConfigDir:    boot.ConfigDir,
		AgentID:      boot.AgentID,
		AgentName:    boot.AgentName,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ctx = axoncontext.WithThreadID(ctx, r.ThreadID)
	ctx = axoncontext.WithWorkspace(ctx, r.Workspace)

	msgCh := make(chan string, 64)

	// Separate goroutine for polling messages so that new messages can still
	// be received while the agent is processing (enabling steering).
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

		case text := <-msgCh:
			if r.processing.Load() {
				// Agent is busy — deliver as a steering message so the
				// current tool-call loop can be interrupted.
				r.Logger.Info("agent busy, delivering as steering", "text_len", len(text))
				t := text
				r.Agent.Steer(agent.Message{
					Role:    agent.RoleUser,
					Content: &agent.Content{Text: &t},
				})
			} else {
				// Agent is idle — start a new processing run in background
				// so the main loop remains responsive.
				t := text

				go func() {
					if err := r.processMessage(ctx, t); err != nil {
						r.Logger.Warn("agent process failed", "error", err)
					}
				}()
			}
		}
	}
}

// pollMessages continuously pulls chat messages from the server and sends
// their text to out. It runs until ctx is canceled.
func (r *Runner) pollMessages(ctx context.Context, out chan<- string) {
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
				case out <- r.formatMessageForLLM(msg):
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

func (r *Runner) formatMessageForLLM(msg *api.PullAgentMessagesPullAgentMessagesAgentMessage) string {
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

func (r *Runner) processMessage(ctx context.Context, text string) error {
	r.processMu.Lock()
	defer r.processMu.Unlock()

	r.processing.Store(true)
	defer r.processing.Store(false)

	traceID := uuid.New().String()
	ctx = axoncontext.WithThreadID(ctx, r.ThreadID)
	ctx = axoncontext.WithTraceID(ctx, traceID)
	return r.Agent.Process(ctx, agent.Content{Text: &text})
}

func (r *Runner) autoUpdateConfig(ctx context.Context) {
	r.processMu.Lock()
	defer r.processMu.Unlock()

	newBoot, err := bootstrap.Do(ctx, r.Client, bootstrap.Params{
		Workspace:  r.Workspace,
		SkillsRoot: r.Boot.SkillsRoot,
		ConfigDir:  r.Boot.ConfigDir,
	})
	if err != nil {
		r.Logger.Warn("auto-update config failed", "error", err)
		return
	}

	r.Boot.AgentID = newBoot.AgentID
	r.Boot.AgentName = newBoot.AgentName
	r.Boot.Model = newBoot.Model
	r.Boot.SystemPrompt = newBoot.SystemPrompt
	r.Boot.Tools = newBoot.Tools
	r.Boot.Skills = newBoot.Skills
	r.Boot.BuiltinTools = newBoot.BuiltinTools
	r.Boot.AxonClawPath = newBoot.AxonClawPath
	r.Boot.Date = newBoot.Date
	r.Boot.Timezone = newBoot.Timezone
	r.Boot.OS = newBoot.OS

	env := buildPromptEnv(newBoot, r.Workspace)
	serverPrompt := buildServerSystemPrompt(newBoot.SystemPrompt, env)
	serverPrompt = appendSkillsToPrompt(serverPrompt, newBoot.Skills)
	localPrompt := buildLocalSystemPrompt(env)

	r.Agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.Model = newBoot.Model
		cfg.SystemPrompts = []string{serverPrompt, localPrompt}
		return cfg
	})

	r.Logger.Info("auto-update config completed", "agent_name", newBoot.AgentName, "model", newBoot.Model)
}

func (r *Runner) ProcessScheduledMessage(ctx context.Context, text string) error {
	return r.processMessage(ctx, text)
}

func (r *Runner) SetTaskScheduler(s *task.Scheduler) {
	r.TaskScheduler = s
}

func (r *Runner) Close() error {
	if r.mcpManager == nil {
		return nil
	}

	return r.mcpManager.Close()
}
