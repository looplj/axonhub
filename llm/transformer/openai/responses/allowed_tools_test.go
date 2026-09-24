package responses

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestAllowedToolsRoundTrip(t *testing.T) {
	for _, mode := range []string{"auto", "required"} {
		for _, count := range []int{1, 2} {
			for _, representation := range []string{"raw", "structured", "serialized"} {
				t.Run(fmt.Sprintf("%s/count=%d/%s", mode, count, representation), func(t *testing.T) {
					choices := []ToolOption{{Type: "function", Name: "a"}, {Type: "function", Name: "b"}}
					input := Request{Model: "test", Input: Input{Text: lo.ToPtr("hi")}, Tools: []Tool{{Type: "function", Name: "a"}, {Type: "function", Name: "b"}}, ToolChoice: &ToolChoice{Type: lo.ToPtr("allowed_tools"), Mode: lo.ToPtr(mode), Tools: choices[:count]}}

					raw, err := json.Marshal(input)
					require.NoError(t, err)
					req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
					require.NoError(t, err)
					require.Equal(t, "allowed_tools", req.ToolChoice.NamedToolChoice.Type)
					require.Equal(t, mode, *req.ToolChoice.ToolChoice)
					require.Len(t, req.ToolChoice.Tools, count)
					require.Equal(t, "a", req.ToolChoice.Tools[0].Name)
					if representation == "structured" {
						req.ProviderExtensions = nil
					}
					if representation == "serialized" {
						encoded, err := json.Marshal(req)
						require.NoError(t, err)
						req = &llm.Request{}
						require.NoError(t, json.Unmarshal(encoded, req))
					}
					out, err := NewOutboundTransformer("https://example.com", "test")
					require.NoError(t, err)
					wire, err := out.TransformRequest(t.Context(), req)
					require.NoError(t, err)
					var body map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(wire.Body, &body))
					expected, err := json.Marshal(input.ToolChoice)
					require.NoError(t, err)
					require.JSONEq(t, string(expected), string(body["tool_choice"]))
				})
			}
		}
	}
}

func TestAllowedToolsRawChoiceDoesNotOverwriteChanges(t *testing.T) {
	for _, change := range []string{"tools", "mode", "unrestricted", "clear"} {
		t.Run(change, func(t *testing.T) {
			raw := []byte(`{"model":"test","input":"hi","tools":[{"type":"function","name":"a"},{"type":"function","name":"b"}],"tool_choice":{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"a"}]}}`)
			req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
			require.NoError(t, err)
			expected := `{"type":"allowed_tools","mode":"auto","tools":[{"type":"function","name":"b"}]}`
			switch change {
			case "tools":
				req.ToolChoice.Tools[0].Name = "b"
			case "mode":
				req.ToolChoice.ToolChoice = lo.ToPtr("required")
				expected = `{"type":"allowed_tools","mode":"required","tools":[{"type":"function","name":"a"}]}`
			case "unrestricted":
				req.ToolChoice = &llm.ToolChoice{ToolChoice: lo.ToPtr("auto")}
				expected = `"auto"`
			case "clear":
				req.ToolChoice = nil
			}
			out, err := NewOutboundTransformer("https://example.com", "test")
			require.NoError(t, err)
			wire, err := out.TransformRequest(t.Context(), req)
			require.NoError(t, err)
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(wire.Body, &body))
			if change == "clear" {
				require.NotContains(t, body, "tool_choice")
			} else {
				require.JSONEq(t, expected, string(body["tool_choice"]))
			}
		})
	}
}
