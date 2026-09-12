package transformer_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/oauth"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/anthropic/claudecode"
	"github.com/looplj/axonhub/llm/transformer/gemini"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestNamespaceProviderMetadataRoundTrip(t *testing.T) {
	for _, provider := range []string{"anthropic", "gemini", "claudecode"} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", provider, streaming), func(t *testing.T) {
				var outbound transformer.Outbound
				var err error
				switch provider {
				case "anthropic":
					outbound, err = anthropic.NewOutboundTransformer("https://example.com", "test")
				case "gemini":
					outbound, err = gemini.NewOutboundTransformer("https://example.com", "test")
				case "claudecode":
					outbound, err = claudecode.NewOutboundTransformer(claudecode.Params{BaseURL: "https://example.com", IsOfficial: true, TokenProvider: oauth.NewStaticTokenProvider(&oauth.OAuthCredentials{AccessToken: "test"})})
				}
				require.NoError(t, err)
				inbound := responses.NewInboundTransformer()
				req, err := inbound.TransformRequest(t.Context(), &httpclient.Request{Body: []byte(`{"model":"test","input":"hi","tools":[{"type":"namespace","name":"docs","tools":[{"type":"function","name":"search","parameters":{"type":"object","properties":{}}}]}]}`)})
				require.NoError(t, err)
				req.Stream = lo.ToPtr(streaming)
				wire, err := outbound.TransformRequest(t.Context(), req)
				require.NoError(t, err)
				name := "docs__search"
				if provider == "claudecode" {
					name = "proxy_" + name
				}
				require.Contains(t, string(wire.Body), name)
				require.NotContains(t, string(wire.Body), "namespace_mapping")
				raw := []byte(fmt.Sprintf(`{"id":"msg_1","type":"message","role":"assistant","model":"test","content":[{"type":"tool_use","id":"call_1","name":%q,"input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`, name))
				events := []*httpclient.StreamEvent{
					{Type: "message_start", Data: []byte(`{"type":"message_start","message":{"id":"msg_1","role":"assistant","model":"test","content":[]}}`)},
					{Type: "content_block_start", Data: []byte(fmt.Sprintf(`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call_1","name":%q,"input":{}}}`, name))},
					{Type: "content_block_delta", Data: []byte(`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{}"}}`)},
					{Type: "message_delta", Data: []byte(`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":1}}`)},
					{Type: "message_stop", Data: []byte(`{"type":"message_stop"}`)},
				}
				if provider == "gemini" {
					raw = []byte(`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"docs__search","args":{}}}]},"finishReason":"STOP"}]}`)
					events = []*httpclient.StreamEvent{{Data: raw}}
				}
				var response responses.Response
				if streaming {
					decoded, err := outbound.TransformStream(t.Context(), wire, streams.SliceStream(events))
					require.NoError(t, err)
					client, err := inbound.TransformStream(t.Context(), streamWithTestMetadata(req, decoded))
					require.NoError(t, err)
					defer client.Close()
					for client.Next() {
						var event responses.StreamEvent
						require.NoError(t, json.Unmarshal(client.Current().Data, &event))
						if event.Type == "response.completed" {
							require.NotNil(t, event.Response)
							response = *event.Response
						}
					}
					require.NoError(t, client.Err())
				} else {
					decoded, err := outbound.TransformResponse(t.Context(), &httpclient.Response{Request: wire, Body: raw})
					require.NoError(t, err)
					client, err := inbound.TransformResponse(t.Context(), responseWithTestMetadata(req, decoded))
					require.NoError(t, err)
					require.NoError(t, json.Unmarshal(client.Body, &response))
				}
				var calls []responses.Item
				for _, item := range response.Output {
					if item.Type == "function_call" {
						calls = append(calls, item)
					}
				}
				require.Len(t, calls, 1)
				require.Equal(t, "search", calls[0].Name)
				require.Equal(t, "docs", calls[0].Namespace)
			})
		}
	}
}
