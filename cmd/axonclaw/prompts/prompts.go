package prompts

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/looplj/axonhub/axon/api"
)

type PromptEnv struct {
	Date              string
	Timezone          string
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
	case SystemFileName:
		return "How You Should Operate Here", "Use this file as workspace-specific system instruction that should guide how you operate in this environment."
	case MemoryFileName:
		return "Your Long-Term Memory", "This is your curated long-term memory. Use it as persistent context for preferences, decisions, and durable facts."
	default:
		return "Context You Should Use", "Use this file as persistent workspace context when deciding how to act."
	}
}

func BuildSystemPrompts(env PromptEnv, p *Bootstrap, skills []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition) []string {
	var out []string

	if p != nil {
		systemContent := buildSystemPromptContent(p.System.Content, skills)
		if strings.TrimSpace(systemContent) != "" {
			out = append(out, wrapBootstrapMarkdown(SystemFileName, systemContent, p.System.Path))
		}

		if instructionContent, err := RenderInstructionTemplate(env); err == nil && strings.TrimSpace(instructionContent) != "" {
			out = append(out, instructionContent)
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

		// Inject memory context: MEMORY.md + recent daily logs
		if memoryPrompt := buildMemoryPrompt(p); strings.TrimSpace(memoryPrompt) != "" {
			out = append(out, memoryPrompt)
		}

		return out
	}

	instructionContent, err := RenderInstructionTemplate(env)
	if err == nil && strings.TrimSpace(instructionContent) != "" {
		out = append(out, instructionContent)
	}

	return out
}

func BuildHeartbeatTaskSystemPrompts() []string {
	return []string{DefaultHeartbeatTaskPrompt}
}

func BuildSelfReflectTaskSystemPrompts(axonclawPath string) []string {
	return []string{strings.ReplaceAll(DefaultSelfReflectPrompt, "{{.AxonClawPath}}", axonclawPath)}
}

func BuildSelfEvolveTaskSystemPrompts() []string {
	return []string{DefaultSelfEvolvePrompt}
}

// buildMemoryPrompt builds a system prompt section from MEMORY.md and recent daily logs.
func buildMemoryPrompt(p *Bootstrap) string {
	var sections []string

	// MEMORY.md — curated long-term memory
	if !p.Memory.IsEmpty() {
		sections = append(sections, wrapBootstrapMarkdown(MemoryFileName, p.Memory.Content, p.Memory.Path))
	}

	// Recent daily logs
	for _, log := range p.DailyLogs {
		if log.IsEmpty() {
			continue
		}
		// Extract date from filename for the section title
		title := fmt.Sprintf("Daily Memory Log (%s)", extractDateFromPath(log.Path))
		sections = append(sections, fmt.Sprintf("# %s\n\n%s", title, log.Content))
	}

	if len(sections) == 0 {
		return ""
	}

	return strings.Join(sections, "\n\n")
}

// extractDateFromPath extracts a date string from a file path like "memory/2006-01-02.md".
func extractDateFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".md")
	if base == "" {
		return "unknown"
	}

	return base
}

func buildSystemPromptContent(basePrompt string, skills []*api.AgentBootstrapAgentBootstrapSkillsAgentSkillDefinition) string {
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
