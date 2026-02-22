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

	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/provider/anthropic"
	"github.com/looplj/axonhub/cmd/axonclaw/api"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
	"github.com/looplj/axonhub/cmd/axonclaw/conf"
	"github.com/looplj/axonhub/cmd/axonclaw/runner"
)

const defaultMaxIterations = 30

func main() {
	var (
		baseURL = flag.String("base-url", "", "AxonHub base URL")
		apiKey  = flag.String("api-key", "", "Agent API key")
	)
	flag.Parse()

	cfg, err := conf.LoadOrSaveConfig(*baseURL, *apiKey)
	if err != nil {
		fatalf("%v", err)
	}

	var logger *slog.Logger
	if cfg.Debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	ws := mustGetwd()
	threadID := uuid.New().String()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gqlClient := api.NewClient(cfg.BaseURL, cfg.APIKey)

	boot, err := bootstrap.Do(ctx, gqlClient, bootstrap.SystemPromptData{
		Workspace:  ws,
		ThreadID:   threadID,
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
		"thread", threadID,
		"workspace", ws,
	)

	provider := anthropic.New(strings.TrimRight(cfg.BaseURL, "/")+"/anthropic", cfg.APIKey)

	name := "axonclaw"
	platform := runtime.GOOS
	version := "dev"

	if _, err := api.RegisterAgentInstance(ctx, gqlClient, &api.RegisterAgentInstanceInput{
		InstanceID: cfg.InstanceID,
		Name:       &name,
		Platform:   &platform,
		Version:    &version,
	}); err != nil {
		fatalf("register instance failed: %v", err)
	}

	threadWorkspace := filepath.Join(ws, "threads", threadID)
	if err := os.MkdirAll(threadWorkspace, 0o755); err != nil {
		fatalf("create thread workspace: %v", err)
	}

	a := agent.New(agent.Config{
		Model:         boot.Model,
		MaxIterations: defaultMaxIterations,
		SystemPrompt:  boot.SystemPrompt,
	}, provider)

	runner.RegisterToolsFromBootstrap(a, threadWorkspace, ws, boot, logger)

	r := runner.New(logger, gqlClient, a, cfg, threadID)
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
