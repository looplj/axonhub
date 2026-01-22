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
	TokenProvider oauth.TokenGetter // For OAuth channels
	Config        *anthropic.Config // For API key channels (backward compat)
}

// NewOutboundTransformer creates a new ClaudeCodeTransformer.
// It supports both OAuth and API key authentication modes.
func NewOutboundTransformer(params Params) (*ClaudeCodeTransformer, error) {
	var outbound transformer.Outbound
	var err error

	if params.Config != nil {
		outbound, err = anthropic.NewOutboundTransformerWithConfig(params.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}
	} else if params.TokenProvider != nil {
		// For OAuth mode, create a minimal config
		outbound, err = anthropic.NewOutboundTransformerWithConfig(&anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create outbound transformer: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either TokenProvider or Config must be provided")
	}

	return &ClaudeCodeTransformer{
		Outbound: outbound,
		tokens:   params.TokenProvider,
	}, nil
}

// NewClaudeCodeTransformer creates a new ClaudeCodeTransformer with API key authentication.
// This is for backward compatibility.
func NewClaudeCodeTransformer(config *anthropic.Config) (*ClaudeCodeTransformer, error) {
	return NewOutboundTransformer(Params{Config: config})
}

// ClaudeCodeTransformer implements the transformer for Claude Code CLI.
// It wraps an OutboundTransformer and adds Claude Code specific headers and system message.
type ClaudeCodeTransformer struct {
	transformer.Outbound
	tokens  oauth.TokenGetter
	baseURL string
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

	// Call the base transformer first
	httpReq, err := t.Outbound.TransformRequest(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}

	// Get API key (either from OAuth or from config)
	apiKey := ""
	if httpReq.Auth != nil {
		apiKey = httpReq.Auth.APIKey
	}

	if t.tokens != nil {
		creds, err := t.tokens.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get oauth token: %w", err)
		}
		apiKey = creds.AccessToken
	}

	// Modify the request body
	if len(httpReq.Body) > 0 {
		bodyBytes := httpReq.Body

		// Extract and remove betas array from body
		extraBetas, bodyBytes := extractAndRemoveBetas(bodyBytes)

		// Inject Claude Code system message with cache_control
		bodyBytes, err = injectClaudeCodeSystemMessage(bodyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to inject system message: %w", err)
		}

		// Inject fake user ID if needed
		bodyBytes = injectFakeUserID(bodyBytes)

		// Disable thinking if tool_choice forces tool use
		bodyBytes = disableThinkingIfToolChoiceForced(bodyBytes)

		// Apply tool prefix for OAuth tokens from non-Claude-CLI clients
		// Skip if: not OAuth token OR is Claude CLI client
		if isClaudeOAuthToken(apiKey) && !keepClientUA {
			bodyBytes = applyClaudeToolPrefix(bodyBytes, toolPrefix)
		}

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

	// Determine authentication method based on endpoint
	// Parse URL to check if it's api.anthropic.com
	parsedURL, err := url.Parse(httpReq.URL)
	if err != nil {
		// Fall back to Bearer auth if URL parsing fails
		httpReq.Headers.Set("Authorization", "Bearer "+apiKey)
		httpReq.Auth = &httpclient.AuthConfig{
			Type:   httpclient.AuthTypeBearer,
			APIKey: apiKey,
		}
	} else {
		isAnthropicBase := strings.EqualFold(parsedURL.Scheme, "https") &&
			strings.EqualFold(parsedURL.Host, "api.anthropic.com")

		// For api.anthropic.com with API key: use x-api-key header
		// For custom endpoints or OAuth tokens: use Bearer token
		if isAnthropicBase && !isClaudeOAuthToken(apiKey) {
			httpReq.Headers.Del("Authorization")
			httpReq.Headers.Set("X-Api-Key", apiKey)
			httpReq.Auth = &httpclient.AuthConfig{
				Type:      httpclient.AuthTypeAPIKey,
				APIKey:    apiKey,
				HeaderKey: "x-api-key",
			}
		} else {
			httpReq.Headers.Set("Authorization", "Bearer "+apiKey)
			httpReq.Auth = &httpclient.AuthConfig{
				Type:   httpclient.AuthTypeBearer,
				APIKey: apiKey,
			}
		}
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
