package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/looplj/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillTool(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "demo-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))

	skillContent := `---
name: demo-skill
description: Demo skill for testing
---

# Demo Skill

Follow these instructions.
`
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644))

	tool := NewSkillTool(tmpDir)

	t.Run("Definition", func(t *testing.T) {
		def := tool.Definition()
		assert.Equal(t, "Skill", def.Name)
		assert.NotEmpty(t, def.Description)
	})

	t.Run("Execute success", func(t *testing.T) {
		result := tool.Execute(context.Background(), skillInput{Skill: "demo-skill"})

		assert.Nil(t, result.Error)
		require.NotNil(t, result.Content.Text)
		assert.Contains(t, *result.Content.Text, `<skill name="demo-skill">`)
		assert.Contains(t, *result.Content.Text, "Follow these instructions")
		assert.Contains(t, *result.Content.Text, "</skill>")
	})

	t.Run("Execute with args", func(t *testing.T) {
		result := tool.Execute(context.Background(), skillInput{Skill: "demo-skill", Args: "-m 'test message'"})

		assert.Nil(t, result.Error)
		require.NotNil(t, result.Content.Text)
		assert.Contains(t, *result.Content.Text, "Arguments: -m 'test message'")
	})

	t.Run("Execute with qualified name", func(t *testing.T) {
		result := tool.Execute(context.Background(), skillInput{Skill: "namespace:demo-skill"})

		assert.Nil(t, result.Error)
		require.NotNil(t, result.Content.Text)
		assert.Contains(t, *result.Content.Text, `<skill name="demo-skill">`)
	})

	t.Run("Execute not found", func(t *testing.T) {
		result := tool.Execute(context.Background(), skillInput{Skill: "nonexistent"})

		assert.Error(t, result.Error)
		assert.Contains(t, result.Error.Error(), "not found")
	})

	t.Run("Execute empty skill name", func(t *testing.T) {
		result := tool.Execute(context.Background(), skillInput{Skill: ""})

		assert.Error(t, result.Error)
	})
}

func TestSkillToolListSkills(t *testing.T) {
	tmpDir := t.TempDir()

	skill1Dir := filepath.Join(tmpDir, "skill-one")
	require.NoError(t, os.MkdirAll(skill1Dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(`---
name: skill-one
description: First skill
---
# Skill One
`), 0644))

	skill2Dir := filepath.Join(tmpDir, "skill-two")
	require.NoError(t, os.MkdirAll(skill2Dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(`---
name: skill-two
description: Second skill
---
# Skill Two
`), 0644))

	tool := NewSkillTool(tmpDir)

	skills, err := tool.ListSkills()
	require.NoError(t, err)
	assert.Len(t, skills, 2)
}

func TestSkillToolBundledSkillsFallback(t *testing.T) {
	tool := NewSkillToolWithOptions(SkillToolOptions{
		Dirs: []string{t.TempDir()},
		BundledSkills: []skills.Skill{
			{
				Name:        "memory-management",
				Description: "Manage memory",
				Dir:         "/tmp/bundled/memory-management",
				Content: `---
name: memory-management
description: Manage memory
---
# Memory`,
			},
		},
	})

	result := tool.Execute(context.Background(), skillInput{Skill: "memory-management"})

	assert.Nil(t, result.Error)
	require.NotNil(t, result.Content.Text)
	assert.Contains(t, *result.Content.Text, `<skill name="memory-management">`)
}

func TestSkillToolBundledSkillWithoutDirUsesPlaceholder(t *testing.T) {
	tool := NewSkillToolWithOptions(SkillToolOptions{
		Dirs: []string{t.TempDir()},
		BundledSkills: []skills.Skill{
			{
				Name:        "builtin-skill",
				Description: "Built-in skill without reference dir",
				Content: `---
name: builtin-skill
description: Built-in skill without reference dir
---
# Builtin`,
			},
		},
	})

	result := tool.Execute(context.Background(), skillInput{Skill: "builtin-skill"})

	assert.Nil(t, result.Error)
	require.NotNil(t, result.Content.Text)
	assert.Contains(t, *result.Content.Text, "<skill_dir>BUILTIN_SKILL_NO_REFERENCE</skill_dir>")
	assert.NotContains(t, *result.Content.Text, "you MUST use absolute paths based on the skill directory above")
}
