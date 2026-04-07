package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/looplj/axonhub/axon/permission"
	"github.com/looplj/axonhub/axon/permission/approval"
	"github.com/looplj/axonhub/axon/permission/grant"
	"github.com/looplj/axonhub/axon/permission/policy"
	"github.com/looplj/axonhub/axon/provider/anthropic"
	"github.com/looplj/axonhub/axon/provider/retry"
	"github.com/looplj/axonhub/axon/subagent"
	"github.com/looplj/axonhub/axon/task"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/build"
	"github.com/looplj/axonhub/cmd/axonclaw/claw"
	"github.com/looplj/axonhub/cmd/axonclaw/cmds"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
	"github.com/looplj/axonhub/cmd/axonclaw/skills"
)

const logsDirName = "logs"

type loggerCloser func()

func main() {
	workspaceDir := mustGetwd()

	rootCmd := newRootCommand(newRootCommandOptions{
		WorkspaceDir: workspaceDir,
		RunAgent:     runAgent,
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type newRootCommandOptions struct {
	WorkspaceDir string
	RunAgent     func(cfg claw.Config, wd string, debug bool) error
}

func newRootCommand(opts newRootCommandOptions) *cobra.Command {
	var (
		autoSyncConfig bool
		debug          bool
	)

	rootCmd := &cobra.Command{
		Use:     "axonclaw",
		Short:   "AxonClaw - AxonHub managed agent",
		Version: build.GetVersion(),
		Long: fmt.Sprintf(`AxonClaw - AxonHub managed agent

Version:    %s
Build Time: %s
Git Commit: %s`, build.GetVersion(), build.GetBuildTime(), build.GetGitCommit()),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := conf.LoadOrSaveConfig()
			if err != nil {
				return err
			}
			wd := mustGetwd()

			if autoSyncConfig {
				cfg.AutoSyncConfig = true
			}
			runDebug := debug || cfg.Debug
			return opts.RunAgent(cfg, wd, runDebug)
		},
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.Flags().BoolVar(&autoSyncConfig, "auto-sync-config", false, "Automatically sync agent configuration from server")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	rootCmd.SetHelpCommand(cmds.NewHelpCommand(rootCmd))

	workspaceDir := opts.WorkspaceDir
	rootCmd.AddCommand(skills.NewCommand(workspaceDir, func() ([]skills.BuiltinSkillConfig, error) {
		items, err := conf.LoadBuiltinSkills()
		if err != nil {
			return nil, err
		}

		return lo.Map(items, func(item claw.BuiltinSkill, _ int) skills.BuiltinSkillConfig {
			return skills.BuiltinSkillConfig{
				Name:    item.Name,
				Enabled: item.Enabled,
				Order:   item.Order,
			}
		}), nil
	}))
	rootCmd.AddCommand(cmds.NewConfCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewMemoryCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}, workspaceDir))
	rootCmd.AddCommand(cmds.NewDiscoverCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewTaskCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewDeployCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewMCPCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewHeartbeatCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewModelsCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewSelfCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewVersionCommand(cmds.StdioOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))

	return rootCmd
}

//nolint:maintidx // Init agent.
func runAgent(cfg claw.Config, wd string, debug bool) error {
	logger, closeLogger := mustInitLogger(wd, debug)
	defer closeLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtimeDir, err := conf.RuntimeDirForWorkspace(wd)
	if err != nil {
		return fmt.Errorf("resolve runtime directory: %w", err)
	}

	gqlClient := api.NewClient(cfg.BaseURL, cfg.APIKey)

	boot, err := bootstrap.Do(ctx, gqlClient, bootstrap.Params{
		Workspace:  wd,
		SkillsRoot: filepath.Join(wd, conf.DefaultDir, "skills"),
		PromptDir:  filepath.Join(wd, conf.DefaultDir),
		RuntimeDir: runtimeDir,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	if err := conf.SaveBuiltinSkills(boot.BuiltinSkills); err != nil {
		logger.Warn("save builtin skills config failed", "error", err)
	}

	logger.Info("axonclaw starting",
		"agent_id", boot.AgentID,
		"agent_name", boot.AgentName,
		"base_url", cfg.BaseURL,
		"model", boot.Model,
		"workspace", wd,
		"soul", boot.Prompts != nil && !boot.Prompts.Soul.IsEmpty(),
		"identity", boot.Prompts != nil && !boot.Prompts.Identity.IsEmpty(),
		"user", boot.Prompts != nil && !boot.Prompts.User.IsEmpty(),
	)

	provider := retry.New(anthropic.New(strings.TrimRight(cfg.BaseURL, "/")+"/anthropic", cfg.APIKey,
		anthropic.WithReasoningEffort(boot.ReasoningEffort),
	))

	platform := runtime.GOOS

	if _, err := api.RegisterAgentInstance(ctx, gqlClient, &api.RegisterAgentInstanceInput{
		ThreadID: &boot.ThreadID,
		Platform: &platform,
	}); err != nil {
		return fmt.Errorf("register instance: %w", err)
	}

	var contextMgr agent.ContextManager

	contextCfg := claw.DefaultContextManagerConfig()
	contextCfg.Enabled = true
	contextCfg.Logger = logger
	if cfg.ContextTokenLimit > 0 {
		contextCfg.TokenLimit = cfg.ContextTokenLimit
	}

	if cfg.ContextSummaryMaxChars > 0 {
		contextCfg.SummaryMaxChars = cfg.ContextSummaryMaxChars
	}

	subagentMgr := subagent.NewManagerFromPath(filepath.Join(wd, "subagents"))

	if err := subagentMgr.Load(); err != nil {
		logger.Warn("failed to load subagent definitions", "error", err)
	}

	eventBus := bus.New(bus.WithRecover(logger), bus.WithTracing())
	defer eventBus.Close()

	eventBus.Subscribe(agent.TopicAgentEvent, bus.TypedHandler(func(ctx context.Context, _ bus.Event, ev agent.AgentEvent) error {
		switch ev.Type {
		case agent.EventMessageAdded:
			if ev.Message != nil {
				if err := claw.AppendArchiveMessage(ctx, filepath.Join(wd, "messages"), *ev.Message); err != nil {
					logger.Warn("archive append failed", "error", err)
				}
			}
		case agent.EventToolStart:
			logger.Debug("tool started", "tool", ev.ToolName)
		case agent.EventToolEnd:
			if ev.Result != nil && ev.Result.Error != nil {
				logger.Warn("tool failed", "tool", ev.ToolName, "error", ev.Result.Error)
			}
		case agent.EventError:
			logger.Error("agent error", "error", ev.Error)
		}
		return nil
	}))

	skillMgr := claw.NewSkillManager(filepath.Join(wd, conf.DefaultDir, "skills"), boot, logger)

	contextStore := claw.NewContextManagerFileStore(filepath.Join(runtimeDir, "messages"))

	cm, err := claw.NewSmartContextManager(contextCfg, contextStore)
	if err != nil {
		return fmt.Errorf("initialize context manager: %w", err)
	}

	contextMgr = cm

	permissionDir, err := conf.PermissionDirForWorkspace(wd)
	if err != nil {
		return fmt.Errorf("resolve permission directory: %w", err)
	}

	grantsStore := grant.NewMemoryStore(grant.NewFileStore(permissionDir))
	if err := grantsStore.LoadGlobal(); err != nil {
		return fmt.Errorf("load global grants: %w", err)
	}
	if err := grantsStore.LoadWorkspace(wd); err != nil {
		return fmt.Errorf("load workspace grants: %w", err)
	}

	pdoc, err := conf.LoadOrCreatePolicy(wd)
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}
	eng, err := policy.New(pdoc)
	if err != nil {
		return fmt.Errorf("build policy engine: %w", err)
	}

	remoteApprover := approval.NewRemoteApprover(logger, gqlClient, cfg.PollInterval)
	permEvaluator := permission.NewEvaluator(permission.EvaluatorOptions{
		Logger:   logger,
		Policy:   eng,
		Approver: remoteApprover,
		Grants:   grantsStore,
	})

	r := claw.New(claw.NewOptions{
		Logger:         logger,
		Client:         gqlClient,
		Provider:       provider,
		ContextManager: contextMgr,
		Config:         cfg,
		Workspace:      wd,
		Boot:           boot,
		PermEvaluator:  permEvaluator,
		Bus:            eventBus,
		SubagentMgr:    subagentMgr,
		SkillMgr:       skillMgr,
	})

	cm.SetSummarizer(claw.NewForkedCompactSummarizer(claw.ForkedCompactSummarizerOptions{
		Agent:    r.Agent,
		Provider: provider,
		Model:    boot.Model,
		Logger:   logger,
		Bus:      eventBus,
	}))

	cm.OnCompaction(func() {
		r.ReloadSystemPrompts()
	})

	defer func() {
		if err := r.Close(); err != nil {
			logger.Warn("close runner failed", "error", err)
		}
	}()

	taskStore, err := task.NewStore(filepath.Join(runtimeDir, "tasks"))
	if err != nil {
		return fmt.Errorf("init task store: %w", err)
	}

	if err := claw.EnsureDefaultTasks(taskStore); err != nil {
		return fmt.Errorf("ensure system tasks: %w", err)
	}

	r.TaskStore = taskStore

	taskHandler := claw.NewTaskHandler(logger, wd, r)
	taskScheduler, err := task.NewScheduler(logger, taskStore, taskHandler, task.SchedulerOptions{
		TickInterval: time.Minute,
	})
	if err != nil {
		return fmt.Errorf("init task scheduler: %w", err)
	}
	taskScheduler.Start(ctx)
	defer taskScheduler.Stop()

	if err := r.Run(ctx); err != nil {
		if err != context.Canceled {
			logger.Error("runner stopped with error", "error", err)
			return err
		}
	}
	return nil
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		fatalf("getwd: %v", err)
	}
	return dir
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func mustInitLogger(wd string, debug bool) (*slog.Logger, loggerCloser) {
	logsDir := filepath.Join(wd, conf.DefaultDir, logsDirName)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		fatalf("cannot create logs directory: %v", err)
	}

	logFilePath := filepath.Join(logsDir, "axonclaw.log")
	ljLogger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10,
		MaxAge:     7,
		MaxBackups: 3,
		LocalTime:  true,
	}

	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(ljLogger, &slog.HandlerOptions{Level: level}))
	return logger, func() { ljLogger.Close() }
}

var _ graphql.Client = (graphql.Client)(nil)
