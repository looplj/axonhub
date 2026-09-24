package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// newRetryGuardChannel builds a channel snapshot for retry-guard tests.
func newRetryGuardChannel(t *testing.T, id int, keys []string, disabled []objects.DisabledAPIKey) *biz.Channel {
	t.Helper()

	outbound, err := openai.NewOutboundTransformer("https://example.com", "test-key")
	require.NoError(t, err)

	return &biz.Channel{
		Channel: &ent.Channel{
			ID:              id,
			Name:            "retry-guard",
			BaseURL:         "https://example.com",
			Status:          channel.StatusEnabled,
			Credentials:     objects.ChannelCredentials{APIKeys: keys},
			DisabledAPIKeys: disabled,
		},
		Outbound: outbound,
	}
}

func newRetryGuardTransformer(channelService *biz.ChannelService, old *biz.Channel) *PersistentOutboundTransformer {
	transformer := &PersistentOutboundTransformer{
		state: &PersistenceState{
			ChannelService: channelService,
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: old,
				Models:  []biz.ChannelModelEntry{{ActualModel: "test-model"}},
			},
		},
	}
	transformer.wrapped = old.Outbound

	return transformer
}

func TestRefreshChannelBeforeRetry(t *testing.T) {
	_, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)

	tests := []struct {
		name        string
		currentKey  string
		freshKeys   []string
		disabled    []objects.DisabledAPIKey
		channelGone bool
		wantSwapped bool
	}{
		{
			name:        "credential still enabled leaves the candidate untouched",
			currentKey:  "key-1",
			freshKeys:   []string{"key-1", "key-2"},
			wantSwapped: false,
		},
		{
			name:        "credential in use was disabled swaps to the latest snapshot",
			currentKey:  "key-1",
			freshKeys:   []string{"key-1", "key-2"},
			disabled:    []objects.DisabledAPIKey{{Key: "key-1"}},
			wantSwapped: true,
		},
		{
			name:        "a different credential being disabled does not swap",
			currentKey:  "key-2",
			freshKeys:   []string{"key-1", "key-2"},
			disabled:    []objects.DisabledAPIKey{{Key: "key-1"}},
			wantSwapped: false,
		},
		{
			name:        "expired disable does not count as disabled",
			currentKey:  "key-1",
			freshKeys:   []string{"key-1"},
			disabled:    []objects.DisabledAPIKey{{Key: "key-1", ExpiresAt: lo.ToPtr(time.Now().Add(-time.Minute))}},
			wantSwapped: false,
		},
		{
			name:        "channel absent from the enabled set and unknown to the database keeps retrying",
			currentKey:  "key-1",
			channelGone: true,
			wantSwapped: false,
		},
		{
			name:        "oauth credential reference is skipped",
			currentKey:  objects.OAuthCredentialRef,
			freshKeys:   []string{"key-1"},
			wantSwapped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := newRetryGuardChannel(t, 1, []string{"key-1", "key-2"}, nil)

			if tt.channelGone {
				channelService.SetEnabledChannelsForTest(nil)
			} else {
				channelService.SetEnabledChannelsForTest([]*biz.Channel{
					newRetryGuardChannel(t, 1, tt.freshKeys, tt.disabled),
				})
			}

			transformer := newRetryGuardTransformer(channelService, old)

			ctx := contexts.EnsureContainer(context.Background())
			ctx = contexts.WithChannelAPIKey(ctx, tt.currentKey)

			require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))

			if tt.wantSwapped {
				require.NotSame(t, old, transformer.state.CurrentCandidate.Channel)
			} else {
				require.Same(t, old, transformer.state.CurrentCandidate.Channel)
			}
		})
	}
}

func TestRefreshChannelBeforeRetryChannelUnavailable(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)

	// A disabled channel is still authoritative in the database, so the retry
	// must stop instead of burning the same-channel budget on it.
	disabled := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("guard-disabled").
		SetBaseURL("https://example.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key-1"}}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusDisabled).
		SaveX(ctx)

	// An enabled channel missing from the shared cache must keep retrying:
	// absence alone is not proof it left service.
	cached := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("guard-enabled").
		SetBaseURL("https://example.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key-1"}}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		SaveX(ctx)

	// Enabled but every credential expired/disabled: unusable, and the only
	// state where ChannelService.GetChannel would panic while rebuilding.
	empty := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("guard-no-keys").
		SetBaseURL("https://example.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key-1"}}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		SaveX(ctx)

	// An expired temporary disable is not a disable.
	revived := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("guard-revived").
		SetBaseURL("https://example.com").
		SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key-1"}}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{
			Key:       "key-1",
			ExpiresAt: lo.ToPtr(time.Now().Add(-time.Minute)),
		}}).
		SaveX(ctx)

	client.Channel.UpdateOneID(empty.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "key-1"}}).
		SaveX(ctx)

	cases := []struct {
		name    string
		channel *ent.Channel
		wantErr bool
	}{
		{name: "disabled in database gives up", channel: disabled, wantErr: true},
		{name: "enabled but not cached keeps retrying", channel: cached, wantErr: false},
		{name: "enabled with no usable credential gives up", channel: empty, wantErr: true},
		{name: "expired disable keeps retrying", channel: revived, wantErr: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			channelService.SetEnabledChannelsForTest(nil)

			old := newRetryGuardChannel(t, tc.channel.ID, []string{"key-1"}, nil)
			transformer := newRetryGuardTransformer(channelService, old)

			// The database read is privacy-guarded, so the guard runs with the same
			// authorization bypass the orchestrator installs for a real request.
			guardCtx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
			guardCtx = contexts.EnsureContainer(guardCtx)
			guardCtx = contexts.WithChannelAPIKey(guardCtx, "key-1")

			err := transformer.refreshChannelBeforeRetry(guardCtx)
			if tc.wantErr {
				require.ErrorIs(t, err, errChannelUnavailableForRetry)
				require.Same(t, old, transformer.state.CurrentCandidate.Channel)

				return
			}

			require.NoError(t, err)
			require.Same(t, old, transformer.state.CurrentCandidate.Channel)
		})
	}

	// Without a database in the context the guard must stay silent rather than
	// guess: a degraded read can never cancel a legitimate retry.
	t.Run("no database client keeps retrying", func(t *testing.T) {
		channelService.SetEnabledChannelsForTest(nil)

		old := newRetryGuardChannel(t, disabled.ID, []string{"key-1"}, nil)
		transformer := newRetryGuardTransformer(channelService, old)

		bareCtx := contexts.EnsureContainer(context.Background())
		bareCtx = contexts.WithChannelAPIKey(bareCtx, "key-1")

		require.NoError(t, transformer.refreshChannelBeforeRetry(bareCtx))
		require.Same(t, old, transformer.state.CurrentCandidate.Channel)
	})
}
