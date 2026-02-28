package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/looplj/axonhub/axon/api"
)

type Result struct {
	AgentID      string
	AgentName    string
	Model        string
	SystemPrompt string
	ThreadID     string
	Tools        []*api.AgentBootstrapAgentBootstrapToolsAgentToolDefinition
	Skills       []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition
	BuiltinTools []*api.AgentBootstrapAgentBootstrapBuiltinToolsAgentBuiltinTool
	AxonClawPath string
}

type SystemPromptData struct {
	Date         string
	Timezone     string
	OS           string
	Workspace    string
	AgentID      string
	AgentName    string
	AxonClawPath string
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

	data.Date = now.Format("2006-01-02")
	data.Timezone = timezone
	data.OS = runtime.GOOS
	data.AgentID = bootstrap.AgentID
	data.AgentName = bootstrap.AgentName
	data.AxonClawPath = getAxonClawPath()

	systemPrompt, err := buildSystemPrompt(bootstrap.SystemPrompt, data)
	if err != nil {
		return nil, fmt.Errorf("build system prompt: %w", err)
	}

	systemPrompt = appendSkillsToPrompt(systemPrompt, bootstrap.Skills)

	return &Result{
		AgentID:      bootstrap.AgentID,
		AgentName:    bootstrap.AgentName,
		Model:        model,
		SystemPrompt: systemPrompt,
		ThreadID:     fmt.Sprintf("th-%s", uuid.New().String()),
		Tools:        bootstrap.Tools,
		Skills:       bootstrap.Skills,
		BuiltinTools: bootstrap.BuiltinTools,
		AxonClawPath: data.AxonClawPath,
	}, nil
}

func buildSystemPrompt(tmplStr string, data SystemPromptData) (string, error) {
	tmpl, err := template.New("system").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse system prompt template: %w", err)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("execute system prompt template: %w", err)
	}

	return result.String(), nil
}

func appendSkillsToPrompt(basePrompt string, skills []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition) string {
	var sb strings.Builder
	sb.WriteString(basePrompt)

	for _, sk := range skills {
		if sk.Name == "" || sk.Content == nil || strings.TrimSpace(*sk.Content) == "" {
			continue
		}
		sb.WriteString("\n\n---\n\n")
		sb.WriteString("## Skill: ")
		sb.WriteString(sk.Name)
		sb.WriteString("\n\n")
		sb.WriteString(*sk.Content)
	}

	return sb.String()
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
