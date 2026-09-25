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

	chatOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)

	_, ok := chatOutbound.(*requesty.OutboundTransformer)
	require.True(t, ok, "configured chat endpoint should keep the requesty transformer")

	req, err := chatOutbound.TransformRequest(ctx, requestyTestChatRequest())
	require.NoError(t, err)
	require.Equal(t, "https://router.eu.requesty.ai/v1/chat/completions", req.URL)

	imageOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIImageGeneration.String())
	require.NoError(t, err)

	imageTransformer, ok := imageOutbound.(*requesty.OutboundTransformer)
	require.True(t, ok, "configured image endpoint should keep the requesty transformer")
	require.Equal(t, "https://router.requesty.ai/v1", imageTransformer.BaseURL, "image endpoint without a base URL falls back to the channel URL")

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.NoError(t, err)

	_, ok = outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "embeddings go straight to /embeddings through the openai transformer")
}

func TestRequestyChannel_ExplicitPathUsesGenericTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Requesty Explicit Path").
		SetType(channel.TypeRequesty).
		SetBaseURL("https://router.requesty.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"openai/gpt-4o-mini"}).
		SetDefaultTestModel("openai/gpt-4o-mini").
		SetEndpoints([]objects.ChannelEndpoint{{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Path:      "/custom/chat",
		}}).
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	chatOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)

	_, isRequesty := chatOutbound.(*requesty.OutboundTransformer)
	require.False(t, isRequesty, "an explicit path should not use the requesty transformer")

	req, err := chatOutbound.TransformRequest(ctx, requestyTestChatRequest())
	require.NoError(t, err)
	require.Equal(t, "https://router.requesty.ai/v1/custom/chat", req.URL)
}

func requestyTestChatRequest() *llm.Request {
	content := "hello"

	return &llm.Request{
		Model: "openai/gpt-4o-mini",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	}
}
