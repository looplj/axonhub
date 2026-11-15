// 职责: 验证 Anthropic 原生 web_search 的专属字段不会泄漏进 Responses 请求。
// 关联文件:
//
//	llm/transformer/openai/responses/outbound_convert.go
//	llm/tools.go
//
// 注意事项:
//
//	只允许映射 Responses 认识的 filters 与 user_location；version、allowed_callers 等必须丢弃。
package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

// TestConvertWebSearchToTool_DropsAnthropicOnlyConfig ensures a native Anthropic
// web search tool that carries a version and Anthropic-only options does not leak
// those options into the Responses request.
func TestConvertWebSearchToTool_DropsAnthropicOnlyConfig(t *testing.T) {
	tool := convertWebSearchToTool(llm.Tool{
		Type: llm.ToolTypeWebSearch,
		WebSearch: &llm.WebSearch{
			Version:        "web_search_20260209",
			MaxUses:        lo.ToPtr(int64(5)),
			Strict:         lo.ToPtr(true),
			AllowedCallers: []string{"direct"},
			AllowedDomains: []string{"example.com"},
			UserLocation: llm.WebSearchToolUserLocation{
				City: "San Francisco",
				Type: "approximate",
			},
		},
	})

	require.Equal(t, "web_search", tool.Type)
	require.NotNil(t, tool.Filters)
	require.Equal(t, []string{"example.com"}, tool.Filters.AllowedDomains)
	require.NotNil(t, tool.UserLocation)
	require.Equal(t, "San Francisco", tool.UserLocation.City)

	body, err := json.Marshal(tool)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &fields))
	for _, key := range []string{"version", "allowed_callers", "max_uses", "strict"} {
		require.NotContains(t, fields, key)
	}
}
