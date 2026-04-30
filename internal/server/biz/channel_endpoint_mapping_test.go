package biz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
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
				llm.APIFormatOpenAIVideo.String(),
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
				llm.APIFormatOpenAIVideo.String(),
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
				llm.APIFormatOpenAIVideo.String(),
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
				llm.APIFormatOpenAIVideo.String(),
			},
		},
		{
			name: "doubao exposes chat and seedance video",
			typ:  channel.TypeDoubao,
			expected: []string{
				llm.APIFormatOpenAIChatCompletion.String(),
				llm.APIFormatSeedanceVideo.String(),
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

func TestValidateEndpoints(t *testing.T) {
	t.Run("empty api_format returns error", func(t *testing.T) {
		err := ValidateEndpoints([]objects.ChannelEndpoint{{APIFormat: ""}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "api_format is required")
	})

	t.Run("unsupported api_format returns error", func(t *testing.T) {
		err := ValidateEndpoints([]objects.ChannelEndpoint{{APIFormat: "unknown/format"}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported api_format")
	})

	t.Run("duplicate api_format returns error", func(t *testing.T) {
		err := ValidateEndpoints([]objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "duplicate api_format")
	})

	t.Run("path must start with slash", func(t *testing.T) {
		err := ValidateEndpoints([]objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "v1/chat"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "path must start with '/'")
	})

	t.Run("path must not be a full URL", func(t *testing.T) {
		err := ValidateEndpoints([]objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "https://example.com/v1"},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "path must not be a full URL")
	})

	t.Run("valid endpoints pass validation", func(t *testing.T) {
		err := ValidateEndpoints([]objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
			{APIFormat: llm.APIFormatGeminiContents.String(), Path: "/custom/gemini"},
		})
		require.NoError(t, err)
	})

	t.Run("empty endpoints list passes validation", func(t *testing.T) {
		err := ValidateEndpoints(nil)
		require.NoError(t, err)
	})
}

func TestResolveEndpoints_MergesDefaultsAndUserOverrides(t *testing.T) {
	ch := &Channel{
		Channel: &ent.Channel{
			Type: channel.TypeOpenai,
			Endpoints: []objects.ChannelEndpoint{
				{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "/v1/custom/chat"},
				{APIFormat: llm.APIFormatGeminiContents.String(), Path: "/v1/gemini"},
			},
		},
	}

	endpoints := ch.ResolveEndpoints()
	require.Equal(t, []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String(), Path: "/v1/custom/chat"},
		{APIFormat: llm.APIFormatOpenAIEmbedding.String()},
		{APIFormat: llm.APIFormatOpenAIImageGeneration.String()},
		{APIFormat: llm.APIFormatOpenAIImageEdit.String()},
		{APIFormat: llm.APIFormatOpenAIImageVariation.String()},
		{APIFormat: llm.APIFormatOpenAIVideo.String()},
		{APIFormat: llm.APIFormatGeminiContents.String(), Path: "/v1/gemini"},
	}, endpoints)
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
	}

	for _, format := range formats {
		_, ok := SupportedAPIFormats[format]
		require.Truef(t, ok, "expected %s to be supported", format)
	}
}
