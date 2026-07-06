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
	"github.com/looplj/axonhub/llm/transformer/cline"
)

func TestClineChannel_BuildsClineTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Cline Channel").
		SetType(channel.TypeCline).
		SetBaseURL("https://api.cline.bot/api/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"cline-pass/deepseek-v4-flash"}).
		SetDefaultTestModel("cline-pass/deepseek-v4-flash").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)
	_, ok := built.Outbound.(*cline.OutboundTransformer)
	require.True(t, ok, "TypeCline should create cline.OutboundTransformer")

	chatOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, chatOutbound)
}

func TestBuildOutboundByAPIFormat_ClineUsesConfiguredEndpointPath(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Cline Channel").
		SetType(channel.TypeCline).
		SetBaseURL("https://api.cline.bot/api/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"cline-pass/deepseek-v4-flash"}).
		SetDefaultTestModel("cline-pass/deepseek-v4-flash").
		SetEndpoints([]objects.ChannelEndpoint{{
			APIFormat: llm.APIFormatOpenAIChatCompletion.String(),
			Path:      "/custom/chat/completions",
		}}).
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)

	chatOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIChatCompletion.String())
	require.NoError(t, err)

	content := "hello"
	req, err := chatOutbound.TransformRequest(ctx, &llm.Request{
		Model: "cline-pass/deepseek-v4-flash",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.cline.bot/api/v1/custom/chat/completions", req.URL)
}
