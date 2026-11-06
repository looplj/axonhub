package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/events"
)

func TestChannelCredentialDisabledEvent_NilBusAndNoSubscriber(t *testing.T) {
	secret := "sk-nil-bus-credential-secret"

	t.Run("nil bus", func(t *testing.T) {
		svc, client, ctx, ch := setupChannelEventFixture(t, []string{secret, "other-key"})
		svc.EventBus = nil

		require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, secret, 401, "invalid key"))

		updated := reloadChannel(t, ctx, client, ch.ID)
		require.Len(t, updated.DisabledAPIKeys, 1)
		require.Equal(t, secret, updated.DisabledAPIKeys[0].Key)
		require.Equal(t, channel.StatusEnabled, updated.Status)
	})

	t.Run("no subscriber", func(t *testing.T) {
		svc, client, ctx, ch := setupChannelEventFixture(t, []string{secret, "other-key"})
		bus := events.NewBus()
		t.Cleanup(func() {
			require.NoError(t, bus.Close(context.Background()))
		})
		svc.EventBus = bus

		require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, secret, 401, "invalid key"))

		updated := reloadChannel(t, ctx, client, ch.ID)
		require.Len(t, updated.DisabledAPIKeys, 1)
		require.Equal(t, secret, updated.DisabledAPIKeys[0].Key)
		require.Equal(t, int64(0), bus.Stats().Published)
		require.False(t, bus.HasSubscribers(events.TypeChannelCredentialDisabled))
	})
}

func TestChannelCredentialDisabledEvent_SubscriberGetsFingerprint(t *testing.T) {
	secret := "sk-live-credential-plaintext-secret"
	expires := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	svc, _, ctx, ch := setupChannelEventFixture(t, []string{secret, "other-key"})
	bus := events.NewBus()
	t.Cleanup(func() {
		require.NoError(t, bus.Close(context.Background()))
	})
	svc.EventBus = bus
	received := subscribeEvent(t, bus, events.TypeChannelCredentialDisabled)

	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, secret, 401, "invalid key", &expires))

	event := requireEvent(t, received)
	require.Equal(t, events.TypeChannelCredentialDisabled, event.Type)
	require.Equal(t, events.SeverityWarning, event.Severity)
	require.NotEmpty(t, event.ID)
	require.Equal(t, time.UTC, event.OccurredAt.Location())
	require.False(t, event.OccurredAt.IsZero())
	require.Empty(t, event.Origin)
	require.Equal(t, 0, event.Depth)
	require.NotNil(t, event.Subject.Channel)
	require.Equal(t, ch.ID, event.Subject.Channel.ID)
	require.Equal(t, ch.Name, event.Subject.Channel.Name)
	require.Equal(t, channel.TypeOpenai.String(), event.Subject.Channel.Type)
	require.Equal(t, ch.BaseURL, event.Subject.Channel.BaseURL)
	require.Equal(t, channel.StatusEnabled.String(), event.Subject.Channel.Status)
	require.NotNil(t, event.Subject.Credential)
	require.Equal(t, events.Fingerprint(secret), event.Subject.Credential.Fingerprint)
	require.Equal(t, events.Mask(secret), event.Subject.Credential.Masked)
	require.Equal(t, 401, event.Data["errorCode"])
	require.Equal(t, "invalid key", event.Data["reason"])
	require.Equal(t, expires.Format(time.RFC3339), event.Data["expiresAt"])
	requireNoSecret(t, event, secret)
}

func TestChannelCredentialDisabledEvent_OAuthDoesNotHashToken(t *testing.T) {
	token := "super-secret-oauth-access-token"
	client := enttest.NewEntClient(t, "sqlite3", channelEventDSN(t))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestOAuthChannel(t, client, ctx, "codex-oauth-events")
	_, err := client.Channel.UpdateOneID(ch.ID).
		SetCredentials(objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{AccessToken: token, RefreshToken: "refresh-token-secret"},
		}).
		Save(ctx)
	require.NoError(t, err)

	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	svc.EventBus = bus
	received := subscribeEvent(t, bus, events.TypeChannelCredentialDisabled)

	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, objects.OAuthCredentialRef, 401, "oauth rejected"))

	event := requireEvent(t, received)
	require.NotNil(t, event.Subject.Credential)
	require.Equal(t, events.OAuthFingerprint, event.Subject.Credential.Fingerprint)
	require.Equal(t, events.OAuthFingerprint, event.Subject.Credential.Masked)
	require.NotEqual(t, events.Fingerprint(token), event.Subject.Credential.Fingerprint)
	require.Equal(t, channel.StatusDisabled.String(), event.Subject.Channel.Status)
	requireNoSecret(t, event, token)
	requireNoSecret(t, event, "refresh-token-secret")
	require.Nil(t, event.Data["expiresAt"])
}

func TestChannelCredentialEnabledEvent_ManualAndExpired(t *testing.T) {
	secret := "sk-enabled-credential-plaintext"
	svc, client, ctx, ch := setupChannelEventFixture(t, []string{secret, "other-key"})
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	svc.EventBus = bus
	received := subscribeEvent(t, bus, events.TypeChannelCredentialEnabled)

	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, secret, 429, "rate limit"))
	require.NoError(t, svc.EnableAPIKey(ctx, ch.ID, secret))

	event := requireEvent(t, received)
	require.Equal(t, events.TypeChannelCredentialEnabled, event.Type)
	require.Equal(t, events.SeverityInfo, event.Severity)
	require.Equal(t, "manual", event.Data["trigger"])
	require.Equal(t, events.Fingerprint(secret), event.Subject.Credential.Fingerprint)
	require.Equal(t, events.Mask(secret), event.Subject.Credential.Masked)
	require.Equal(t, ch.ID, event.Subject.Channel.ID)
	requireNoSecret(t, event, secret)

	updated := reloadChannel(t, ctx, client, ch.ID)
	require.Empty(t, updated.DisabledAPIKeys)

	past := time.Now().Add(-time.Minute)
	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, secret, 429, "rate limit", &past))
	cleaned, removed, err := svc.cleanupChannelExpiredDisabledAPIKeys(ctx, ch.ID)
	require.NoError(t, err)
	require.True(t, cleaned)
	require.Equal(t, 1, removed)

	expiredEvent := requireEvent(t, received)
	require.Equal(t, "expired", expiredEvent.Data["trigger"])
	require.Equal(t, events.Fingerprint(secret), expiredEvent.Subject.Credential.Fingerprint)
	requireNoSecret(t, expiredEvent, secret)
	require.Empty(t, reloadChannel(t, ctx, client, ch.ID).DisabledAPIKeys)
}

func TestChannelAutoEnabledEvent_NilBusNoSubscriberAndDelivery(t *testing.T) {
	t.Run("nil bus", func(t *testing.T) {
		svc, client, ctx, ch := setupChannelEventFixture(t, []string{"key-a"})
		svc.EventBus = nil
		markAutoDisableExpired(t, ctx, client, ch.ID)

		require.True(t, svc.cleanupExpiredAutoDisabledChannels(ctx))
		require.Equal(t, channel.StatusEnabled, reloadChannel(t, ctx, client, ch.ID).Status)
	})

	t.Run("no subscriber", func(t *testing.T) {
		svc, client, ctx, ch := setupChannelEventFixture(t, []string{"key-a"})
		bus := events.NewBus()
		t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
		svc.EventBus = bus
		markAutoDisableExpired(t, ctx, client, ch.ID)

		require.True(t, svc.cleanupExpiredAutoDisabledChannels(ctx))
		require.Equal(t, channel.StatusEnabled, reloadChannel(t, ctx, client, ch.ID).Status)
		require.Equal(t, int64(0), bus.Stats().Published)
	})

	t.Run("subscriber", func(t *testing.T) {
		svc, client, ctx, ch := setupChannelEventFixture(t, []string{"key-a"})
		bus := events.NewBus()
		t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
		svc.EventBus = bus
		received := subscribeEvent(t, bus, events.TypeChannelAutoEnabled)
		markAutoDisableExpired(t, ctx, client, ch.ID)

		require.True(t, svc.cleanupExpiredAutoDisabledChannels(ctx))

		event := requireEvent(t, received)
		require.Equal(t, events.TypeChannelAutoEnabled, event.Type)
		require.Equal(t, events.SeverityInfo, event.Severity)
		require.NotEmpty(t, event.ID)
		require.Empty(t, event.Origin)
		require.Equal(t, 0, event.Depth)
		require.Nil(t, event.Subject.Credential)
		require.Equal(t, ch.ID, event.Subject.Channel.ID)
		require.Equal(t, ch.Name, event.Subject.Channel.Name)
		require.Equal(t, channel.TypeOpenai.String(), event.Subject.Channel.Type)
		require.Equal(t, ch.BaseURL, event.Subject.Channel.BaseURL)
		require.Equal(t, channel.StatusEnabled.String(), event.Subject.Channel.Status)
		require.Empty(t, event.Data)
		require.Equal(t, channel.StatusEnabled, reloadChannel(t, ctx, client, ch.ID).Status)
	})
}

func TestChannelCredentialDisabledEvent_PluginOrigin(t *testing.T) {
	secret := "sk-plugin-origin-key"
	svc, _, ctx, ch := setupChannelEventFixture(t, []string{secret})
	bus := events.NewBus()
	t.Cleanup(func() { require.NoError(t, bus.Close(context.Background())) })
	svc.EventBus = bus
	received := subscribeEvent(t, bus, events.TypeChannelCredentialDisabled)

	ctx = WithPluginEventMeta(ctx, "plugin:acme.guard", 2)
	require.NoError(t, svc.DisableAPIKey(ctx, ch.ID, secret, 401, "plugin action"))

	event := requireEvent(t, received)
	require.Equal(t, "plugin:acme.guard", event.Origin)
	require.Equal(t, 2, event.Depth)
	require.Equal(t, events.Fingerprint(secret), event.Subject.Credential.Fingerprint)
	requireNoSecret(t, event, secret)
}

func setupChannelEventFixture(t *testing.T, keys []string) (*ChannelService, *ent.Client, context.Context, *ent.Channel) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", channelEventDSN(t))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := newTestChannelService(client)
	ch := createTestChannelWithAPIKeys(t, client, ctx, "event-channel", keys)
	return svc, client, ctx, ch
}

func markAutoDisableExpired(t *testing.T, ctx context.Context, client *ent.Client, channelID int) {
	t.Helper()
	past := time.Now().Add(-time.Minute)
	_, err := client.Channel.UpdateOneID(channelID).
		SetStatus(channel.StatusDisabled).
		SetErrorMessage("temporarily disabled").
		SetAutoDisabledAt(past).
		SetAutoDisableExpiresAt(past).
		Save(ctx)
	require.NoError(t, err)
}

func reloadChannel(t *testing.T, ctx context.Context, client *ent.Client, channelID int) *ent.Channel {
	t.Helper()
	updated, err := client.Channel.Get(ctx, channelID)
	require.NoError(t, err)
	return updated
}

func channelEventDSN(t *testing.T) string {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	return fmt.Sprintf("file:%s?mode=memory&cache=private&_fk=0", name)
}

func subscribeEvent(t *testing.T, bus *events.Bus, eventType string) <-chan events.Event {
	t.Helper()
	received := make(chan events.Event, 4)
	bus.Subscribe("test-"+eventType, []string{eventType}, func(_ context.Context, event events.Event) {
		received <- event
	})
	return received
}

func requireEvent(t *testing.T, received <-chan events.Event) events.Event {
	t.Helper()
	select {
	case event := <-received:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for plugin event")
		return events.Event{}
	}
}

func requireNoSecret(t *testing.T, event events.Event, secret string) {
	t.Helper()
	encoded, err := json.Marshal(event.Map())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), secret)
	require.NotContains(t, fmt.Sprintf("%#v", event), secret)
}
