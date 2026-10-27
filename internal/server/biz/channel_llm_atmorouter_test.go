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
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestAtmorouterChannel_UsesOpenAITransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("AtmoRouter Channel").
		SetType(channel.TypeAtmorouter).
		SetBaseURL("https://atmorouter.dev/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"cc/claude-sonnet-4-6"}).
		SetDefaultTestModel("cc/claude-sonnet-4-6").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "TypeAtmorouter should create openai.OutboundTransformer")
	require.Equal(t, llm.APIFormatOpenAIChatCompletion, built.Outbound.APIFormat())

	// AtmoRouter only serves chat completions on its OpenAI-compatible surface.
	require.Len(t, built.Outbounds, 1)

	_, err = BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIEmbedding.String())
	require.Error(t, err, "atmorouter should not expose embeddings")

	content := "hello"
	request, err := built.Outbound.TransformRequest(ctx, &llm.Request{
		Model: "cc/claude-sonnet-4-6",
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "https://atmorouter.dev/v1/chat/completions", request.URL)
	require.Equal(t, "test-key", request.Auth.APIKey)
}

func TestAtmorouterAnthropicChannel_UsesAnthropicTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("AtmoRouter Anthropic Channel").
		SetType(channel.TypeAtmorouterAnthropic).
		SetBaseURL("https://atmorouter.dev").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"cc/claude-sonnet-4-6"}).
		SetDefaultTestModel("cc/claude-sonnet-4-6").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	anthropicOutbound, ok := built.Outbound.(*anthropic.OutboundTransformer)
	require.True(t, ok, "TypeAtmorouterAnthropic should create anthropic.OutboundTransformer")
	require.Equal(t, anthropic.PlatformDirect, anthropicOutbound.GetConfig().Type)
	require.Equal(t, llm.APIFormatAnthropicMessage, built.Outbound.APIFormat())

	maxTokens := int64(16)
	content := "hello"
	request, err := built.Outbound.TransformRequest(ctx, &llm.Request{
		Model:     "cc/claude-sonnet-4-6",
		MaxTokens: &maxTokens,
		Messages: []llm.Message{{
			Role:    "user",
			Content: llm.MessageContent{Content: &content},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "https://atmorouter.dev/v1/messages", request.URL)
	require.Equal(t, "test-key", request.Auth.APIKey)
}
