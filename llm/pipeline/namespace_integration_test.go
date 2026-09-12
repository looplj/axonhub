package pipeline_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/ollama"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type namespaceExecutor struct {
	do     func(*httpclient.Request) (*httpclient.Response, error)
	stream func(*httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error)
}

func (e namespaceExecutor) Do(_ context.Context, r *httpclient.Request) (*httpclient.Response, error) {
	return e.do(r)
}

func (e namespaceExecutor) DoStream(_ context.Context, r *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	return e.stream(r)
}

type namespaceSwitch struct {
	transformer.Outbound

	next transformer.Outbound
}

func (s *namespaceSwitch) HasMoreChannels() bool { return s.next != nil }
func (s *namespaceSwitch) NextChannel(context.Context) error {
	s.Outbound, s.next = s.next, nil
	return nil
}

type namespaceForcedStream struct{ transformer.Outbound }

func (s namespaceForcedStream) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	req.Stream = lo.ToPtr(true)
	return s.Outbound.TransformRequest(ctx, req)
}

func TestNamespacePipeline(t *testing.T) {
	t.Parallel()

	// This inbound is intentionally shared by concurrently executing pipelines.
	inbound := responses.NewInboundTransformer()
	for _, provider := range []string{"chat", "ollama", "responses", "responses_flat_name", "chat_then_responses", "responses_then_chat", "chat_auto_stream"} {
		for _, streaming := range []bool{false, true} {
			for _, local := range []string{"search", "mcp__docs__search"} {
				t.Run(fmt.Sprintf("%s/stream=%v/%s", provider, streaming, local), func(t *testing.T) {
					t.Parallel()
					namespace := "mcp__docs"
					flat := namespace + "__" + local
					upstreamName, upstreamNamespace := local, namespace
					if provider == "responses_flat_name" {
						upstreamName, upstreamNamespace = flat, ""
					}
					chat, err := openai.NewOutboundTransformer("https://example.com", "test")
					require.NoError(t, err)
					native, err := responses.NewOutboundTransformer("https://example.com", "test")
					require.NoError(t, err)
					var out transformer.Outbound = chat
					switch provider {
					case "chat_auto_stream":
						out = namespaceForcedStream{Outbound: chat}
					case "ollama":
						out, err = ollama.NewOutboundTransformerWithConfig(&ollama.Config{BaseURL: "https://example.com"})
						require.NoError(t, err)
					case "responses", "responses_flat_name":
						out = native
					case "chat_then_responses":
						out = &namespaceSwitch{Outbound: chat, next: native}
					case "responses_then_chat":
						out = &namespaceSwitch{Outbound: native, next: chat}
					}
					attempts := 0
					prepare := func(r *httpclient.Request) ([]byte, error) {
						attempts++
						var body map[string]json.RawMessage
						require.NoError(t, json.Unmarshal(r.Body, &body))
						wantName := flat
						if r.APIFormat == string(llm.APIFormatOpenAIResponse) {
							wantName = local
							var tools []responses.Tool
							require.NoError(t, json.Unmarshal(body["tools"], &tools))
							require.Equal(t, namespace, tools[0].Name)
							require.Equal(t, wantName, tools[0].Tools[0].Name)
						} else {
							var tools []struct{ Function struct{ Name string } }
							require.NoError(t, json.Unmarshal(body["tools"], &tools))
							require.Equal(t, wantName, tools[0].Function.Name)
							require.NotEmpty(t, r.TransformerMetadata)
						}
						if (provider == "chat_then_responses" || provider == "responses_then_chat") && attempts == 1 {
							return nil, &httpclient.Error{StatusCode: 503, Body: []byte(`{"error":{"message":"retry"}}`)}
						}
						if r.APIFormat == string(llm.APIFormatOpenAIResponse) {
							raw := []byte(fmt.Sprintf(`{"id":"resp_1","object":"response","status":"completed","model":"test","output":[{"type":"function_call","call_id":"call_1","name":%q,"namespace":%q,"arguments":"{}"}]}`, upstreamName, upstreamNamespace))
							return bytes.ReplaceAll(raw, []byte(`,"namespace":""`), nil), nil
						}
						if r.APIFormat == string(llm.APIFormatOllamaChat) {
							return []byte(fmt.Sprintf(`{"model":"test","message":{"role":"assistant","tool_calls":[{"function":{"name":%q,"arguments":{}}}]},"done":true}`, flat)), nil
						}
						return []byte(fmt.Sprintf(`{"id":"resp_1","model":"test","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"type":"function","id":"call_1","function":{"name":%q,"arguments":"{}"}}]},"finish_reason":"tool_calls"}]}`, flat)), nil
					}
					executor := namespaceExecutor{
						do: func(r *httpclient.Request) (*httpclient.Response, error) {
							raw, err := prepare(r)
							if err != nil {
								return nil, err
							}
							return &httpclient.Response{StatusCode: 200, Request: r, Body: raw}, nil
						},
						stream: func(r *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
							raw, err := prepare(r)
							if err != nil {
								return nil, err
							}
							var events []*httpclient.StreamEvent
							switch r.APIFormat {
							case string(llm.APIFormatOpenAIResponse):
								events = []*httpclient.StreamEvent{
									{Data: []byte(fmt.Sprintf(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":%q,"namespace":%q,"arguments":""}}`, upstreamName, upstreamNamespace))},
									{Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{}"}`)},
									{Data: []byte(fmt.Sprintf(`{"type":"response.completed","response":%s}`, raw))},
								}
							case string(llm.APIFormatOllamaChat):
								events = []*httpclient.StreamEvent{{Data: raw}}
							default:
								events = []*httpclient.StreamEvent{
									{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"arguments":""}}]}}]}`)},
									{Data: []byte(fmt.Sprintf(`{"id":"resp_1","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"name":%q,"arguments":"{}"}}]}}]}`, flat))},
									{Data: []byte(`{"id":"resp_1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)},
									{Data: []byte(`[DONE]`)},
								}
							}
							if provider == "responses_flat_name" {
								for _, event := range events {
									event.Data = bytes.ReplaceAll(event.Data, []byte(`,"namespace":""`), nil)
								}
							}
							return streams.NoNil(streams.SliceStream(events)), nil
						},
					}
					// JSON roundtrip removes private raw fragments. Name and Namespace alone
					// must carry enough identity for both directions and retries.
					roundtrip := pipeline.OnLlmRequest("namespace-json-roundtrip", func(_ context.Context, r *llm.Request) (*llm.Request, error) {
						require.Equal(t, flat, r.Tools[0].Function.Name)
						raw, err := json.Marshal(r)
						if err != nil {
							return nil, err
						}
						var result llm.Request
						err = json.Unmarshal(raw, &result)
						return &result, err
					})
					pipe := pipeline.NewFactory(executor).Pipeline(inbound, out, pipeline.WithRetry(1, 0, 0), pipeline.WithMiddlewares(roundtrip))
					input := responses.Request{Model: "test", Stream: lo.ToPtr(streaming), Input: responses.Input{Text: lo.ToPtr("hi")}, Tools: []responses.Tool{{Type: "namespace", Name: namespace, Tools: []responses.Tool{{Type: "function", Name: local}}}}}
					body, err := json.Marshal(input)
					require.NoError(t, err)
					result, err := pipe.Process(t.Context(), &httpclient.Request{Body: body})
					require.NoError(t, err)
					var response responses.Response
					if streaming {
						defer result.EventStream.Close()
						for result.EventStream.Next() {
							var event responses.StreamEvent
							require.NoError(t, json.Unmarshal(result.EventStream.Current().Data, &event))
							if event.Item != nil && event.Item.Type == "function_call" {
								require.Equal(t, local, event.Item.Name)
								require.Equal(t, namespace, event.Item.Namespace)
							}
							if event.Type == "response.completed" {
								require.NotNil(t, event.Response)
								response = *event.Response
							}
						}
						require.NoError(t, result.EventStream.Err())
					} else {
						require.NoError(t, json.Unmarshal(result.Response.Body, &response))
					}
					require.Len(t, response.Output, 1)
					require.Equal(t, local, response.Output[0].Name)
					require.Equal(t, namespace, response.Output[0].Namespace)
					require.JSONEq(t, `{}`, response.Output[0].Arguments)
					if provider == "chat_then_responses" || provider == "responses_then_chat" {
						require.Equal(t, 2, attempts)
					} else {
						require.Equal(t, 1, attempts)
					}
				})
			}
		}
	}
}

func TestNamespaceInvalidInputStopsBeforeOutbound(t *testing.T) {
	outbound, err := openai.NewOutboundTransformer("https://example.com", "test")
	require.NoError(t, err)
	executor := namespaceExecutor{do: func(*httpclient.Request) (*httpclient.Response, error) {
		t.Fatal("invalid input reached executor")
		return nil, nil
	}}
	pipe := pipeline.NewFactory(executor).Pipeline(responses.NewInboundTransformer(), outbound)
	for _, raw := range []string{
		`{"model":"test","input":"hi","tools":[{"type":"function","name":"docs__search"},{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search"}]}]}`,
		`{"model":"test","input":"hi","tools":[{"type":"function","name":"search"},{"type":"function","name":"search"}]}`,
	} {
		result, err := pipe.Process(t.Context(), &httpclient.Request{Body: []byte(raw)})
		require.ErrorIs(t, err, transformer.ErrInvalidRequest)
		require.Nil(t, result)
	}
}
