package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/bus"
	"github.com/looplj/axonhub/axon/mcp"
	"github.com/looplj/axonhub/axon/permission"
	"github.com/looplj/axonhub/axon/permission/approval"
	"github.com/looplj/axonhub/axon/permission/grant"
	"github.com/looplj/axonhub/axon/permission/policy"
	"github.com/looplj/axonhub/axon/pkg/search"
	"github.com/looplj/axonhub/axon/provider/reloadable"
	"github.com/looplj/axonhub/axon/subagent"
	"github.com/looplj/axonhub/axon/thread"
	"github.com/looplj/axonhub/axon/tools"
	"github.com/looplj/skills/skillscmd"
	"github.com/spf13/cobra"

	_ "embed"

	tea "charm.land/bubbletea/v2"
	axonconf "github.com/looplj/axonhub/axon/conf"
	axoncontext "github.com/looplj/axonhub/axon/context"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/looplj/axonhub/cmd/axoncli/cmds"
	"github.com/looplj/axonhub/cmd/axoncli/conf"
	cliruntime "github.com/looplj/axonhub/cmd/axoncli/runtime"
	clitools "github.com/looplj/axonhub/cmd/axoncli/tools"
	"github.com/looplj/axonhub/cmd/axoncli/tui"
)

//go:embed system.md
var systemPromptTemplate string

const (
	defaultMaxIter = 30
	configDirName  = ".axoncli"
	threadsDirName = "threads"
	logsDirName    = "logs"
)

type systemPromptData struct {
	Date        string
	Timezone    string
	OS          string
	Workspace   string
	ThreadID    string
	ConfigDir   string
	AxonCliPath string
}

type permissionMiddleware struct {
	evaluator *permission.Evaluator
}

func (m *permissionMiddleware) BeforeTool(ctx context.Context, req agent.ToolRequest) error {
	permReq := permission.ToolRequest{
		ThreadID:   req.ThreadID,
		Workspace:  req.Workspace,
		ToolCallID: req.ToolCallID,
		ToolName:   req.ToolName,
		ToolInput:  json.RawMessage(req.ToolInput),
		StartedAt:  req.StartedAt,
	}
	return m.evaluator.Evaluate(ctx, permReq)
}

func (m *permissionMiddleware) AfterTool(ctx context.Context, req agent.ToolRequest, toolErr error) error {
	permReq := permission.ToolRequest{
		ThreadID:   req.ThreadID,
		Workspace:  req.Workspace,
		ToolCallID: req.ToolCallID,
		ToolName:   req.ToolName,
		ToolInput:  json.RawMessage(req.ToolInput),
		StartedAt:  req.StartedAt,
	}
	m.evaluator.LogToolResult(permReq, toolErr)
	return nil
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get home directory: %v\n", err)
		os.Exit(1)
	}

	configDir := filepath.Join(homeDir, configDirName)
	workspaceDir := mustGetwd()

	rootCmd := &cobra.Command{
		Use:          "axoncli",
		Short:        "Axon CLI",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			debug, _ := cmd.Flags().GetBool("debug")
			cfg, err := conf.LoadEffectiveConfig(configDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := conf.ValidateConfig(cfg, configDir); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return runTUI(cfg, configDir, workspaceDir, debug)
		},
	}
	rootCmd.Flags().Bool("debug", false, "enable debug mode")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(cmds.NewHelpCommand(rootCmd))

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
	rootCmd.AddCommand(cmds.NewCommand(cmds.MemoryOptions{
		Dir:    configDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
	rootCmd.AddCommand(cmds.NewMCPCommand(cmds.MCPOptions{
		Dir:    configDir,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runTUI(cfg conf.Config, configDir string, workspaceDir string, debug bool) error {
	threadID := uuid.New().String()

	threadsDir := filepath.Join(configDir, threadsDirName)
	if err := os.MkdirAll(threadsDir, 0o755); err != nil {
		return fmt.Errorf("cannot create threads directory: %v", err)
	}

	logFilePath := filepath.Join(threadsDir, threadID+".log")
	ljLogger := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10, // megabytes
		MaxAge:     7,  // days
		MaxBackups: 3,  // files
		LocalTime:  true,
	}
	defer ljLogger.Close()

	logger := slog.New(slog.NewTextHandler(ljLogger, &slog.HandlerOptions{Level: slog.LevelDebug}))

	eventBus := bus.New(
		bus.WithRecover(logger),
		bus.WithTracing(),
	)
	defer eventBus.Close()

	threadStore, err := thread.NewJSONLStore(threadsDir)
	if err != nil {
		return fmt.Errorf("failed to initialize thread store: %v", err)
	}
	threadMgr := thread.NewManager(threadStore)

	systemPrompt := mustLoadSystemPrompt(configDir, workspaceDir, threadID, debug)

	provider := reloadable.New(cliruntime.BuildProvider(cfg))

	approver := approval.NewInProcessService()

	grants := grant.NewMemoryStore(grant.NewFileStore(filepath.Join(configDir, "permission")))
	if err := grants.LoadGlobal(); err != nil {
		return fmt.Errorf("failed to load global grants: %w", err)
	}
	if err := grants.LoadWorkspace(workspaceDir); err != nil {
		return fmt.Errorf("failed to load workspace grants: %w", err)
	}

	pdoc, err := conf.LoadOrCreatePolicy(configDir, workspaceDir)
	if err != nil {
		return fmt.Errorf("failed to load policy: %w", err)
	}
	eng, err := policy.New(pdoc)
	if err != nil {
		return fmt.Errorf("failed to build policy engine: %w", err)
	}

	permEvaluator := permission.NewEvaluator(permission.EvaluatorOptions{
		Logger:   logger,
		Policy:   eng,
		Approver: approver,
		Grants:   grants,
	})
	permMiddleware := &permissionMiddleware{evaluator: permEvaluator}

	a := agent.New(agent.Config{
		Model:         cfg.Model,
		MaxIterations: defaultMaxIter,
		SystemPrompts: []string{systemPrompt},
	}, provider, agent.WithBus(eventBus), agent.WithLogger(logger), agent.WithMiddlewares(permMiddleware))

	a.RegisterTool(tools.NewAgentTool(tools.NewReadTool(workspaceDir, false)))
	a.RegisterTool(tools.NewAgentTool(tools.NewWriteTool(workspaceDir, false)))
	a.RegisterTool(tools.NewAgentTool(tools.NewEditTool(workspaceDir, false)))
	a.RegisterTool(tools.NewAgentTool(tools.NewBashTool(workspaceDir, false, true)))
	a.RegisterTool(tools.NewAgentTool(tools.NewGrepTool(workspaceDir, false)))
	a.RegisterTool(tools.NewAgentTool(tools.NewGlobTool(workspaceDir, false)))

	skillMgr := tools.NewSkillManager(tools.SkillManagerOptions{
		Dirs: []string{
			filepath.Join(workspaceDir, "skills"),
			filepath.Join(configDir, "skills"),
		},
	})
	a.RegisterTool(tools.NewAgentTool(tools.NewSkillTool(skillMgr)))
	a.RegisterTool(tools.NewAgentTool(tools.NewWebSearchTool(search.NewDuckDuckGoProvider())))
	a.RegisterTool(tools.NewAgentTool(tools.NewWebFetchTool()))
	a.RegisterTool(tools.NewAgentTool(clitools.NewAxonHelpTool()))

	agentDir := filepath.Join(workspaceDir, ".agent")

	subagentDir := filepath.Join(agentDir, "subagents")

	subagentMgr := subagent.NewManagerFromPath(subagentDir)
	if err := subagentMgr.Load(); err != nil {
		logger.Warn("failed to load subagent definitions", "error", err, "path", subagentDir)
	}

	a.RegisterTool(tools.NewAgentTool(subagent.NewTool(subagent.ToolOptions{
		Manager:     subagentMgr,
		Provider:    provider,
		ToolSource:  &agentToolSource{agent: a},
		Model:       cfg.Model,
		Middlewares: a.Middlewares(),
		Logger:      logger,
	})))

	mcpMgr := mcp.NewManager(mcp.ManagerOptions{
		Logger:    logger,
		ConfigDir: configDir,
	})
	mcpMgr.RegisterTools(a, workspaceDir, map[string]struct{}{
		"Read":      {},
		"Write":     {},
		"Edit":      {},
		"Bash":      {},
		"Grep":      {},
		"Glob":      {},
		"Skill":     {},
		"WebSearch": {},
		"WebFetch":  {},
		"AxonHelp":  {},
	})

	store := axonconf.NewStore(cfg)
	loader := axonconf.NewViperLoader[conf.Config](axonconf.ViperLoaderOptions{
		ConfigName:     "config",
		ConfigType:     "yml",
		SearchPaths:    []string{configDir, "."},
		AllowMissing:   true,
		EnvPrefix:      "AXONCLI",
		EnvKeyReplacer: strings.NewReplacer(".", "_"),
		UnmarshalTag:   "conf",
	})
	applier := cliruntime.NewApplier(a, provider, systemPrompt, defaultMaxIter)
	confMgr, err := axonconf.NewManager(axonconf.ManagerOptions[conf.Config]{
		Store:  store,
		Loader: loader,
		Validate: func(_ context.Context, v conf.Config) error {
			return conf.ValidateConfig(v, configDir)
		},
		Diff:    cliruntime.DiffConfig,
		Applier: applier,
		Bus:     eventBus,
		Logger:  logger,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctx = axoncontext.WithThreadID(ctx, threadID)
	ctx = axoncontext.WithWorkspace(ctx, workspaceDir)
	defer cancel()

	m := tui.NewModel(tui.ModelOpts{
		Agent:     a,
		Bus:       eventBus,
		ThreadID:  threadID,
		ThreadMgr: threadMgr,
		Ctx:       ctx,
		Cancel:    cancel,
		Model:     cfg.Model,
		Workspace: workspaceDir,
		ConfigDir: configDir,
		Approval:  approver,
		ReloadConf: func(ctx context.Context) error {
			return confMgr.Reload(ctx, axonconf.ReloadSourceManual)
		},
	})

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func mustLoadSystemPrompt(configDir, workspace, threadID string, debug bool) string {
	now := time.Now()
	_, offset := now.Zone()
	timezone := fmt.Sprintf("UTC%+d", offset/3600)

	var axonCliPath string
	if debug {
		execPath, err := os.Executable()
		if err != nil {
			axonCliPath = "axoncli"
		} else {
			axonCliPath = execPath
		}
	} else {
		if _, err := exec.LookPath("axoncli"); err == nil {
			axonCliPath = "axoncli"
		} else {
			execPath, err := os.Executable()
			if err != nil {
				axonCliPath = "axoncli"
			} else {
				axonCliPath = execPath
			}
		}
	}

	tmplData := systemPromptData{
		Date:        now.Format("2006-01-02"),
		Timezone:    timezone,
		OS:          runtime.GOOS,
		Workspace:   workspace,
		ThreadID:    threadID,
		ConfigDir:   configDir,
		AxonCliPath: axonCliPath,
	}

	tmpl, err := template.New("system").Parse(systemPromptTemplate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot parse system prompt template: %v\n", err)
		os.Exit(1)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, tmplData); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot execute system prompt template: %v\n", err)
		os.Exit(1)
	}

	return result.String()
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot get working directory: %v\n", err)
		os.Exit(1)
	}
	return dir
}

type agentToolSource struct {
	agent *agent.Agent
}

func (s *agentToolSource) AvailableTools() []agent.Tool {
	return s.agent.RegisteredTools()
}

func (s *agentToolSource) Middlewares() []agent.Middleware {
	return s.agent.Middlewares()
}
