package anthropic

import (
	"context"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/samber/lo"
)

// ClaudeCodeTransformer implements the transformer for Claude Code CLI.
// It wraps an OutboundTransformer and adds Claude Code specific headers and system message.
type ClaudeCodeTransformer struct {
	transformer.Outbound
}

// TransformRequest overrides the base TransformRequest to add Claude Code specific modifications.
func (t *ClaudeCodeTransformer) TransformRequest(
	ctx context.Context,
	llmReq *llm.Request,
) (*httpclient.Request, error) {
	// Clone the request to avoid mutating the original
	reqCopy := *llmReq

	// Prepend the Claude Code system message
	systemMsg := llm.Message{
		Role: "system",
		Content: llm.MessageContent{
			Content: lo.ToPtr("You are Claude Code, Anthropic's official CLI for Claude."),
		},
	}

	// Insert at the beginning of messages
	reqCopy.Messages = append([]llm.Message{systemMsg}, llmReq.Messages...)

	// Call the base transformer
	httpReq, err := t.Outbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Override the URL to the fixed Claude Code endpoint
	httpReq.URL = "https://api.anthropic.com/v1/messages?beta=true"

	// Add/overwrite Claude Code specific headers
	httpReq.Headers.Set("Anthropic-Beta", "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14")
	httpReq.Headers.Set("Anthropic-Version", "2023-06-01")
	httpReq.Headers.Set("Anthropic-Dangerous-Direct-Browser-Access", "true")
	httpReq.Headers.Set("User-Agent", "claude-cli/1.0.83 (external, cli)")
	httpReq.Headers.Set("X-App", "cli")
	httpReq.Headers.Set("X-Stainless-Helper-Method", "stream")
	httpReq.Headers.Set("X-Stainless-Retry-Count", "0")
	httpReq.Headers.Set("X-Stainless-Runtime-Version", "v24.3.0")
	httpReq.Headers.Set("X-Stainless-Package-Version", "0.55.1")
	httpReq.Headers.Set("X-Stainless-Runtime", "node")

	// Set authentication to Bearer token
	httpReq.Auth = &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: httpReq.Auth.APIKey, // Preserve the API key from base transformer
	}

	return httpReq, nil
}
