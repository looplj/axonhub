package transformer_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/bailian"
	"github.com/looplj/axonhub/llm/transformer/cerebras"
	"github.com/looplj/axonhub/llm/transformer/cline"
	"github.com/looplj/axonhub/llm/transformer/deepseek"
	"github.com/looplj/axonhub/llm/transformer/doubao"
	"github.com/looplj/axonhub/llm/transformer/fireworks"
	geminioai "github.com/looplj/axonhub/llm/transformer/gemini/openai"
	"github.com/looplj/axonhub/llm/transformer/longcat"
	"github.com/looplj/axonhub/llm/transformer/modelscope"
	"github.com/looplj/axonhub/llm/transformer/moonshot"
	"github.com/looplj/axonhub/llm/transformer/nanogpt"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
	"github.com/looplj/axonhub/llm/transformer/opencode"
	"github.com/looplj/axonhub/llm/transformer/openrouter"
	"github.com/looplj/axonhub/llm/transformer/xai"
	"github.com/looplj/axonhub/llm/transformer/zai"
	"github.com/looplj/axonhub/llm/transformer/zenmux"
)

type namespaceTestTokenProvider struct{}

func (namespaceTestTokenProvider) GetToken(context.Context) (string, error) {
	return "test", nil
}

// Keep custom Chat decoders, inherited implementations, and protocol dispatchers
// under the same request/response contract. No upstream network calls are made.
func TestChatNamespaceRoundTrip(t *testing.T) {
	const input = `{"model":"test","tools":[{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{}}}]}],"input":[{"type":"function_call","call_id":"call_1","name":"search","namespace":"docs","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}],"tool_choice":"auto"}`
	for _, tt := range []struct {
		name    string
		factory func(string, string) (transformer.Outbound, error)
		model   string
		wrapped bool
	}{
		{name: "openai", factory: openai.NewOutboundTransformer},
		{name: "bailian", factory: bailian.NewOutboundTransformer},
		{name: "cerebras", factory: func(baseURL, key string) (transformer.Outbound, error) {
			return cerebras.NewOutboundTransformerWithConfig(&cerebras.Config{BaseURL: baseURL, APIKeyProvider: auth.NewStaticKeyProvider(key)})
		}},
		{name: "cline", factory: cline.NewOutboundTransformer},
		{name: "cline_wrapped", factory: cline.NewOutboundTransformer, wrapped: true},
		{name: "copilot_chat", model: "gpt-4o", factory: func(baseURL, _ string) (transformer.Outbound, error) {
			return copilot.NewOutboundTransformer(copilot.OutboundTransformerParams{BaseURL: baseURL, TokenProvider: namespaceTestTokenProvider{}})
		}},
		{name: "deepseek", factory: deepseek.NewOutboundTransformer},
		{name: "doubao", factory: doubao.NewOutboundTransformer},
		{name: "fireworks", factory: fireworks.NewOutboundTransformer},
		{name: "gemini_chat", factory: geminioai.NewOutboundTransformer},
		{name: "longcat", factory: longcat.NewOutboundTransformer},
		{name: "modelscope", factory: modelscope.NewOutboundTransformer},
		{name: "moonshot", factory: moonshot.NewOutboundTransformer},
		{name: "nanogpt", factory: nanogpt.NewOutboundTransformer},
		{name: "opencode_chat", factory: opencode.NewOutboundTransformer},
		{name: "opencode_deepseek", model: "deepseek-v4-flash", factory: opencode.NewOutboundTransformer},
		{name: "openrouter", factory: openrouter.NewOutboundTransformer},
		{name: "xai", factory: xai.NewOutboundTransformer},
		{name: "zai", factory: zai.NewOutboundTransformer},
		{name: "zenmux_chat", factory: zenmux.NewOutboundTransformer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			inbound := responses.NewInboundTransformer()
			req, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(input)})
			require.NoError(t, err)
			if tt.model != "" {
				req.Model = tt.model
			}
			out, err := tt.factory("https://example.com", "test")
			require.NoError(t, err)
			wire, err := out.TransformRequest(t.Context(), req)
			require.NoError(t, err)
			require.Equal(t, string(llm.APIFormatOpenAIChatCompletion), wire.APIFormat)
			var body openai.Request
			require.NoError(t, json.Unmarshal(wire.Body, &body))
			require.Equal(t, "docs__search", body.Tools[0].Function.Name)
			require.Equal(t, "docs__search", body.Messages[0].ToolCalls[0].Function.Name)
			require.Equal(t, "auto", *body.ToolChoice.ToolChoice)
			require.NotContains(t, string(wire.Body), `"namespace"`)
			require.Equal(t, "docs__search", req.Tools[0].Function.Name)
			require.Equal(t, "docs", req.Tools[0].Function.Namespace)

			wrap := func(data []byte) []byte {
				if !tt.wrapped {
					return data
				}
				wrapped, err := json.Marshal(map[string]any{"success": true, "data": json.RawMessage(data)})
				require.NoError(t, err)
				return wrapped
			}
			t.Run("response", func(t *testing.T) {
				response, err := out.TransformResponse(t.Context(), &httpclient.Response{
					StatusCode: 200,
					Request:    wire,
					Body:       wrap([]byte(`{"choices":[{"index":0,"message":{"tool_calls":[{"id":"call_2","type":"function","function":{"name":"docs__search","arguments":"{}"}}]}}]}`)),
				})
				require.NoError(t, err)
				client, err := inbound.TransformResponse(t.Context(), responseWithTestMetadata(req, response))
				require.NoError(t, err)
				var result responses.Response
				require.NoError(t, json.Unmarshal(client.Body, &result))
				require.Len(t, result.Output, 1)
				require.Equal(t, "search", result.Output[0].Name)
				require.Equal(t, "docs", result.Output[0].Namespace)
			})
			t.Run("stream", func(t *testing.T) {
				// NoNil caches Current so peeking adapters can read an event repeatedly.
				source := streams.NoNil(streams.SliceStream([]*httpclient.StreamEvent{
					{Data: wrap([]byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_2","type":"function","function":{"arguments":""}}]}}]}`))},
					{Data: wrap([]byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":"docs__search","arguments":"{}"}}]}}]}`))},
					{Data: wrap([]byte(`{"id":"resp_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`))},
					{Data: []byte(`[DONE]`)},
				}))
				stream, err := out.TransformStream(t.Context(), wire, source)
				require.NoError(t, err)
				client, err := inbound.TransformStream(t.Context(), streamWithTestMetadata(req, stream))
				require.NoError(t, err)
				defer client.Close()
				items := 0
				completed := false
				for client.Next() {
					var event responses.StreamEvent
					require.NoError(t, json.Unmarshal(client.Current().Data, &event))
					if event.Item != nil && event.Item.Type == "function_call" {
						items++
						require.Equal(t, "search", event.Item.Name)
						require.Equal(t, "docs", event.Item.Namespace)
					}
					if event.Type == "response.completed" {
						completed = true
						require.NotNil(t, event.Response)
						require.Len(t, event.Response.Output, 1)
						require.Equal(t, "search", event.Response.Output[0].Name)
						require.Equal(t, "docs", event.Response.Output[0].Namespace)
						require.JSONEq(t, `{}`, event.Response.Output[0].Arguments)
					}
				}
				require.NoError(t, client.Err())
				require.Equal(t, 2, items)
				require.True(t, completed)
			})
		})
	}
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
