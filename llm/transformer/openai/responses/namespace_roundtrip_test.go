package responses

import (
	"encoding/json"
	"testing"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	geminioai "github.com/looplj/axonhub/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/llm/transformer/moonshot"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/looplj/axonhub/llm/transformer/zai"
	"github.com/stretchr/testify/require"
)

const namespaceReviewRequest = `{"model":"test","tools":[{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{}}}]}],"input":[{"type":"function_call","call_id":"call_1","name":"search","namespace":"docs","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}],"tool_choice":{"type":"namespace","name":"docs"}}`

func TestNamespaceReview_ResponsesHistory(t *testing.T) {
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(namespaceReviewRequest)})
	require.NoError(t, err)
	out, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var body Request
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Equal(t, "search", body.Input.Items[0].Name)
	require.Equal(t, "docs", body.Input.Items[0].Namespace)
}

func TestNamespaceReview_ResponsesChoice(t *testing.T) {
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(namespaceReviewRequest)})
	require.NoError(t, err)
	out, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var body Request
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Equal(t, "namespace", *body.ToolChoice.Type)
	require.Equal(t, "docs", *body.ToolChoice.Name)
}

func TestNamespaceReview_ChatRejectsAmbiguousChoice(t *testing.T) {
	var body Request
	require.NoError(t, json.Unmarshal([]byte(namespaceReviewRequest), &body))
	body.Tools[0].Tools = append(body.Tools[0].Tools, Tool{Type: "function", Name: "read"})
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
	require.NoError(t, err)
	native, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	_, err = native.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	out, err := openai.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	_, err = out.TransformRequest(t.Context(), req)
	require.ErrorIs(t, err, transformer.ErrInvalidRequest)
}

func TestNamespaceReview_LateStreamName(t *testing.T) {
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(namespaceReviewRequest)})
	require.NoError(t, err)
	chat, err := openai.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := chat.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	source, err := chat.TransformStream(t.Context(), wire, streams.SliceStream([]*httpclient.StreamEvent{
		{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"arguments":"{"}}]}}]}`)},
		{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"docs__search","arguments":"}"}}]}}]}`)},
		{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)},
		{Data: []byte(`[DONE]`)},
	}))
	require.NoError(t, err)
	events, err := NewInboundTransformer().TransformStream(t.Context(), source)
	require.NoError(t, err)
	defer events.Close()
	count := 0
	for events.Next() {
		var event StreamEvent
		require.NoError(t, json.Unmarshal(events.Current().Data, &event))
		if event.Item != nil && event.Item.Type == "function_call" {
			count++
			require.Equal(t, "search", event.Item.Name)
			require.Equal(t, "docs", event.Item.Namespace)
		}
		if event.Response != nil && event.Type == "response.completed" {
			require.Equal(t, "docs", event.Response.Output[0].Namespace)
			require.JSONEq(t, `{}`, event.Response.Output[0].Arguments)
		}
	}
	require.NoError(t, events.Err())
	require.Equal(t, 2, count)
}

func TestNamespaceReview_ChoiceProtocolBoundary(t *testing.T) {
	for _, choice := range []string{
		`{"type":"namespace","name":"docs"}`,
		`{"type":"function","name":"search","namespace":"docs"}`,
		`{"tools":[{"type":"function","name":"search","namespace":"docs"}]}`,
		`{"tools":[{"type":"namespace","name":"docs"}]}`,
		`{"tools":[{"type":"function","name":"search","namespace":"docs"},{"type":"function","name":"read","namespace":"docs"}]}`,
	} {
		t.Run(choice, func(t *testing.T) {
			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(namespaceReviewRequest), &raw))
			raw["tool_choice"] = json.RawMessage(choice)
			data, err := json.Marshal(raw)
			require.NoError(t, err)
			req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: data})
			require.NoError(t, err)
			// This must also work from the standard model without raw replay metadata.
			for _, serialized := range []bool{false, true} {
				if serialized {
					data, err = json.Marshal(req)
					require.NoError(t, err)
					req = &llm.Request{}
					require.NoError(t, json.Unmarshal(data, req))
				}
				native, err := NewOutboundTransformer("https://example.com", "test")
				require.NoError(t, err)
				wire, err := native.TransformRequest(t.Context(), req)
				require.NoError(t, err)
				var body map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(wire.Body, &body))
				require.JSONEq(t, choice, string(body["tool_choice"]))
				var tools []Tool
				require.NoError(t, json.Unmarshal(body["tools"], &tools))
				require.Len(t, tools, 1)
				require.Equal(t, "namespace", tools[0].Type)
				require.Equal(t, "docs", tools[0].Name)
				require.Equal(t, "search", tools[0].Tools[0].Name)
			}
		})
	}
}

func TestNamespaceReview_ChatThenResponses(t *testing.T) {
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(namespaceReviewRequest)})
	require.NoError(t, err)
	chat, err := openai.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	_, err = chat.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	native, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := native.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var body Request
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Equal(t, "search", body.Input.Items[0].Name)
	require.Equal(t, "docs", body.Input.Items[0].Namespace)
	require.Equal(t, "namespace", *body.ToolChoice.Type)
	require.Equal(t, "docs", *body.ToolChoice.Name)
	require.Equal(t, "search", body.Tools[0].Tools[0].Name)
}

func TestNamespaceReview_RawNamespaceAndCatalogChange(t *testing.T) {
	data := []byte(`{"model":"test","input":"hi","tools":[{"type":"function","name":"plain"},{"type":"namespace","name":"docs","description":"Keep this description","tools":[{"type":"function","name":"search","defer_loading":true},{"type":"custom","name":"shell","format":{"type":"text"}}]},{"type":"namespace","name":"other","tools":[{"type":"function","name":"search"}]}]}`)
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: data})
	require.NoError(t, err)
	native, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := native.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var original, body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &original))
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.JSONEq(t, string(original["tools"]), string(body["tools"]))
	// Catalog changes must produce a fresh namespace definition, not stale raw tools.
	req.Tools[1].Function.Name = "read"
	wire, err = native.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var updated Request
	require.NoError(t, json.Unmarshal(wire.Body, &updated))
	require.Equal(t, "read", updated.Tools[1].Tools[0].Name)
	require.Len(t, updated.Tools[1].Tools, 1)
}

func TestNamespaceReview_LegacyOutboundNames(t *testing.T) {
	for _, tt := range []struct {
		name    string
		factory func(string, string) (transformer.Outbound, error)
	}{
		{"anthropic", anthropic.NewOutboundTransformer},
		{"gemini", gemini.NewOutboundTransformer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(namespaceReviewRequest)})
			require.NoError(t, err)
			out, err := tt.factory("https://example.com", "test")
			require.NoError(t, err)
			wire, err := out.TransformRequest(t.Context(), req)
			require.NoError(t, err)
			if tt.name == "anthropic" {
				var body anthropic.MessageRequest
				require.NoError(t, json.Unmarshal(wire.Body, &body))
				require.Equal(t, "docs__search", body.Tools[0].Name)
				require.Equal(t, "docs__search", *body.ToolChoice.Name)
			} else {
				var body gemini.GenerateContentRequest
				require.NoError(t, json.Unmarshal(wire.Body, &body))
				require.Equal(t, "docs__search", body.Tools[0].FunctionDeclarations[0].Name)
				require.Equal(t, []string{"docs__search"}, body.ToolConfig.FunctionCallingConfig.AllowedFunctionNames)
			}
			require.NotContains(t, string(wire.Body), `"name":"search"`)
			require.Equal(t, "search", req.Tools[0].Function.Name)
			require.Equal(t, "docs", req.Tools[0].Function.Namespace)
		})
	}
}

func TestNamespaceReview_ChatAdapters(t *testing.T) {
	for _, tt := range []struct {
		name    string
		factory func(string, string) (transformer.Outbound, error)
	}{
		{"openai", openai.NewOutboundTransformer},
		{"deepseek", deepseek.NewOutboundTransformer},
		{"doubao", doubao.NewOutboundTransformer},
		{"moonshot", moonshot.NewOutboundTransformer},
		{"openrouter", openrouter.NewOutboundTransformer},
		{"zai", zai.NewOutboundTransformer},
		{"gemini_chat", geminioai.NewOutboundTransformer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: []byte(namespaceReviewRequest)})
			require.NoError(t, err)
			out, err := tt.factory("https://example.com", "test")
			require.NoError(t, err)
			wire, err := out.TransformRequest(t.Context(), req)
			require.NoError(t, err)
			var body openai.Request
			require.NoError(t, json.Unmarshal(wire.Body, &body))
			require.Equal(t, "docs__search", body.Tools[0].Function.Name)
			require.Equal(t, "docs__search", body.Messages[0].ToolCalls[0].Function.Name)
			if tt.name == "zai" {
				// Preserve the adapter's existing provider-specific auto choice.
				require.Equal(t, "auto", *body.ToolChoice.ToolChoice)
			} else {
				require.Equal(t, "docs__search", body.ToolChoice.NamedToolChoice.Function.Name)
			}
			require.NotContains(t, string(wire.Body), `"namespace"`)
			response, err := out.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Request: wire, Body: []byte(`{"choices":[{"index":0,"message":{"tool_calls":[{"id":"call_2","type":"function","function":{"name":"docs__search","arguments":"{}"}}]}}]}`)})
			require.NoError(t, err)
			require.Equal(t, "search", response.Choices[0].Message.ToolCalls[0].Function.Name)
			require.Equal(t, "docs", response.Choices[0].Message.ToolCalls[0].Function.Namespace)
			clientResponse := convertToResponsesAPIResponse(response)
			require.Equal(t, "search", clientResponse.Output[0].Name)
			require.Equal(t, "docs", clientResponse.Output[0].Namespace)
			stream, err := out.TransformStream(t.Context(), wire, streams.SliceStream([]*httpclient.StreamEvent{
				{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_2","type":"function","function":{"arguments":""}}]}}]}`)},
				{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"docs__search","arguments":"{}"}}]}}]}`)},
				{Data: []byte(`[DONE]`)},
			}))
			require.NoError(t, err)
			defer stream.Close()
			found := false
			for stream.Next() {
				chunk := stream.Current()
				for _, choice := range chunk.Choices {
					if choice.Delta == nil {
						continue
					}
					for _, call := range choice.Delta.ToolCalls {
						if call.Function.Name != "" {
							found = true
							require.Equal(t, "search", call.Function.Name)
							require.Equal(t, "docs", call.Function.Namespace)
						}
					}
				}
			}
			require.NoError(t, stream.Err())
			require.True(t, found)
			req.Tools = append(req.Tools, llm.Tool{Type: "function", Function: llm.Function{Name: "read", Namespace: "docs"}})
			_, err = out.TransformRequest(t.Context(), req)
			require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		})
	}
}
