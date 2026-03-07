package extractor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/axon/permission/policy"
)

func TestExtract_Skill_Basic(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"code-review"}`)

	resources, err := ext.Extract("/workspace", "Skill", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceSkill, resources[0].Type)
	assert.Equal(t, "code-review", resources[0].Skill)
}

func TestExtract_Skill_WithNamespace(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"my-namespace:walkthrough"}`)

	resources, err := ext.Extract("/workspace", "Skill", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, policy.ResourceSkill, resources[0].Type)
	assert.Equal(t, "walkthrough", resources[0].Skill)
}

func TestExtract_Skill_EmptyName(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":""}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty skill name")
}

func TestExtract_Skill_WhitespaceOnly(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"   "}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty skill name")
}

func TestExtract_Skill_InvalidJSON(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{invalid}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid json")
}

func TestExtract_Skill_NoNamespaceStripping(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"simple-skill"}`)

	resources, err := ext.Extract("/workspace", "Skill", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "simple-skill", resources[0].Skill)
}

func TestExtract_Skill_EmptyAfterNamespaceStrip(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"ns:"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty skill name after namespace strip")
}

func TestExtract_Skill_NormalizedToLowercase(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"Code-Review"}`)

	resources, err := ext.Extract("/workspace", "Skill", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "code-review", resources[0].Skill)
}

func TestExtract_Skill_InvalidName_Uppercase(t *testing.T) {
	ext := DefaultExtractor{}
	// After lowering, "INVALID_NAME" becomes "invalid_name" which contains underscore
	input := json.RawMessage(`{"skill":"invalid_name"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}

func TestExtract_Skill_InvalidName_LeadingHyphen(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"-leading"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}

func TestExtract_Skill_InvalidName_TrailingHyphen(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"trailing-"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}

func TestExtract_Skill_InvalidName_ConsecutiveHyphens(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"bad--name"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}

func TestExtract_Skill_InvalidName_TooLong(t *testing.T) {
	ext := DefaultExtractor{}
	// 65 chars
	input := json.RawMessage(`{"skill":"a2345678901234567890123456789012345678901234567890123456789012345"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}

func TestExtract_Skill_ValidName_MaxLength(t *testing.T) {
	ext := DefaultExtractor{}
	// Exactly 64 chars
	input := json.RawMessage(`{"skill":"a234567890123456789012345678901234567890123456789012345678901234"}`)

	resources, err := ext.Extract("/workspace", "Skill", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "a234567890123456789012345678901234567890123456789012345678901234", resources[0].Skill)
}

func TestExtract_Skill_ValidName_SingleChar(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"a"}`)

	resources, err := ext.Extract("/workspace", "Skill", input)

	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "a", resources[0].Skill)
}

func TestExtract_Skill_InvalidName_SpecialChars(t *testing.T) {
	ext := DefaultExtractor{}
	input := json.RawMessage(`{"skill":"skill.name"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}

func TestExtract_Skill_InvalidName_ColonsInName(t *testing.T) {
	ext := DefaultExtractor{}
	// After namespace strip: "skill:with:colons" — contains colons, invalid
	input := json.RawMessage(`{"skill":"ns:skill:with:colons"}`)

	_, err := ext.Extract("/workspace", "Skill", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
}
