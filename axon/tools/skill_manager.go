package tools

import (
	"github.com/looplj/skills"
)

type SkillManager struct {
	dirs          []string
	bundledSkills []skills.Skill
}

type SkillManagerOptions struct {
	Dirs          []string
	BundledSkills []skills.Skill
}

func NewSkillManager(opts SkillManagerOptions) *SkillManager {
	return &SkillManager{
		dirs:          opts.Dirs,
		bundledSkills: opts.BundledSkills,
	}
}

func (m *SkillManager) Get(skillName string) (skills.GetResult, error) {
	return skills.Get(skills.GetOptions{
		Skill:         skillName,
		Dirs:          m.dirs,
		BundledSkills: m.bundledSkills,
	})
}

func (m *SkillManager) List() ([]skills.ListedSkill, error) {
	return skills.List(skills.ListOptions{
		Dirs:          m.dirs,
		BundledSkills: m.bundledSkills,
	})
}
