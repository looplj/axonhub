package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestHostedWebSearchToolChoiceRequiresNativeTool(t *testing.T) {
	hostedChoice := &llm.ToolChoice{
		NamedToolChoice: &llm.NamedToolChoice{
			Type: llm.ToolTypeWebSearch,
		},
	}
	request := func() *llm.Request {
		return &llm.Request{
			Model:      "claude-sonnet-4-5",
			MaxTokens:  lo.ToPtr[int64](128),
			Messages:   []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
			ToolChoice: hostedChoice,
			Tools: []llm.Tool{
				{
					Type:     "function",
					Function: llm.Function{Name: WebSearchFunctionName},
				},
				{
					Type:      llm.ToolTypeWebSearch,
					WebSearch: &llm.WebSearch{},
				},
			},
		}
	}

	t.Run("native tool enabled", func(t *testing.T) {
		outbound, err := NewOutboundTransformerWithConfig(&Config{
			Type:           PlatformDirect,
			BaseURL:        "https://api.anthropic.com",
			APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
		})
		require.NoError(t, err)

		wire, err := outbound.TransformRequest(context.Background(), request())
		require.NoError(t, err)

		var payload MessageRequest
		require.NoError(t, json.Unmarshal(wire.Body, &payload))
		require.Equal(t, "tool", payload.ToolChoice.Type)
		require.NotNil(t, payload.ToolChoice.Name)
		require.Equal(t, WebSearchFunctionName, *payload.ToolChoice.Name)
		require.True(t, lo.ContainsBy(payload.Tools, func(tool Tool) bool {
			return tool.Type == ToolTypeWebSearch20250305
		}))
	})

	t.Run("same-named function cannot replace filtered native tool", func(t *testing.T) {
		outbound, err := NewOutboundTransformerWithConfig(&Config{
			Type:                PlatformDirect,
			NativeToolsDisabled: true,
			BaseURL:             "https://api.anthropic.com",
			APIKeyProvider:      auth.NewStaticKeyProvider("test-key"),
		})
		require.NoError(t, err)

		_, err = outbound.TransformRequest(context.Background(), request())
		require.ErrorIs(t, err, transformer.ErrUnsupportedConversion)
	})
}
