package anthropic

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

const (
	claudeCodeSystemMessage = "You are Claude Code, Anthropic's official CLI for Claude."
	claudeCodeUserAgent     = "claude-cli/1.0.83 (external, cli)"
)

// claudeCodeHeaders contains all headers to set for Claude Code requests.
// Each entry is a [name, value] pair.
var claudeCodeHeaders = [][]string{
	{"Anthropic-Beta", "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14"},
	{"Anthropic-Version", "2023-06-01"},
	{"Anthropic-Dangerous-Direct-Browser-Access", "true"},
	{"X-App", "cli"},
	{"X-Stainless-Helper-Method", "stream"},
	{"X-Stainless-Retry-Count", "0"},
	{"X-Stainless-Runtime-Version", "v24.3.0"},
	{"X-Stainless-Package-Version", "0.55.1"},
	{"X-Stainless-Runtime", "node"},
}

func NewClaudeCodeTransformer(config *Config) (*ClaudeCodeTransformer, error) {
	outbound, err := NewOutboundTransformerWithConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
	}

	return &ClaudeCodeTransformer{
		Outbound: outbound,
	}, nil
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
	if llmReq == nil {
		return nil, fmt.Errorf("request is nil")
	}

	rawUA := ""
	keepClientUA := false

	if llmReq.RawRequest != nil && llmReq.RawRequest.Headers != nil {
		rawUA = llmReq.RawRequest.Headers.Get("User-Agent")
		keepClientUA = isClaudeCLIUserAgent(rawUA)

		for _, header := range claudeCodeHeaders {
			llmReq.RawRequest.Headers.Del(header[0])
		}

		if !keepClientUA {
			llmReq.RawRequest.Headers.Del("User-Agent")
		}
	}

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

	// Add beta=true query parameter if not present
	if httpReq.Query == nil {
		httpReq.Query = make(url.Values)
	}

	if httpReq.Query.Get("beta") == "" {
		httpReq.Query.Set("beta", "true")
	}

	// Add/overwrite Claude Code specific headers
	for _, header := range claudeCodeHeaders {
		httpReq.Headers.Set(header[0], header[1])
	}

	if keepClientUA && rawUA != "" {
		httpReq.Headers.Set("User-Agent", rawUA)
	} else {
		httpReq.Headers.Set("User-Agent", claudeCodeUserAgent)
	}

	// Set authentication to Bearer token
	httpReq.Auth = &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: httpReq.Auth.APIKey,
	}

	return httpReq, nil
}

func isClaudeCLIUserAgent(value string) bool {
	return strings.HasPrefix(value, "claude-cli/")
}
