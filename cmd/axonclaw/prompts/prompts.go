package prompts

import (
	"fmt"
	"path/filepath"
	"strings"
)

type PromptEnv struct {
	Date              string
	Timezone          string
	Model             string
	OS                string
	Workspace         string
	ThreadID          string
	AxonClawPath      string
	SkillsRoot        string
	AgentID           string
	AgentName         string
	AgentInstanceName string
	CreatedByUserName string
}

func wrapBootstrapMarkdown(fileName, content, path string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	title, desc := bootstrapFileEnvelope(fileName)

	return fmt.Sprintf("# %s\n\n%s\n\n%s", title, desc, content)
}

func bootstrapFileEnvelope(fileName string) (title, desc string) {
	switch fileName {
	case IdentityFileName:
		return "Your Identity", "Use this file as the source of truth for who you are, what role you play, and the default style you should maintain."
	case UserFileName:
		return "How You Should Work With the User", "Use this file to understand how you should collaborate with the user who owns or operates this workspace."
	case SoulFileName:
		return "Your Soul", "Use this file as your enduring guide for principles, behavioral standards, and operating boundaries."
	case AgentsFileName:
		return "How You Should Operate Here", "Use this file as workspace-specific system instruction that should guide how you operate in this environment."
	case MemoryFileName:
		return "Your Long-Term Memory", "This is your curated long-term memory. Use it as persistent context for preferences, decisions, and durable facts."
	default:
		return "Context You Should Use", "Use this file as persistent workspace context when deciding how to act."
	}
}

func BuildSystemPrompts(env PromptEnv, p *Bootstrap) []string {
	var out []string

	if strings.TrimSpace(p.System.Content) != "" {
		out = append(out, wrapBootstrapMarkdown(AgentsFileName, p.System.Content, p.System.Path))
	}

	if envPrompt, err := RenderTemplate(DefaultEnvironmentTemplate, env); err == nil && strings.TrimSpace(envPrompt) != "" {
		out = append(out, envPrompt)
	}

	otherContext := []string{
		wrapBootstrapMarkdown(IdentityFileName, p.Identity.Content, p.Identity.Path),
		wrapBootstrapMarkdown(UserFileName, p.User.Content, p.User.Path),
		wrapBootstrapMarkdown(SoulFileName, p.Soul.Content, p.Soul.Path),
	}

	for _, prompt := range otherContext {
		if strings.TrimSpace(prompt) != "" {
			out = append(out, prompt)
		}
	}

	if memoryPrompt := buildMemoryPrompt(p); strings.TrimSpace(memoryPrompt) != "" {
		out = append(out, memoryPrompt)
	}

	return out
}

func buildMemoryPrompt(p *Bootstrap) string {
	var sections []string

	if !p.Memory.IsEmpty() {
		sections = append(sections, wrapBootstrapMarkdown(MemoryFileName, p.Memory.Content, p.Memory.Path))
	}

	for _, log := range p.DailyLogs {
		if log.IsEmpty() {
			continue
		}
		title := fmt.Sprintf("Daily Memory Log (%s)", extractDateFromPath(log.Path))
		sections = append(sections, fmt.Sprintf("# %s\n\n%s", title, log.Content))
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}

func extractDateFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	if base == "" {
		return "unknown"
	}

	return base
}
