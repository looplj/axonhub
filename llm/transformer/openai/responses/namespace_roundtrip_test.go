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
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/ollama"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

const namespaceReviewRequest = `{"model":"test","tools":[{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{}}}]}],"input":[{"type":"function_call","call_id":"call_1","name":"search","namespace":"docs","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}],"tool_choice":"auto"}`

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

func TestNamespaceReview_PreservesMultiFunctionGroup(t *testing.T) {
	var body Request
	require.NoError(t, json.Unmarshal([]byte(namespaceReviewRequest), &body))
	body.Tools[0].Tools = append(body.Tools[0].Tools, Tool{Type: "function", Name: "read"})
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: raw})
	require.NoError(t, err)
	native, err := NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	wire, err := native.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var result Request
	require.NoError(t, json.Unmarshal(wire.Body, &result))
	require.Equal(t, body.ToolChoice, result.ToolChoice)
	require.Len(t, result.Tools[0].Tools, 2)
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
	events, err := NewInboundTransformer().TransformStream(t.Context(), streamWithTestMetadata(req, source))
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
	require.Equal(t, "auto", *body.ToolChoice.Mode)
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
	req.Tools[1].Function.Name = "docs__read"
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
			req.ToolChoice = &llm.ToolChoice{NamedToolChoice: &llm.NamedToolChoice{Type: "function", Function: llm.ToolFunction{Name: "docs__search"}}}
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
			require.Equal(t, "docs__search", req.Tools[0].Function.Name)
			require.Equal(t, "docs", req.Tools[0].Function.Namespace)
		})
	}
}

func TestNamespaceReview_OllamaOutboundNames(t *testing.T) {
	data := []byte(`{
		"model":"test",
		"tools":[
			{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search"}]},
			{"type":"namespace","name":"code","tools":[{"type":"function","name":"search"}]},
			{"type":"function","name":"plain__function"}
		],
		"input":[
			{"type":"function_call","call_id":"call_1","name":"search","namespace":"docs","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_1","output":"ok"},
			{"type":"function_call","call_id":"call_2","name":"query","namespace":"retired","arguments":"{}"},
			{"type":"function_call_output","call_id":"call_2","output":"ok"}
		]
	}`)
	req, err := NewInboundTransformer().TransformRequest(t.Context(), &httpclient.Request{Body: data})
	require.NoError(t, err)
	before, err := json.Marshal(req)
	require.NoError(t, err)
	out, err := ollama.NewOutboundTransformerWithConfig(&ollama.Config{BaseURL: "https://example.com"})
	require.NoError(t, err)
	wire, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	var body ollama.ChatRequest
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Len(t, body.Tools, 3)
	require.Equal(t, "docs__search", body.Tools[0].Function.Name)
	require.Equal(t, "code__search", body.Tools[1].Function.Name)
	require.Equal(t, "plain__function", body.Tools[2].Function.Name)
	var historicalNames []string
	for _, message := range body.Messages {
		for _, call := range message.ToolCalls {
			historicalNames = append(historicalNames, call.Function.Name)
		}
	}
	require.Equal(t, []string{"docs__search", "retired__query"}, historicalNames)

	for _, streaming := range []bool{false, true} {
		name := "response"
		if streaming {
			name = "stream"
		}
		t.Run(name, func(t *testing.T) {
			attempt := *req
			attempt.Stream = lo.ToPtr(streaming)
			wire, err := out.TransformRequest(t.Context(), &attempt)
			require.NoError(t, err)
			raw := []byte(`{"model":"test","message":{"role":"assistant","tool_calls":[
				{"function":{"name":"docs__search","arguments":{}}},
				{"function":{"name":"code__search","arguments":{}}},
				{"function":{"name":"plain__function","arguments":{}}},
				{"function":{"name":"retired__query","arguments":{}}}
			]},"done":true}`)
			inbound := NewInboundTransformer()
			var result Response
			if streaming {
				stream, err := out.TransformStream(t.Context(), wire, streams.SliceStream([]*httpclient.StreamEvent{{Data: raw}}))
				require.NoError(t, err)
				client, err := inbound.TransformStream(t.Context(), streamWithTestMetadata(req, stream))
				require.NoError(t, err)
				defer client.Close()
				for client.Next() {
					var event StreamEvent
					require.NoError(t, json.Unmarshal(client.Current().Data, &event))
					if event.Type == "response.completed" {
						require.NotNil(t, event.Response)
						result = *event.Response
					}
				}
				require.NoError(t, client.Err())
			} else {
				response, err := out.TransformResponse(t.Context(), &httpclient.Response{StatusCode: 200, Request: wire, Body: raw})
				require.NoError(t, err)
				client, err := inbound.TransformResponse(t.Context(), responseWithTestMetadata(req, response))
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(client.Body, &result))
			}
			want := []llm.FunctionCall{
				{Name: "search", Namespace: "docs"},
				{Name: "search", Namespace: "code"},
				{Name: "plain__function"},
				{Name: "query", Namespace: "retired"},
			}
			require.Len(t, result.Output, len(want))
			var history []any
			for i, item := range result.Output {
				require.Equal(t, "function_call", item.Type)
				require.Equal(t, want[i], llm.FunctionCall{Name: item.Name, Namespace: item.Namespace})
				require.JSONEq(t, `{}`, item.Arguments)
				history = append(history, item, map[string]any{
					"type": "function_call_output", "call_id": item.CallID, "output": "ok",
				})
			}
			// Replay exactly the identities returned to the client in a new request.
			var original map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &original))
			replay, err := json.Marshal(map[string]any{"model": "test", "tools": original["tools"], "input": history})
			require.NoError(t, err)
			next, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: replay})
			require.NoError(t, err)
			_, err = out.TransformRequest(t.Context(), next)
			require.NoError(t, err)
		})
	}

	after, err := json.Marshal(req)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}

// These tests compose converters directly; production pipeline supplies metadata.
func responseWithTestMetadata(req *llm.Request, src *llm.Response) *llm.Response {
	if src == nil || src == llm.DoneResponse {
		return src
	}
	result := *src
	result.TransformerMetadata = req.TransformerMetadata
	return &result
}

func streamWithTestMetadata(req *llm.Request, source streams.Stream[*llm.Response]) streams.Stream[*llm.Response] {
	return streams.MapErr(source, func(resp *llm.Response) (*llm.Response, error) {
		return responseWithTestMetadata(req, resp), nil
	})
}
