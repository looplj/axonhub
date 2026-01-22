package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	llmtransformer "github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestClaudeCodeTransformer_TransformRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("api.anthropic.com uses x-api-key auth", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Should use x-api-key for api.anthropic.com
		assert.Equal(t, "sk-ant-api-key", httpReq.Headers.Get("X-Api-Key"))
		assert.Empty(t, httpReq.Headers.Get("Authorization"))
		assert.Equal(t, httpclient.AuthTypeAPIKey, httpReq.Auth.Type)
	})

	t.Run("custom endpoint uses Bearer auth", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://custom.example.com/v1",
			APIKey:  "sk-ant-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Should use Bearer for custom endpoints
		assert.Equal(t, "Bearer sk-ant-api-key", httpReq.Headers.Get("Authorization"))
		assert.Empty(t, httpReq.Headers.Get("X-Api-Key"))
		assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	})

	t.Run("OAuth token uses Bearer auth", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-oat01-oauth-token",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// OAuth tokens always use Bearer
		assert.Equal(t, "Bearer sk-ant-oat01-oauth-token", httpReq.Headers.Get("Authorization"))
		assert.Empty(t, httpReq.Headers.Get("X-Api-Key"))
		assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
	})

	t.Run("injects Claude Code system message with cache_control", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "test-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Check system message is injected with cache_control
		system := gjson.GetBytes(httpReq.Body, "system")
		require.True(t, system.Exists())
		require.True(t, system.IsArray())

		firstMsg := system.Array()[0]
		assert.Equal(t, "text", firstMsg.Get("type").String())
		assert.Equal(t, claudeCodeSystemMessage, firstMsg.Get("text").String())
		assert.Equal(t, "ephemeral", firstMsg.Get("cache_control.type").String())
	})

	t.Run("sets all Claude Code headers", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "test-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Verify all Claude Code headers
		assert.Contains(t, httpReq.Headers.Get("Anthropic-Beta"), "claude-code-20250219")
		assert.Equal(t, "2023-06-01", httpReq.Headers.Get("Anthropic-Version"))
		assert.Equal(t, "true", httpReq.Headers.Get("Anthropic-Dangerous-Direct-Browser-Access"))
		assert.Equal(t, "cli", httpReq.Headers.Get("X-App"))
		assert.Equal(t, "stream", httpReq.Headers.Get("X-Stainless-Helper-Method"))
		assert.Equal(t, UserAgent, httpReq.Headers.Get("User-Agent"))
	})

	t.Run("adds beta=true query parameter", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "test-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "true", httpReq.Query.Get("beta"))
	})

	t.Run("applies tool prefix for OAuth tokens from non-CLI clients", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-oat01-oauth-token",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
			Tools: []llm.Tool{
				{
					Type:     "function",
					Function: llm.Function{Name: "bash", Description: "Execute bash"},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Tool name should have proxy_ prefix
		toolName := gjson.GetBytes(httpReq.Body, "tools.0.name").String()
		assert.Equal(t, "proxy_bash", toolName)

		// Metadata should indicate prefix was applied
		assert.Equal(t, "true", httpReq.Metadata["strip_tool_prefix"])
	})

	t.Run("does not apply tool prefix for API keys", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
			Tools: []llm.Tool{
				{
					Type:     "function",
					Function: llm.Function{Name: "bash", Description: "Execute bash"},
				},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Tool name should NOT have proxy_ prefix
		toolName := gjson.GetBytes(httpReq.Body, "tools.0.name").String()
		assert.Equal(t, "bash", toolName)

		// Metadata should not indicate prefix
		assert.Empty(t, httpReq.Metadata["strip_tool_prefix"])
	})

	t.Run("does not apply tool prefix for Claude CLI clients", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-oat01-oauth-token",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
			Tools: []llm.Tool{
				{
					Type:     "function",
					Function: llm.Function{Name: "bash", Description: "Execute bash"},
				},
			},
			RawRequest: &httpclient.Request{
				Headers: http.Header{"User-Agent": []string{"claude-cli/1.0.83"}},
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Tool name should NOT have proxy_ prefix (Claude CLI client detected)
		toolName := gjson.GetBytes(httpReq.Body, "tools.0.name").String()
		assert.Equal(t, "bash", toolName)

		// Metadata should not indicate prefix
		assert.Empty(t, httpReq.Metadata["strip_tool_prefix"])
	})

	t.Run("injects fake user ID", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "test-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Should have generated user ID
		userID := gjson.GetBytes(httpReq.Body, "metadata.user_id").String()
		assert.NotEmpty(t, userID)
		assert.True(t, isValidUserID(userID))
	})

	t.Run("disables thinking when tool_choice forces tool use", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "test-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		toolChoiceAny := "any"
		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
			Tools: []llm.Tool{
				{
					Type:     "function",
					Function: llm.Function{Name: "bash", Description: "Execute bash"},
				},
			},
			ToolChoice: &llm.ToolChoice{ToolChoice: &toolChoiceAny},
			RawRequest: &httpclient.Request{
				Body: mustMarshal(map[string]any{
					"thinking": map[string]any{
						"type":   "enabled",
						"budget": 10000,
					},
				}),
			},
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Thinking should be removed
		thinking := gjson.GetBytes(httpReq.Body, "thinking")
		assert.False(t, thinking.Exists())
	})
}

func TestClaudeCodeTransformer_TransformResponse(t *testing.T) {
	ctx := context.Background()

	t.Run("strips tool prefix when it was applied", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-oat01-oauth-token",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		// Simulate response from Claude with prefixed tool name
		responseBody := mustMarshal(map[string]any{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-sonnet-4-5",
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"id":    "toolu_123",
					"name":  "proxy_bash",
					"input": map[string]any{"command": "ls"},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]any{
				"input_tokens":  100,
				"output_tokens": 50,
			},
		})

		httpResp := &httpclient.Response{
			StatusCode: 200,
			Body:       responseBody,
			Request: &httpclient.Request{
				Metadata: map[string]string{"strip_tool_prefix": "true"},
			},
		}

		llmResp, err := transformer.TransformResponse(ctx, httpResp)
		require.NoError(t, err)

		// Tool name should have prefix stripped
		require.Len(t, llmResp.Choices, 1)
		require.Len(t, llmResp.Choices[0].Message.ToolCalls, 1)
		assert.Equal(t, "bash", llmResp.Choices[0].Message.ToolCalls[0].Function.Name)
	})

	t.Run("does not strip when prefix was not applied", func(t *testing.T) {
		config := &anthropic.Config{
			Type:    anthropic.PlatformClaudeCode,
			BaseURL: "https://api.anthropic.com/v1",
			APIKey:  "sk-ant-api-key",
		}

		transformer, err := NewClaudeCodeTransformer(config)
		require.NoError(t, err)

		// Simulate response from Claude
		responseBody := mustMarshal(map[string]any{
			"id":    "msg_123",
			"type":  "message",
			"role":  "assistant",
			"model": "claude-sonnet-4-5",
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"id":    "toolu_123",
					"name":  "bash",
					"input": map[string]any{"command": "ls"},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]any{
				"input_tokens":  100,
				"output_tokens": 50,
			},
		})

		httpResp := &httpclient.Response{
			StatusCode: 200,
			Body:       responseBody,
			Request:    &httpclient.Request{},
		}

		llmResp, err := transformer.TransformResponse(ctx, httpResp)
		require.NoError(t, err)

		// Tool name should remain unchanged
		require.Len(t, llmResp.Choices, 1)
		require.Len(t, llmResp.Choices[0].Message.ToolCalls, 1)
		assert.Equal(t, "bash", llmResp.Choices[0].Message.ToolCalls[0].Function.Name)
	})
}

func TestClaudeCodeTransformer_APIFormat(t *testing.T) {
	config := &anthropic.Config{
		Type:   anthropic.PlatformClaudeCode,
		APIKey: "test-api-key",
	}

	transformer, err := NewClaudeCodeTransformer(config)
	require.NoError(t, err)

	assert.Equal(t, llm.APIFormatAnthropicMessage, transformer.APIFormat())
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return b
}

// Fake outbound transformer for testing

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
