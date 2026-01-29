package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

const (
	claudeCodeSystemMessage = "You are Claude Code, Anthropic's official CLI for Claude."
	toolPrefix              = "proxy_"
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
	{"X-Stainless-Lang", "js"},
	{"X-Stainless-Arch", "arm64"},
	{"X-Stainless-Os", "MacOS"},
	{"X-Stainless-Timeout", "60"},
	{"Connection", "keep-alive"},
	{"Accept-Encoding", "gzip, deflate, br, zstd"},
}

// Params contains parameters for creating a ClaudeCodeTransformer.
type Params struct {
	TokenProvider oauth.TokenGetter // OAuth token provider (required)
	BaseURL       string            // Base URL for the Anthropic API (optional)
}

// NewOutboundTransformer creates a new ClaudeCodeTransformer with OAuth authentication.
func NewOutboundTransformer(params Params) (*ClaudeCodeTransformer, error) {
	if params.TokenProvider == nil {
		return nil, fmt.Errorf("TokenProvider is required")
	}

	baseURL := params.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1"
	}

	// Create base transformer with minimal config
	outbound, err := anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
		Type:    anthropic.PlatformClaudeCode,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
	}

	return &ClaudeCodeTransformer{
		Outbound: outbound,
		tokens:   params.TokenProvider,
	}, nil
}

// ClaudeCodeTransformer implements the transformer for Claude Code CLI.
// It wraps an OutboundTransformer and adds Claude Code specific headers and system message.
type ClaudeCodeTransformer struct {
	transformer.Outbound
	tokens oauth.TokenGetter
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

	// Get OAuth token early - needed for determining tool prefix logic
	creds, err := t.tokens.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth token: %w", err)
	}
	apiKey := creds.AccessToken

	// Apply structured transformations before serialization
	reqCopy = *disableThinkingIfToolChoiceForcedStructured(&reqCopy)
	reqCopy = *injectClaudeCodeSystemMessageStructured(&reqCopy)
	reqCopy = *injectFakeUserIDStructured(&reqCopy)
	if isClaudeOAuthToken(apiKey) && !keepClientUA {
		reqCopy = *applyClaudeToolPrefixStructured(&reqCopy, toolPrefix)
	}

	// Call the base transformer
	httpReq, err := t.Outbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Post-process: extract and merge betas (Anthropic-specific, not in llm.Request)
	if len(httpReq.Body) > 0 {
		bodyBytes := httpReq.Body

		// Extract and remove betas array from body
		extraBetas, bodyBytes := extractAndRemoveBetas(bodyBytes)

		// Replace the body
		httpReq.Body = bodyBytes

		// Merge extra betas into Anthropic-Beta header
		if len(extraBetas) > 0 {
			baseBetas := httpReq.Headers.Get("Anthropic-Beta")
			if baseBetas == "" {
				baseBetas = claudeCodeHeaders[0][1] // Use default
			}

			httpReq.Headers.Set("Anthropic-Beta", mergeBetasIntoHeader(baseBetas, extraBetas))
		}
	}

	// Add beta=true query parameter if not present
	if httpReq.Query == nil {
		httpReq.Query = make(url.Values)
	}

	if httpReq.Query.Get("beta") == "" {
		httpReq.Query.Set("beta", "true")
	}

	// Store whether we applied tool prefix (for response processing)
	if httpReq.Metadata == nil {
		httpReq.Metadata = make(map[string]string)
	}

	if isClaudeOAuthToken(apiKey) && !keepClientUA {
		httpReq.Metadata["strip_tool_prefix"] = "true"
	}

	// Add/overwrite Claude Code specific headers
	for _, header := range claudeCodeHeaders {
		httpReq.Headers.Set(header[0], header[1])
	}

	// Set Accept header based on streaming
	if llmReq.Stream != nil && *llmReq.Stream {
		httpReq.Headers.Set("Accept", "text/event-stream")
	} else {
		httpReq.Headers.Set("Accept", "application/json")
	}

	if keepClientUA && rawUA != "" {
		httpReq.Headers.Set("User-Agent", rawUA)
	} else {
		httpReq.Headers.Set("User-Agent", UserAgent)
	}

	// Claude Code OAuth always uses Bearer token authentication.
	// Note: For API key authentication, use the standard Anthropic channel type instead.
	// HttpClient will automatically set the Authorization header based on httpReq.Auth.
	httpReq.Auth = &httpclient.AuthConfig{
		Type:   httpclient.AuthTypeBearer,
		APIKey: apiKey,
	}

	return httpReq, nil
}

// TransformResponse overrides the base TransformResponse to strip tool prefixes from responses.
func (t *ClaudeCodeTransformer) TransformResponse(
	ctx context.Context,
	httpResp *httpclient.Response,
) (*llm.Response, error) {
	// Check if we should strip tool prefix (only if we added it in the request)
	shouldStripPrefix := false
	if httpResp.Request != nil && httpResp.Request.Metadata != nil {
		shouldStripPrefix = httpResp.Request.Metadata["strip_tool_prefix"] == "true"
	}

	if !shouldStripPrefix {
		// Call the base transformer and return as-is
		return t.Outbound.TransformResponse(ctx, httpResp)
	}

	// Strip the tool prefix from the response body
	if len(httpResp.Body) > 0 {
		httpResp.Body = stripClaudeToolPrefixFromResponse(httpResp.Body, toolPrefix)
	}

	// Call the base transformer with the modified response
	return t.Outbound.TransformResponse(ctx, httpResp)
}

// TransformStream overrides the base TransformStream to strip tool prefixes from streaming responses.
func (t *ClaudeCodeTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	// Call the base transformer to get the response stream
	baseStream, err := t.Outbound.TransformStream(ctx, stream)
	if err != nil {
		return nil, err
	}

	// Wrap the stream to strip tool prefixes
	// Note: We blindly strip proxy_ prefix from all streaming responses. This is safe because:
	// 1. We only add proxy_ prefix for OAuth tokens from non-CLI clients
	// 2. Claude CLI clients don't send proxy_ prefixed tool names
	// 3. If no prefix was added, stripping won't find anything to strip
	return &toolPrefixStripperStream{
		base:   baseStream,
		prefix: toolPrefix,
	}, nil
}

// toolPrefixStripperStream wraps a stream and strips tool prefixes from responses.
type toolPrefixStripperStream struct {
	base    streams.Stream[*llm.Response]
	prefix  string
	current *llm.Response
}

func (s *toolPrefixStripperStream) Next() bool {
	if !s.base.Next() {
		return false
	}

	resp := s.base.Current()
	if resp == nil {
		s.current = resp
		return true
	}

	// Strip prefix from tool calls in the response
	for i := range resp.Choices {
		choice := &resp.Choices[i]
		if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
			for j := range choice.Delta.ToolCalls {
				toolCall := &choice.Delta.ToolCalls[j]
				if toolCall.Function.Name != "" && strings.HasPrefix(toolCall.Function.Name, s.prefix) {
					toolCall.Function.Name = strings.TrimPrefix(toolCall.Function.Name, s.prefix)
				}
			}
		}
	}

	s.current = resp
	return true
}

func (s *toolPrefixStripperStream) Current() *llm.Response {
	return s.current
}

func (s *toolPrefixStripperStream) Err() error {
	return s.base.Err()
}

func (s *toolPrefixStripperStream) Close() error {
	return s.base.Close()
}

// AggregateStreamChunks overrides the base AggregateStreamChunks to strip tool prefixes from stream chunks.
func (t *ClaudeCodeTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	// Note: We can't access request metadata here, so we blindly strip proxy_ prefix
	// from all streaming responses. This is safe because:
	// 1. We only add proxy_ prefix for OAuth tokens from non-CLI clients
	// 2. Claude CLI clients don't send proxy_ prefixed tool names
	// 3. If no prefix was added, stripping won't find anything to strip

	// Strip prefix from each chunk's data
	for i, chunk := range chunks {
		if chunk != nil && len(chunk.Data) > 0 && strings.Contains(string(chunk.Data), `"type":"tool_use"`) {
			chunks[i].Data = stripClaudeToolPrefixFromStreamLine(chunk.Data, toolPrefix)
		}
	}

	// Call the base transformer
	return t.Outbound.AggregateStreamChunks(ctx, chunks)
}

// stripClaudeToolPrefixFromStreamLine removes the prefix from tool names in streaming events.
func stripClaudeToolPrefixFromStreamLine(line []byte, prefix string) []byte {
	if prefix == "" {
		return line
	}

	// Try to parse as JSON
	var data map[string]any
	if err := json.Unmarshal(line, &data); err != nil {
		return line
	}

	// Check if this is a content_block event with tool_use
	if contentBlock, ok := data["content_block"].(map[string]any); ok {
		if contentBlock["type"] == "tool_use" {
			if name, ok := contentBlock["name"].(string); ok && strings.HasPrefix(name, prefix) {
				contentBlock["name"] = strings.TrimPrefix(name, prefix)

				if modified, err := json.Marshal(data); err == nil {
					return modified
				}
			}
		}
	}

	return line
}

func isClaudeCLIUserAgent(value string) bool {
	return strings.HasPrefix(value, "claude-cli/")
}
