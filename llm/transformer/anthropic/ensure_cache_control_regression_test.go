package anthropic

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestOptimizeCacheControl_trimsStructuralOnlyOverflow(t *testing.T) {
	// Given
	req := &MessageRequest{
		Tools: []Tool{
			{Name: "tool-1", CacheControl: &CacheControl{Type: "ephemeral"}},
			{Name: "tool-2", CacheControl: &CacheControl{Type: "ephemeral"}},
			{Name: "tool-3", CacheControl: &CacheControl{Type: "ephemeral"}},
		},
		System: &SystemPrompt{MultiplePrompts: []SystemPromptPart{
			{Type: "text", Text: "system-1", CacheControl: &CacheControl{Type: "ephemeral"}},
			{Type: "text", Text: "system-2", CacheControl: &CacheControl{Type: "ephemeral"}},
		}},
	}

	// When
	optimizeCacheControl(req)

	// Then
	require.Equal(t, maxCacheControlBreakpoints, countCacheControls(req))
	require.Nil(t, req.Tools[0].CacheControl)
	require.NotNil(t, req.Tools[1].CacheControl)
	require.NotNil(t, req.Tools[2].CacheControl)
	require.NotNil(t, req.System.MultiplePrompts[0].CacheControl)
	require.NotNil(t, req.System.MultiplePrompts[1].CacheControl)
}

func TestOptimizeCacheControl_generatedAnchorsDoNotDisplaceFourClientBreakpoints(t *testing.T) {
	// Given
	req := &MessageRequest{
		Tools: []Tool{{Name: "tool"}},
		System: &SystemPrompt{MultiplePrompts: []SystemPromptPart{
			{Type: "text", Text: "system"},
		}},
		Messages: []MessageParam{{
			Role: "user",
			Content: MessageContent{MultipleContent: []MessageContentBlock{
				{Type: "text", Text: lo.ToPtr("client-1"), CacheControl: &CacheControl{Type: "ephemeral"}},
				{Type: "text", Text: lo.ToPtr("client-2"), CacheControl: &CacheControl{Type: "ephemeral"}},
				{Type: "text", Text: lo.ToPtr("client-3"), CacheControl: &CacheControl{Type: "ephemeral"}},
				{Type: "text", Text: lo.ToPtr("client-4"), CacheControl: &CacheControl{Type: "ephemeral"}},
			}},
		}},
	}

	// When
	optimizeCacheControl(req)

	// Then
	require.Equal(t, maxCacheControlBreakpoints, countCacheControls(req))
	require.Nil(t, req.Tools[0].CacheControl)
	require.Nil(t, req.System.MultiplePrompts[0].CacheControl)
	for i := range req.Messages[0].Content.MultipleContent {
		require.NotNil(t, req.Messages[0].Content.MultipleContent[i].CacheControl)
	}
}

func TestOptimizeCacheControl_placesGeneratedFiveMinuteAnchorAfterClientOneHourAnchor(t *testing.T) {
	// Given
	req := &MessageRequest{
		Tools: []Tool{{Name: "tool"}},
		System: &SystemPrompt{MultiplePrompts: []SystemPromptPart{
			{
				Type:         "text",
				Text:         "system",
				CacheControl: &CacheControl{Type: "ephemeral", TTL: "1h"},
			},
		}},
		Messages: []MessageParam{{
			Role:    "user",
			Content: MessageContent{Content: lo.ToPtr("question")},
		}},
	}

	// When
	optimizeCacheControl(req)

	// Then
	require.Nil(t, req.Tools[0].CacheControl)
	require.Equal(t, "1h", req.System.MultiplePrompts[0].CacheControl.TTL)
	require.Len(t, req.Messages[0].Content.MultipleContent, 1)
	require.NotNil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
	require.Empty(t, req.Messages[0].Content.MultipleContent[0].CacheControl.TTL)
	require.Equal(t, 2, countCacheControls(req))
}

func TestOptimizeCacheControl_sanitizesUnsupportedMarkerBeforePlanning(t *testing.T) {
	// Given
	req := &MessageRequest{Messages: []MessageParam{{
		Role: "assistant",
		Content: MessageContent{MultipleContent: []MessageContentBlock{
			{
				Type:         "thinking",
				Thinking:     lo.ToPtr("thought"),
				CacheControl: &CacheControl{Type: "ephemeral"},
			},
			{Type: "text", Text: lo.ToPtr("answer")},
		}},
	}}}

	// When
	optimizeCacheControl(req)

	// Then
	require.Nil(t, req.Messages[0].Content.MultipleContent[0].CacheControl)
	require.NotNil(t, req.Messages[0].Content.MultipleContent[1].CacheControl)
	require.Equal(t, 1, countCacheControls(req))
}

func TestOptimizeCacheControl_preservesClientAnchorsAsConversationGrows(t *testing.T) {
	// Given
	first := &MessageRequest{
		Tools: []Tool{{Name: "tool", CacheControl: &CacheControl{Type: "ephemeral"}}},
		System: &SystemPrompt{MultiplePrompts: []SystemPromptPart{
			{Type: "text", Text: "system", CacheControl: &CacheControl{Type: "ephemeral"}},
		}},
		Messages: []MessageParam{
			{
				Role: "user",
				Content: MessageContent{MultipleContent: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("stable-1"), CacheControl: &CacheControl{Type: "ephemeral"}},
				}},
			},
			{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("reply")}},
			{
				Role: "user",
				Content: MessageContent{MultipleContent: []MessageContentBlock{
					{Type: "text", Text: lo.ToPtr("stable-2"), CacheControl: &CacheControl{Type: "ephemeral"}},
				}},
			},
		},
	}
	second := &MessageRequest{
		Tools: []Tool{{Name: "tool", CacheControl: &CacheControl{Type: "ephemeral"}}},
		System: &SystemPrompt{MultiplePrompts: []SystemPromptPart{
			{Type: "text", Text: "system", CacheControl: &CacheControl{Type: "ephemeral"}},
		}},
		Messages: append(append([]MessageParam(nil), first.Messages...),
			MessageParam{Role: "assistant", Content: MessageContent{Content: lo.ToPtr("new reply")}},
			MessageParam{Role: "user", Content: MessageContent{Content: lo.ToPtr("new question")}},
		),
	}

	// When
	optimizeCacheControl(first)
	optimizeCacheControl(second)

	// Then
	require.Equal(t, maxCacheControlBreakpoints, countCacheControls(first))
	require.Equal(t, maxCacheControlBreakpoints, countCacheControls(second))
	require.NotNil(t, first.Messages[0].Content.MultipleContent[0].CacheControl)
	require.NotNil(t, first.Messages[2].Content.MultipleContent[0].CacheControl)
	require.NotNil(t, second.Messages[0].Content.MultipleContent[0].CacheControl)
	require.NotNil(t, second.Messages[2].Content.MultipleContent[0].CacheControl)
	require.Nil(t, second.Messages[3].Content.MultipleContent[0].CacheControl)
	require.Nil(t, second.Messages[4].Content.MultipleContent[0].CacheControl)
}
