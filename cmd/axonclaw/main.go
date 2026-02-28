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

	"github.com/Khan/genqlient/graphql"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/api"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/looplj/axonhub/axon/permission"
	"github.com/looplj/axonhub/axon/permission/approval"
	"github.com/looplj/axonhub/axon/permission/grant"
	"github.com/looplj/axonhub/axon/permission/policy"
	"github.com/looplj/axonhub/axon/provider/anthropic"
	"github.com/looplj/axonhub/axon/thread"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/cmds"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
	"github.com/looplj/axonhub/cmd/axonclaw/runner"
	"github.com/looplj/skills/skillscmd"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logsDirName    = "logs"
	threadsDirName = "threads"
	configDirName  = ".axonclaw"
)

type loggerCloser func()

func main() {
	workspaceDir := mustGetwd()
	configDir := filepath.Join(workspaceDir, configDirName)

	rootCmd := newRootCommand(newRootCommandOptions{
		WorkspaceDir: workspaceDir,
		ConfigDir:    configDir,
		RunAgent:     runAgent,
	})

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type newRootCommandOptions struct {
	WorkspaceDir string
	ConfigDir    string
	RunAgent     func(cfg conf.Config, wd string, debug bool) error
}

func newRootCommand(opts newRootCommandOptions) *cobra.Command {
	var (
		baseURL string
		apiKey  string
		name    string
		debug   bool
	)

	rootCmd := &cobra.Command{
		Use:          "axonclaw",
		Short:        "AxonClaw - AxonHub managed agent",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := conf.LoadOrSaveConfig(baseURL, apiKey, name)
			if err != nil {
				return err
			}
			wd := mustGetwd()

			// CLI flag overrides config file, but config can still enable debug by default.
			runDebug := debug || cfg.Debug
			return opts.RunAgent(cfg, wd, runDebug)
		},
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.Flags().StringVar(&baseURL, "base-url", "", "AxonHub base URL")
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "Agent API key")
	rootCmd.Flags().StringVar(&name, "name", "", "Agent instance name")
	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	rootCmd.SetHelpCommand(cmds.NewHelpCommand(rootCmd))

	workspaceDir := opts.WorkspaceDir
	configDir := opts.ConfigDir

	rootCmd.AddCommand(skillscmd.NewRootCommand(skillscmd.RootOptions{
		Use:                  "skills",
		Stdout:               os.Stdout,
		Stderr:               os.Stderr,
		WorkspaceDir:         filepath.Join(workspaceDir, "skills"),
		GlobalDir:            filepath.Join(configDir, "skills"),
		Commands:             []string{"search", "list", "add", "remove"},
		EnableAgentDiscovery: false,
		EnableAgentFlags:     false,
	}))
	rootCmd.AddCommand(cmds.NewConfCommand(cmds.ConfOptions{
		Dir:    configDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewMemoryCommand(cmds.MemoryOptions{
		Dir:    configDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewDiscoverCommand(cmds.DiscoverOptions{
		ConfigDir: configDir,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
	}))

	return rootCmd
}

func runAgent(cfg conf.Config, wd string, debug bool) error {
	logger, closeLogger := mustInitLogger(wd, debug)
	defer closeLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gqlClient := api.NewClient(cfg.BaseURL, cfg.APIKey)

	boot, err := bootstrap.Do(ctx, gqlClient, bootstrap.SystemPromptData{
		Workspace: wd,
	})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	logger.Info("axonclaw starting",
		"agent_id", boot.AgentID,
		"agent_name", boot.AgentName,
		"base_url", cfg.BaseURL,
		"model", boot.Model,
		"workspace", wd,
	)

	provider := anthropic.New(strings.TrimRight(cfg.BaseURL, "/")+"/anthropic", cfg.APIKey)

	instanceName := "axonclaw"
	if cfg.Name != "" {
		instanceName = cfg.Name
	}
	platform := runtime.GOOS

	if _, err := api.RegisterAgentInstance(ctx, gqlClient, &api.RegisterAgentInstanceInput{
		ThreadID: &boot.ThreadID,
		Name:     &instanceName,
		Platform: &platform,
	}); err != nil {
		return fmt.Errorf("register instance: %w", err)
	}

	axonclawDir := filepath.Join(wd, ".axonclaw")
	threadsDir := filepath.Join(axonclawDir, threadsDirName)
	if err := os.MkdirAll(threadsDir, 0o755); err != nil {
		return fmt.Errorf("create threads directory: %w", err)
	}

	threadStore, err := thread.NewJSONLStore(threadsDir)
	if err != nil {
		return fmt.Errorf("initialize thread store: %w", err)
	}
	threadMgr := thread.NewManager(threadStore)

	eventBus := bus.New(
		bus.WithRecover(logger),
		bus.WithTracing(),
	)
	defer eventBus.Close()

	eventBus.Subscribe(agent.TopicAgentEvent, bus.TypedHandler(func(_ context.Context, _ bus.Event, ev agent.AgentEvent) error {
		switch ev.Type {
		case agent.EventMessageAdded:
			if ev.Message != nil {
				threadMgr.AddMessage(boot.ThreadID, *ev.Message)
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

	grantsStore := grant.NewMemoryStore(grant.NewLocalFileStore(wd))
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

	r := runner.New(runner.NewOptions{
		Logger:        logger,
		Client:        gqlClient,
		Provider:      provider,
		Config:        cfg,
		Workspace:     wd,
		Boot:          boot,
		ThreadMgr:     threadMgr,
		PermEvaluator: permEvaluator,
		Bus:           eventBus,
	})
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
	logsDir := filepath.Join(wd, ".axonclaw", logsDirName)
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
