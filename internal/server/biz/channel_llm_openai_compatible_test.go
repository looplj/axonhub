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
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
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
	require.Len(t, built.Outbounds, 6)

	require.Equal(t, llm.APIFormatOpenAIChatCompletion, built.Outbound.APIFormat())

	embeddingOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.NoError(t, err)
	require.NotNil(t, embeddingOutbound)
	_, ok := embeddingOutbound.(*openai.OutboundTransformer)
	require.True(t, ok)

	imageOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIImageGeneration.String())
	require.NoError(t, err)
	require.NotNil(t, imageOutbound)
	_, ok = imageOutbound.(*openai.OutboundTransformer)
	require.True(t, ok)

	videoOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIVideo.String())
	require.NoError(t, err)
	require.NotNil(t, videoOutbound)
	_, ok = videoOutbound.(*openai.OutboundTransformer)
	require.True(t, ok)
}

func TestAtlasCloudChannel_BuildChannelWithOutbounds(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("AtlasCloud Channel").
		SetType(channel.TypeAtlascloud).
		SetBaseURL("https://api.atlascloud.ai/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"deepseek-v3"}).
		SetDefaultTestModel("deepseek-v3").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)
	require.Len(t, built.Outbounds, 6)

	require.Equal(t, llm.APIFormatOpenAIChatCompletion, built.Outbound.APIFormat())

	embeddingOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.NoError(t, err)
	require.NotNil(t, embeddingOutbound)
	_, ok := embeddingOutbound.(*openai.OutboundTransformer)
	require.True(t, ok)
}

func TestOpenAIResponsesEndpoint_InheritsWebSocketTransportFromBaseURL(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Responses WebSocket Channel").
		SetType(channel.TypeOpenaiResponses).
		SetBaseURL("wss://api.openai.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-5"}).
		SetDefaultTestModel("gpt-5").
		SetEndpoints([]objects.ChannelEndpoint{{
			APIFormat: llm.APIFormatOpenAIResponse.String(),
			Path:      "/custom/responses",
		}}).
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIResponse.String())
	require.NoError(t, err)
	custom, ok := outbound.(pipeline.ChannelCustomizedExecutor)
	require.True(t, ok)

	executor := custom.CustomizeExecutor(nil)
	_, ok = executor.(*responses.WebSocketExecutor)
	require.True(t, ok)
}

func TestCodexOAuthWebSocketEndpointBuildsWithoutAPIKey(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Codex OAuth WebSocket Channel").
		SetType(channel.TypeCodex).
		SetBaseURL("wss://chatgpt.com/backend-api/codex#").
		SetCredentials(objects.ChannelCredentials{
			OAuth: &objects.OAuthCredentials{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				ExpiresAt:    time.Now().Add(time.Hour),
			},
		}).
		SetSupportedModels([]string{"gpt-5.5"}).
		SetDefaultTestModel("gpt-5.5").
		SetEndpoints([]objects.ChannelEndpoint{{
			APIFormat: llm.APIFormatOpenAIResponse.String(),
			Transport: objects.ChannelEndpointTransportWebSocket,
		}}).
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	primary, ok := built.Outbound.(*codex.OutboundTransformer)
	require.True(t, ok)
	require.NotNil(t, primary.TokenProvider())

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIResponse.String())
	require.NoError(t, err)
	override, ok := outbound.(*codex.OutboundTransformer)
	require.True(t, ok)
	require.True(t, primary.TokenProvider() == override.TokenProvider())

	custom, ok := outbound.(pipeline.ChannelCustomizedExecutor)
	require.True(t, ok)
	require.NotNil(t, custom.CustomizeExecutor(nil))
}

type testStoppableOutbound struct {
	stops int
}

func (t *testStoppableOutbound) APIFormat() llm.APIFormat { return llm.APIFormatOpenAIResponse }

func (t *testStoppableOutbound) TransformRequest(context.Context, *llm.Request) (*httpclient.Request, error) {
	return nil, nil
}

func (t *testStoppableOutbound) TransformResponse(context.Context, *httpclient.Response) (*llm.Response, error) {
	return nil, nil
}

func (t *testStoppableOutbound) TransformStream(context.Context, *httpclient.Request, streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, nil
}

func (t *testStoppableOutbound) TransformError(context.Context, *httpclient.Error) *llm.ResponseError {
	return nil
}

func (t *testStoppableOutbound) AggregateStreamChunks(context.Context, *httpclient.Request, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, nil
}

func (t *testStoppableOutbound) Stop() {
	t.stops++
}

func TestStopChannelOutboundsStopsEachOutboundOnce(t *testing.T) {
	primary := &testStoppableOutbound{}
	secondary := &testStoppableOutbound{}

	stopChannelOutbounds(&Channel{
		Outbound: primary,
		Outbounds: map[string]transformer.Outbound{
			llm.APIFormatOpenAIResponse.String():        primary,
			llm.APIFormatOpenAIResponseCompact.String(): secondary,
		},
	})

	require.Equal(t, 1, primary.stops)
	require.Equal(t, 1, secondary.stops)
}
