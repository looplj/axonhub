package biz

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
)

func withClientCancellationPolicy(
	t *testing.T,
	svc *ChannelService,
	ctx context.Context,
	ch *ent.Channel,
	policy ClientCancellationPolicy,
) {
	t.Helper()

	require.NoError(t, svc.SystemService.SetRetryPolicy(ctx, &RetryPolicy{
		Enabled:                  true,
		ClientCancellationPolicy: policy,
	}))
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})
}

func canceledPerf(channelID int, projectID int, apiKey string) *PerformanceRecord {
	perf := &PerformanceRecord{ChannelID: channelID, ProjectID: projectID, APIKey: apiKey}
	perf.MarkCanceled()

	return perf
}

func TestNormalizeClientCancellationPolicy_DefaultsAndClamp(t *testing.T) {
	policy := ClientCancellationPolicy{Enabled: true}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))

	require.Equal(t, defaultClientCancellationThreshold, policy.Threshold)
	require.Equal(t, defaultClientCancellationCooldownMinutes, policy.CooldownMinutes)
	require.Equal(t, ClientCancellationActionSwitchAndCooldown, policy.Action)
	require.Equal(t, ClientCancellationScopeGlobal, policy.Scope)
	require.NotNil(t, policy.ProjectOverrides)

	policy = ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       -3,
		CooldownMinutes: maxClientCancellationCooldownMinutes + 60,
		Action:          "unknown",
		Scope:           "unknown",
	}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))
	require.Equal(t, defaultClientCancellationThreshold, policy.Threshold)
	require.Equal(t, maxClientCancellationCooldownMinutes, policy.CooldownMinutes)
	require.Equal(t, ClientCancellationActionSwitchAndCooldown, policy.Action)
	require.Equal(t, ClientCancellationScopeGlobal, policy.Scope)
}

func TestNormalizeClientCancellationPolicy_LegacyEmptyPolicyStaysEnabled(t *testing.T) {
	// A retry policy stored before this feature existed decodes to zero values.
	policy := ClientCancellationPolicy{}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))
	require.True(t, policy.Enabled)

	// An explicit disable keeps the other normalized fields, so it stays off.
	policy = ClientCancellationPolicy{Enabled: false, Threshold: 2, CooldownMinutes: 30, Action: ClientCancellationActionCooldownOnly}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))
	require.False(t, policy.Enabled)
}

func TestNormalizeClientCancellationPolicy_ProjectOverrides(t *testing.T) {
	policy := ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       3,
		CooldownMinutes: 15,
		Action:          ClientCancellationActionCooldownOnly,
		Scope:           ClientCancellationScopeProjectFirst,
		ProjectOverrides: []ClientCancellationProjectPolicy{
			{ProjectID: 0, Enabled: true},
			{ProjectID: 7, Enabled: true},
			{ProjectID: 7, Enabled: false, Threshold: 9},
			{ProjectID: 11, Enabled: true, Threshold: 1, CooldownMinutes: 5, Action: "invalid"},
		},
	}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))

	require.Len(t, policy.ProjectOverrides, 2)
	require.Equal(t, 7, policy.ProjectOverrides[0].ProjectID)
	require.Equal(t, 3, policy.ProjectOverrides[0].Threshold)
	require.Equal(t, 15, policy.ProjectOverrides[0].CooldownMinutes)
	require.Equal(t, ClientCancellationActionCooldownOnly, policy.ProjectOverrides[0].Action)

	require.Equal(t, 11, policy.ProjectOverrides[1].ProjectID)
	require.Equal(t, 1, policy.ProjectOverrides[1].Threshold)
	require.Equal(t, 5, policy.ProjectOverrides[1].CooldownMinutes)
	require.Equal(t, ClientCancellationActionCooldownOnly, policy.ProjectOverrides[1].Action)
}

func TestClientCancellationPolicy_EffectiveGlobalScope(t *testing.T) {
	policy := ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       2,
		CooldownMinutes: 30,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeGlobal,
		ProjectOverrides: []ClientCancellationProjectPolicy{
			{ProjectID: 5, Enabled: true, Threshold: 1, CooldownMinutes: 5, Action: ClientCancellationActionCooldownOnly},
		},
	}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))

	effective, source := policy.EffectiveClientCancellationPolicy(5)
	require.Equal(t, "global", source)
	require.Equal(t, 2, effective.Threshold)
	require.Equal(t, 30, effective.CooldownMinutes)
}

func TestClientCancellationPolicy_EffectiveProjectFirst(t *testing.T) {
	policy := ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       2,
		CooldownMinutes: 30,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeProjectFirst,
		ProjectOverrides: []ClientCancellationProjectPolicy{
			{ProjectID: 5, Enabled: true, Threshold: 4, CooldownMinutes: 10, Action: ClientCancellationActionCooldownOnly},
			{ProjectID: 6, Enabled: false, Threshold: 1},
		},
	}
	require.NoError(t, normalizeClientCancellationPolicy(&policy))

	effective, source := policy.EffectiveClientCancellationPolicy(5)
	require.Equal(t, "project", source)
	require.Equal(t, 4, effective.Threshold)
	require.Equal(t, 10, effective.CooldownMinutes)
	require.Equal(t, ClientCancellationActionCooldownOnly, effective.Action)

	// A disabled override falls back to the global settings.
	effective, source = policy.EffectiveClientCancellationPolicy(6)
	require.Equal(t, "global", source)
	require.Equal(t, 2, effective.Threshold)

	// Projects without an override use the global settings too.
	effective, source = policy.EffectiveClientCancellationPolicy(99)
	require.Equal(t, "global", source)
	require.Equal(t, 2, effective.Threshold)
}

func TestChannelService_ClientCancellationSwitchAndCooldown(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-channel", []string{"key1", "key2"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       2,
		CooldownMinutes: 10,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeGlobal,
	})

	require.False(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))
	before := time.Now()
	require.True(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.NotNil(t, updated.AutoDisabledAt)
	require.NotNil(t, updated.AutoDisableExpiresAt)
	require.WithinDuration(t, before.Add(10*time.Minute), *updated.AutoDisableExpiresAt, time.Minute)
	require.NotNil(t, updated.ErrorMessage)
	require.Contains(t, *updated.ErrorMessage, "Client cancellation cooldown")
	// switch_and_cooldown takes the whole channel out of rotation, so no
	// credential is disabled individually.
	require.Empty(t, updated.DisabledAPIKeys)
}

func TestChannelService_ClientCancellationCooldownOnly(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-channel-keys", []string{"key1", "key2"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       1,
		CooldownMinutes: 5,
		Action:          ClientCancellationActionCooldownOnly,
		Scope:           ClientCancellationScopeGlobal,
	})

	require.True(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	// cooldown_only keeps the channel routable with its remaining credential.
	require.Equal(t, channel.StatusEnabled, updated.Status)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Equal(t, "key1", updated.DisabledAPIKeys[0].Key)
	require.NotNil(t, updated.DisabledAPIKeys[0].ExpiresAt)
	require.Contains(t, updated.DisabledAPIKeys[0].Reason, "Client cancellation cooldown")
}

func TestChannelService_ClientCancellationCountResetsOnSuccess(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-reset", []string{"key1"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       2,
		CooldownMinutes: 10,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeGlobal,
	})

	require.False(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	success := &PerformanceRecord{ChannelID: ch.ID, APIKey: "key1"}
	success.MarkSuccess()
	svc.RecordPerformance(ctx, success)

	// The first cancellation after a successful request must not trigger the
	// cooldown, because the counter restarted.
	require.False(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, updated.Status)
}

func TestChannelService_ClientCancellationDisabled(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-disabled", []string{"key1"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         false,
		Threshold:       1,
		CooldownMinutes: 10,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeGlobal,
	})

	require.False(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))
	require.False(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, updated.Status)
}

func TestChannelService_ClientCancellationProjectOverride(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-project", []string{"key1"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       5,
		CooldownMinutes: 30,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeProjectFirst,
		ProjectOverrides: []ClientCancellationProjectPolicy{
			{ProjectID: 42, Enabled: true, Threshold: 1, CooldownMinutes: 1},
		},
	})

	// Project 42 uses the aggressive override and cools down on the first cancel.
	require.True(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 42, "key1")))

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, updated.Status)
	require.NotNil(t, updated.AutoDisableExpiresAt)

	// Other projects inherit the global threshold of 5.
	require.NoError(t, client.Channel.UpdateOneID(ch.ID).
		SetStatus(channel.StatusEnabled).
		ClearAutoDisableExpiresAt().
		ClearAutoDisabledAt().
		ClearErrorMessage().
		Exec(ctx))
	for range 4 {
		require.False(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 7, "key1")))
	}
	require.True(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 7, "key1")))
}
func TestChannelService_ClientCancellationCooldownRecoversAfterExpiry(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-recover", []string{"key1"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       1,
		CooldownMinutes: 5,
		Action:          ClientCancellationActionSwitchAndCooldown,
		Scope:           ClientCancellationScopeGlobal,
	})

	require.True(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	disabled, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusDisabled, disabled.Status)

	// Simulate the cooldown elapsing and let the cleanup task pick it up.
	require.NoError(t, client.Channel.UpdateOneID(ch.ID).
		SetAutoDisableExpiresAt(time.Now().Add(-time.Minute)).
		Exec(ctx))
	svc.cleanupExpiredDisabledAPIKeys(ctx)

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Equal(t, channel.StatusEnabled, recovered.Status)
	require.Nil(t, recovered.AutoDisabledAt)
	require.Nil(t, recovered.AutoDisableExpiresAt)
	require.Nil(t, recovered.ErrorMessage)
}

func TestChannelService_ClientCancellationCredentialRecoversAfterExpiry(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "cancel-recover-key", []string{"key1", "key2"})
	withClientCancellationPolicy(t, svc, ctx, ch, ClientCancellationPolicy{
		Enabled:         true,
		Threshold:       1,
		CooldownMinutes: 5,
		Action:          ClientCancellationActionCooldownOnly,
		Scope:           ClientCancellationScopeGlobal,
	})

	require.True(t, svc.evaluateClientCancellationForFailure(ctx, canceledPerf(ch.ID, 0, "key1")))

	disabled, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, disabled.DisabledAPIKeys, 1)

	// Elapsed credentials are pruned by the same cleanup task.
	expired := disabled.DisabledAPIKeys
	expired[0].ExpiresAt = lo.ToPtr(time.Now().Add(-time.Minute))
	require.NoError(t, client.Channel.UpdateOneID(ch.ID).SetDisabledAPIKeys(expired).Exec(ctx))
	svc.cleanupExpiredDisabledAPIKeys(ctx)

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, recovered.DisabledAPIKeys)
	require.Equal(t, channel.StatusEnabled, recovered.Status)
}
