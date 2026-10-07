package orchestrator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

// TestBuildTestRequestUsesConfiguredPrompts verifies ordinary channel tests retain their configured prompts.
func TestBuildTestRequestUsesConfiguredPrompts(t *testing.T) {
	req := buildChannelTestRequest("test-model", true, "system prompt", "user prompt", false, llm.APIFormatOpenAIChatCompletion)

	require.Equal(t, "test-model", req.Model)
	require.Len(t, req.Messages, 2)
	require.Equal(t, "system", req.Messages[0].Role)
	require.Equal(t, "system prompt", *req.Messages[0].Content.Content)
	require.Equal(t, "user", req.Messages[1].Role)
	require.Equal(t, "user prompt", *req.Messages[1].Content.Content)
	require.Equal(t, int64(256), *req.MaxCompletionTokens)
	require.True(t, *req.Stream)
}

// TestBuildTestRequestUsesPingForResponsesWebSocket verifies WebSocket tests use a minimal compatible payload.
func TestBuildTestRequestUsesPingForResponsesWebSocket(t *testing.T) {
	req := buildChannelTestRequest("test-model", false, "system prompt", "user prompt", true, llm.APIFormatOpenAIChatCompletion)

	require.Equal(t, "test-model", req.Model)
	require.Len(t, req.Messages, 1)
	require.Equal(t, "user", req.Messages[0].Role)
	require.Equal(t, responsesWebSocketTestPrompt, *req.Messages[0].Content.Content)
	require.Nil(t, req.MaxCompletionTokens)
	require.True(t, *req.Stream)
}

func TestBuildTestRequestUsesDecisionsFormat(t *testing.T) {
	req := buildChannelTestRequest("gpt-6-luna", true, "ignored", "classify this", false, llm.APIFormatOpenAIDecisions)

	require.Equal(t, llm.RequestTypeDecisions, req.RequestType)
	require.Equal(t, llm.APIFormatOpenAIDecisions, req.APIFormat)
	require.NotNil(t, req.Decisions)
	require.False(t, *req.Stream)

	var body struct {
		Model     string            `json:"model"`
		Input     string            `json:"input"`
		Questions []json.RawMessage `json:"questions"`
	}
	require.NoError(t, json.Unmarshal(req.Decisions.Body, &body))
	require.Equal(t, "gpt-6-luna", body.Model)
	require.Equal(t, "classify this", body.Input)
	require.Len(t, body.Questions, 1)
}

func TestChannelTestAPIFormatPrefersDecisionsEndpoint(t *testing.T) {
	ch := &biz.Channel{Channel: &ent.Channel{
		Endpoints: []objects.ChannelEndpoint{
			{APIFormat: llm.APIFormatOpenAIChatCompletion.String()},
			{APIFormat: llm.APIFormatOpenAIDecisions.String()},
		},
	}}

	require.Equal(t, llm.APIFormatOpenAIDecisions, channelTestAPIFormat(ch))
}

func TestParseDecisionsTestResponseRequiresAnswers(t *testing.T) {
	message, err := parseDecisionsTestResponse([]byte(`{"model":"gpt-6-luna","answers":[{"name":"answer","probability":1}]}`))
	require.NoError(t, err)
	require.Contains(t, message, `"answers"`)

	_, err = parseDecisionsTestResponse([]byte(`{"model":"gpt-6-luna","answers":[]}`))
	require.EqualError(t, err, "no answers in Decisions response")
}

// TestUsesResponsesWebSocket verifies explicit and URL-inferred WebSocket transports.
func TestUsesResponsesWebSocket(t *testing.T) {
	t.Run("inferred from channel base URL", func(t *testing.T) {
		ch := &biz.Channel{Channel: &ent.Channel{
			Type:    channel.TypeOpenaiResponses,
			BaseURL: "wss://api.openai.com/v1",
		}}

		require.True(t, usesResponsesWebSocket(ch))
	})

	t.Run("explicit transport", func(t *testing.T) {
		ch := &biz.Channel{Channel: &ent.Channel{
			Type:    channel.TypeOpenai,
			BaseURL: "https://api.example.com/v1",
			Endpoints: []objects.ChannelEndpoint{{
				APIFormat: llm.APIFormatOpenAIResponse.String(),
				Transport: objects.ChannelEndpointTransportWebSocket,
			}},
		}}

		require.True(t, usesResponsesWebSocket(ch))
	})

	t.Run("HTTP transport", func(t *testing.T) {
		ch := &biz.Channel{Channel: &ent.Channel{
			Type:    channel.TypeOpenaiResponses,
			BaseURL: "https://api.openai.com/v1",
		}}

		require.False(t, usesResponsesWebSocket(ch))
	})
}
