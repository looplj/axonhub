package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/xai"
	"github.com/looplj/axonhub/llm/transformer/xai/subscription"
)

func TestXaiChannel_TypeXaiKeepsChatCompletionsTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("xAI Chat Channel").
		SetType(channel.TypeXai).
		SetBaseURL("https://api.x.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"grok-4"}).
		SetDefaultTestModel("grok-4").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*xai.OutboundTransformer)
	require.True(t, ok, "TypeXai should create xai.OutboundTransformer")
}

func TestXaiChannel_CreateResponsesTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("xAI Responses Channel").
		SetType(channel.TypeXaiResponses).
		SetBaseURL("https://api.x.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"grok-4"}).
		SetDefaultTestModel("grok-4").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*responses.OutboundTransformer)
	require.True(t, ok, "TypeXaiResponses should create responses.OutboundTransformer")
}

func TestXaiChannel_VerifyAPIFormat(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	channelSvc := NewChannelServiceForTest(client)

	t.Run("TypeXaiResponses returns OpenAI Responses format", func(t *testing.T) {
		entChannel := client.Channel.Create().
			SetName("xAI Responses").
			SetType(channel.TypeXaiResponses).
			SetBaseURL("https://api.x.ai/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"grok-4"}).
			SetDefaultTestModel("grok-4").
			SaveX(ctx)

		built, err := channelSvc.buildChannelWithTransformer(entChannel)
		require.NoError(t, err)
		require.Equal(t, "openai/responses", string(built.Outbound.APIFormat()))
	})
}

func TestXaiChannel_APIKeyBuildsDistinctResponsesPeerOutbound(t *testing.T) {
	// Given
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())
	entity := client.Channel.Create().
		SetName("xAI API key peer").
		SetType(channel.TypeXai).
		SetBaseURL("https://api.x.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "synthetic-key"}).
		SetSupportedModels([]string{"grok-4.5"}).
		SetDefaultTestModel("grok-4.5").
		SaveX(ctx)

	// When
	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)

	// Then
	require.NoError(t, err)
	peer, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIResponse.String())
	require.NoError(t, err)
	require.IsType(t, &responses.OutboundTransformer{}, peer)
	require.NotSame(t, built.Outbound, peer)
}

func TestXaiSubscriptionChannel_BuildsOfficialResponsesOutbound(t *testing.T) {
	// Given
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())
	entity := client.Channel.Create().
		SetName("xAI subscription").
		SetType(channel.TypeXaiSubscription).
		SetBaseURL("https://attacker.example/v1").
		SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
			AccessToken:  "synthetic-token",
			RefreshToken: "synthetic-refresh",
			ClientID:     subscription.ClientID,
			ExpiresAt:    time.Now().Add(time.Hour),
		}}).
		SetSupportedModels([]string{"grok-4.5"}).
		SetDefaultTestModel("grok-4.5").
		SaveX(ctx)

	// When
	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)

	// Then
	require.NoError(t, err)
	require.IsType(t, &subscription.OutboundTransformer{}, built.Outbound)
	request, err := built.Outbound.TransformRequest(ctx, &llm.Request{Model: "grok-4.5"})
	require.NoError(t, err)
	require.Equal(t, subscription.DefaultBaseURL+"/responses", request.URL)
	require.Equal(t, "synthetic-token", request.Auth.APIKey)
}

func TestXaiSubscriptionChannel_EmptyOAuthObjectFallsBackToLegacyJSON(t *testing.T) {
	// Given
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())
	entity := client.Channel.Create().
		SetName("legacy xAI subscription").
		SetType(channel.TypeXaiSubscription).
		SetBaseURL(subscription.DefaultBaseURL).
		SetCredentials(objects.ChannelCredentials{
			OAuth:  &objects.OAuthCredentials{},
			APIKey: `{"access_token":"legacy-token","refresh_token":"legacy-refresh"}`,
		}).
		SetSupportedModels([]string{"grok-4.5"}).
		SetDefaultTestModel("grok-4.5").
		SaveX(ctx)

	// When
	built, err := NewChannelServiceForTest(client).buildChannelWithTransformer(entity)

	// Then
	require.NoError(t, err)
	request, err := built.Outbound.TransformRequest(ctx, &llm.Request{Model: "grok-4.5"})
	require.NoError(t, err)
	require.Equal(t, "legacy-token", request.Auth.APIKey)
}

func TestXaiSubscriptionChannel_RejectsPlainAPIKeyCredentials(t *testing.T) {
	// Given
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())
	entity := client.Channel.Create().
		SetName("invalid xAI subscription").
		SetType(channel.TypeXaiSubscription).
		SetBaseURL(subscription.DefaultBaseURL).
		SetCredentials(objects.ChannelCredentials{APIKey: "plain-api-key"}).
		SetSupportedModels([]string{"grok-4.5"}).
		SetDefaultTestModel("grok-4.5").
		SaveX(ctx)

	// When
	_, err := NewChannelServiceForTest(client).buildChannelWithTransformer(entity)

	// Then
	require.ErrorContains(t, err, "missing oauth credentials")
}
