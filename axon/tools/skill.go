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

type SkillTool struct {
	manager *SkillManager
}

type SkillToolOptions struct {
	Manager *SkillManager
}

func NewSkillTool(manager *SkillManager) *SkillTool {
	return &SkillTool{manager: manager}
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

	result, err := t.manager.Get(skillName)
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

func (t *SkillTool) ListSkills() ([]skills.ListedSkill, error) {
	return t.manager.List()
}
