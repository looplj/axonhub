package cc

import (
	"context"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestSystemCacheCompatibilityAffectedFormats(t *testing.T) {
	formats := []llm.APIFormat{
		llm.APIFormatOpenAIChatCompletion,
		llm.APIFormatOpenAIResponse,
		llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatAnthropicMessage,
	}

	middleware := SystemCacheCompatibility()
	for _, format := range formats {
		t.Run(format.String(), func(t *testing.T) {
			request := newClaudeCodeRequest([]llm.Message{
				{Role: "system"},
				{Role: "SYSTEM"},
				{Role: "user"},
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("reminder 1")}},
				{Role: "assistant"},
				{Role: "system", Content: llm.MessageContent{Content: lo.ToPtr("reminder 2")}},
			})

			result, err := middleware.OnOutboundLlmRequest(context.Background(), request, format)
			require.NoError(t, err)

			require.NotSame(t, request, result)
			require.Equal(t, []string{"system", "SYSTEM", "user", "user", "assistant", "user"}, messageRoles(result.Messages))
			require.Equal(t, []string{"system", "SYSTEM", "user", "system", "assistant", "system"}, messageRoles(request.Messages),
				"compatibility handling must not mutate the shared inbound request")
			require.Equal(t, "reminder 1", *result.Messages[3].Content.Content)
			require.Equal(t, "reminder 2", *result.Messages[5].Content.Content)
		})
	}
}

func TestSystemCacheCompatibilityNoOpConditions(t *testing.T) {
	tests := []struct {
		name    string
		request *llm.Request
		format  llm.APIFormat
	}{
		{
			name: "non Claude Code client",
			request: requestWithUserAgent("codex_cli_rs/1.0", []llm.Message{
				{Role: "user"},
				{Role: "system"},
			}),
			format: llm.APIFormatOpenAIResponse,
		},
		{
			name: "leading system block only",
			request: newClaudeCodeRequest([]llm.Message{
				{Role: "system"},
				{Role: "system"},
				{Role: "user"},
			}),
			format: llm.APIFormatAnthropicMessage,
		},
		{
			name: "reminder already carried as user content",
			request: newClaudeCodeRequest([]llm.Message{
				{Role: "system"},
				{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("<system-reminder>safe</system-reminder>")}},
			}),
			format: llm.APIFormatOpenAIChatCompletion,
		},
		{
			name: "unaffected outbound protocol",
			request: newClaudeCodeRequest([]llm.Message{
				{Role: "user"},
				{Role: "system"},
			}),
			format: llm.APIFormatGeminiContents,
		},
		{
			name:    "missing raw request",
			request: &llm.Request{Messages: []llm.Message{{Role: "user"}, {Role: "system"}}},
			format:  llm.APIFormatOpenAIResponse,
		},
	}

	middleware := SystemCacheCompatibility()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := middleware.OnOutboundLlmRequest(context.Background(), tt.request, tt.format)
			require.NoError(t, err)
			require.Same(t, tt.request, result)
		})
	}
}

func newClaudeCodeRequest(messages []llm.Message) *llm.Request {
	return requestWithUserAgent("claude-cli/2.1.170 (external, cli)", messages)
}

func requestWithUserAgent(userAgent string, messages []llm.Message) *llm.Request {
	return &llm.Request{
		Messages: messages,
		RawRequest: &httpclient.Request{
			Headers: http.Header{"User-Agent": []string{userAgent}},
		},
	}
}

func messageRoles(messages []llm.Message) []string {
	roles := make([]string, len(messages))
	for i, message := range messages {
		roles[i] = message.Role
	}

	return roles
}
