package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	llmtransformer "github.com/looplj/axonhub/llm/transformer"
)

func TestClaudeCodeTransformer_TransformRequest(t *testing.T) {
	ctx := context.Background()

	// Create a ClaudeCode transformer
	config := &Config{
		Type:    PlatformClaudeCode,
		BaseURL: "https://example.com",
		APIKey:  "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)
	require.NotNil(t, transformer)

	t.Run("prepends system message", func(t *testing.T) {
		req := &llm.Request{
			Model: "claude-sonnet-4-5-20250514",
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
			},
			MaxTokens: lo.ToPtr(int64(1024)),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		assert.Equal(t, "https://example.com/v1/messages", httpReq.URL)
		require.NotNil(t, httpReq.Query)
		assert.Equal(t, "true", httpReq.Query.Get("beta"))

		// Verify Claude Code specific headers
		assert.Equal(t, "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,fine-grained-tool-streaming-2025-05-14", httpReq.Headers.Get("Anthropic-Beta"))
		assert.Equal(t, "2023-06-01", httpReq.Headers.Get("Anthropic-Version"))
		assert.Equal(t, "true", httpReq.Headers.Get("Anthropic-Dangerous-Direct-Browser-Access"))
		assert.Equal(t, "claude-cli/1.0.83 (external, cli)", httpReq.Headers.Get("User-Agent"))
		assert.Equal(t, "cli", httpReq.Headers.Get("X-App"))
		assert.Equal(t, "stream", httpReq.Headers.Get("X-Stainless-Helper-Method"))
		assert.Equal(t, "0", httpReq.Headers.Get("X-Stainless-Retry-Count"))
		assert.Equal(t, "v24.3.0", httpReq.Headers.Get("X-Stainless-Runtime-Version"))
		assert.Equal(t, "0.55.1", httpReq.Headers.Get("X-Stainless-Package-Version"))
		assert.Equal(t, "node", httpReq.Headers.Get("X-Stainless-Runtime"))

		// Verify Bearer authentication
		require.NotNil(t, httpReq.Auth)
		assert.Equal(t, "bearer", httpReq.Auth.Type)
		assert.Equal(t, "test-api-key", httpReq.Auth.APIKey)

		// Verify the prepended system message
		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		// The outbound transformer should move the system message to the dedicated `system` field.
		require.NotNil(t, anthropicReq.System)
		require.NotNil(t, anthropicReq.System.Prompt)
		assert.Contains(t, *anthropicReq.System.Prompt, claudeCodeSystemMessage)
	})

	t.Run("works with existing system message", func(t *testing.T) {
		req := &llm.Request{
			Model: "claude-sonnet-4-5-20250514",
			Messages: []llm.Message{
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: lo.ToPtr("You are a helpful assistant"),
					},
				},
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
			},
			MaxTokens: lo.ToPtr(int64(1024)),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		assert.Equal(t, "https://example.com/v1/messages", httpReq.URL)
		require.NotNil(t, httpReq.Query)
		assert.Equal(t, "true", httpReq.Query.Get("beta"))
		assert.NotEmpty(t, httpReq.Body)
	})

	t.Run("does not duplicate Claude Code system message", func(t *testing.T) {
		// Simulate a request that already has the Claude Code system message
		// (as would come from the Claude Code CLI)
		req := &llm.Request{
			Model: "claude-sonnet-4-5-20250514",
			Messages: []llm.Message{
				{
					Role: "system",
					Content: llm.MessageContent{
						Content: lo.ToPtr(claudeCodeSystemMessage),
					},
				},
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
			},
			MaxTokens: lo.ToPtr(int64(1024)),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		// Verify the system message is not duplicated
		var anthropicReq MessageRequest

		err = json.Unmarshal(httpReq.Body, &anthropicReq)
		require.NoError(t, err)

		// The outbound transformer should move the system message to the dedicated `system` field.
		require.NotNil(t, anthropicReq.System)
		require.NotNil(t, anthropicReq.System.Prompt)

		// Verify it contains the Claude Code message exactly once (not duplicated)
		systemContent := *anthropicReq.System.Prompt
		assert.Equal(t, claudeCodeSystemMessage, systemContent, "System message should be exactly the Claude Code message, not duplicated")
	})

	t.Run("works with streaming", func(t *testing.T) {
		req := &llm.Request{
			Model: "claude-sonnet-4-5-20250514",
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
			},
			MaxTokens: lo.ToPtr(int64(1024)),
			Stream:    lo.ToPtr(true),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, httpReq)

		assert.Equal(t, "https://example.com/v1/messages", httpReq.URL)
		require.NotNil(t, httpReq.Query)
		assert.Equal(t, "true", httpReq.Query.Get("beta"))
	})

	t.Run("requires model", func(t *testing.T) {
		req := &llm.Request{
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
			},
		}

		_, err := transformer.TransformRequest(ctx, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "model is required")
	})

	t.Run("does not duplicate beta query", func(t *testing.T) {
		t1 := &ClaudeCodeTransformer{
			Outbound: &fakeOutbound{
				req: &httpclient.Request{
					Method:  http.MethodPost,
					URL:     "https://example.com/v1/messages",
					Query:   url.Values{"beta": []string{"true"}},
					Headers: http.Header{},
					Auth: &httpclient.AuthConfig{
						Type:   httpclient.AuthTypeAPIKey,
						APIKey: "test-api-key",
					},
				},
			},
		}

		httpReq, err := t1.TransformRequest(ctx, &llm.Request{
			Model: "claude-sonnet-4-5-20250514",
			Messages: []llm.Message{
				{
					Role: "user",
					Content: llm.MessageContent{
						Content: lo.ToPtr("Hello"),
					},
				},
			},
			MaxTokens: lo.ToPtr(int64(1024)),
		})
		require.NoError(t, err)
		require.NotNil(t, httpReq.Query)
		assert.Equal(t, "https://example.com/v1/messages", httpReq.URL)
		assert.Equal(t, []string{"true"}, httpReq.Query["beta"])
	})
}

func TestClaudeCodeTransformer_APIFormat(t *testing.T) {
	config := &Config{
		Type:   PlatformClaudeCode,
		APIKey: "test-api-key",
	}

	transformer, err := NewOutboundTransformerWithConfig(config)
	require.NoError(t, err)

	assert.Equal(t, llm.APIFormatAnthropicMessage, transformer.APIFormat())
}

type fakeOutbound struct {
	req *httpclient.Request
}

func (t *fakeOutbound) APIFormat() llm.APIFormat {
	return llm.APIFormatAnthropicMessage
}

func (t *fakeOutbound) TransformRequest(_ context.Context, _ *llm.Request) (*httpclient.Request, error) {
	return t.req, nil
}

func (t *fakeOutbound) TransformResponse(_ context.Context, _ *httpclient.Response) (*llm.Response, error) {
	return nil, nil
}

func (t *fakeOutbound) TransformStream(_ context.Context, _ streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return nil, nil
}

func (t *fakeOutbound) TransformError(_ context.Context, _ *httpclient.Error) *llm.ResponseError {
	return nil
}

func (t *fakeOutbound) AggregateStreamChunks(_ context.Context, _ []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	return nil, llm.ResponseMeta{}, nil
}

var _ llmtransformer.Outbound = (*fakeOutbound)(nil)
