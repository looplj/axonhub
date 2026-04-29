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
)

func TestOpenAICompatibleChannel_BuildChannelWithOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Vercel Multi Endpoint Channel").
		SetType(channel.TypeVercel).
		SetBaseURL("https://ai-gateway.vercel.sh/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4o-mini"}).
		SetDefaultTestModel("gpt-4o-mini").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)
	require.Len(t, built.Outbounds, 5)

	require.Equal(t, llm.APIFormatOpenAIChatCompletion, built.Outbound.APIFormat())

	embeddingOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatOpenAIEmbedding, embeddingOutbound.APIFormat())
	_, ok := embeddingOutbound.(*openai.EmbeddingOutboundTransformer)
	require.True(t, ok)

	imageOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIImageGeneration.String())
	require.NoError(t, err)
	require.Equal(t, llm.APIFormatOpenAIImageGeneration, imageOutbound.APIFormat())
}
