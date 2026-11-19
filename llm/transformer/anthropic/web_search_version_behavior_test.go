// 职责: 从公开 JSON 请求验证 web_search 版本在入站、同协议出站和插件 JSON hop 上的行为。
// 关联文件:
//
//	llm/transformer/anthropic/inbound_convert.go
//	llm/transformer/anthropic/outbound.go
//	llm/transformer/anthropic/outbound_convert.go
//
// 注意事项:
//
//	行为测试经公开 JSON 请求接口验证，不依赖内部表示。
//	插件 JSON hop 用例要求未知非空版本返回 ErrInvalidRequest，不能降级或静默丢弃。
package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
)

// TestInboundRequestKeepsWebSearch20260209Tool is the behavior baseline for the
// 2026-02-09 native web search tool. It deliberately uses only fields that
// already existed, so it can run against the previous implementation: the tool
// must not disappear while crossing the inbound transformer.
func TestInboundRequestKeepsWebSearch20260209Tool(t *testing.T) {
	inbound := NewInboundTransformer()

	result, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "claude-sonnet-4-5",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": "search the web"}],
			"tools": [{
				"type": "web_search_20260209",
				"name": "web_search",
				"max_uses": 4,
				"allowed_domains": ["example.com"],
				"allowed_callers": ["direct"]
			}]
		}`),
	})
	require.NoError(t, err)
	require.Len(t, result.Tools, 1, "the native web search tool must not be dropped")
	require.Equal(t, llm.ToolTypeWebSearch, result.Tools[0].Type)
	require.NotNil(t, result.Tools[0].WebSearch)
	require.Equal(t, int64(4), lo.FromPtr(result.Tools[0].WebSearch.MaxUses))
	require.Equal(t, []string{"example.com"}, result.Tools[0].WebSearch.AllowedDomains)
}

// TestAnthropicToAnthropicKeepsWebSearch20260209 covers the full same-protocol
// path: an inbound Anthropic request is transformed into the unified request and
// back out to the Anthropic wire format. The versioned tool and its legal
// configuration must survive both hops.
func TestAnthropicToAnthropicKeepsWebSearch20260209(t *testing.T) {
	inbound := NewInboundTransformer()
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	unified, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "claude-sonnet-4-5",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": "search the web"}],
			"tools": [{
				"type": "web_search_20260209",
				"name": "web_search",
				"max_uses": 4,
				"allowed_domains": ["example.com"],
				"blocked_domains": ["blocked.example"],
				"allowed_callers": ["direct"]
			}]
		}`),
	})
	require.NoError(t, err)

	result, err := outbound.TransformRequest(context.Background(), unified)
	require.NoError(t, err)

	var wire MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &wire))
	require.Len(t, wire.Tools, 1, "the native web search tool must not be dropped")
	require.Equal(t, "web_search_20260209", wire.Tools[0].Type)
	require.Equal(t, WebSearchFunctionName, wire.Tools[0].Name)
	require.Equal(t, int64(4), lo.FromPtr(wire.Tools[0].MaxUses))
	require.Equal(t, []string{"example.com"}, wire.Tools[0].AllowedDomains)
	require.Equal(t, []string{"blocked.example"}, wire.Tools[0].BlockedDomains)
	require.Equal(t, []string{"direct"}, wire.Tools[0].AllowedCallers)
	require.Empty(t, result.Headers.Get("Anthropic-Beta"))
}

// TestPluginJSONHopUnknownWebSearchVersionIsRejected turns the review
// counterexample into a source test: a plugin request hook can populate
// llm.Request from public JSON, so an unknown non-empty version must fail with
// the explicit invalid-request error instead of being silently rewritten to the
// original web_search_20250305 contract.
func TestPluginJSONHopUnknownWebSearchVersionIsRejected(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	var decoded llm.Request
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "search the web"}],
		"tools": [{"type": "web_search", "web_search": {"version": "web_search_29990101"}}]
	}`), &decoded))

	result, err := outbound.TransformRequest(context.Background(), &decoded)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
	require.Nil(t, result, "a rejected version must not produce an upstream request")
	require.Contains(t, err.Error(), "web_search_29990101")
	require.Contains(t, err.Error(), ToolTypeWebSearch20250305)
}

// TestPluginJSONHopEmptyWebSearchVersionKeepsLegacyDefault is the positive
// control for the test above: no version keeps the original wire contract.
func TestPluginJSONHopEmptyWebSearchVersionKeepsLegacyDefault(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	var decoded llm.Request
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "claude-sonnet-4-5",
		"max_tokens": 1024,
		"messages": [{"role": "user", "content": "search the web"}],
		"tools": [{"type": "web_search", "web_search": {}}]
	}`), &decoded))

	result, err := outbound.TransformRequest(context.Background(), &decoded)
	require.NoError(t, err)

	var wire MessageRequest
	require.NoError(t, json.Unmarshal(result.Body, &wire))
	require.Len(t, wire.Tools, 1)
	require.Equal(t, ToolTypeWebSearch20250305, wire.Tools[0].Type)
}
