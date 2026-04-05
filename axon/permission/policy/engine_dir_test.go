package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluate_PathMatches_MatchesDirResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-src",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"src/**"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src/pkg/util"},
	}
	dec := engine.Evaluate("Read", resources)

	assert.Equal(t, EffectAllow, dec.Effect)
	assert.Equal(t, "allow-src", dec.RuleID)
}

func TestEvaluate_PathMatches_MatchesDirResource_ExactMatch(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-src",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"src"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src"},
	}
	dec := engine.Evaluate("Read", resources)

	assert.Equal(t, EffectAllow, dec.Effect)
}

func TestEvaluate_PathMatches_DirResourceNoMatch(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-src",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"src/**"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceDir, WorkspaceRel: "test/data"},
	}
	dec := engine.Evaluate("Read", resources)

	assert.Equal(t, EffectDeny, dec.Effect)
	assert.Equal(t, "default.deny", dec.RuleID)
}

func TestEvaluate_PathMatches_DirResourceAbsPath(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-tmp",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"/tmp/**"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceDir, Path: "/tmp/data/sub"},
	}
	dec := engine.Evaluate("Read", resources)

	assert.Equal(t, EffectAllow, dec.Effect)
}

func TestEvaluate_PathMatches_StillMatchesPathResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-src",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"src/**"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourcePath, WorkspaceRel: "src/main.go"},
	}
	dec := engine.Evaluate("Read", resources)

	assert.Equal(t, EffectAllow, dec.Effect)
}

func TestEvaluate_DirMatches_OnlyMatchesDirResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "deny-git",
				Effect: EffectDeny,
				When: When{
					Resource: ResourceWhen{
						DirMatches: []string{".git/**"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "allow_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	dirRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: ".git/objects"},
	}
	dec := engine.Evaluate("Read", dirRes)
	assert.Equal(t, EffectDeny, dec.Effect)

	pathRes := []Resource{
		{Type: ResourcePath, WorkspaceRel: ".git/objects/abc"},
	}
	dec = engine.Evaluate("Read", pathRes)
	assert.Equal(t, EffectAllow, dec.Effect)
}

func TestEvaluate_OutsideWorkspace_DirResource(t *testing.T) {
	outside := true
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "deny-outside",
				Effect: EffectDeny,
				When: When{
					Resource: ResourceWhen{
						OutsideWorkspace: &outside,
					},
				},
			},
		},
		Defaults: Defaults{Mode: "allow_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{
		{Type: ResourceDir, Path: "/etc", OutsideWorkspace: true},
	}
	dec := engine.Evaluate("Read", resources)
	assert.Equal(t, EffectDeny, dec.Effect)

	insideRes := []Resource{
		{Type: ResourceDir, WorkspaceRel: "src", OutsideWorkspace: false},
	}
	dec = engine.Evaluate("Read", insideRes)
	assert.Equal(t, EffectAllow, dec.Effect)
}

func TestEvaluate_PathMatches_WildcardSingleLevel(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-top-level",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"*"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	topLevel := []Resource{{Type: ResourceDir, WorkspaceRel: "src"}}
	dec := engine.Evaluate("Read", topLevel)
	assert.Equal(t, EffectAllow, dec.Effect)

	nested := []Resource{{Type: ResourceDir, WorkspaceRel: "src/pkg"}}
	dec = engine.Evaluate("Read", nested)
	assert.Equal(t, EffectDeny, dec.Effect)
}

func TestEvaluate_PathMatches_CombinedWithToolIn(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-read-src",
				Effect: EffectAllow,
				When: When{
					ToolIn: []string{"Read"},
					Resource: ResourceWhen{
						PathMatches: []string{"src/**"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{{Type: ResourceDir, WorkspaceRel: "src/pkg"}}

	dec := engine.Evaluate("Read", resources)
	assert.Equal(t, EffectAllow, dec.Effect)

	dec = engine.Evaluate("Write", resources)
	assert.Equal(t, EffectDeny, dec.Effect)
}

func TestEvaluate_PathMatches_DenyOverridesAllow_DirResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "deny-secret",
				Effect: EffectDeny,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"**/.secret/**"},
					},
				},
			},
			{
				ID:     "allow-all",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"**"},
					},
				},
			},
		},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{{Type: ResourceDir, WorkspaceRel: "src/.secret/keys"}}
	dec := engine.Evaluate("Read", resources)
	assert.Equal(t, EffectDeny, dec.Effect)
}

func TestEvaluate_PathMatches_QuestionMarkGlob_DirResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "allow-single-char-dir",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"?"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	single := []Resource{{Type: ResourceDir, WorkspaceRel: "a"}}
	dec := engine.Evaluate("Read", single)
	assert.Equal(t, EffectAllow, dec.Effect)

	multi := []Resource{{Type: ResourceDir, WorkspaceRel: "ab"}}
	dec = engine.Evaluate("Read", multi)
	assert.Equal(t, EffectDeny, dec.Effect)
}

func TestEvaluate_PathMatchesAndDirMatches_BothRequired(t *testing.T) {
	doc := Document{
		Version: 1,
		Rules: []Rule{
			{
				ID:     "combined",
				Effect: EffectAllow,
				When: When{
					Resource: ResourceWhen{
						PathMatches: []string{"src/**"},
						DirMatches:  []string{"src/pkg"},
					},
				},
			},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{{Type: ResourceDir, WorkspaceRel: "src/pkg"}}
	dec := engine.Evaluate("Read", resources)
	assert.Equal(t, EffectAllow, dec.Effect)

	noDir := []Resource{{Type: ResourceDir, WorkspaceRel: "src/cmd"}}
	dec = engine.Evaluate("Read", noDir)
	assert.Equal(t, EffectDeny, dec.Effect)
}

func TestEvaluate_DefaultModes(t *testing.T) {
	tests := []struct {
		mode   string
		effect Effect
		ruleID string
	}{
		{"allow_by_default", EffectAllow, "default.allow"},
		{"deny_by_default", EffectDeny, "default.deny"},
		{"require_approval_by_default", EffectRequireApproval, "default.require_approval"},
		{"", EffectRequireApproval, "default.require_approval"},
		{"unknown_mode", EffectRequireApproval, "default.require_approval"},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			doc := Document{
				Version:  1,
				Defaults: Defaults{Mode: tt.mode},
			}
			engine, err := New(doc)
			require.NoError(t, err)

			resources := []Resource{{Type: ResourceDir, WorkspaceRel: "src"}}
			dec := engine.Evaluate("Read", resources)
			assert.Equal(t, tt.effect, dec.Effect)
			assert.Equal(t, tt.ruleID, dec.RuleID)
		})
	}
}

func TestEvaluate_AllowList_DirResource(t *testing.T) {
	doc := Document{
		Version: 1,
		Allow: []AllowEntry{
			{Tool: "Read"},
		},
		Defaults: Defaults{Mode: "deny_by_default"},
	}
	engine, err := New(doc)
	require.NoError(t, err)

	resources := []Resource{{Type: ResourceDir, WorkspaceRel: "src"}}

	dec := engine.Evaluate("Read", resources)
	assert.Equal(t, EffectAllow, dec.Effect)
	assert.Equal(t, "allow.tool", dec.RuleID)

	dec = engine.Evaluate("Write", resources)
	assert.Equal(t, EffectDeny, dec.Effect)
}
