package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

func TestChannelEndpoint_TypesafeStrictness(t *testing.T) {
	// 1. Typesafe channel with typesafe/systemone endpoint should pass
	validEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	require.NoError(t, validateEndpointsForChannelType(channel.TypeTypesafe, validEndpoints))

	// 2. Typesafe channel with openai/chat_completions endpoint should FAIL
	invalidEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}
	err := validateEndpointsForChannelType(channel.TypeTypesafe, invalidEndpoints)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only supports api_format")

	// 3. OpenAI channel with typesafe/systemone endpoint should PASS
	openaiWithSystemOne := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	require.NoError(t, validateEndpointsForChannelType(channel.TypeOpenai, openaiWithSystemOne))

	// 4. Cloudflare channel type with typesafe/systemone endpoint should PASS
	cloudflareWithSystemOne := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	require.NoError(t, validateEndpointsForChannelType(channel.TypeCloudflare, cloudflareWithSystemOne))

	// 5. Anthropic channels with typesafe/systemone endpoint should FAIL
	anthropicWithSystemOne := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	err = validateEndpointsForChannelType(channel.TypeAnthropic, anthropicWithSystemOne)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only supported by TypeSafe and channel types with built-in OpenAI endpoints")

	// 6. Codex has OpenAI endpoints but must not expose SystemOne.
	codexWithSystemOne := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIResponse.String()},
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	err = validateEndpointsForChannelType(channel.TypeCodex, codexWithSystemOne)
	require.Error(t, err)
	require.Contains(t, err.Error(), "except Codex")

	// 7. OAuth-only GitHub Copilot channels must not accept an API-key SystemOne endpoint.
	copilotWithSystemOne := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatTypeSafeSystemOne.String()},
	}
	err = validateEndpointsForChannelType(channel.TypeGithubCopilot, copilotWithSystemOne)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GitHub Copilot")

	// 8. Default endpoints for TypeSafe channel must only be typesafe/systemone
	defaults := DefaultEndpointsForChannelType(channel.TypeTypesafe)
	require.Len(t, defaults, 1)
	require.Equal(t, llm.APIFormatTypeSafeSystemOne.String(), defaults[0].APIFormat)
}

func TestChannelEndpoint_NativeCustomFormats(t *testing.T) {
	t.Parallel()
	for _, format := range []llm.APIFormat{llm.APIFormatCohereEmbed, llm.APIFormatAutoTrustDecide} {
		t.Run(format.String(), func(t *testing.T) {
			endpoints := []objects.ChannelEndpoint{{APIFormat: format.String(), Path: "/native"}}
			require.NoError(t, ValidateEndpoints(endpoints))
			require.NoError(t, validateEndpointsForChannelType(channel.TypeOpenai, endpoints))
			require.Error(t, validateEndpointsForChannelType(channel.TypeAnthropic, endpoints))
			require.Error(t, validateEndpointsForChannelType(channel.TypeCodex, endpoints))
			for _, endpoint := range DefaultEndpointsForChannelType(channel.TypeOpenai) {
				require.NotEqual(t, format.String(), endpoint.APIFormat)
			}
		})
	}
}
