package biz

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/safeguards"
)

func TestClaudeCodeSafeguardsPersistence(t *testing.T) {
	s, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer db.Close()
	s.safeguardsDefaults = safeguards.Config{ClassifierModel: "yaml-model", Policy: "YAML policy", Timeout: 30 * time.Second, MaxTokens: 4096, TranscriptCharBudget: 24000}
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	got, err := s.ClaudeCodeSafeguardsSettings(ctx)
	require.NoError(t, err)
	require.True(t, got.UsingServerDefaults)
	require.Equal(t, "yaml-model", got.Config.ClassifierModel)
	require.Equal(t, safeguards.DefaultPolicy, got.DefaultPolicy)

	input := got.Config
	input.ClassifierModel = " ui-model "
	input.Policy = "Custom policy\nAllow feature branch updates."
	input.FallbackModels = []string{" backup "}
	input.AttemptTimeoutSeconds = 0.5
	input.EscalateOnDenial = true
	require.NoError(t, s.SetClaudeCodeSafeguardsSettings(ctx, input))
	// Runtime settings are available without granting the model caller admin scopes.
	runtimeCtx := ent.NewContext(t.Context(), db)
	runtime, err := s.ClaudeCodeSafeguardsConfig(runtimeCtx)
	require.NoError(t, err)
	require.Equal(t, "ui-model", runtime.ClassifierModel)
	require.Equal(t, []string{"backup"}, runtime.FallbackModels)
	require.Equal(t, 500*time.Millisecond, runtime.AttemptTimeout)
	require.Equal(t, input.Policy, runtime.Policy)
	require.True(t, runtime.EscalateOnDenial)
	// A new service reads persisted overrides, not just an in-process value.
	other := NewSystemService(SystemServiceParams{Ent: db, SafeguardsConfig: s.safeguardsDefaults})
	got, err = other.ClaudeCodeSafeguardsSettings(ctx)
	require.NoError(t, err)
	require.False(t, got.UsingServerDefaults)
	require.Equal(t, input.Policy, got.Config.Policy)
	// Saving an empty model disables the YAML-enabled reviewer.
	input.ClassifierModel = ""
	require.NoError(t, s.SetClaudeCodeSafeguardsSettings(ctx, input))
	runtime, err = s.ClaudeCodeSafeguardsConfig(runtimeCtx)
	require.NoError(t, err)
	require.Empty(t, runtime.ClassifierModel)
	require.NoError(t, s.ResetClaudeCodeSafeguardsSettings(ctx))
	runtime, err = s.ClaudeCodeSafeguardsConfig(runtimeCtx)
	require.NoError(t, err)
	require.Equal(t, s.safeguardsDefaults.ClassifierModel, runtime.ClassifierModel)
	require.Equal(t, s.safeguardsDefaults.Policy, runtime.Policy)
	// Corrupt persisted settings must not silently disable review.
	require.NoError(t, s.setSystemValue(ctx, SystemKeyClaudeCodeSafeguards, "{"))
	_, err = s.ClaudeCodeSafeguardsConfig(runtimeCtx)
	require.Error(t, err)
}

func TestClaudeCodeSafeguardsIgnoresStaleCache(t *testing.T) {
	s, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	initial, err := s.ClaudeCodeSafeguardsSettings(ctx)
	require.NoError(t, err)
	old := initial.Config
	old.ClassifierModel = "old-reviewer"
	old.Policy = "Old policy"
	stale, err := json.Marshal(old)
	require.NoError(t, err)
	for _, disable := range []bool{false, true} {
		updated := initial.Config
		updated.Policy = "Updated policy"
		if !disable {
			updated.ClassifierModel = "new-reviewer"
		}
		require.NoError(t, s.SetClaudeCodeSafeguardsSettings(ctx, updated))
		// Reproduce an old database read filling the cache after save invalidation.
		require.NoError(t, s.Cache.Set(ctx, "system:"+SystemKeyClaudeCodeSafeguards, ent.System{Key: SystemKeyClaudeCodeSafeguards, Value: string(stale)}))
		got, err := s.ClaudeCodeSafeguardsSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, updated, got.Config)
		runtime, err := s.ClaudeCodeSafeguardsConfig(ent.NewContext(t.Context(), db))
		require.NoError(t, err)
		require.Equal(t, updated.ClassifierModel, runtime.ClassifierModel)
		require.Equal(t, updated.Policy, runtime.Policy)
	}
	require.NoError(t, s.ResetClaudeCodeSafeguardsSettings(ctx))
	require.NoError(t, s.Cache.Set(ctx, "system:"+SystemKeyClaudeCodeSafeguards, ent.System{Key: SystemKeyClaudeCodeSafeguards, Value: string(stale)}))
	got, err := s.ClaudeCodeSafeguardsSettings(ctx)
	require.NoError(t, err)
	require.True(t, got.UsingServerDefaults)
	require.Equal(t, initial.Config, got.Config)
	// A stale cache must not hide a failed database read either.
	require.NoError(t, db.Close())
	_, err = s.ClaudeCodeSafeguardsConfig(ent.NewContext(t.Context(), db))
	require.Error(t, err)
}

func TestClaudeCodeSafeguardsValidationAndPermissions(t *testing.T) {
	s, db := setupTestSystemService(t, xcache.Config{Mode: xcache.ModeMemory})
	defer db.Close()
	ctx := authz.WithTestBypass(ent.NewContext(t.Context(), db))
	settings, err := s.ClaudeCodeSafeguardsSettings(ctx)
	require.NoError(t, err)
	for _, mutate := range []func(*ClaudeCodeSafeguardsConfig){
		func(c *ClaudeCodeSafeguardsConfig) { c.TimeoutSeconds = 0 },
		func(c *ClaudeCodeSafeguardsConfig) { c.TimeoutSeconds = 121 },
		func(c *ClaudeCodeSafeguardsConfig) { c.AttemptTimeoutSeconds = -1 },
		func(c *ClaudeCodeSafeguardsConfig) { c.MaxTokens = 0 },
		func(c *ClaudeCodeSafeguardsConfig) { c.TranscriptCharBudget = 1000001 },
		func(c *ClaudeCodeSafeguardsConfig) { c.FallbackModels = []string{"backup", " backup "} },
		func(c *ClaudeCodeSafeguardsConfig) {
			c.ClassifierModel = "primary"
			c.FallbackModels = []string{"primary"}
		},
		func(c *ClaudeCodeSafeguardsConfig) { c.FallbackModels = []string{" "} },
	} {
		input := settings.Config
		mutate(&input)
		require.Error(t, s.SetClaudeCodeSafeguardsSettings(ctx, input))
	}
	for _, scopes := range [][]string{nil, {"read_settings"}, {"write_settings"}} {
		userCtx := authz.NewUserContext(ent.NewContext(t.Context(), db), 123)
		userCtx = contexts.WithUser(userCtx, &ent.User{ID: 123, Scopes: scopes})
		_, readErr := s.ClaudeCodeSafeguardsSettings(userCtx)
		writeErr := s.SetClaudeCodeSafeguardsSettings(userCtx, settings.Config)
		resetErr := s.ResetClaudeCodeSafeguardsSettings(userCtx)
		if len(scopes) > 0 && scopes[0] == "read_settings" {
			require.NoError(t, readErr)
		} else {
			require.Error(t, readErr)
		}
		if len(scopes) > 0 && scopes[0] == "write_settings" {
			require.NoError(t, writeErr)
			require.NoError(t, resetErr)
		} else {
			require.Error(t, writeErr)
			require.Error(t, resetErr)
		}
	}
}
