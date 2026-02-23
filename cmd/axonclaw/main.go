package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

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
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
	"github.com/looplj/axonhub/cmd/axonclaw/runner"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logsDirName    = "logs"
	threadsDirName = "threads"
)

type loggerCloser func()

func main() {
	var (
		baseURL = flag.String("base-url", "", "AxonHub base URL")
		apiKey  = flag.String("api-key", "", "Agent API key")
		debug   = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	cfg, err := conf.LoadOrSaveConfig(*baseURL, *apiKey)
	if err != nil {
		fatalf("%v", err)
	}
	wd := mustGetwd()

	logger, closeLogger := mustInitLogger(wd, *debug)
	defer closeLogger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gqlClient := api.NewClient(cfg.BaseURL, cfg.APIKey)

	boot, err := bootstrap.Do(ctx, gqlClient, bootstrap.SystemPromptData{
		Workspace:  wd,
		InstanceID: cfg.InstanceID,
	})
	if err != nil {
		fatalf("%v", err)
	}

	logger.Info("axonclaw starting",
		"agent_id", boot.AgentID,
		"agent_name", boot.AgentName,
		"instance_id", cfg.InstanceID,
		"base_url", cfg.BaseURL,
		"model", boot.Model,
		"workspace", wd,
	)

	provider := anthropic.New(strings.TrimRight(cfg.BaseURL, "/")+"/anthropic", cfg.APIKey)

	name := "axonclaw"
	platform := runtime.GOOS
	version := "dev"

	if _, err := api.RegisterAgentInstance(ctx, gqlClient, &api.RegisterAgentInstanceInput{
		InstanceID: cfg.InstanceID,
		ThreadID:   &boot.ThreadID,
		Name:       &name,
		Platform:   &platform,
		Version:    &version,
	}); err != nil {
		fatalf("register instance failed: %v", err)
	}

	axonclawDir := filepath.Join(wd, ".axonclaw")
	threadsDir := filepath.Join(axonclawDir, threadsDirName)
	if err := os.MkdirAll(threadsDir, 0o755); err != nil {
		fatalf("cannot create threads directory: %v", err)
	}

	threadStore, err := thread.NewJSONLStore(threadsDir)
	if err != nil {
		fatalf("failed to initialize thread store: %v", err)
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
		fatalf("load workspace grants: %v", err)
	}

	pdoc, err := conf.LoadOrCreatePolicy(wd)
	if err != nil {
		fatalf("load policy: %v", err)
	}
	eng, err := policy.New(pdoc)
	if err != nil {
		fatalf("build policy engine: %v", err)
	}

	remoteApprover := approval.NewRemoteApprover(logger, gqlClient, cfg.InstanceID, cfg.PollInterval)
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
			os.Exit(1)
		}
	}
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
