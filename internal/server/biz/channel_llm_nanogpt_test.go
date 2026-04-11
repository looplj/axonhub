package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestNanogptChannel_ChatCompletions(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("NanoGPT Channel").
		SetType(channel.TypeNanogpt).
		SetBaseURL("https://api.nanogpt.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	// TypeNanogpt should create openai.OutboundTransformer (chat completions)
	_, ok := built.Outbound.(*openai.OutboundTransformer)
	require.True(t, ok, "TypeNanogpt should create openai.OutboundTransformer for chat completions")
}

func TestNanogptChannel_CreateResponsesTransformer(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("NanoGPT Responses Channel").
		SetType(channel.TypeNanogptResponses).
		SetBaseURL("https://api.nanogpt.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gpt-4"}).
		SetDefaultTestModel("gpt-4").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	_, ok := built.Outbound.(*responses.OutboundTransformer)
	require.True(t, ok, "TypeNanogptResponses should create responses.OutboundTransformer")
}

func TestNanogptChannel_VerifyAPIFormat(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	channelSvc := NewChannelServiceForTest(client)

	t.Run("TypeNanogpt returns OpenAI Chat Completions format", func(t *testing.T) {
		entChannel := client.Channel.Create().
			SetName("NanoGPT Chat").
			SetType(channel.TypeNanogpt).
			SetBaseURL("https://api.nanogpt.example.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SaveX(ctx)

		built, err := channelSvc.buildChannelWithTransformer(entChannel)
		require.NoError(t, err)
		require.Equal(t, "openai/chat_completions", string(built.Outbound.APIFormat()))
	})

	t.Run("TypeNanogptResponses returns OpenAI Responses format", func(t *testing.T) {
		entChannel := client.Channel.Create().
			SetName("NanoGPT Responses").
			SetType(channel.TypeNanogptResponses).
			SetBaseURL("https://api.nanogpt.example.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SaveX(ctx)

		built, err := channelSvc.buildChannelWithTransformer(entChannel)
		require.NoError(t, err)
		require.Equal(t, "openai/responses", string(built.Outbound.APIFormat()))
	})
}
