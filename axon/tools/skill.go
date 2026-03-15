package tools

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/skills"

	_ "embed"

	"github.com/looplj/axonhub/axon/agent"
)

//go:embed skill.md
var skillDescription string

//go:embed skill_result.md
var skillResultTemplateStr string

var skillResultTemplate = template.Must(template.New("skill_result").Parse(skillResultTemplateStr))

// SkillTool executes skills within the conversation.
type SkillTool struct {
	// dirs are the directories to search for skills.
	// First directory has highest priority (workspace), then global.
	dirs []string
	// bundledSkills are fallback skills injected directly in code.
	// Installed skills with the same name override bundled ones.
	bundledSkills []skills.Skill
}

type SkillToolOptions struct {
	Dirs          []string
	BundledSkills []skills.Skill
}

// NewSkillTool creates a new skill execution tool.
// dirs should be provided in priority order: workspace dir first, then global dir.
func NewSkillTool(dirs ...string) *SkillTool {
	return NewSkillToolWithOptions(SkillToolOptions{Dirs: dirs})
}

func NewSkillToolWithOptions(opts SkillToolOptions) *SkillTool {
	return &SkillTool{
		dirs:          opts.Dirs,
		bundledSkills: opts.BundledSkills,
	}
}

type skillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args,omitempty"`
}

var skillParameters = jsonschema.Schema{
	Schema: "https://json-schema.org/draft/2020-12/schema",
	Type:   "object",
	Properties: map[string]*jsonschema.Schema{
		"skill": {
			Type:        "string",
			MinLength:   new(1),
			Description: "The skill name to invoke (e.g., \"pdf\", \"commit\", \"ms-office-suite:pdf\")",
		},
		"args": {
			Type:        "string",
			Description: "Optional arguments to pass to the skill",
		},
	},
	Required: []string{"skill"},
}

func (t *SkillTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Skill",
		Description: skillDescription,
		Parameters:  skillParameters,
	}
}

func (t *SkillTool) Execute(ctx context.Context, input skillInput) agent.ToolResult {
	parts := strings.SplitN(input.Skill, ":", 2)
	skillName := parts[len(parts)-1]

	result, err := skills.Get(skills.GetOptions{
		Skill:         skillName,
		Dirs:          t.dirs,
		BundledSkills: t.bundledSkills,
	})
	if err != nil {
		return ErrorResult(fmt.Errorf("skill %q not found: %w", input.Skill, err))
	}

	data := struct {
		Name    string
		Dir     string
		Content string
	}{
		Name:    result.Skill.Name,
		Dir:     result.Skill.Dir,
		Content: result.Skill.Content,
	}

	var sb strings.Builder
	if err := skillResultTemplate.Execute(&sb, data); err != nil {
		return ErrorResult(fmt.Errorf("failed to execute skill result template: %w", err))
	}

	if input.Args != "" {
		fmt.Fprintf(&sb, "\nArguments: %s", input.Args)
	}

	return TextResult(sb.String())
}

// ListSkills returns all available skills from the configured directories.
func (t *SkillTool) ListSkills() ([]skills.ListedSkill, error) {
	return skills.List(skills.ListOptions{
		Dirs:          t.dirs,
		BundledSkills: t.bundledSkills,
	})
}
