package biz

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/promptprotectionrule"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupPromptProtectionRuleService(t *testing.T) (*PromptProtectionRuleService, *ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	svc := NewPromptProtectionRuleService(PromptProtectionRuleServiceParams{
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
		Ent:         client,
	})

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	return svc, client, ctx
}

func TestPromptProtectionRuleService_ValidateSettings(t *testing.T) {
	svc, client, _ := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	validMask := &objects.PromptProtectionSettings{
		Action:      objects.PromptProtectionActionMask,
		Replacement: "***",
		Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
	}

	validReject := &objects.PromptProtectionSettings{
		Action: objects.PromptProtectionActionReject,
		Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeSystem},
	}

	testCases := []struct {
		name     string
		pattern  string
		settings *objects.PromptProtectionSettings
		wantErr  string
	}{
		{
			name:     "invalid_regex",
			pattern:  "[",
			settings: validReject,
			wantErr:  "invalid regex pattern",
		},
		{
			name:     "nil_settings",
			pattern:  "secret",
			settings: nil,
			wantErr:  "settings are required",
		},
		{
			name:    "invalid_action",
			pattern: "secret",
			settings: &objects.PromptProtectionSettings{
				Action: "unknown",
			},
			wantErr: "invalid action",
		},
		{
			name:    "mask_requires_replacement",
			pattern: "secret",
			settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionMask,
			},
			wantErr: "replacement is required",
		},
		{
			name:    "invalid_scope",
			pattern: "secret",
			settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionReject,
				Scopes: []objects.PromptProtectionScope{"bad"},
			},
			wantErr: "invalid scope",
		},
		{
			name:     "valid_mask",
			pattern:  "secret",
			settings: validMask,
		},
		{
			name:     "valid_reject",
			pattern:  "secret",
			settings: validReject,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.ValidateSettings(tc.pattern, tc.settings)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestPromptProtectionRule_MatchAndReplace(t *testing.T) {
	require.False(t, MatchPromptProtectionRule("[", "anything"))
	require.Equal(t, "anything", ReplacePromptProtectionRule("[", "anything", "x"))

	require.True(t, MatchPromptProtectionRule("secret", "my secret is here"))
	require.False(t, MatchPromptProtectionRule("secret", "nothing to see"))

	require.Equal(t, "my *** is here", ReplacePromptProtectionRule("secret", "my secret is here", "***"))

	// regexp2-specific feature: lookbehind is not supported by Go regexp.
	require.True(t, MatchPromptProtectionRule(`(?<![A-Za-z0-9._%+-])[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}(?=$|[^A-Za-z0-9._%+-])`, "email: test@example.com"))
	require.Equal(t, "email: [EMAIL]", ReplacePromptProtectionRule(`(?<![A-Za-z0-9._%+-])[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}(?=$|[^A-Za-z0-9._%+-])`, "email: test@example.com", "[EMAIL]"))
}

func TestPromptProtectionRuleService_CreateRule(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	name := uuid.NewString()
	description := "desc"

	settings := &objects.PromptProtectionSettings{
		Action:      objects.PromptProtectionActionMask,
		Replacement: "***",
		Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
	}

	rule, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:        name,
		Description: &description,
		Pattern:     "secret",
		Settings:    settings,
	})
	require.NoError(t, err)
	require.NotNil(t, rule)

	enabled, err := svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Empty(t, enabled)
	require.Equal(t, name, rule.Name)
	require.Equal(t, description, rule.Description)
	require.Equal(t, "secret", rule.Pattern)
	require.NotNil(t, rule.Settings)
	require.Equal(t, settings.Action, rule.Settings.Action)
	require.Equal(t, settings.Replacement, rule.Settings.Replacement)
	require.Equal(t, settings.Scopes, rule.Settings.Scopes)

	_, err = svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:     name,
		Pattern:  "secret",
		Settings: settings,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
	enabled, err = svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Empty(t, enabled)
}

func TestPromptProtectionRuleService_CreateRule_SettingsRequired(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	_, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:    uuid.NewString(),
		Pattern: "secret",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "settings are required")
}

func TestPromptProtectionRuleService_UpdateRule(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	name := uuid.NewString()
	initialSettings := &objects.PromptProtectionSettings{
		Action: objects.PromptProtectionActionReject,
		Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeSystem},
	}

	created, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:     name,
		Pattern:  "secret",
		Settings: initialSettings,
	})
	require.NoError(t, err)

	invalidPattern := "["
	_, err = svc.UpdateRule(ctx, created.ID, &ent.UpdatePromptProtectionRuleInput{
		Pattern: &invalidPattern,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid regex pattern")

	newPattern := "token"
	newSettings := &objects.PromptProtectionSettings{
		Action:      objects.PromptProtectionActionMask,
		Replacement: "***",
		Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
	}
	newStatus := promptprotectionrule.StatusEnabled

	updated, err := svc.UpdateRule(ctx, created.ID, &ent.UpdatePromptProtectionRuleInput{
		Pattern:  &newPattern,
		Settings: newSettings,
		Status:   &newStatus,
	})
	require.NoError(t, err)
	require.Equal(t, newPattern, updated.Pattern)
	require.Equal(t, newStatus, updated.Status)
	require.Equal(t, newSettings.Action, updated.Settings.Action)
	require.Equal(t, newSettings.Replacement, updated.Settings.Replacement)
	require.Equal(t, newSettings.Scopes, updated.Settings.Scopes)
}

func TestPromptProtectionRuleService_DeleteRule(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	created, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:    uuid.NewString(),
		Pattern: "secret",
		Settings: &objects.PromptProtectionSettings{
			Action: objects.PromptProtectionActionReject,
		},
	})
	require.NoError(t, err)
	_, err = svc.UpdateRuleStatus(ctx, created.ID, promptprotectionrule.StatusEnabled)
	require.NoError(t, err)
	require.NoError(t, svc.enabledRulesCache.Load(ctx, true))
	enabled, err := svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 1)

	err = svc.DeleteRule(ctx, created.ID)
	require.NoError(t, err)
	require.NoError(t, svc.enabledRulesCache.Load(ctx, true))
	enabled, err = svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Empty(t, enabled)

	_, err = client.PromptProtectionRule.Get(ctx, created.ID)
	require.Error(t, err)
	require.True(t, ent.IsNotFound(err))
}

func TestPromptProtectionRuleService_UpdateRuleStatus(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	created, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:    uuid.NewString(),
		Pattern: "secret",
		Settings: &objects.PromptProtectionSettings{
			Action: objects.PromptProtectionActionReject,
		},
	})
	require.NoError(t, err)

	updated, err := svc.UpdateRuleStatus(ctx, created.ID, promptprotectionrule.StatusEnabled)
	require.NoError(t, err)
	require.Equal(t, promptprotectionrule.StatusEnabled, updated.Status)
}

func TestPromptProtectionRuleService_BulkOpsAndListEnabled(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	baseSettings := &objects.PromptProtectionSettings{
		Action: objects.PromptProtectionActionReject,
	}

	r1, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:     uuid.NewString(),
		Pattern:  "a",
		Settings: baseSettings,
	})
	require.NoError(t, err)

	r2, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:     uuid.NewString(),
		Pattern:  "b",
		Settings: baseSettings,
	})
	require.NoError(t, err)

	r3, err := svc.CreateRule(ctx, ent.CreatePromptProtectionRuleInput{
		Name:     uuid.NewString(),
		Pattern:  "c",
		Settings: baseSettings,
	})
	require.NoError(t, err)

	for _, id := range []int{r1.ID, r2.ID, r3.ID} {
		_, err := svc.UpdateRuleStatus(ctx, id, promptprotectionrule.StatusEnabled)
		require.NoError(t, err)
	}

	require.NoError(t, svc.enabledRulesCache.Load(ctx, true))

	enabled, err := svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 3)
	require.Equal(t, r1.ID, enabled[0].ID)
	require.Equal(t, r2.ID, enabled[1].ID)
	require.Equal(t, r3.ID, enabled[2].ID)

	require.NoError(t, svc.BulkDisableRules(ctx, []int{r1.ID, r3.ID}))
	require.NoError(t, svc.enabledRulesCache.Load(ctx, true))

	enabled, err = svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 1)
	require.Equal(t, r2.ID, enabled[0].ID)

	require.NoError(t, svc.BulkEnableRules(ctx, []int{r1.ID}))
	require.NoError(t, svc.enabledRulesCache.Load(ctx, true))
	enabled, err = svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 2)

	require.NoError(t, svc.BulkDeleteRules(ctx, []int{r1.ID, r2.ID}))
	require.NoError(t, svc.enabledRulesCache.Load(ctx, true))
	enabled, err = svc.ListEnabledRules(ctx)
	require.NoError(t, err)
	require.Len(t, enabled, 0)

	require.NoError(t, svc.BulkEnableRules(ctx, nil))
	require.NoError(t, svc.BulkDisableRules(ctx, []int{}))
	require.NoError(t, svc.BulkDeleteRules(ctx, []int{}))
}

func TestPromptProtectionRuleService_Preview(t *testing.T) {
	svc, client, ctx := setupPromptProtectionRuleService(t)
	defer svc.Stop()
	defer client.Close()

	t.Run("mask action returns replaced preview", func(t *testing.T) {
		result, err := svc.Preview(ctx, PromptProtectionPreviewInput{
			Pattern:  `secret-[0-9]+`,
			TestText: "token is secret-123",
			Settings: &objects.PromptProtectionSettings{
				Action:      objects.PromptProtectionActionMask,
				Replacement: "[MASKED]",
				Scopes:      []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.HasMatch)
		require.Equal(t, "token is [MASKED]", result.Result)
	})

	t.Run("reject action returns reject marker", func(t *testing.T) {
		result, err := svc.Preview(ctx, PromptProtectionPreviewInput{
			Pattern:  `secret`,
			TestText: "contains secret",
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionReject,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.HasMatch)
		require.Equal(t, "reject", result.Result)
	})

	t.Run("invalid pattern returns error", func(t *testing.T) {
		result, err := svc.Preview(ctx, PromptProtectionPreviewInput{
			Pattern:  `(`,
			TestText: "anything",
			Settings: &objects.PromptProtectionSettings{
				Action: objects.PromptProtectionActionReject,
				Scopes: []objects.PromptProtectionScope{objects.PromptProtectionScopeUser},
			},
		})
		require.Error(t, err)
		require.Nil(t, result)
	})
}
