package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestBailianResponsesChannel_RoutesOnlyConfiguredModels(t *testing.T) {
	ctx, client := setupTest(t)
	created, err := client.Channel.Create().
		SetType(channel.TypeBailianResponses).
		SetName("Bailian Responses").
		SetBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"qwen3.7-max"}).
		SetDefaultTestModel("qwen3.7-max").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	systemService := newTestSystemService(client)
	requestService := newTestRequestServiceForChannels(client, systemService)
	selector := newTestLoadBalancedSelector(channelService, client, systemService, requestService)

	supported, err := selector.Select(ctx, &llm.Request{
		Model:       "qwen3.7-max",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIResponse,
	})
	require.NoError(t, err)
	require.Len(t, supported, 1)
	require.Equal(t, created.ID, supported[0].Channel.ID)
	require.Equal(t, llm.APIFormatOpenAIResponse.String(), supported[0].APIFormat)

	unsupported, err := selector.Select(ctx, &llm.Request{
		Model:       "qwen-vl-max",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIResponse,
	})
	require.NoError(t, err)
	require.Empty(t, unsupported)
}
