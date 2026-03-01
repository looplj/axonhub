package runner

import (
	"context"
	"log/slog"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/bus"
	axoncontext "github.com/looplj/axonhub/axon/context"
	"github.com/looplj/axonhub/axon/permission"
	"github.com/looplj/axonhub/axon/thread"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
)

const defaultMaxIterations = 30

type Runner struct {
	Client       graphql.Client
	Agent        *agent.Agent
	Logger       *slog.Logger
	Workspace    string
	Config       conf.Config
	ThreadID     string
	ThreadMgr    *thread.Manager
	lastSequence int
}

type NewOptions struct {
	Logger        *slog.Logger
	Client        graphql.Client
	Provider      agent.Provider
	Config        conf.Config
	Workspace     string
	Boot          *bootstrap.Result
	ThreadMgr     *thread.Manager
	PermEvaluator *permission.Evaluator
	Bus           bus.EventBus
}

func New(opts NewOptions) *Runner {
	permMw := NewPermissionMiddleware(opts.PermEvaluator)

	localPrompt := buildLocalSystemPrompt(PromptEnv{
		Date:         opts.Boot.Date,
		Timezone:     opts.Boot.Timezone,
		OS:           opts.Boot.OS,
		Workspace:    opts.Workspace,
		ThreadID:     opts.Boot.ThreadID,
		AxonClawPath: opts.Boot.AxonClawPath,
		SkillsRoot:   opts.Boot.SkillsRoot,
		ConfigDir:    opts.Boot.ConfigDir,
	})

	a := agent.New(agent.Config{
		Model:         opts.Boot.Model,
		MaxIterations: defaultMaxIterations,
		SystemPrompts: []string{opts.Boot.SystemPrompt, localPrompt},
	}, opts.Provider,
		agent.WithBus(opts.Bus),
		agent.WithMiddlewares(permMw),
	)

	registerTools(a, opts.Workspace, opts.Boot, opts.Logger, opts.Client, opts.ThreadMgr, opts.Boot.ThreadID)

	return &Runner{
		Client:    opts.Client,
		Agent:     a,
		Logger:    opts.Logger,
		Workspace: opts.Workspace,
		Config:    opts.Config,
		ThreadID:  opts.Boot.ThreadID,
		ThreadMgr: opts.ThreadMgr,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	ctx = axoncontext.WithThreadID(ctx, r.ThreadID)
	ctx = agent.WithWorkspace(ctx, r.Workspace)

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
			_, err := api.HeartbeatAgentInstance(ctx, r.Client, &api.HeartbeatAgentInstanceInput{})
			if err != nil {
				r.Logger.Warn("heartbeat failed", "error", err)
				continue
			}

		case <-pollTicker.C:
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

				if err := r.processMessage(ctx, msg.Text); err != nil {
					r.Logger.Warn("agent process failed", "error", err, "message_id", msg.Id, "sequence", msg.Sequence)
					continue
				}

				ackedIDs = append(ackedIDs, msg.Id)
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

func (r *Runner) processMessage(ctx context.Context, text string) error {
	traceID := uuid.New().String()
	// 显式设置 ThreadID 和 TraceID，确保 provider 调用时能正确传递到 HTTP Header
	ctx = axoncontext.WithThreadID(ctx, r.ThreadID)
	ctx = axoncontext.WithTraceID(ctx, traceID)
	return r.Agent.Process(ctx, agent.Content{Text: &text})
}
