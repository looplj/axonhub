package cc

import (
	"context"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

const claudeCodeUserAgentPrefix = "claude-cli/"

type systemCacheCompatibilityMiddleware struct {
	pipeline.DummyMiddleware
}

var _ pipeline.OutboundLlmRequestMiddleware = (*systemCacheCompatibilityMiddleware)(nil)

// SystemCacheCompatibility keeps Claude Code's leading system prompt stable
// when an outbound protocol hoists later system messages into the prompt prefix.
// Only real system-role messages after the leading contiguous system block are
// downgraded; user messages containing reminder text are already cache-safe.
func SystemCacheCompatibility() pipeline.OutboundLlmRequestMiddleware {
	return &systemCacheCompatibilityMiddleware{}
}

func (m *systemCacheCompatibilityMiddleware) Name() string {
	return "claudecode-system-cache-compatibility"
}

func (m *systemCacheCompatibilityMiddleware) OnOutboundLlmRequest(
	ctx context.Context,
	request *llm.Request,
	format llm.APIFormat,
) (*llm.Request, error) {
	if !IsClaudeCodeRequest(request) || !supportsSystemCacheCompatibility(format) {
		return request, nil
	}

	messages, changed := downgradeMidConversationSystemMessages(request.Messages)
	if !changed {
		return request, nil
	}

	cloned := *request
	cloned.Messages = messages

	return &cloned, nil
}

// IsClaudeCodeRequest reports whether the unified request originated from the
// Claude Code CLI.
func IsClaudeCodeRequest(request *llm.Request) bool {
	if request == nil || request.RawRequest == nil || request.RawRequest.Headers == nil {
		return false
	}

	userAgent := strings.TrimSpace(request.RawRequest.Headers.Get("User-Agent"))

	return strings.HasPrefix(userAgent, claudeCodeUserAgentPrefix)
}

func supportsSystemCacheCompatibility(format llm.APIFormat) bool {
	//nolint:exhaustive // Only text protocols known to hoist system messages are relevant.
	switch format {
	case llm.APIFormatOpenAIChatCompletion,
		llm.APIFormatOpenAIResponse,
		llm.APIFormatOpenAIResponseCompact,
		llm.APIFormatAnthropicMessage:
		return true
	default:
		return false
	}
}

func downgradeMidConversationSystemMessages(messages []llm.Message) ([]llm.Message, bool) {
	leadingSystemEnd := 0
	for leadingSystemEnd < len(messages) && strings.EqualFold(messages[leadingSystemEnd].Role, "system") {
		leadingSystemEnd++
	}

	var result []llm.Message
	for i := leadingSystemEnd; i < len(messages); i++ {
		if !strings.EqualFold(messages[i].Role, "system") {
			continue
		}

		if result == nil {
			result = append([]llm.Message(nil), messages...)
		}
		result[i].Role = "user"
	}

	if result == nil {
		return messages, false
	}

	return result, true
}
