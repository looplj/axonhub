package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/xai"
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
