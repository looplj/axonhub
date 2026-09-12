package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

func TestNamespaceNameRules(t *testing.T) {
	for _, tt := range []struct{ namespace, local, flat string }{
		{"", "search", "search"},
		{"", "docs__search", "docs__search"},
		{"docs", "search", "docs__search"},
		{"mcp__docs", "search__v2", "mcp__docs__search__v2"},
		{"docs", "docs__search", "docs__docs__search"},
	} {
		t.Run(tt.flat, func(t *testing.T) {
			input := Request{Model: "test", Input: Input{Items: []Item{
				{Type: "function_call", CallID: "call_1", Name: tt.local, Namespace: tt.namespace, Arguments: "{}"},
				{Type: "function_call_output", CallID: "call_1", Output: &Input{Text: lo.ToPtr("ok")}},
			}}}
			tool := Tool{Type: "function", Name: tt.local}
			if tt.namespace != "" {
				tool = Tool{Type: "namespace", Name: tt.namespace, Tools: []Tool{tool}}
			}
			input.Tools = []Tool{tool}
			raw, err := json.Marshal(input)
			require.NoError(t, err)
			inbound := NewInboundTransformer()
			req, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: raw})
			require.NoError(t, err)
			require.Equal(t, tt.flat, req.Tools[0].Function.Name)
			require.Equal(t, tt.namespace, req.Tools[0].Function.Namespace)
			require.Equal(t, tt.flat, req.Messages[0].ToolCalls[0].Function.Name)
			encoded, err := json.Marshal(req)
			require.NoError(t, err)
			req = &llm.Request{}
			require.NoError(t, json.Unmarshal(encoded, req))
			req.TransformerMetadata = namespaceMetadata(req)
			// A later attempt may modify the request; response identity stays tied to
			// the inbound metadata, including declarations and historical calls.
			req.Tools[0].Function.Namespace = "changed"
			req.Messages[0].ToolCalls[0].Function.Namespace = "changed"
			original := &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{ToolCalls: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: tt.flat, Arguments: "{}"}}}}}}}
			for range 2 {
				result, err := inbound.TransformResponse(t.Context(), responseWithTestMetadata(req, original))
				require.NoError(t, err)
				var body Response
				require.NoError(t, json.Unmarshal(result.Body, &body))
				require.Equal(t, tt.local, body.Output[0].Name)
				require.Equal(t, tt.namespace, body.Output[0].Namespace)
			}
			require.Equal(t, tt.flat, original.Choices[0].Message.ToolCalls[0].Function.Name)
			require.Empty(t, original.Choices[0].Message.ToolCalls[0].Function.Namespace)
		})
	}
}

func TestNamespaceResponseIdentityFallback(t *testing.T) {
	for _, tt := range []struct {
		name, inputName, inputNamespace, wantName, wantNamespace string
	}{
		{"request fallback", "docs__search", "", "search", "docs"},
		{"explicit namespace wins", "docs__search", "other", "docs__search", "other"},
		{"unknown identity unchanged", "unknown__search", "", "unknown__search", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := &llm.Request{Tools: []llm.Tool{{Type: "function", Function: llm.Function{Name: "docs__search", Namespace: "docs"}}}}
			req.TransformerMetadata = namespaceMetadata(req)
			response := &llm.Response{Choices: []llm.Choice{{Message: &llm.Message{ToolCalls: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: tt.inputName, Namespace: tt.inputNamespace, Arguments: "{}"}}}}}}}
			result, err := NewInboundTransformer().TransformResponse(t.Context(), responseWithTestMetadata(req, response))
			require.NoError(t, err)
			var body Response
			require.NoError(t, json.Unmarshal(result.Body, &body))
			require.Equal(t, tt.wantName, body.Output[0].Name)
			require.Equal(t, tt.wantNamespace, body.Output[0].Namespace)
		})
	}
}

func TestNamespaceNativeStreamRepeatedCurrent(t *testing.T) {
	out, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	stream, err := out.TransformStream(t.Context(), nil, streams.SliceStream([]*httpclient.StreamEvent{
		{Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"inner__tool","namespace":"outer","arguments":""}}`)},
		{Data: []byte(`{"type":"response.function_call_arguments.done","item_id":"fc_1","name":"inner__tool","namespace":"outer","arguments":"{}"}`)},
		{Data: []byte(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[]}}`)},
	}))
	require.NoError(t, err)
	defer stream.Close()
	count := 0
	for stream.Next() {
		first := stream.Current()
		require.Same(t, first, stream.Current())
		for _, choice := range first.Choices {
			if choice.Delta == nil {
				continue
			}
			for _, call := range choice.Delta.ToolCalls {
				if call.Function.Name == "" {
					continue
				}
				count++
				require.Equal(t, "outer__inner__tool", call.Function.Name)
				require.Equal(t, "outer", call.Function.Namespace)
			}
		}
	}
	require.NoError(t, stream.Err())
	require.GreaterOrEqual(t, count, 1)
}

func TestNamespaceCompactInputRoundTrip(t *testing.T) {
	raw := []byte(`{"model":"test","input":[{"type":"function_call","call_id":"call_1","name":"docs__search","namespace":"docs","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`)
	req, err := NewCompactInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
	require.NoError(t, err)
	require.Equal(t, "docs__docs__search", req.Compact.Input[0].ToolCalls[0].Function.Name)
	require.Equal(t, "docs", req.Compact.Input[0].ToolCalls[0].Function.Namespace)
	out, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var body CompactAPIRequest
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Equal(t, "docs__search", body.Input.Items[0].Name)
	require.Equal(t, "docs", body.Input.Items[0].Namespace)
}

// A function name containing a namespace prefix remains an ordinary name in
// tool_choice, even when the request also declares a namespace with that prefix.
func TestNamespaceRequestPreservesFunctionChoiceNames(t *testing.T) {
	for _, choice := range []string{
		`{"type":"function","name":"docs__search"}`,
		`{"type":"allowed_tools","mode":"required","tools":[{"type":"function","name":"docs__search"}]}`,
	} {
		t.Run(choice, func(t *testing.T) {
			raw := []byte(`{"model":"test","input":"hi","tools":[{"type":"function","name":"docs__search"},{"type":"namespace","name":"docs","tools":[{"type":"function","name":"read"}]}],"tool_choice":` + choice + `}`)
			req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
			require.NoError(t, err)
			req.ProviderExtensions = nil
			encoded, err := json.Marshal(req)
			require.NoError(t, err)
			req = &llm.Request{}
			require.NoError(t, json.Unmarshal(encoded, req))
			out, err := NewOutboundTransformer("https://example.com", "test")
			require.NoError(t, err)
			wire, err := out.TransformRequest(t.Context(), req)
			require.NoError(t, err)
			var body map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(wire.Body, &body))
			require.JSONEq(t, choice, string(body["tool_choice"]))
		})
	}
}

func TestFlattenRequestFunctionNamesPreservesSource(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		name := "success"
		if conflict {
			name = "conflict"
		}
		t.Run(name, func(t *testing.T) {
			src := &llm.Request{
				Tools:    []llm.Tool{{Type: "function", Function: llm.Function{Name: "search", Namespace: "docs"}}},
				Messages: []llm.Message{{ToolCalls: []llm.ToolCall{{Type: "function", Function: llm.FunctionCall{Name: "read", Namespace: "docs"}}}}},
			}
			if conflict {
				src.Tools = append(src.Tools, llm.Tool{Type: "function", Function: llm.Function{Name: "docs__search"}})
			}
			before, err := json.Marshal(src)
			require.NoError(t, err)
			result, err := flattenRequestFunctionNames(src)
			if conflict {
				require.ErrorIs(t, err, transformer.ErrInvalidRequest)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotSame(t, src, result)
				require.Equal(t, "docs__search", result.Tools[0].Function.Name)
				require.Equal(t, "docs__read", result.Messages[0].ToolCalls[0].Function.Name)
			}
			after, err := json.Marshal(src)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after))
		})
	}
}

func TestNamespaceHistoryKeepsLocalNamesUntilRequestConversion(t *testing.T) {
	for _, scenario := range []string{"function_call", "reasoning_then_function_call"} {
		t.Run(scenario, func(t *testing.T) {
			input := Input{Items: []Item{{Type: "function_call", CallID: "call_1", Name: "docs__search", Namespace: "docs", Arguments: "{}"}}}
			if scenario == "reasoning_then_function_call" {
				input.Items = append([]Item{{Type: "reasoning", ID: "rs_1"}}, input.Items...)
			}
			messages, err := convertInputToMessages(&input)
			require.NoError(t, err)
			require.Equal(t, "docs__search", messages[0].ToolCalls[0].Function.Name)
			require.Equal(t, "docs", messages[0].ToolCalls[0].Function.Namespace)

			request, err := convertToLLMRequest(&Request{Model: "test", Input: input})
			require.NoError(t, err)
			require.Equal(t, "docs__docs__search", request.Messages[0].ToolCalls[0].Function.Name)
			require.Equal(t, "docs__search", messages[0].ToolCalls[0].Function.Name)
		})
	}
}

func TestNamespaceCompactOutputEncoding(t *testing.T) {
	outbound, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	response, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		Request: &httpclient.Request{RequestType: string(llm.RequestTypeCompact)},
		Body:    []byte(`{"id":"cmp_1","model":"test","output":[{"type":"reasoning","id":"rs_1"},{"type":"function_call","call_id":"call_1","name":"docs__search","namespace":"docs","arguments":"{}"}]}`),
	})
	require.NoError(t, err)
	require.Equal(t, "docs__docs__search", response.Compact.Output[0].ToolCalls[0].Function.Name)
	require.Equal(t, "docs", response.Compact.Output[0].ToolCalls[0].Function.Namespace)
	input := convertInputFromMessages(response.Compact.Output, llm.TransformOptions{ArrayInputs: lo.ToPtr(true)})
	calls := lo.Filter(input.Items, func(item Item, _ int) bool { return item.Type == "function_call" })
	require.Len(t, calls, 1)
	require.Equal(t, "docs__search", calls[0].Name)
	require.Equal(t, "docs", calls[0].Namespace)
}
