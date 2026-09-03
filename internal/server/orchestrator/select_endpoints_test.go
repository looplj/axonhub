//nolint:exhaustruct_v5 // Test fixtures intentionally set only fields relevant to each scenario.
package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestSelectAPIFormat(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/embeddings"},
		{APIFormat: "openai/image_generation"},
		{APIFormat: "openai/moderations"},
	}

	require.Equal(t, "openai/responses", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeChat}))
	require.Equal(t, "openai/embeddings", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeEmbedding}))
	require.Equal(t, "openai/image_generation", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeImage}))
	require.Equal(t, "openai/moderations", SelectAPIFormat(endpoints, &llm.Request{RequestType: llm.RequestTypeModeration}))

	geminiEndpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatGeminiContents.String()},
		{APIFormat: llm.APIFormatGeminiEmbedding.String()},
	}

	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormat(geminiEndpoints, &llm.Request{RequestType: llm.RequestTypeChat}))
	require.Equal(t, llm.APIFormatGeminiEmbedding.String(), SelectAPIFormat(geminiEndpoints, &llm.Request{RequestType: llm.RequestTypeEmbedding}))
	require.Equal(t, llm.APIFormatGeminiContents.String(), SelectAPIFormat(geminiEndpoints, &llm.Request{RequestType: llm.RequestTypeImage}))
}

func TestSelectAPIFormat_PrefersMatchingFormat(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/chat_completions"},
	}

	require.Equal(t, "openai/chat_completions", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}))
}

func TestSelectAPIFormat_FallsBackWhenNoMatch(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
	}

	require.Equal(t, "openai/responses", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}))
}

func TestSelectAPIFormat_Video(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/video"},
		{APIFormat: "seedance/video"},
	}

	require.Equal(t, "openai/video", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatOpenAIVideo,
	}))

	require.Equal(t, "seedance/video", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeVideo,
		APIFormat:   llm.APIFormatSeedanceVideo,
	}))
}

func TestSelectAPIFormat_Compact(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/responses_compact"},
	}

	require.Equal(t, "openai/responses_compact", SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeCompact,
		APIFormat:   llm.APIFormatOpenAIResponseCompact,
	}))
}

func TestSelectAPIFormat_AlphaSearchRequiresExplicitEndpoint(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
	}

	require.Empty(t, SelectAPIFormat(endpoints, &llm.Request{
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
	}))
}

func TestFilterEndpointsByAPIFormats(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/responses"},
		{APIFormat: "openai/chat_completions"},
		{APIFormat: "anthropic/messages"},
	}

	t.Run("restricts to allowed formats in allowed priority order", func(t *testing.T) {
		filtered := FilterEndpointsByAPIFormats(endpoints, []string{"anthropic/messages", "openai/chat_completions"})
		require.Equal(t, []objects.ChannelEndpoint{
			{APIFormat: "anthropic/messages"},
			{APIFormat: "openai/chat_completions"},
		}, filtered)
	})

	t.Run("drops formats the channel has no endpoint for", func(t *testing.T) {
		filtered := FilterEndpointsByAPIFormats(endpoints, []string{"gemini/contents", "openai/responses"})
		require.Equal(t, []objects.ChannelEndpoint{{APIFormat: "openai/responses"}}, filtered)
	})

	t.Run("nil allowed returns nil", func(t *testing.T) {
		require.Nil(t, FilterEndpointsByAPIFormats(endpoints, nil))
	})

	t.Run("no intersection returns nil", func(t *testing.T) {
		require.Nil(t, FilterEndpointsByAPIFormats(endpoints, []string{"gemini/contents"}))
	})
}

// TestSelectAPIFormat_ForcedModelProtocols covers the per-model protocol override
// flow: the forced format list is applied before SelectAPIFormat, so a client
// protocol inside the list is preferred and otherwise the first forced format wins.
func TestSelectAPIFormat_ForcedModelProtocols(t *testing.T) {
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: "openai/chat_completions"},
		{APIFormat: "openai/responses"},
		{APIFormat: "anthropic/messages"},
	}

	claudeCode := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatAnthropicMessage}
	codex := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatOpenAIResponse}
	chatClient := &llm.Request{RequestType: llm.RequestTypeChat, APIFormat: llm.APIFormatOpenAIChatCompletion}

	forced := []string{"openai/responses", "anthropic/messages"}
	filtered := FilterEndpointsByAPIFormats(endpoints, forced)

	require.Equal(t, "anthropic/messages", SelectAPIFormat(filtered, claudeCode))
	require.Equal(t, "openai/responses", SelectAPIFormat(filtered, codex))
	require.Equal(t, "openai/responses", SelectAPIFormat(filtered, chatClient))
}

func TestForcedAPIFormatsForCandidate(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{
		Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "glm-5.3-flash", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}},
			{Model: "grok-4", APIFormats: []string{
				llm.APIFormatOpenAIResponse.String(),
				llm.APIFormatOpenAIChatCompletion.String(),
			}},
		}},
	}}

	t.Run("matches the request model name", func(t *testing.T) {
		forced := forcedAPIFormatsForCandidate(ch, []biz.ChannelModelEntry{
			{RequestModel: "glm-5.3-flash", ActualModel: "glm-5.3-flash"},
		}, "glm-5.3-flash")
		require.Equal(t, []string{llm.APIFormatAnthropicMessage.String()}, forced)
	})

	t.Run("matches the mapped actual model name", func(t *testing.T) {
		forced := forcedAPIFormatsForCandidate(ch, []biz.ChannelModelEntry{
			{RequestModel: "claude-sonnet", ActualModel: "glm-5.3-flash"},
		}, "claude-sonnet")
		require.Equal(t, []string{llm.APIFormatAnthropicMessage.String()}, forced)
	})

	t.Run("unions request and actual model overrides in config order", func(t *testing.T) {
		forced := forcedAPIFormatsForCandidate(ch, []biz.ChannelModelEntry{
			{RequestModel: "grok-4", ActualModel: "glm-5.3-flash"},
		}, "grok-4")
		require.Equal(t, []string{
			llm.APIFormatOpenAIResponse.String(),
			llm.APIFormatOpenAIChatCompletion.String(),
			llm.APIFormatAnthropicMessage.String(),
		}, forced)
	})

	t.Run("no override returns nil", func(t *testing.T) {
		forced := forcedAPIFormatsForCandidate(ch, []biz.ChannelModelEntry{
			{RequestModel: "other", ActualModel: "other-actual"},
		}, "other")
		require.Nil(t, forced)
	})

	t.Run("nil settings returns nil", func(t *testing.T) {
		require.Nil(t, forcedAPIFormatsForCandidate(&biz.Channel{Channel: &ent.Channel{}}, nil, "grok-4"))
		require.Nil(t, forcedAPIFormatsForCandidate(nil, nil, "grok-4"))
	})
}

func TestApplyForcedAPIFormats(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{
		Name: "multi",
		Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "m", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}},
		}},
	}}
	endpoints := []objects.ChannelEndpoint{
		{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
		{APIFormat: llm.APIFormatAnthropicMessage.String()},
	}
	ctx := context.Background()

	t.Run("narrows to the forced formats", func(t *testing.T) {
		filtered := applyForcedAPIFormats(ctx, ch, []biz.ChannelModelEntry{{ActualModel: "m"}}, "m", endpoints)
		require.Equal(t, []objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatAnthropicMessage.String()},
		}, filtered)
	})

	t.Run("falls back to all endpoints when the override matches nothing configured", func(t *testing.T) {
		drifted := &biz.Channel{Channel: &ent.Channel{
			Name: "drifted",
			Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
				{Model: "m", APIFormats: []string{llm.APIFormatGeminiContents.String()}},
			}},
		}}

		filtered := applyForcedAPIFormats(ctx, drifted, nil, "m", endpoints)
		require.Equal(t, endpoints, filtered)
	})

	t.Run("no override keeps all endpoints", func(t *testing.T) {
		filtered := applyForcedAPIFormats(ctx, ch, nil, "other", endpoints)
		require.Equal(t, endpoints, filtered)
	})
}

// TestPopulateAPIFormat_AppliesModelProtocols covers the main model-selection path:
// the per-model protocol override must be applied there too, not only on the legacy
// channel fallback path.
func TestPopulateAPIFormat_AppliesModelProtocols(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{
		ID:   1,
		Name: "multi",
		Type: channel.TypeZai, // defaults to openai/chat_completions
		Endpoints: []objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatAnthropicMessage.String()},
		},
		Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "glm-5.3-flash", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}},
		}},
	}}

	t.Run("override wins over the client protocol", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{{
			Channel: ch,
			Models:  []biz.ChannelModelEntry{{RequestModel: "glm-5.3-flash", ActualModel: "glm-5.3-flash"}},
		}}
		req := &llm.Request{
			Model:       "glm-5.3-flash",
			RequestType: llm.RequestTypeChat,
			APIFormat:   llm.APIFormatOpenAIChatCompletion,
		}

		result := populateAPIFormat(context.Background(), candidates, req)
		require.Len(t, result, 1)
		require.Equal(t, llm.APIFormatAnthropicMessage.String(), result[0].APIFormat)
	})

	t.Run("override matches the mapped actual model name", func(t *testing.T) {
		candidates := []*ChannelModelsCandidate{{
			Channel: ch,
			Models:  []biz.ChannelModelEntry{{RequestModel: "axonhub/glm", ActualModel: "glm-5.3-flash"}},
		}}
		req := &llm.Request{
			Model:       "axonhub/glm",
			RequestType: llm.RequestTypeChat,
			APIFormat:   llm.APIFormatOpenAIChatCompletion,
		}

		result := populateAPIFormat(context.Background(), candidates, req)
		require.Len(t, result, 1)
		require.Equal(t, llm.APIFormatAnthropicMessage.String(), result[0].APIFormat)
	})

	t.Run("no override prefers the client protocol", func(t *testing.T) {
		other := &biz.Channel{Channel: &ent.Channel{
			ID:   2,
			Name: "plain",
			Type: channel.TypeZai,
		}}
		candidates := []*ChannelModelsCandidate{{
			Channel: other,
			Models:  []biz.ChannelModelEntry{{RequestModel: "glm-5.3-flash", ActualModel: "glm-5.3-flash"}},
		}}
		req := &llm.Request{
			Model:       "glm-5.3-flash",
			RequestType: llm.RequestTypeChat,
			APIFormat:   llm.APIFormatOpenAIChatCompletion,
		}

		result := populateAPIFormat(context.Background(), candidates, req)
		require.Len(t, result, 1)
		require.Equal(t, llm.APIFormatOpenAIChatCompletion.String(), result[0].APIFormat)
	})
}

func TestPopulateAPIFormat_SelectsProtocolPerRetryModel(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{
		ID:   1,
		Name: "multi-model",
		Endpoints: []objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
			{APIFormat: llm.APIFormatOpenAIResponse.String()},
			{APIFormat: llm.APIFormatAnthropicMessage.String()},
		},
		Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "model-a", APIFormats: []string{llm.APIFormatAnthropicMessage.String()}},
			{Model: "model-b", APIFormats: []string{llm.APIFormatOpenAIResponse.String()}},
		}},
	}}
	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models: []biz.ChannelModelEntry{
			{RequestModel: "model-a", ActualModel: "model-a"},
			{RequestModel: "model-b", ActualModel: "model-b"},
		},
	}
	req := &llm.Request{
		Model:       "requested-model",
		RequestType: llm.RequestTypeChat,
		APIFormat:   llm.APIFormatOpenAIChatCompletion,
	}

	result := populateAPIFormat(context.Background(), []*ChannelModelsCandidate{candidate}, req)

	require.Len(t, result, 1)
	require.Equal(t, llm.APIFormatAnthropicMessage.String(), candidate.APIFormat)
	require.Equal(t, []string{
		llm.APIFormatAnthropicMessage.String(),
		llm.APIFormatOpenAIResponse.String(),
	}, candidate.modelAPIFormats)
}

func TestPopulateAPIFormat_AlphaSearchDropsModelsWithoutAlphaEndpoint(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{
		ID:   1,
		Name: "codex-multi-model",
		Type: channel.TypeCodex,
		Settings: &objects.ChannelSettings{ModelProtocols: []objects.ModelProtocol{
			{Model: "model-a", APIFormats: []string{llm.APIFormatOpenAIResponse.String()}},
			{Model: "model-b", APIFormats: []string{llm.APIFormatOpenAIAlphaSearch.String()}},
		}},
	}}
	candidate := &ChannelModelsCandidate{
		Channel: ch,
		Models: []biz.ChannelModelEntry{
			{RequestModel: "model-a", ActualModel: "model-a"},
			{RequestModel: "model-b", ActualModel: "model-b"},
		},
	}

	result := populateAPIFormat(context.Background(), []*ChannelModelsCandidate{candidate}, &llm.Request{
		Model:       "requested-model",
		RequestType: llm.RequestTypeAlphaSearch,
		APIFormat:   llm.APIFormatOpenAIAlphaSearch,
	})

	require.Len(t, result, 1)
	require.Equal(t, []biz.ChannelModelEntry{{RequestModel: "model-b", ActualModel: "model-b"}}, candidate.Models)
	require.Equal(t, []string{llm.APIFormatOpenAIAlphaSearch.String()}, candidate.modelAPIFormats)
	require.Equal(t, llm.APIFormatOpenAIAlphaSearch.String(), candidate.APIFormat)
}
