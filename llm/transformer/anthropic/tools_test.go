package anthropic

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestContainsAnthropicNativeTools(t *testing.T) {
	tests := []struct {
		name  string
		tools []llm.Tool
		want  bool
	}{
		{
			name:  "nil tools",
			tools: nil,
			want:  false,
		},
		{
			name:  "empty tools",
			tools: []llm.Tool{},
			want:  false,
		},
		{
			name: "only function tools without web_search",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: "function", Function: llm.Function{Name: "calculator"}},
			},
			want: false,
		},
		{
			name: "contains web_search function",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: "function", Function: llm.Function{Name: "web_search"}},
			},
			want: false,
		},
		{
			name: "web_search at the beginning",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "web_search"}},
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
			},
			want: false,
		},
		{
			name: "only web_search",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "web_search"}},
			},
			want: false,
		},
		{
			name: "web_search with different type (not function)",
			tools: []llm.Tool{
				{Type: "not_function", Function: llm.Function{Name: "web_search"}},
			},
			want: false,
		},
		{
			name: "contains native web_search_20250305 type",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: ToolTypeWebSearch20250305},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsAnthropicNativeTools(tt.tools)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsAnthropicNativeTool(t *testing.T) {
	tests := []struct {
		name string
		tool llm.Tool
		want bool
	}{
		{
			name: "function tool with web_search name",
			tool: llm.Tool{Type: "function", Function: llm.Function{Name: "web_search"}},
			want: false,
		},
		{
			name: "function tool with other name",
			tool: llm.Tool{Type: "function", Function: llm.Function{Name: "get_weather"}},
			want: false,
		},
		{
			name: "non-function tool with web_search name",
			tool: llm.Tool{Type: "other", Function: llm.Function{Name: "web_search"}},
			want: false,
		},
		{
			name: "google_search tool",
			tool: llm.Tool{Type: llm.ToolTypeGoogleSearch},
			want: false,
		},
		{
			name: "empty tool",
			tool: llm.Tool{},
			want: false,
		},
		{
			name: "native web_search_20250305 type (already transformed)",
			tool: llm.Tool{Type: ToolTypeWebSearch20250305},
			want: true,
		},
		{
			name: "native web_search_20250305 type with name",
			tool: llm.Tool{Type: ToolTypeWebSearch20250305, Function: llm.Function{Name: "web_search"}},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAnthropicNativeTool(tt.tool)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFilterOutAnthropicNativeTools(t *testing.T) {
	tests := []struct {
		name     string
		tools    []llm.Tool
		wantLen  int
		wantType []string
	}{
		{
			name:     "nil tools",
			tools:    nil,
			wantLen:  0,
			wantType: nil,
		},
		{
			name:     "empty tools",
			tools:    []llm.Tool{},
			wantLen:  0,
			wantType: nil,
		},
		{
			name: "only function tools without web_search - no filtering",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: "function", Function: llm.Function{Name: "calculator"}},
			},
			wantLen:  2,
			wantType: []string{"function", "function"},
		},
		{
			name: "filter web_search",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: "function", Function: llm.Function{Name: "web_search"}},
			},
			wantLen:  2,
			wantType: []string{"function", "function"},
		},
		{
			name: "all web_search tools - no filtering for function type",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "web_search"}},
			},
			wantLen:  1,
			wantType: []string{"function"},
		},
		{
			name: "mixed tools",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "fn1"}},
				{Type: "function", Function: llm.Function{Name: "web_search"}},
				{Type: "function", Function: llm.Function{Name: "fn2"}},
			},
			wantLen:  3,
			wantType: []string{"function", "function", "function"},
		},
		{
			name: "filter native web_search_20250305 type",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "fn1"}},
				{Type: ToolTypeWebSearch20250305},
			},
			wantLen:  1,
			wantType: []string{"function"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterOutAnthropicNativeTools(tt.tools)
			require.Len(t, got, tt.wantLen)

			if len(tt.wantType) > 0 {
				gotTypes := make([]string, len(got))
				for i, tool := range got {
					gotTypes[i] = tool.Type
				}

				require.Equal(t, tt.wantType, gotTypes)
			}
		})
	}
}
