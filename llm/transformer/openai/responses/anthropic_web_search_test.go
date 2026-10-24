package responses

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
)

func TestAnthropicWebSearchToolChoice(t *testing.T) {
	for _, tc := range []struct{ name, tool, want string }{
		{"native search", `{"type":"web_search_20250305","name":"web_search"}`, `{"type":"web_search"}`},
		{"function named web_search", `{"name":"web_search","input_schema":{"type":"object"}}`, `{"type":"function","name":"web_search"}`},
		{"native search alongside functions", `{"type":"web_search_20250305","name":"web_search"},{"name":"calculator","input_schema":{"type":"object"}}`, `{"type":"web_search"}`},
		{"function takes precedence on name collision", `{"type":"web_search_20250305","name":"web_search"},{"name":"web_search","input_schema":{"type":"object"}}`, `{"type":"function","name":"web_search"}`},
		{"no native declaration", `{"name":"calculator","input_schema":{"type":"object"}}`, `{"type":"function","name":"web_search"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			in, err := anthropic.NewInboundTransformer().TransformRequest(ctx, &httpclient.Request{
				Headers: http.Header{"Content-Type": []string{"application/json"}},
				Body:    []byte(`{"model":"gpt-6-luna","max_tokens":1024,"messages":[{"role":"user","content":"Search Debian"}],"tools":[` + tc.tool + `],"tool_choice":{"type":"tool","name":"web_search"}}`),
			})
			require.NoError(t, err)
			if in.TransformerMetadata == nil {
				in.TransformerMetadata = map[string]any{}
			}
			originalIncludes := []string{"reasoning.encrypted_content"}
			in.TransformerMetadata["include"] = originalIncludes
			out, err := NewOutboundTransformer("https://api.openai.com", "test-key")
			require.NoError(t, err)
			req, err := out.TransformRequest(ctx, in)
			require.NoError(t, err)
			native := false
			for _, tool := range in.Tools {
				if tool.Type == llm.ToolTypeWebSearch {
					native = true
				}
			}
			includes := gjson.GetBytes(req.Body, "include").Array()
			require.Equal(t, "reasoning.encrypted_content", includes[0].String())
			if native {
				require.Len(t, includes, 2)
				require.Equal(t, "web_search_call.action.sources", includes[1].String())
				in.TransformerMetadata["include"] = []string{"reasoning.encrypted_content", "web_search_call.action.sources"}
				again, err := out.TransformRequest(ctx, in)
				require.NoError(t, err)
				require.Len(t, gjson.GetBytes(again.Body, "include").Array(), 2, "source include must not duplicate")
			} else {
				require.Len(t, includes, 1)
			}
			require.Equal(t, []string{"reasoning.encrypted_content"}, originalIncludes)
			require.JSONEq(t, tc.want, gjson.GetBytes(req.Body, "tool_choice").Raw)
			require.Equal(t, "function", in.ToolChoice.NamedToolChoice.Type, "must not mutate the shared request")
		})
	}
}
