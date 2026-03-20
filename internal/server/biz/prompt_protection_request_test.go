package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestApplyPromptProtectionRulesMaskContent(t *testing.T) {
	content := "token is secret-123"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	protected, matchedRule, matched := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "mask-secret",
			Pattern: `secret-[0-9]+`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
				Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	})

	require.True(t, matched)
	require.NotNil(t, matchedRule)
	require.NotSame(t, request, protected)
	require.NotNil(t, protected.Messages[0].Content.Content)
	assert.Equal(t, "mask-secret", matchedRule.Name)
	assert.Equal(t, "token is [MASKED]", *protected.Messages[0].Content.Content)
	assert.Equal(t, "token is secret-123", *request.Messages[0].Content.Content)
}

func TestApplyPromptProtectionRulesRejectContent(t *testing.T) {
	content := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	protected, matchedRule, matched := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "reject-secret",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionReject,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	})

	require.True(t, matched)
	require.NotNil(t, matchedRule)
	require.NotSame(t, request, protected)
	assert.Equal(t, "reject-secret", matchedRule.Name)
	require.NotNil(t, protected.Messages[0].Content.Content)
	assert.Equal(t, "contains secret", *protected.Messages[0].Content.Content)
}

func TestApplyPromptProtectionRulesScopeFiltering(t *testing.T) {
	assistantContent := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "assistant", Content: llm.MessageContent{Content: &assistantContent}},
		},
	}

	protected, matchedRule, matched := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "user-only",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
				Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	})

	require.False(t, matched)
	assert.Nil(t, matchedRule)
	assert.Same(t, request, protected)
	assert.Equal(t, "contains secret", *protected.Messages[0].Content.Content)
}

func TestApplyPromptProtectionRulesMaskMultipleContent(t *testing.T) {
	partText := "secret text"
	request := &llm.Request{
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					MultipleContent: []llm.MessageContentPart{
						{Type: "text", Text: &partText},
					},
				},
			},
		},
	}

	protected, matchedRule, matched := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "mask-part",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
			},
		},
	})

	require.True(t, matched)
	require.NotNil(t, matchedRule)
	require.Len(t, protected.Messages[0].Content.MultipleContent, 1)
	require.NotNil(t, protected.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "mask-part", matchedRule.Name)
	assert.Equal(t, "[MASKED] text", *protected.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "secret text", *request.Messages[0].Content.MultipleContent[0].Text)
}

func TestPromptProtectionRuleService_ProtectMask(t *testing.T) {
	svc, _, ctx := setupPromptProtectionRuleService(t)
	svc.enabledRulesCache.Stop()
	svc.enabledRulesCache = nil

	rule, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:    "mask-secret",
		Pattern: `secret-[0-9]+`,
		Settings: &objects.PromptProtectionSettings{
			Action:      objects.PromptProtectionActionMask,
			Replacement: "[MASKED]",
			Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
		},
	})
	require.NoError(t, err)
	_, err = svc.UpdateRuleStatus(ctx, rule.ID, "enabled")
	require.NoError(t, err)

	content := "token is secret-123"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result, err := svc.Protect(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Messages[0].Content.Content)
	assert.Equal(t, "token is [MASKED]", *result.Messages[0].Content.Content)
}

func TestPromptProtectionRuleService_ProtectReject(t *testing.T) {
	svc, _, ctx := setupPromptProtectionRuleService(t)
	svc.enabledRulesCache.Stop()
	svc.enabledRulesCache = nil

	rule, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:    "reject-secret",
		Pattern: `secret`,
		Settings: &objects.PromptProtectionSettings{
			Action: objects.PromptProtectionActionReject,
			Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
		},
	})
	require.NoError(t, err)
	_, err = svc.UpdateRuleStatus(ctx, rule.ID, "enabled")
	require.NoError(t, err)

	content := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result, err := svc.Protect(ctx, request)
	require.ErrorIs(t, err, ErrPromptProtectionRejected)
	require.NotNil(t, result)
}

func TestPromptProtectionRuleService_ProtectLoadError(t *testing.T) {
	svc, client, _ := setupPromptProtectionRuleService(t)
	svc.enabledRulesCache.Stop()
	svc.enabledRulesCache = nil

	require.NoError(t, client.Close())

	result, err := svc.Protect(context.Background(), &llm.Request{})
	require.Error(t, err)
	assert.Nil(t, result)
}
