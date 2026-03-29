package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/api"

	"github.com/looplj/axonhub/cmd/axonclaw/prompts"
)

type Bootstrap struct {
	AgentID           string
	AgentName         string
	CreatedByUserName string
	Model             string
	ReasoningEffort   string
	ThreadID          string
	Tools             []*api.AgentBootstrapAgentBootstrapToolsAgentToolDefinition
	Skills            []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition
	BuiltinTools      []*api.AgentBootstrapAgentBootstrapBuiltinToolsAgentBuiltinTool
	BuiltinSkills     []BuiltinSkill
	Prompts           *prompts.Bootstrap
	AxonClawPath      string
	SkillsRoot        string
	PromptDir         string
	RuntimeDir        string
	Date              string
	Timezone          string
	OS                string
}

type Params struct {
	Workspace  string
	SkillsRoot string
	PromptDir  string
	RuntimeDir string
}

type BuiltinSkill struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
	Order   int    `yaml:"order"`
}

func Do(ctx context.Context, client graphql.Client, data Params) (*Bootstrap, error) {
	resp, err := api.AgentBootstrap(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("agent bootstrap failed: %w", err)
	}
	bootstrap := resp.AgentBootstrap

	model := ""
	if bootstrap.Model != nil {
		model = strings.TrimSpace(*bootstrap.Model)
	}
	if model == "" {
		return nil, fmt.Errorf("missing model: agent bootstrap.model is empty")
	}

	now := time.Now()
	_, offset := now.Zone()
	timezone := fmt.Sprintf("UTC%+d", offset/3600)

	threadID := fmt.Sprintf("th-%s", uuid.New().String())

	axonClawPath := getAxonClawPath()
	osName := humanReadableOS(runtime.GOOS)

	tmplData := prompts.PromptEnv{
		Date:              now.Format("2006-01-02"),
		Timezone:          timezone,
		OS:                osName,
		Workspace:         data.Workspace,
		ThreadID:          threadID,
		AxonClawPath:      axonClawPath,
		SkillsRoot:        data.SkillsRoot,
		AgentID:           bootstrap.AgentID,
		AgentName:         bootstrap.AgentName,
		AgentInstanceName: bootstrap.AgentInstanceName,
		CreatedByUserName: bootstrap.CreatedByUserName,
	}

	prompt, err := prompts.Load(data.PromptDir, &prompts.InitParams{
		Env:                tmplData,
		ServerSystemPrompt: bootstrap.SystemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("load bootstrap prompts: %w", err)
	}

	return &Bootstrap{
		AgentID:           bootstrap.AgentID,
		AgentName:         bootstrap.AgentName,
		CreatedByUserName: bootstrap.CreatedByUserName,
		Model:             model,
		ReasoningEffort:   bootstrap.ReasoningEffort,
		ThreadID:          threadID,
		Tools:             bootstrap.Tools,
		Skills:            bootstrap.Skills,
		BuiltinTools:      bootstrap.BuiltinTools,
		BuiltinSkills:     convertBuiltinSkills(bootstrap.BuiltinSkills),
		Prompts:           prompt,
		AxonClawPath:      axonClawPath,
		SkillsRoot:        data.SkillsRoot,
		PromptDir:         data.PromptDir,
		RuntimeDir:        data.RuntimeDir,
		Date:              now.Format("2006-01-02"),
		Timezone:          timezone,
		OS:                osName,
	}, nil
}

func convertBuiltinSkills(items []*api.AgentBootstrapAgentBootstrapBuiltinSkillsAgentBuiltinSkill) []BuiltinSkill {
	out := make([]BuiltinSkill, 0, len(items))
	for _, item := range items {
		if item == nil || strings.TrimSpace(item.Name) == "" {
			continue
		}

		out = append(out, BuiltinSkill{
			Name:    item.Name,
			Enabled: item.Enabled,
			Order:   item.Order,
		})
	}

	return out
}

func humanReadableOS(goos string) string {
	switch goos {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		if goos == "" {
			return "Unknown"
		}

		return strings.ToUpper(goos[:1]) + goos[1:]
	}
}

func getAxonClawPath() string {
	if execPath, err := os.Executable(); err == nil {
		return execPath
	}
	if path, err := exec.LookPath("axonclaw"); err == nil {
		if absPath, err := filepath.Abs(path); err == nil {
			return absPath
		}
		return path
	}
	return "axonclaw"
}
