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

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
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

	require.False(t, result.Rejected)
	require.Len(t, result.MatchedRules, 1)
	require.Same(t, request, result.Request)
	require.NotNil(t, result.Request.Messages[0].Content.Content)
	assert.Equal(t, "mask-secret", result.MatchedRules[0].Name)
	assert.Equal(t, "token is [MASKED]", *result.Request.Messages[0].Content.Content)
	assert.Equal(t, "token is [MASKED]", *request.Messages[0].Content.Content)
}

func TestApplyPromptProtectionRulesRejectContent(t *testing.T) {
	content := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "reject-secret",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionReject,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		},
	})

	require.True(t, result.Rejected)
	require.Len(t, result.MatchedRules, 1)
	assert.Nil(t, result.Request)
	assert.Equal(t, "reject-secret", result.MatchedRules[0].Name)
}

func TestApplyPromptProtectionRulesScopeFiltering(t *testing.T) {
	assistantContent := "contains secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "assistant", Content: llm.MessageContent{Content: &assistantContent}},
		},
	}

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
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

	require.False(t, result.Rejected)
	assert.Nil(t, result.MatchedRules)
	assert.Same(t, request, result.Request)
	assert.Equal(t, "contains secret", *result.Request.Messages[0].Content.Content)
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

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "mask-part",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
			},
		},
	})

	require.False(t, result.Rejected)
	require.Len(t, result.MatchedRules, 1)
	require.Len(t, result.Request.Messages[0].Content.MultipleContent, 1)
	require.NotNil(t, result.Request.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "mask-part", result.MatchedRules[0].Name)
	assert.Equal(t, "[MASKED] text", *result.Request.Messages[0].Content.MultipleContent[0].Text)
	assert.Equal(t, "[MASKED] text", *request.Messages[0].Content.MultipleContent[0].Text)
}

func TestApplyPromptProtectionRules_AppliesMultipleRulesInOrder(t *testing.T) {
	content := "alice secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "mask-name",
			Pattern: `alice`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[USER]",
			},
		},
		{
			Name:    "mask-secret",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
			},
		},
	})

	require.False(t, result.Rejected)
	require.Len(t, result.MatchedRules, 2)
	require.Same(t, request, result.Request)
	require.NotNil(t, result.Request.Messages[0].Content.Content)
	assert.Equal(t, "[USER] [MASKED]", *result.Request.Messages[0].Content.Content)
	assert.Equal(t, "mask-name", result.MatchedRules[0].Name)
	assert.Equal(t, "mask-secret", result.MatchedRules[1].Name)
}

func TestApplyPromptProtectionRules_RejectAfterEarlierMask(t *testing.T) {
	content := "secret"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "mask-secret",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
			},
		},
		{
			Name:    "reject-masked",
			Pattern: `\[MASKED\]`,
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionReject,
			},
		},
	})

	require.True(t, result.Rejected)
	require.Len(t, result.MatchedRules, 1)
	assert.Nil(t, result.Request)
	assert.Equal(t, "reject-masked", result.MatchedRules[0].Name)
}

func TestApplyPromptProtectionRules_NoMatchReturnsOriginalRequest(t *testing.T) {
	content := "hello world"
	request := &llm.Request{
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
		},
	}

	result := ApplyPromptProtectionRules(request, []*ent.PromptProtectionRule{
		{
			Name:    "mask-secret",
			Pattern: `secret`,
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
			},
		},
	})

	require.False(t, result.Rejected)
	assert.Nil(t, result.MatchedRules)
	assert.Same(t, request, result.Request)
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
	assert.Nil(t, result)
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
