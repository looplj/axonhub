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
)

type Result struct {
	AgentID         string
	AgentName       string
	Model           string
	ReasoningEffort string
	SystemPrompt    string
	ThreadID        string
	Tools           []*api.AgentBootstrapAgentBootstrapToolsAgentToolDefinition
	Skills          []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition
	BuiltinTools    []*api.AgentBootstrapAgentBootstrapBuiltinToolsAgentBuiltinTool
	AxonClawPath    string
	SkillsRoot      string
	ConfigDir       string
	Date            string
	Timezone        string
	OS              string
}

type SystemPromptData struct {
	Workspace  string
	SkillsRoot string
	ConfigDir  string
}

func Do(ctx context.Context, client graphql.Client, data SystemPromptData) (*Result, error) {
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

	return &Result{
		AgentID:         bootstrap.AgentID,
		AgentName:       bootstrap.AgentName,
		Model:           model,
		ReasoningEffort: bootstrap.ReasoningEffort,
		SystemPrompt:    bootstrap.SystemPrompt,
		ThreadID:        threadID,
		Tools:           bootstrap.Tools,
		Skills:          bootstrap.Skills,
		BuiltinTools:    bootstrap.BuiltinTools,
		AxonClawPath:    getAxonClawPath(),
		SkillsRoot:      data.SkillsRoot,
		ConfigDir:       data.ConfigDir,
		Date:            now.Format("2006-01-02"),
		Timezone:        timezone,
		OS:              runtime.GOOS,
	}, nil
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
