package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

// createRetryGuardChannel stores an enabled OpenAI channel in the database.
func createRetryGuardChannel(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	name string,
	credentials objects.ChannelCredentials,
) *ent.Channel {
	t.Helper()

	return client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL("https://example.com").
		SetCredentials(credentials).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		SaveX(ctx)
}

// retryGuardSnapshot takes a channel snapshot the way a request does when its
// candidates are selected, i.e. from the stored channel as it is right now.
func retryGuardSnapshot(t *testing.T, ctx context.Context, svc *biz.ChannelService, id int) *biz.Channel {
	t.Helper()

	snapshot, err := svc.GetChannel(ctx, id)
	require.NoError(t, err)

	return snapshot
}

// pinnedRetryGuardChannel builds a snapshot that is not backed by the database,
// for the cases where the guard must keep its hands off (a channel that never
// existed in the database, a channel without credentials).
func pinnedRetryGuardChannel(t *testing.T, id int, keys []string) *biz.Channel {
	t.Helper()

	outbound, err := openai.NewOutboundTransformer("https://example.com", "stale-key")
	require.NoError(t, err)

	credentials := objects.ChannelCredentials{}
	if len(keys) > 0 {
		credentials.APIKeys = keys
	}

	return &biz.Channel{
		Channel: &ent.Channel{
			ID:          id,
			Name:        "guard-pinned",
			BaseURL:     "https://example.com",
			Status:      channel.StatusEnabled,
			Credentials: credentials,
		},
		Outbound: outbound,
	}
}

// newRetryGuardTransformer wires a transformer whose current candidate is the
// given snapshot. failedCredential mirrors what the performance middleware
// records for the attempt that is being retried.
func newRetryGuardTransformer(
	channelService *biz.ChannelService,
	snapshot *biz.Channel,
	failedCredential string,
) *PersistentOutboundTransformer {
	transformer := &PersistentOutboundTransformer{
		state: &PersistenceState{
			ChannelService: channelService,
			CurrentCandidate: &ChannelModelsCandidate{
				Channel: snapshot,
				Models:  []biz.ChannelModelEntry{{ActualModel: "test-model"}},
			},
		},
	}
	if failedCredential != "" {
		transformer.state.Perf = &biz.PerformanceRecord{APIKey: failedCredential}
	}
	transformer.wrapped = snapshot.Outbound

	return transformer
}

// authorizationCredential returns the credential the channel's outbound
// transformer would authenticate the retried request with. It is what the HTTP
// client turns into the Authorization header, so it asserts the real credential
// rather than the candidate bookkeeping.
func authorizationCredential(t *testing.T, ctx context.Context, snapshot *biz.Channel) string {
	t.Helper()

	httpReq, err := snapshot.Outbound.TransformRequest(ctx, &llm.Request{
		Model:    "test-model",
		Messages: []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hi")}}},
	})
	require.NoError(t, err)
	require.NotNil(t, httpReq.Auth)

	return httpReq.Auth.APIKey
}

func TestRefreshChannelBeforeRetry(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)

	t.Run("credential still enabled keeps the candidate untouched", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-live",
			objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}})
		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)
		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("credential disabled after the snapshot switches the retry to an enabled one", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-key-disabled",
			objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}})
		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)

		// The disable record is committed by the asynchronous performance worker
		// after the snapshot was pinned. The guard reads the stored state, so a
		// committed invalidation is always observed by the next attempt.
		client.Channel.UpdateOneID(entity.ID).
			SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "key-1"}}).
			SaveX(ctx)

		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		// Go through the pipeline entry point: the guard is only useful while
		// PrepareForRetry keeps calling it.
		require.NoError(t, transformer.PrepareForRetry(ctx))

		refreshed := transformer.state.CurrentCandidate.Channel
		require.NotSame(t, snapshot, refreshed)
		require.Equal(t, []string{"key-2"}, refreshed.GetEnabledAPIKeys())
		// The retry must authenticate with the enabled credential, not merely point
		// at another snapshot.
		require.Equal(t, "key-2", authorizationCredential(t, ctx, refreshed))
	})

	t.Run("credential from the context is preferred over the failed attempt record", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-context-key",
			objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}})
		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)

		client.Channel.UpdateOneID(entity.ID).
			SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "key-2"}}).
			SaveX(ctx)

		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		requestCtx := contexts.WithChannelAPIKey(ctx, "key-2")

		require.NoError(t, transformer.refreshChannelBeforeRetry(requestCtx))

		refreshed := transformer.state.CurrentCandidate.Channel
		require.NotSame(t, snapshot, refreshed)
		require.Equal(t, []string{"key-1"}, refreshed.GetEnabledAPIKeys())
	})

	t.Run("credential replaced while the request was in flight switches to the new one", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-credential-replaced",
			objects.ChannelCredentials{APIKeys: []string{"key-1"}})
		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)

		client.Channel.UpdateOneID(entity.ID).
			SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key-3"}}).
			SaveX(ctx)

		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))

		refreshed := transformer.state.CurrentCandidate.Channel
		require.NotSame(t, snapshot, refreshed)
		require.Equal(t, []string{"key-3"}, refreshed.GetEnabledAPIKeys())
		require.Equal(t, "key-3", authorizationCredential(t, ctx, refreshed))
	})

	t.Run("another credential being disabled keeps the candidate untouched", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-other-key-disabled",
			objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}})
		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)

		client.Channel.UpdateOneID(entity.ID).
			SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "key-2"}}).
			SaveX(ctx)

		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("expired disable records keep the candidate untouched", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-expired-disable",
			objects.ChannelCredentials{APIKeys: []string{"key-1"}})

		client.Channel.UpdateOneID(entity.ID).
			SetDisabledAPIKeys([]objects.DisabledAPIKey{{
				Key:       "key-1",
				ExpiresAt: lo.ToPtr(time.Now().Add(-time.Minute)),
			}}).
			SaveX(ctx)

		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)
		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("oauth channel with an intact credential reference keeps retrying", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-oauth",
			objects.ChannelCredentials{APIKey: `{"access_token":"oauth-token","refresh_token":"r","expires_at":"2099-01-01T00:00:00Z"}`})
		snapshot := pinnedRetryGuardChannel(t, entity.ID, []string{"oauth-token"})

		// An OAuth channel has no API keys, yet it is a one-credential channel. The
		// credential reference must be compared, otherwise the channel looks like it
		// has no usable credential at all.
		transformer := newRetryGuardTransformer(channelService, snapshot, objects.OAuthCredentialRef)

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("channel served without credentials keeps retrying", func(t *testing.T) {
		snapshot := pinnedRetryGuardChannel(t, 1, nil)
		transformer := newRetryGuardTransformer(channelService, snapshot, "")

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("channel missing from the enabled cache keeps retrying", func(t *testing.T) {
		entity := createRetryGuardChannel(t, ctx, client, "guard-not-cached",
			objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}})
		snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)

		// The shared cache may not have loaded the channel yet (or at all). The
		// guard reads the stored channel, so a cache miss must not end the retry.
		channelService.SetEnabledChannelsForTest(nil)

		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})
}

func TestRefreshChannelBeforeRetryChannelUnavailable(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)

	disabled := createRetryGuardChannel(t, ctx, client, "guard-disabled",
		objects.ChannelCredentials{APIKeys: []string{"key-1"}})
	client.Channel.UpdateOneID(disabled.ID).SetStatus(channel.StatusDisabled).SaveX(ctx)

	archived := createRetryGuardChannel(t, ctx, client, "guard-archived",
		objects.ChannelCredentials{APIKeys: []string{"key-1"}})
	client.Channel.UpdateOneID(archived.ID).SetStatus(channel.StatusArchived).SaveX(ctx)

	deleted := createRetryGuardChannel(t, ctx, client, "guard-deleted",
		objects.ChannelCredentials{APIKeys: []string{"key-1"}})

	// A soft delete keeps the row but filters it out of normal queries, so the
	// lookup reports not-found. That is a confirmed removal, not a transient
	// failure, and the retry must stop instead of hitting a deleted channel again.
	require.NoError(t, client.Channel.DeleteOneID(deleted.ID).Exec(ctx))

	// Every credential is disabled while the channel itself is still marked
	// enabled: credentials can be replaced while a request is in flight. The
	// candidate was credentialed and has nothing usable left, so the retry stops
	// instead of reusing a credential the channel no longer accepts.
	empty := createRetryGuardChannel(t, ctx, client, "guard-no-usable-credential",
		objects.ChannelCredentials{APIKeys: []string{"key-1"}})
	client.Channel.UpdateOneID(empty.ID).
		SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "key-1"}}).
		SaveX(ctx)

	cases := []struct {
		name    string
		channel *ent.Channel
		wantErr bool
	}{
		{name: "disabled in the database ends the retry", channel: disabled, wantErr: true},
		{name: "archived in the database ends the retry", channel: archived, wantErr: true},
		{name: "soft-deleted channel ends the retry", channel: deleted, wantErr: true},
		{name: "channel left without a usable credential ends the retry", channel: empty, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := pinnedRetryGuardChannel(t, tc.channel.ID, []string{"key-1"})
			transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

			err := transformer.refreshChannelBeforeRetry(ctx)
			if tc.wantErr {
				require.ErrorIs(t, err, errChannelUnavailableForRetry)
				require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)

				return
			}

			require.NoError(t, err)
			require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
		})
	}

	t.Run("candidate that is not in service is left alone", func(t *testing.T) {
		snapshot := pinnedRetryGuardChannel(t, disabled.ID, []string{"key-1"})
		snapshot.Status = channel.StatusDisabled
		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		// Only a channel that was in service can leave it. Upstream keeps retrying
		// a candidate that is not enabled, and the guard must not change that.
		require.NoError(t, transformer.refreshChannelBeforeRetry(ctx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("channel unknown to the database ends the retry", func(t *testing.T) {
		snapshot := pinnedRetryGuardChannel(t, 4242, []string{"key-1"})
		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		err := transformer.refreshChannelBeforeRetry(ctx)
		require.ErrorIs(t, err, errChannelUnavailableForRetry)
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("no database client keeps retrying", func(t *testing.T) {
		snapshot := pinnedRetryGuardChannel(t, disabled.ID, []string{"key-1"})
		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		// A degraded read can never cancel an otherwise legitimate retry.
		bareCtx := contexts.EnsureContainer(context.Background())

		require.NoError(t, transformer.refreshChannelBeforeRetry(bareCtx))
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})

	t.Run("a refresh that cannot be rebuilt ends the retry", func(t *testing.T) {
		// A github-copilot channel is only usable with OAuth credentials, so the
		// stored channel cannot be rebuilt into an outbound transformer even though
		// its credentials still list another usable API key. The candidate can only
		// keep authenticating with the credential that was just disabled, so the
		// retry stops instead of spending the remaining budget on it.
		entity := client.Channel.Create().
			SetType(channel.TypeGithubCopilot).
			SetName("guard-unbuildable").
			SetBaseURL("https://example.com").
			SetCredentials(objects.ChannelCredentials{APIKeys: []string{"key-1", "key-2"}}).
			SetSupportedModels([]string{"test-model"}).
			SetDefaultTestModel("test-model").
			SetStatus(channel.StatusEnabled).
			SaveX(ctx)

		client.Channel.UpdateOneID(entity.ID).
			SetDisabledAPIKeys([]objects.DisabledAPIKey{{Key: "key-1"}}).
			SaveX(ctx)

		snapshot := pinnedRetryGuardChannel(t, entity.ID, []string{"key-1"})
		transformer := newRetryGuardTransformer(channelService, snapshot, "key-1")

		err := transformer.refreshChannelBeforeRetry(ctx)
		require.ErrorIs(t, err, errChannelUnavailableForRetry)
		require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
	})
}

// TestRefreshChannelBeforeRetry_CredentiallessChannelAfterKeyedChannel reproduces
// the case greptile flagged on PR #2545: a request starts on an API-key channel,
// which leaves its key in the shared request context, then fails over to a channel
// that authenticates without any credential (a local Ollama). Such a channel never
// rewrites the context key, so the leftover key survives into its same-channel
// retry. The guard must not read that leftover as this channel's credential: doing
// so marks the credentialless channel as unavailable and drops its remaining
// retries even though it is perfectly healthy.
func TestRefreshChannelBeforeRetry_CredentiallessChannelAfterKeyedChannel(t *testing.T) {
	ctx, client := setupTest(t)

	channelService := newTestChannelServiceForChannels(client)

	// A local Ollama deployment: enabled and reachable, deliberately without
	// credentials.
	entity := client.Channel.Create().
		SetType(channel.TypeOllama).
		SetName("guard-ollama-no-key").
		SetBaseURL("http://localhost:11434").
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"test-model"}).
		SetDefaultTestModel("test-model").
		SetStatus(channel.StatusEnabled).
		SaveX(ctx)

	snapshot := retryGuardSnapshot(t, ctx, channelService, entity.ID)

	// This attempt authenticated with no credential at all, so nothing was
	// recorded for it.
	transformer := newRetryGuardTransformer(channelService, snapshot, "")

	// The previous candidate was an API-key channel and its key is still in the
	// shared request context, because the credentialless channel never rewrote it.
	requestCtx := contexts.WithChannelAPIKey(ctx, "key-1")

	require.NoError(t, transformer.refreshChannelBeforeRetry(requestCtx))
	require.Same(t, snapshot, transformer.state.CurrentCandidate.Channel)
}
