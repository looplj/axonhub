package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
)

func TestNextAPIKeyRuleCronOccurrence(t *testing.T) {
	// 2026-08-09 14:37 UTC is deliberately not on a cron boundary, so every case
	// exercises "first occurrence strictly after the failure" rather than "now".
	now := time.Date(2026, 8, 9, 14, 37, 0, 0, time.UTC)

	tests := []struct {
		name     string
		rule     objects.APIKeyAutoDisableRule
		expected time.Time
		wantErr  bool
	}{
		{
			name:     "daily midnight rolls to next day",
			rule:     objects.APIKeyAutoDisableRule{DisableUntilCron: "0 0 * * *"},
			expected: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "hourly picks the next hour",
			rule:     objects.APIKeyAutoDisableRule{DisableUntilCron: "0 * * * *"},
			expected: time.Date(2026, 8, 9, 15, 0, 0, 0, time.UTC),
		},
		{
			name: "timezone shifts the wall-clock target",
			rule: objects.APIKeyAutoDisableRule{
				DisableUntilCron:     "0 0 * * *",
				DisableUntilTimezone: "Asia/Shanghai",
			},
			// 14:37 UTC is 22:37 in Asia/Shanghai, so the next local midnight is
			// 2026-08-10 00:00 +08:00 == 2026-08-09 16:00 UTC.
			expected: time.Date(2026, 8, 9, 16, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid cron is rejected",
			rule:    objects.APIKeyAutoDisableRule{DisableUntilCron: "not-a-cron"},
			wantErr: true,
		},
		{
			name: "invalid timezone is rejected",
			rule: objects.APIKeyAutoDisableRule{
				DisableUntilCron:     "0 0 * * *",
				DisableUntilTimezone: "Mars/Olympus",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nextAPIKeyRuleCronOccurrence(tt.rule, now)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, got.After(now), "recovery must be strictly after the failure")
			require.Equal(t, tt.expected.UTC(), got.UTC())
		})
	}
}

func TestChannelCredentials_OAuthCountsAsOneCredential(t *testing.T) {
	oauthCreds := objects.ChannelCredentials{
		APIKey: "legacy-oauth-json-blob",
		OAuth:  &objects.OAuthCredentials{AccessToken: "at-1", RefreshToken: "rt-1"},
	}

	// The sentinel must never leak into the outbound key set, otherwise the
	// tester, backups and credential UI would show a fake key.
	require.Empty(t, oauthCreds.GetAllAPIKeys())
	require.Equal(t, []string{objects.OAuthCredentialRef}, oauthCreds.GetAllCredentialRefs())

	disabled := []objects.DisabledAPIKey{{Key: objects.OAuthCredentialRef, DisabledAt: time.Now()}}
	require.Empty(t, oauthCreds.GetEnabledCredentialRefs(disabled))

	keyCreds := objects.ChannelCredentials{APIKeys: []string{"key1", "key2"}}
	require.Equal(t, []string{"key1", "key2"}, keyCreds.GetAllCredentialRefs())
	require.Equal(t,
		[]string{"key2"},
		keyCreds.GetEnabledCredentialRefs([]objects.DisabledAPIKey{{Key: "key1", DisabledAt: time.Now()}}),
	)
}

func createTestOAuthChannel(t *testing.T, client *ent.Client, ctx context.Context, name string) *ent.Channel {
	t.Helper()

	ch, err := client.Channel.Create().
		SetName(name).
		SetType(channel.TypeCodex).
		SetBaseURL("https://chatgpt.com").
		SetCredentials(objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: "at-1", RefreshToken: "rt-1"},
		}).
		SetSupportedModels([]string{"gpt-5"}).
		SetDefaultTestModel("gpt-5").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return ch
}

// An OAuth channel holds exactly one credential, so disabling it must take the
// channel down too, and the scheduled cleanup must bring both back together.
func TestChannelService_OAuthCredentialDisableAndCronRecovery(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)

	ch := createTestOAuthChannel(t, client, ctx, "codex-oauth")
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes:      []int{429},
				Times:            2,
				Action:           objects.APIKeyAutoDisableActionUntilCron,
				DisableUntilCron: "0 0 * * *",
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	perf := &PerformanceRecord{
		ChannelID:          ch.ID,
		APIKey:             objects.OAuthCredentialRef,
		ResponseStatusCode: 429,
	}

	matched, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.False(t, acted, "threshold of 2 must not fire on the first failure")

	matched, acted = svc.checkAndHandleChannelAPIKeyRules(ctx, perf)
	require.True(t, matched)
	require.True(t, acted)

	disabled, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, disabled.DisabledAPIKeys, 1)
	require.Equal(t, objects.OAuthCredentialRef, disabled.DisabledAPIKeys[0].Key)
	require.NotNil(t, disabled.DisabledAPIKeys[0].ExpiresAt)
	require.True(t, disabled.DisabledAPIKeys[0].ExpiresAt.After(time.Now()))

	// The only credential is gone, so the channel itself must be disabled and
	// carry the marker that makes it eligible for automatic recovery.
	require.Equal(t, channel.StatusDisabled, disabled.Status)
	require.NotNil(t, disabled.ErrorMessage)
	require.Contains(t, *disabled.ErrorMessage, allKeysDisabledErrorPrefix)
	require.NotNil(t, disabled.AutoDisabledAt)

	// Credentials themselves must be untouched: the sentinel is bookkeeping only.
	require.Empty(t, disabled.Credentials.GetAllAPIKeys())
	require.NotNil(t, disabled.Credentials.OAuth)
	require.Equal(t, "at-1", disabled.Credentials.OAuth.AccessToken)

	// Simulate the scheduled recovery instant arriving.
	elapsed := time.Now().Add(-time.Minute)
	_, err = client.Channel.UpdateOneID(ch.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{
			Key:        objects.OAuthCredentialRef,
			DisabledAt: time.Now().Add(-time.Hour),
			ExpiresAt:  &elapsed,
		}}).
		Save(ctx)
	require.NoError(t, err)

	svc.cleanupExpiredDisabledAPIKeys(ctx)

	recovered, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Empty(t, recovered.DisabledAPIKeys)
	require.Equal(t, channel.StatusEnabled, recovered.Status)
	require.Nil(t, recovered.ErrorMessage)
	require.Nil(t, recovered.AutoDisabledAt)
}

// permanent_disable_delete has nothing to delete on an OAuth channel, and
// DeleteDisabledAPIKeys rejects OAuth channels outright. The rule must therefore
// degrade to a plain permanent disable and still report success, rather than
// failing and retrying forever, and must leave the real credentials untouched.
func TestChannelService_OAuthPermanentRuleDegradesToDisable(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)

	ch := createTestOAuthChannel(t, client, ctx, "codex-oauth-permanent")
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes: []int{401},
				Times:       1,
				Action:      objects.APIKeyAutoDisableActionPermanentDelete,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	_, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID:          ch.ID,
		APIKey:             objects.OAuthCredentialRef,
		ResponseStatusCode: 401,
	})
	require.True(t, acted)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Equal(t, objects.OAuthCredentialRef, updated.DisabledAPIKeys[0].Key)
	require.Nil(t, updated.DisabledAPIKeys[0].ExpiresAt, "permanent disable must never expire")
	require.Equal(t, channel.StatusDisabled, updated.Status)

	// The sentinel is bookkeeping only and must never reach real credentials.
	require.NotContains(t, updated.Credentials.APIKeys, objects.OAuthCredentialRef)
	require.Empty(t, updated.Credentials.GetAllAPIKeys())
	require.NotNil(t, updated.Credentials.OAuth)
}

// permanent_disable keeps the credential on the channel with no expiry, so the
// cleanup task must never revive it and the key list must be left alone.
func TestChannelService_PermanentDisableKeepsCredential(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)

	ch := createTestChannelWithAPIKeys(t, client, ctx, "permanent-keep", []string{"key1", "key2"})
	ch, err := client.Channel.UpdateOneID(ch.ID).
		SetPolicies(objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{
			{
				StatusCodes: []int{401},
				Times:       1,
				Action:      objects.APIKeyAutoDisableActionPermanent,
			},
		}}).
		Save(ctx)
	require.NoError(t, err)
	svc.SetEnabledChannelsForTest([]*Channel{buildChannel(ch, nil)})

	_, acted := svc.checkAndHandleChannelAPIKeyRules(ctx, &PerformanceRecord{
		ChannelID:          ch.ID,
		APIKey:             "key1",
		ResponseStatusCode: 401,
	})
	require.True(t, acted)

	updated, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, updated.DisabledAPIKeys, 1)
	require.Equal(t, "key1", updated.DisabledAPIKeys[0].Key)
	require.Nil(t, updated.DisabledAPIKeys[0].ExpiresAt, "permanent disable must never expire")

	// Unlike permanent_disable_delete, the key stays in the credentials, and the
	// channel keeps serving on the remaining one.
	require.Equal(t, []string{"key1", "key2"}, updated.Credentials.GetAllAPIKeys())
	require.Equal(t, channel.StatusEnabled, updated.Status)

	// The cleanup task must leave a never-expiring disable untouched.
	svc.cleanupExpiredDisabledAPIKeys(ctx)

	after, err := client.Channel.Get(ctx, ch.ID)
	require.NoError(t, err)
	require.Len(t, after.DisabledAPIKeys, 1)
}

func TestNormalizeAPIKeyAutoDisableRules_UntilCron(t *testing.T) {
	duration := 30

	t.Run("cron action clears the duration and trims input", func(t *testing.T) {
		policies := &objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{
			StatusCodes:            []int{429},
			Times:                  2,
			Action:                 objects.APIKeyAutoDisableActionUntilCron,
			DisableUntilCron:       "  0 0 * * *  ",
			DisableUntilTimezone:   " Asia/Shanghai ",
			DisableDurationMinutes: &duration,
		}}}

		require.NoError(t, NormalizeAPIKeyAutoDisableRules(policies))

		rule := policies.APIKeyAutoDisableRules[0]
		require.Equal(t, "0 0 * * *", rule.DisableUntilCron)
		require.Equal(t, "Asia/Shanghai", rule.DisableUntilTimezone)
		require.Nil(t, rule.DisableDurationMinutes)
	})

	t.Run("temporary action clears stale cron settings", func(t *testing.T) {
		policies := &objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{{
			StatusCodes:            []int{429},
			Times:                  2,
			Action:                 objects.APIKeyAutoDisableActionTemporary,
			DisableDurationMinutes: &duration,
			DisableUntilCron:       "0 0 * * *",
			DisableUntilTimezone:   "Asia/Shanghai",
		}}}

		require.NoError(t, NormalizeAPIKeyAutoDisableRules(policies))

		rule := policies.APIKeyAutoDisableRules[0]
		require.Empty(t, rule.DisableUntilCron)
		require.Empty(t, rule.DisableUntilTimezone)
		require.NotNil(t, rule.DisableDurationMinutes)
	})

	for _, tt := range []struct {
		name string
		rule objects.APIKeyAutoDisableRule
	}{
		{
			name: "missing cron expression",
			rule: objects.APIKeyAutoDisableRule{Times: 1, Action: objects.APIKeyAutoDisableActionUntilCron},
		},
		{
			name: "invalid cron expression",
			rule: objects.APIKeyAutoDisableRule{Times: 1, Action: objects.APIKeyAutoDisableActionUntilCron, DisableUntilCron: "bogus"},
		},
		{
			name: "invalid timezone",
			rule: objects.APIKeyAutoDisableRule{
				Times:                1,
				Action:               objects.APIKeyAutoDisableActionUntilCron,
				DisableUntilCron:     "0 0 * * *",
				DisableUntilTimezone: "Mars/Olympus",
			},
		},
	} {
		t.Run(tt.name+" is rejected", func(t *testing.T) {
			policies := &objects.ChannelPolicies{APIKeyAutoDisableRules: []objects.APIKeyAutoDisableRule{tt.rule}}
			require.Error(t, NormalizeAPIKeyAutoDisableRules(policies))
		})
	}
}
