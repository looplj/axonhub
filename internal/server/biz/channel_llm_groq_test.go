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

func TestGroqChannel_TypeGroq(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Groq Channel").
		SetType(channel.TypeGroq).
		SetBaseURL("https://api.groq.com/openai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"openai/gpt-oss-120b"}).
		SetDefaultTestModel("openai/gpt-oss-120b").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "TypeGroq should create openai.OutboundTransformer")

	require.Equal(t, llm.APIFormatOpenAIChatCompletion, built.Outbound.APIFormat())
}

func TestGroqChannel_BuildChannelWithOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Groq Multi Endpoint Channel").
		SetType(channel.TypeGroq).
		SetBaseURL("https://api.groq.com/openai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"openai/gpt-oss-120b", "whisper-large-v3"}).
		SetDefaultTestModel("openai/gpt-oss-120b").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	apiFormats := []llm.APIFormat{
		llm.APIFormatOpenAIChatCompletion,
		llm.APIFormatOpenAISpeech,
		llm.APIFormatOpenAITranscription,
		llm.APIFormatOpenAITranslation,
	}

	require.Len(t, built.Outbounds, len(apiFormats))

	for _, apiFormat := range apiFormats {
		outbound, err := BuildOutboundByAPIFormat(built, apiFormat.String())
		require.NoError(t, err, "groq should support %s", apiFormat)
		require.Same(t, built.Outbound, outbound)
	}

	_, err = BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.Error(t, err, "groq should not expose embeddings")
}
