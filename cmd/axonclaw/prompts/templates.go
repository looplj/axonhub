package prompts

import (
	"embed"
	"strings"
	"text/template"
)

//go:embed templates/*.md
var templatesFS embed.FS

var (
	DefaultSoulTemplate        = mustLoadTemplate("templates/SOUL.md")
	DefaultIdentityTemplate    = mustLoadTemplate("templates/IDENTITY.md")
	DefaultSystemTemplate      = mustLoadTemplate("templates/SYSTEM.md")
	DefaultUserTemplate        = mustLoadTemplate("templates/USER.md")
	DefaultInstructionTemplate = mustLoadTemplate("templates/INSTRUCTION.md")
	DefaultHeartbeatTemplate   = mustLoadTemplate("templates/HEARTBEAT.md")
	DefaultHeartbeatTaskPrompt = mustLoadTemplate("templates/HEARTBEAT.md")
	DefaultSelfReflectPrompt   = mustLoadTemplate("templates/SELF_REFLECT.md")
	DefaultSelfEvolvePrompt    = mustLoadTemplate("templates/SELF_EVOLVE.md")
)

func mustLoadTemplate(name string) string {
	data, err := templatesFS.ReadFile(name)
	if err != nil {
		panic("failed to load template " + name + ": " + err.Error())
	}

	return strings.TrimSpace(string(data))
}

func RenderTemplate(tpl string, data PromptEnv) (string, error) {
	return renderTemplate("personality", tpl, data)
}

func renderTemplate(name, tpl string, data any) (string, error) {
	tmpl, err := template.New(name).Parse(tpl)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", err
	}

	return result.String(), nil
}

func RenderInstructionTemplate(env PromptEnv) (string, error) {
	return renderTemplate("instruction", DefaultInstructionTemplate, env)
}
