// 职责: 字段级锁定原生 web_search 版本的白名单、映射、平台过滤与 beta 头差异。
// 关联文件:
//
//	llm/transformer/anthropic/tools.go
//	llm/transformer/anthropic/outbound_convert.go
//	llm/transformer/anthropic/outbound.go
//
// 注意事项:
//
//	映射层对未知非空版本原样返回，显式 ErrInvalidRequest 只在转换入口产生。
//	不支持原生工具的平台继续整体过滤，不注入 code_execution 或额外工具。
package anthropic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestConvertToolToLLM_NativeWebSearchVersionPreserved covers the inbound direction:
// every known native web search version must survive as web_search with its version,
// allowed_callers, and filters intact.
func TestConvertToolToLLM_NativeWebSearchVersionPreserved(t *testing.T) {
	for _, version := range []string{
		ToolTypeWebSearch20250305,
		ToolTypeWebSearch20260209,
	} {
		t.Run(version, func(t *testing.T) {
			unified, ok := convertToolToLLM(Tool{
				Type:           version,
				Name:           WebSearchFunctionName,
				MaxUses:        lo.ToPtr(int64(3)),
				AllowedDomains: []string{"example.com"},
				AllowedCallers: []string{"direct"},
				UserLocation: WebSearchToolUserLocation{
					City: "San Francisco",
					Type: "approximate",
				},
			})
			require.True(t, ok)
			require.Equal(t, llm.ToolTypeWebSearch, unified.Type)
			require.NotNil(t, unified.WebSearch)
			require.Equal(t, version, unified.WebSearch.Version)
			require.Equal(t, []string{"direct"}, unified.WebSearch.AllowedCallers)
			require.Equal(t, int64(3), lo.FromPtr(unified.WebSearch.MaxUses))
			require.Equal(t, []string{"example.com"}, unified.WebSearch.AllowedDomains)
			require.Equal(t, "San Francisco", unified.WebSearch.UserLocation.City)
		})
	}
}

// TestConvertToolToLLM_UnknownNativeWebSearchVersionDropped pins the behavior for
// versions we do not know: they are dropped instead of being guessed from a prefix
// or downgraded into a function tool.
func TestConvertToolToLLM_UnknownNativeWebSearchVersionDropped(t *testing.T) {
	_, ok := convertToolToLLM(Tool{
		Type: "web_search_29990101",
		Name: WebSearchFunctionName,
	})
	require.False(t, ok)
}

// TestNativeWebSearchRoundTripPreservesVersionAndConfig covers the same-protocol
// direction: an Anthropic native web search tool must come back out as the same
// version with the same legal configuration.
func TestNativeWebSearchRoundTripPreservesVersionAndConfig(t *testing.T) {
	for _, version := range []string{
		ToolTypeWebSearch20250305,
		ToolTypeWebSearch20260209,
	} {
		t.Run(version, func(t *testing.T) {
			unified, ok := convertToolToLLM(Tool{
				Type:           version,
				Name:           WebSearchFunctionName,
				MaxUses:        lo.ToPtr(int64(2)),
				BlockedDomains: []string{"blocked.example"},
				AllowedCallers: []string{"direct"},
			})
			require.True(t, ok)

			out := convertToolsAnthropic([]llm.Tool{unified}, &Config{Type: PlatformDirect})
			require.Len(t, out, 1)
			require.Equal(t, version, out[0].Type)
			require.Equal(t, WebSearchFunctionName, out[0].Name)
			require.Equal(t, []string{"direct"}, out[0].AllowedCallers)
			require.Equal(t, int64(2), lo.FromPtr(out[0].MaxUses))
			require.Equal(t, []string{"blocked.example"}, out[0].BlockedDomains)

			// No tool other than the web search tool may be synthesized.
			body, err := json.Marshal(out[0])
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &fields))
			require.NotContains(t, fields, "input_schema")
			require.NotContains(t, fields, "description")
		})
	}
}

// TestNativeWebSearchLegacyInputUnchanged documents that callers using the legacy
// function-style web_search tool keep the old wire shape and are not upgraded.
func TestNativeWebSearchLegacyInputUnchanged(t *testing.T) {
	unified, ok := convertToolToLLM(Tool{Type: WebSearchFunctionName, Name: WebSearchFunctionName})
	require.True(t, ok)
	require.Equal(t, llm.ToolTypeWebSearch, unified.Type)
	require.NotNil(t, unified.WebSearch)
	require.Empty(t, unified.WebSearch.Version)

	out := convertToolsAnthropic([]llm.Tool{unified}, nil)
	require.Len(t, out, 1)
	require.Equal(t, ToolTypeWebSearch20250305, out[0].Type)

	body, err := json.Marshal(out[0])
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	require.NotContains(t, fields, "allowed_callers")
	require.NotContains(t, fields, "allowed_domains")
	require.NotContains(t, fields, "blocked_domains")
}

// TestConvertToolsAnthropic_UnknownVersionNotDowngraded guards the mapper itself:
// only an empty version may fall back to the original tool. A non-empty unknown
// version is passed through rather than silently rewritten, and the transformer
// entry rejects it before this point.
func TestConvertToolsAnthropic_UnknownVersionNotDowngraded(t *testing.T) {
	out := convertToolsAnthropic([]llm.Tool{{
		Type:      llm.ToolTypeWebSearch,
		WebSearch: &llm.WebSearch{Version: "web_search_29990101"},
	}}, &Config{Type: PlatformDirect})
	require.Len(t, out, 1)
	require.Equal(t, "web_search_29990101", out[0].Type)
}

// TestNativeWebSearchUnsupportedPlatformStaysFiltered ensures newer versions do not
// expand the platform allowlist and never inject a code execution tool.
func TestNativeWebSearchUnsupportedPlatformStaysFiltered(t *testing.T) {
	for _, platform := range []PlatformType{
		PlatformVertex,
		PlatformDoubao,
		PlatformMoonshot,
		PlatformZhipu,
		PlatformZai,
		PlatformCommandCode,
	} {
		t.Run(string(platform), func(t *testing.T) {
			out := convertToolsAnthropic([]llm.Tool{{
				Type: llm.ToolTypeWebSearch,
				WebSearch: &llm.WebSearch{
					Version:        ToolTypeWebSearch20260209,
					AllowedCallers: []string{"direct"},
				},
			}}, &Config{Type: platform})
			require.Empty(t, out)
		})
	}
}

// TestOutboundTransformer_WebSearchBetaHeaderByVersion pins the beta header to the
// version that needs it. The legacy tool keeps its existing header; the newer tool
// does not inherit it.
func TestOutboundTransformer_WebSearchBetaHeaderByVersion(t *testing.T) {
	transformer, err := NewOutboundTransformer("https://api.anthropic.com", "test-api-key")
	require.NoError(t, err)

	build := func(version string) *httpclient.Request {
		text := "Search the web for latest AI news"
		result, err := transformer.TransformRequest(context.Background(), &llm.Request{
			Model:     "claude-3-sonnet-20240229",
			MaxTokens: lo.ToPtr(int64(1024)),
			Messages: []llm.Message{{
				Role:    "user",
				Content: llm.MessageContent{Content: &text},
			}},
			Tools: []llm.Tool{{
				Type:      llm.ToolTypeWebSearch,
				WebSearch: &llm.WebSearch{Version: version},
			}},
		})
		require.NoError(t, err)
		return result
	}

	legacy := build(ToolTypeWebSearch20250305)
	require.Equal(t, "web-search-2025-03-05", legacy.Headers.Get("Anthropic-Beta"))

	current := build(ToolTypeWebSearch20260209)
	require.Empty(t, current.Headers.Get("Anthropic-Beta"))

	var req MessageRequest
	require.NoError(t, json.Unmarshal(current.Body, &req))
	require.Len(t, req.Tools, 1)
	require.Equal(t, ToolTypeWebSearch20260209, req.Tools[0].Type)
}
