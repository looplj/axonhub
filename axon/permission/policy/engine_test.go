package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate_Skill_Deny(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "deny-dangerous-skill",
				Effect: EffectDeny,
				Reason: "skill not allowed",
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"dangerous-skill"},
					},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceSkill, Skill: "dangerous-skill"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectDeny, dec.Effect)
	assert.Equal(t, "deny-dangerous-skill", dec.RuleID)
	assert.Equal(t, "skill not allowed", dec.Reason)
}

func TestEvaluate_Skill_Allow(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-safe-skills",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"code-review", "walkthrough"},
					},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceSkill, Skill: "walkthrough"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectAllow, dec.Effect)
	assert.Equal(t, "allow-safe-skills", dec.RuleID)
}

func TestEvaluate_Skill_CaseInsensitive(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-skill",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"Code-Review"},
					},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceSkill, Skill: "code-review"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectAllow, dec.Effect)
}

func TestEvaluate_Skill_NoMatch_FallsToDefault(t *testing.T) {
	doc := Document{
		Version: 1,
		Defaults: Defaults{
			Mode: "deny_by_default",
		},
		Rules: []Rule{
			{
				ID:     "allow-safe-skills",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"walkthrough"},
					},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceSkill, Skill: "unknown-skill"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectDeny, dec.Effect)
	assert.Equal(t, "default.deny", dec.RuleID)
}

func TestEvaluate_Skill_DenyOverridesAllow(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "deny-skill",
				Effect: EffectDeny,
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"dangerous"},
					},
				},
			},
			{
				ID:     "allow-all-skills",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Skill"},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceSkill, Skill: "dangerous"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectDeny, dec.Effect)
	assert.Equal(t, "deny-skill", dec.RuleID)
}

func TestEvaluate_Skill_RequireApproval(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "review-skill",
				Effect: EffectRequireApproval,
				Reason: "needs human review",
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"deploy"},
					},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceSkill, Skill: "deploy"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectRequireApproval, dec.Effect)
	assert.Equal(t, "review-skill", dec.RuleID)
	assert.Equal(t, "needs human review", dec.Reason)
}

func TestEvaluate_Skill_SkillIn_NoSkillResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-skill",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"walkthrough"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	// No skill resource — SkillIn matcher should fail
	resources := []Resource{
		{Type: ResourcePath, Path: "/tmp/file.txt"},
	}
	dec := engine.Evaluate("Skill", resources)

	assert.Equal(t, EffectDeny, dec.Effect)
	assert.Equal(t, "default.deny", dec.RuleID)
}

func TestEvaluate_Skill_ToolMismatch(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-skill",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Skill"},
					Resource: ResourceWhen{
						SkillIn: []string{"code-review"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	// Tool name doesn't match the rule
	resources := []Resource{
		{Type: ResourceSkill, Skill: "code-review"},
	}
	dec := engine.Evaluate("Read", resources)

	assert.Equal(t, EffectDeny, dec.Effect)
	assert.Equal(t, "default.deny", dec.RuleID)
}
