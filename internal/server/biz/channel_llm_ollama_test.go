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
	"github.com/looplj/axonhub/llm/transformer/ollama"
)

// TestOllamaAnthropicChannel_WithAPIKey verifies that an ollama_anthropic channel
// configured with an API key builds an Anthropic OutboundTransformer (not the native
// Ollama transformer) and exposes the anthropic/messages API format as its default endpoint.
func TestOllamaAnthropicChannel_WithAPIKey(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Ollama Anthropic Channel").
		SetType(channel.TypeOllamaAnthropic).
		SetBaseURL("http://localhost:11434").
		SetCredentials(objects.ChannelCredentials{APIKey: "ollama-key"}).
		SetSupportedModels([]string{"llama3.2"}).
		SetDefaultTestModel("llama3.2").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	// ollama_anthropic must build the Anthropic transformer, not the native Ollama one.
	anthropicOutbound, ok := built.Outbound.(*anthropic.OutboundTransformer)
	require.True(t, ok, "TypeOllamaAnthropic should create anthropic.OutboundTransformer")

	// The platform type must be PlatformOllama so that Bearer auth is selected.
	require.Equal(t, anthropic.PlatformOllama, anthropicOutbound.GetConfig().Type)
	require.Equal(t, llm.APIFormatAnthropicMessage, built.Outbound.APIFormat())
}

// TestOllamaAnthropicChannel_BuildsWithoutAPIKey verifies that an ollama_anthropic
// channel with no API key builds without panicking. This guards the conditional
// apiKeyProvider guard in channel_llm.go — without it, getAPIKeyProvider panics
// when there are zero enabled keys, which is the common local Ollama deployment.
func TestOllamaAnthropicChannel_BuildsWithoutAPIKey(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Ollama Anthropic No-Key Channel").
		SetType(channel.TypeOllamaAnthropic).
		SetBaseURL("http://localhost:11434").
		// No credentials — mirrors a local, unauthenticated Ollama instance.
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"llama3.2"}).
		SetDefaultTestModel("llama3.2").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	anthropicOutbound, ok := built.Outbound.(*anthropic.OutboundTransformer)
	require.True(t, ok, "TypeOllamaAnthropic should create anthropic.OutboundTransformer even without an API key")
	require.Equal(t, anthropic.PlatformOllama, anthropicOutbound.GetConfig().Type)
}

// TestOllamaAnthropicChannel_DefaultEndpointIsAnthropicMessages verifies that the
// resolved default endpoints for ollama_anthropic include only the anthropic/messages
// API format, matching the defaultEndpointsForChannelType mapping.
func TestOllamaAnthropicChannel_DefaultEndpointIsAnthropicMessages(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Ollama Anthropic Endpoints Channel").
		SetType(channel.TypeOllamaAnthropic).
		SetBaseURL("http://localhost:11434").
		SetCredentials(objects.ChannelCredentials{APIKey: "ollama-key"}).
		SetSupportedModels([]string{"llama3.2"}).
		SetDefaultTestModel("llama3.2").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithOutbounds(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)

	// The default endpoints map should expose anthropic/messages for this channel type.
	resolved := built.ResolveEndpoints()
	require.NotEmpty(t, resolved)

	hasAnthropicMessages := false
	for _, ep := range resolved {
		if ep.APIFormat == llm.APIFormatAnthropicMessage.String() {
			hasAnthropicMessages = true
			break
		}
	}
	require.True(t, hasAnthropicMessages, "ollama_anthropic should default to anthropic/messages endpoint")

	// The anthropic/messages outbound must be retrievable via BuildOutboundByAPIFormat.
	anthropicOutbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatAnthropicMessage.String())
	require.NoError(t, err)
	require.NotNil(t, anthropicOutbound)
	require.Equal(t, llm.APIFormatAnthropicMessage, anthropicOutbound.APIFormat())
}

// TestOllamaChannel_NativeTransformerUnaffected verifies that the original ollama
// channel type still builds the native Ollama transformer (not Anthropic), ensuring
// the new ollama_anthropic type did not alter the existing ollama behavior.
func TestOllamaChannel_NativeTransformerUnaffected(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	entChannel := client.Channel.Create().
		SetName("Ollama Native Channel").
		SetType(channel.TypeOllama).
		SetBaseURL("http://localhost:11434").
		SetCredentials(objects.ChannelCredentials{APIKey: "ollama-key"}).
		SetSupportedModels([]string{"llama3.2"}).
		SetDefaultTestModel("llama3.2").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel)
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	// The native ollama channel must still use the Ollama transformer.
	_, ok := built.Outbound.(*ollama.OutboundTransformer)
	require.True(t, ok, "TypeOllama should still create ollama.OutboundTransformer")
	require.Equal(t, llm.APIFormatOllamaChat, built.Outbound.APIFormat())
}

// TestOllamaAnthropicChannel_APIKeyOverrideWithoutStoredKeys verifies that an
// apiKeyOverride (used by the channel key test flow) is honored even when the
// channel has no stored enabled keys. Without the override-aware guard this would
// drop the override and send unauthenticated requests to the upstream.
func TestOllamaAnthropicChannel_APIKeyOverrideWithoutStoredKeys(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())

	// Channel has no credentials, but the caller supplies an override key (test flow).
	entChannel := client.Channel.Create().
		SetName("Ollama Anthropic Override Channel").
		SetType(channel.TypeOllamaAnthropic).
		SetBaseURL("http://localhost:11434").
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"llama3.2"}).
		SetDefaultTestModel("llama3.2").
		SaveX(ctx)

	channelSvc := NewChannelServiceForTest(client)

	built, err := channelSvc.buildChannelWithTransformer(entChannel, "override-test-key")
	require.NoError(t, err)
	require.NotNil(t, built)
	require.NotNil(t, built.Outbound)

	anthropicOutbound, ok := built.Outbound.(*anthropic.OutboundTransformer)
	require.True(t, ok, "TypeOllamaAnthropic should create anthropic.OutboundTransformer")

	// The override key must flow through to the transformer's API key provider.
	cfg := anthropicOutbound.GetConfig()
	require.NotNil(t, cfg.APIKeyProvider, "apiKeyOverride must produce a non-nil APIKeyProvider even without stored keys")
	require.Equal(t, "override-test-key", cfg.APIKeyProvider.Get(ctx))
}
