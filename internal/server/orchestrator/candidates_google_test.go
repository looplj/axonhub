package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/llm"
	"github.com/looplj/axonhub/internal/objects"
)

// createGeminiTestChannels creates test channels including Gemini channels for Google native tools testing.
func createGeminiTestChannels(t *testing.T, ctx context.Context, client *ent.Client) []*ent.Channel {
	t.Helper()

	channels := make([]*ent.Channel, 0)

	// Channel 0: gemini (native format, supports Google native tools)
	ch0, err := client.Channel.Create().
		SetType(channel.TypeGemini).
		SetName("Gemini Native").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-1"}).
		SetSupportedModels([]string{"gemini-2.0-flash", "gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch0)

	// Channel 1: gemini_openai (OpenAI format, does NOT support Google native tools)
	ch1, err := client.Channel.Create().
		SetType(channel.TypeGeminiOpenai).
		SetName("Gemini OpenAI").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-2"}).
		SetSupportedModels([]string{"gemini-2.0-flash", "gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch1)

	// Channel 2: gemini_vertex (Vertex AI, supports Google native tools)
	ch2, err := client.Channel.Create().
		SetType(channel.TypeGeminiVertex).
		SetName("Gemini Vertex").
		SetBaseURL("https://us-central1-aiplatform.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key-3"}).
		SetSupportedModels([]string{"gemini-2.0-flash", "gemini-2.5-pro"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channels = append(channels, ch2)

	return channels
}

// TestGoogleNativeToolsSelector_Select_WithGoogleNativeTools tests filtering when request contains Google native tools.
func TestGoogleNativeToolsSelector_Select_WithGoogleNativeTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)
	selector := WithGoogleNativeToolsSelector(baseSelector)

	// Request with Google native tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			{Type: "function", Function: llm.Function{Name: "get_weather"}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only channels that support Google native tools (gemini, gemini_vertex)
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Gemini native channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Gemini Vertex channel should be included")
	assert.NotContains(t, channelIDs, channels[1].ID, "Gemini OpenAI channel should be excluded")
}

// TestGoogleNativeToolsSelector_Select_WithoutGoogleNativeTools tests that all channels are returned when no Google native tools.
func TestGoogleNativeToolsSelector_Select_WithoutGoogleNativeTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)
	selector := WithGoogleNativeToolsSelector(baseSelector)

	// Request without Google native tools (only function tools)
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: "function", Function: llm.Function{Name: "get_weather"}},
			{Type: "function", Function: llm.Function{Name: "search"}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return all channels when no Google native tools
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Gemini native channel should be included")
	assert.Contains(t, channelIDs, channels[1].ID, "Gemini OpenAI channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Gemini Vertex channel should be included")
}

// TestGoogleNativeToolsSelector_Select_NoCompatibleChannels tests fallback when no compatible channels exist.
func TestGoogleNativeToolsSelector_Select_NoCompatibleChannels(t *testing.T) {
	ctx, client := setupTest(t)

	// Create only gemini_openai channel (does not support Google native tools)
	ch, err := client.Channel.Create().
		SetType(channel.TypeGeminiOpenai).
		SetName("Gemini OpenAI Only").
		SetBaseURL("https://generativelanguage.googleapis.com").
		SetCredentials(&objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"gemini-2.0-flash"}).
		SetDefaultTestModel("gemini-2.0-flash").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)
	selector := WithGoogleNativeToolsSelector(baseSelector)

	// Request with Google native tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should fallback to all channels when no compatible channels exist
	// (downstream outbound will handle the fallback)
	require.Len(t, result, 1)
	assert.Equal(t, ch.ID, result[0].Channel.ID)
}

// TestGoogleNativeToolsSelector_Select_EmptyTools tests that all channels are returned when tools are empty.
func TestGoogleNativeToolsSelector_Select_EmptyTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)
	selector := WithGoogleNativeToolsSelector(baseSelector)

	// Request with no tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return all channels when no tools
	require.Len(t, result, 3)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID)
	assert.Contains(t, channelIDs, channels[1].ID)
	assert.Contains(t, channelIDs, channels[2].ID)
}

// TestGoogleNativeToolsSelector_Select_MultipleGoogleNativeTools tests filtering with multiple Google native tools.
func TestGoogleNativeToolsSelector_Select_MultipleGoogleNativeTools(t *testing.T) {
	ctx, client := setupTest(t)

	channels := createGeminiTestChannels(t, ctx, client)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	baseSelector := NewDefaultSelector(channelService, modelService)
	selector := WithGoogleNativeToolsSelector(baseSelector)

	// Request with multiple Google native tools
	req := &llm.Request{
		Model: "gemini-2.0-flash",
		Tools: []llm.Tool{
			{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			{Type: llm.ToolTypeGoogleUrlContext, Google: &llm.GoogleTools{UrlContext: &llm.GoogleUrlContext{}}},
			{Type: "function", Function: llm.Function{Name: "get_weather"}},
		},
	}

	result, err := selector.Select(ctx, req)
	require.NoError(t, err)

	// Should return only channels that support Google native tools
	require.Len(t, result, 2)

	channelIDs := make([]int, len(result))
	for i, ch := range result {
		channelIDs[i] = ch.Channel.ID
	}

	assert.Contains(t, channelIDs, channels[0].ID, "Gemini native channel should be included")
	assert.Contains(t, channelIDs, channels[2].ID, "Gemini Vertex channel should be included")
	assert.NotContains(t, channelIDs, channels[1].ID, "Gemini OpenAI channel should be excluded")
}
