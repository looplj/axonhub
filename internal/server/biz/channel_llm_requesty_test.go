package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/requesty"
)

func TestRequestyChannel_ConfiguredEndpointsKeepRequestyTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Requesty Configured Endpoints").
		SetType(channel.TypeRequesty).
		SetBaseURL("https://router.requesty.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"openai/gpt-4o-mini"}).
		SetDefaultTestModel("openai/gpt-4o-mini").
		SetEndpoints([]objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), BaseURL: "https://router.eu.requesty.ai/v1"},
			{APIFormat: llm.APIFormatOpenAIImageGeneration.String()},
			{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
		}).
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	for _, apiFormat := range []llm.APIFormat{llm.APIFormatOpenAIChatCompletion, llm.APIFormatOpenAIImageGeneration} {
		outbound, err := BuildOutboundByAPIFormat(built, apiFormat.String())
		require.NoError(t, err)

		_, ok := outbound.(*requesty.OutboundTransformer)
		require.True(t, ok, "configured %s endpoint should keep the requesty transformer", apiFormat)
	}

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.NoError(t, err)

	_, ok := outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "embeddings go straight to /embeddings through the openai transformer")
}

func TestRequestyChannel_ExplicitPathUsesGenericTransformer(t *testing.T) {
	ep := objects.ChannelEndpoint{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "/custom/chat"}
	require.False(t, isRequestyChatEndpoint(channel.TypeRequesty, ep))
	require.False(t, isRequestyChatEndpoint(channel.TypeOpenrouter, objects.ChannelEndpoint{APIFormat: ep.APIFormat}))
	require.True(t, isRequestyChatEndpoint(channel.TypeRequesty, objects.ChannelEndpoint{APIFormat: ep.APIFormat}))
}
