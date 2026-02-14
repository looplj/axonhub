package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/streams"
	llmtransformer "github.com/looplj/axonhub/llm/transformer"
)

func TestClaudeCodeTransformer_TransformRequest(t *testing.T) {
	ctx := context.Background()

	t.Run("Claude Code always uses Bearer auth", func(t *testing.T) {
		transformer, err := NewOutboundTransformer(Params{
			TokenProvider: newMockTokenProvider("sk-ant-oat01-oauth-token"),
		})
		require.NoError(t, err)

		req := &llm.Request{
			Model:     "claude-sonnet-4-5",
			Messages:  []llm.Message{{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}}},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// Claude Code OAuth always uses Bearer. HttpClient will set the Authorization header.
		assert.Equal(t, httpclient.AuthTypeBearer, httpReq.Auth.Type)
		assert.Equal(t, "sk-ant-oat01-oauth-token", httpReq.Auth.APIKey)
	})

	t.Run("injects Claude Code system message with cache_control", func(t *testing.T) {

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("sk-ant-oat01-test-oauth-token")})
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

	t.Run("does not apply tool prefix for Claude CLI clients", func(t *testing.T) {

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("sk-ant-oat01-test-oauth-token")})
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

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

	t.Run("removes billing system messages when not official", func(t *testing.T) {

		transformer, err := NewOutboundTransformer(Params{
			TokenProvider: newMockTokenProvider("test-api-key"),
			IsOfficial:    false,
		})
		require.NoError(t, err)

		billingMsg := "x-anthropic-billing-header: cc_version=2.1.37.fbe; cc_entrypoint=cli; cch=e478e;"
		req := &llm.Request{
			Model: "claude-sonnet-4-5",
			Messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: &billingMsg}},
				{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}},
			},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// The billing system message should be removed
		system := gjson.GetBytes(httpReq.Body, "system")
		require.True(t, system.Exists())

		// Should only contain the injected Claude Code system message, not the billing header
		for _, item := range system.Array() {
			assert.NotContains(t, item.Get("text").String(), "x-anthropic-billing-header")
		}
	})

	t.Run("preserves billing system messages when official", func(t *testing.T) {

		transformer, err := NewOutboundTransformer(Params{
			TokenProvider: newMockTokenProvider("test-api-key"),
			IsOfficial:    true,
		})
		require.NoError(t, err)

		billingMsg := "x-anthropic-billing-header: cc_version=2.1.37.fbe; cc_entrypoint=cli; cch=e478e;"
		req := &llm.Request{
			Model: "claude-sonnet-4-5",
			Messages: []llm.Message{
				{Role: "system", Content: llm.MessageContent{Content: &billingMsg}},
				{Role: "user", Content: llm.MessageContent{Content: strPtr("Hello")}},
			},
			MaxTokens: int64Ptr(1024),
		}

		httpReq, err := transformer.TransformRequest(ctx, req)
		require.NoError(t, err)

		// The billing system message should be preserved
		system := gjson.GetBytes(httpReq.Body, "system")
		require.True(t, system.Exists())

		foundBilling := false
		for _, item := range system.Array() {
			if strings.Contains(item.Get("text").String(), "x-anthropic-billing-header") {
				foundBilling = true
			}
		}
		assert.True(t, foundBilling, "billing system message should be preserved for official channels")
	})

	t.Run("disables thinking when tool_choice forces tool use", func(t *testing.T) {

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

func TestClaudeCodeTransformer_TransformStream(t *testing.T) {
	ctx := context.Background()

	t.Run("strips tool prefix from streaming responses", func(t *testing.T) {
		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
		require.NoError(t, err)

		// Create mock stream events with prefixed tool names
		events := []*httpclient.StreamEvent{
			{
				Type: "content_block_start",
				Data: mustMarshal(map[string]any{
					"type":  "content_block_start",
					"index": 0,
					"content_block": map[string]any{
						"type": "tool_use",
						"id":   "toolu_123",
						"name": "proxy_bash",
					},
				}),
			},
		}

		// Create a mock stream
		mockStream := newMockHTTPStream(events)

		// Transform the stream
		llmStream, err := transformer.TransformStream(ctx, mockStream)
		require.NoError(t, err)

		// Read from the stream and verify prefix is stripped
		responses := []*llm.Response{}
		for llmStream.Next() {
			resp := llmStream.Current()
			if resp != nil {
				responses = append(responses, resp)
			}
		}

		require.NoError(t, llmStream.Err())

		// Verify we got responses and tool names are stripped
		require.NotEmpty(t, responses)
		foundToolCall := false
		for _, resp := range responses {
			for _, choice := range resp.Choices {
				if choice.Delta != nil && len(choice.Delta.ToolCalls) > 0 {
					for _, toolCall := range choice.Delta.ToolCalls {
						if toolCall.Function.Name != "" {
							foundToolCall = true
							assert.Equal(t, "bash", toolCall.Function.Name, "tool prefix should be stripped")
							assert.NotContains(t, toolCall.Function.Name, "proxy_", "proxy_ prefix should be removed")
						}
					}
				}
			}
		}
		assert.True(t, foundToolCall, "should have found at least one tool call")
	})

	t.Run("handles streams without tool calls", func(t *testing.T) {
		transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
		require.NoError(t, err)

		// Create mock stream events without tool calls
		events := []*httpclient.StreamEvent{
			{
				Type: "message_start",
				Data: mustMarshal(map[string]any{
					"type": "message_start",
					"message": map[string]any{
						"id":    "msg_123",
						"model": "claude-3-5-sonnet-20241022",
					},
				}),
			},
		}

		mockStream := newMockHTTPStream(events)
		llmStream, err := transformer.TransformStream(ctx, mockStream)
		require.NoError(t, err)

		// Should not error
		for llmStream.Next() {
			_ = llmStream.Current()
		}
		require.NoError(t, llmStream.Err())
	})
}

func TestClaudeCodeTransformer_APIFormat(t *testing.T) {

	transformer, err := NewOutboundTransformer(Params{TokenProvider: newMockTokenProvider("test-api-key")})
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

// mockTokenProvider is a test implementation of oauth.TokenGetter
type mockTokenProvider struct {
	accessToken string
}

func (m *mockTokenProvider) Get(_ context.Context) (*oauth.OAuthCredentials, error) {
	return &oauth.OAuthCredentials{
		AccessToken:  m.accessToken,
		RefreshToken: "mock-refresh-token",
		TokenType:    "Bearer",
	}, nil
}

func newMockTokenProvider(token string) *mockTokenProvider {
	return &mockTokenProvider{accessToken: token}
}

// mockHTTPStream is a simple mock implementation of streams.Stream[*httpclient.StreamEvent]
type mockHTTPStream struct {
	events  []*httpclient.StreamEvent
	index   int
	current *httpclient.StreamEvent
}

func newMockHTTPStream(events []*httpclient.StreamEvent) *mockHTTPStream {
	return &mockHTTPStream{
		events: events,
		index:  -1,
	}
}

func (m *mockHTTPStream) Next() bool {
	m.index++
	if m.index >= len(m.events) {
		return false
	}
	m.current = m.events[m.index]
	return true
}

func (m *mockHTTPStream) Current() *httpclient.StreamEvent {
	return m.current
}

func (m *mockHTTPStream) Err() error {
	return nil
}

func (m *mockHTTPStream) Close() error {
	return nil
}

