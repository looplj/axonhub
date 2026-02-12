package llm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
)

func TestContainsGoogleNativeTools(t *testing.T) {
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
			name: "only function tools",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: "function", Function: llm.Function{Name: "search"}},
			},
			want: false,
		},
		{
			name: "contains google_search",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			},
			want: true,
		},
		{
			name: "contains google_url_context",
			tools: []llm.Tool{
				{Type: llm.ToolTypeGoogleUrlContext, Google: &llm.GoogleTools{UrlContext: &llm.GoogleUrlContext{}}},
			},
			want: true,
		},
		{
			name: "contains google_code_execution",
			tools: []llm.Tool{
				{Type: llm.ToolTypeGoogleCodeExecution, Google: &llm.GoogleTools{CodeExecution: &llm.GoogleCodeExecution{}}},
			},
			want: true,
		},
		{
			name: "contains multiple google native tools",
			tools: []llm.Tool{
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
				{Type: llm.ToolTypeGoogleUrlContext, Google: &llm.GoogleTools{UrlContext: &llm.GoogleUrlContext{}}},
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
			},
			want: true,
		},
		{
			name: "google native tool at the end",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "fn1"}},
				{Type: "function", Function: llm.Function{Name: "fn2"}},
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := llm.ContainsGoogleNativeTools(tt.tools)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsGoogleNativeTool(t *testing.T) {
	tests := []struct {
		name string
		tool llm.Tool
		want bool
	}{
		{
			name: "function tool",
			tool: llm.Tool{Type: "function"},
			want: false,
		},
		{
			name: "image_generation tool",
			tool: llm.Tool{Type: llm.ToolTypeImageGeneration},
			want: false,
		},
		{
			name: "google_search tool",
			tool: llm.Tool{Type: llm.ToolTypeGoogleSearch},
			want: true,
		},
		{
			name: "google_url_context tool",
			tool: llm.Tool{Type: llm.ToolTypeGoogleUrlContext},
			want: true,
		},
		{
			name: "google_code_execution tool",
			tool: llm.Tool{Type: llm.ToolTypeGoogleCodeExecution},
			want: true,
		},
		{
			name: "empty type",
			tool: llm.Tool{Type: ""},
			want: false,
		},
		{
			name: "unknown type",
			tool: llm.Tool{Type: "unknown_tool_type"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := llm.IsGoogleNativeTool(tt.tool)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestFilterGoogleNativeTools(t *testing.T) {
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
			name: "only function tools - no filtering",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: "function", Function: llm.Function{Name: "search"}},
			},
			wantLen:  2,
			wantType: []string{"function", "function"},
		},
		{
			name: "filter google_search",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
			},
			wantLen:  1,
			wantType: []string{"function"},
		},
		{
			name: "filter all google native tools",
			tools: []llm.Tool{
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
				{Type: "function", Function: llm.Function{Name: "get_weather"}},
				{Type: llm.ToolTypeGoogleUrlContext, Google: &llm.GoogleTools{UrlContext: &llm.GoogleUrlContext{}}},
				{Type: llm.ToolTypeGoogleCodeExecution, Google: &llm.GoogleTools{CodeExecution: &llm.GoogleCodeExecution{}}},
			},
			wantLen:  1,
			wantType: []string{"function"},
		},
		{
			name: "all google native tools - empty result",
			tools: []llm.Tool{
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
				{Type: llm.ToolTypeGoogleUrlContext, Google: &llm.GoogleTools{UrlContext: &llm.GoogleUrlContext{}}},
			},
			wantLen:  0,
			wantType: []string{},
		},
		{
			name: "mixed tools with multiple function tools",
			tools: []llm.Tool{
				{Type: "function", Function: llm.Function{Name: "fn1"}},
				{Type: llm.ToolTypeGoogleSearch, Google: &llm.GoogleTools{Search: &llm.GoogleSearch{}}},
				{Type: "function", Function: llm.Function{Name: "fn2"}},
				{Type: llm.ToolTypeGoogleCodeExecution, Google: &llm.GoogleTools{CodeExecution: &llm.GoogleCodeExecution{}}},
				{Type: "function", Function: llm.Function{Name: "fn3"}},
			},
			wantLen:  3,
			wantType: []string{"function", "function", "function"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := llm.FilterGoogleNativeTools(tt.tools)
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
