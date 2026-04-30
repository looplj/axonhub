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
	geminitransformer "github.com/looplj/axonhub/llm/transformer/gemini"
)

func TestGeminiChannel_BuildChannelWithOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Gemini Multi Endpoint Channel").
		SetType(channel.TypeGemini).
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.5-pro").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)
	require.Len(t, built.Outbounds, 2)

	require.Equal(t, llm.APIFormatGeminiContents, built.Outbound.APIFormat())

	embeddingOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatGeminiEmbedding.String())
	require.NoError(t, err)
	require.NotNil(t, embeddingOutbound)
	_, ok := embeddingOutbound.(*geminitransformer.OutboundTransformer)
	require.True(t, ok)
}
