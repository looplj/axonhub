package anthropic

import (
	"context"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

const (
	claudeCodeSystemMessage = "You are Claude Code, Anthropic's official CLI for Claude."
	claudeCodeAPIURL        = "https://api.anthropic.com/v1/messages?beta=true"
)

// claudeCodeHeaders contains all headers to set for Claude Code requests.
// Each entry is a [name, value] pair.
var claudeCodeHeaders = [][]string{
	{"Anthropic-Beta", "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"},
	{"Anthropic-Version", "2023-06-01"},
	{"Anthropic-Dangerous-Direct-Browser-Access", "true"},
	{"User-Agent", "claude-cli/1.0.83 (external, cli)"},
	{"X-App", "cli"},
	{"X-Stainless-Helper-Method", "stream"},
	{"X-Stainless-Retry-Count", "0"},
	{"X-Stainless-Runtime-Version", "v24.3.0"},
	{"X-Stainless-Package-Version", "0.55.1"},
	{"X-Stainless-Runtime", "node"},
}

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

	// Check if Claude Code system message already exists
	hasClaudeCodeMessage := false

	for _, msg := range reqCopy.Messages {
		if msg.Role == "system" && msg.Content.Content != nil &&
			*msg.Content.Content == claudeCodeSystemMessage {
			hasClaudeCodeMessage = true
			break
		}
	}

	// Only prepend the Claude Code system message if it doesn't already exist
	if !hasClaudeCodeMessage {
		systemMsg := llm.Message{
			Role: "system",
			Content: llm.MessageContent{
				Content: lo.ToPtr(claudeCodeSystemMessage),
			},
		}
		// Insert at the beginning of messages
		reqCopy.Messages = append([]llm.Message{systemMsg}, llmReq.Messages...)
	}

	// Call the base transformer
	httpReq, err := t.Outbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Override the URL to the fixed Claude Code endpoint
	httpReq.URL = claudeCodeAPIURL

	// Add/overwrite Claude Code specific headers
	for _, header := range claudeCodeHeaders {
		httpReq.Headers.Set(header[0], header[1])
	}

	// Set authentication to Bearer token
	httpReq.Auth = &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: httpReq.Auth.APIKey, // Preserve the API key from base transformer
	}

	return httpReq, nil
}
