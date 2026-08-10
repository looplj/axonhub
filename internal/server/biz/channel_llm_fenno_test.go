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
	"github.com/looplj/axonhub/llm/transformer/openai/codex"
)

func TestFennoChannel_UsesThirdPartyCodexTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Fenno Channel").
		SetType(channel.TypeFenno).
		SetBaseURL("https://api.fenno.ai").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-5.2-codex"}).
		SetDefaultTestModel("gpt-5.2-codex").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*codex.OutboundTransformer)
	require.True(t, ok, "TypeFenno should create codex.OutboundTransformer")
	require.Equal(t, llm.APIFormatOpenAIResponse, built.Outbound.APIFormat())

	responseOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIResponse.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, responseOutbound)

	content := "hello"
	request, err := responseOutbound.TransformRequest(ctx, &llm.Request{
		Model: "gpt-5.2-codex",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "https://api.fenno.ai/v1/responses", request.URL)
	require.Equal(t, "bearer", request.Auth.Type)
	require.Equal(t, "test-key", request.Auth.APIKey)
}
