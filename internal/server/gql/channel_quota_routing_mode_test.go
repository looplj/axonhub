package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestChannelQuotaRoutingModeGraphQLRoundTrip(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	service := biz.NewChannelServiceForTest(client)
	resolver := &mutationResolver{&Resolver{channelService: service}}

	created, err := resolver.CreateChannel(ctx, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Quota routing source",
		Credentials:      objects.ChannelCredentials{APIKey: "source-key"},
		SupportedModels:  []string{"test-model"},
		DefaultTestModel: "test-model",
		Settings: &objects.ChannelSettings{
			QuotaRoutingMode: objects.QuotaRoutingModeRemoveOnExhausted,
		},
	})
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, created.Settings.QuotaRoutingMode)

	read, err := client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeRemoveOnExhausted, read.Settings.QuotaRoutingMode)

	updated, err := resolver.UpdateChannel(ctx, objects.GUID{Type: "Channel", ID: created.ID}, ent.UpdateChannelInput{
		Settings: &objects.ChannelSettings{QuotaRoutingMode: objects.QuotaRoutingModeBackpressure},
	})
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeBackpressure, updated.Settings.QuotaRoutingMode)

	read, err = client.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeBackpressure, read.Settings.QuotaRoutingMode)

	duplicated, err := resolver.DuplicateChannel(ctx, objects.GUID{Type: "Channel", ID: created.ID}, ent.CreateChannelInput{
		Type:             channel.TypeOpenai,
		Name:             "Quota routing duplicate",
		Credentials:      objects.ChannelCredentials{APIKey: "duplicate-key"},
		SupportedModels:  []string{"test-model"},
		DefaultTestModel: "test-model",
		Settings: &objects.ChannelSettings{
			QuotaRoutingMode: objects.QuotaRoutingModeBackpressure,
		},
	})
	require.NoError(t, err)
	require.Equal(t, objects.QuotaRoutingModeBackpressure, duplicated.Settings.QuotaRoutingMode)

	var encoded bytes.Buffer
	objects.QuotaRoutingMode("").MarshalGQL(&encoded)
	require.Equal(t, `"INHERIT"`, encoded.String())

	stored, err := json.Marshal(&objects.ChannelSettings{QuotaRoutingMode: ""})
	require.NoError(t, err)
	var storedFields map[string]any
	require.NoError(t, json.Unmarshal(stored, &storedFields))
	_, present := storedFields["quotaRoutingMode"]
	require.False(t, present)
}

func TestChannelQuotaRoutingModeGraphQLRejectsBogusValue(t *testing.T) {
	input, err := (&executionContext{}).unmarshalInputChannelSettingsInput(t.Context(), map[string]any{
		"quotaRoutingMode": "BOGUS",
	})
	require.Error(t, err)
	require.Equal(t, objects.ChannelSettings{}, input)

	var mode objects.QuotaRoutingMode
	err = mode.UnmarshalGQL("BOGUS")
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid QuotaRoutingMode")
}
