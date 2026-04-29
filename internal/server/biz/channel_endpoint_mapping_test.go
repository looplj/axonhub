package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/llm"
)

func TestDefaultEndpointsForChannelType_UseLLMAPIFormatValues(t *testing.T) {
	tests := []struct {
		name     string
		typ      channel.Type
		expected []string
	}{
		{
			name: "openai defaults to chat completions",
			typ:  channel.TypeOpenai,
			expected: []string{
				llm.APIFormatOpenAIChatCompletion.String(),
				llm.APIFormatOpenAIEmbedding.String(),
				llm.APIFormatOpenAIImageGeneration.String(),
				llm.APIFormatOpenAIImageEdit.String(),
				llm.APIFormatOpenAIImageVariation.String(),
			},
		},
		{
			name: "vercel keeps openai-compatible built-in endpoints for compatibility",
			typ:  channel.TypeVercel,
			expected: []string{
				llm.APIFormatOpenAIChatCompletion.String(),
				llm.APIFormatOpenAIEmbedding.String(),
				llm.APIFormatOpenAIImageGeneration.String(),
				llm.APIFormatOpenAIImageEdit.String(),
				llm.APIFormatOpenAIImageVariation.String(),
			},
		},
		{
			name: "github models keeps openai-compatible built-in endpoints for compatibility",
			typ:  channel.TypeGithub,
			expected: []string{
				llm.APIFormatOpenAIChatCompletion.String(),
				llm.APIFormatOpenAIEmbedding.String(),
				llm.APIFormatOpenAIImageGeneration.String(),
				llm.APIFormatOpenAIImageEdit.String(),
				llm.APIFormatOpenAIImageVariation.String(),
			},
		},
		{
			name:     "minimax exposes chat only",
			typ:      channel.TypeMinimax,
			expected: []string{llm.APIFormatOpenAIChatCompletion.String()},
		},
		{
			name:     "xiaomi exposes chat only",
			typ:      channel.TypeXiaomi,
			expected: []string{llm.APIFormatOpenAIChatCompletion.String()},
		},
		{
			name:     "nanogpt responses defaults to responses",
			typ:      channel.TypeNanogptResponses,
			expected: []string{llm.APIFormatOpenAIResponse.String()},
		},
		{
			name:     "jina exposes rerank and embedding",
			typ:      channel.TypeJina,
			expected: []string{llm.APIFormatJinaRerank.String(), llm.APIFormatJinaEmbedding.String()},
		},
		{
			name: "gemini exposes contents and embeddings",
			typ:  channel.TypeGemini,
			expected: []string{
				llm.APIFormatGeminiContents.String(),
				llm.APIFormatGeminiEmbedding.String(),
			},
		},
		{
			name: "nanogpt exposes chat plus delegated openai capability endpoints",
			typ:  channel.TypeNanogpt,
			expected: []string{
				llm.APIFormatOpenAIChatCompletion.String(),
				llm.APIFormatOpenAIEmbedding.String(),
				llm.APIFormatOpenAIImageGeneration.String(),
				llm.APIFormatOpenAIImageEdit.String(),
				llm.APIFormatOpenAIImageVariation.String(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := DefaultEndpointsForChannelType(tt.typ)
			require.Len(t, endpoints, len(tt.expected))

			actual := make([]string, 0, len(endpoints))
			for _, endpoint := range endpoints {
				actual = append(actual, endpoint.APIFormat)
			}

			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestPrimaryEndpointForChannelType_ReturnsFirstDefaultEndpoint(t *testing.T) {
	primary := PrimaryEndpointForChannelType(channel.TypeOpenai)
	require.NotNil(t, primary)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), primary.APIFormat)

	require.Nil(t, PrimaryEndpointForChannelType(channel.Type("")))
}

func TestSupportedAPIFormats_UsesLLMAPIFormatValues(t *testing.T) {
	formats := []string{
		llm.APIFormatOpenAIChatCompletion.String(),
		llm.APIFormatOpenAIResponse.String(),
		llm.APIFormatOpenAIResponseCompact.String(),
		llm.APIFormatOpenAIEmbedding.String(),
		llm.APIFormatOpenAIImageGeneration.String(),
		llm.APIFormatOpenAIImageEdit.String(),
		llm.APIFormatOpenAIImageVariation.String(),
		llm.APIFormatOpenAIVideo.String(),
		llm.APIFormatAnthropicMessage.String(),
		llm.APIFormatGeminiContents.String(),
		llm.APIFormatGeminiEmbedding.String(),
		llm.APIFormatJinaRerank.String(),
		llm.APIFormatJinaEmbedding.String(),
		llm.APIFormatOllamaChat.String(),
	}

	for _, format := range formats {
		_, ok := SupportedAPIFormats[format]
		require.Truef(t, ok, "expected %s to be supported", format)
	}
}
