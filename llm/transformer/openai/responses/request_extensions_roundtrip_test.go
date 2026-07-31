package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/httpclient"
)

func TestRequestExtensions_ReplaysNamespaceWithFutureClientSubtool(t *testing.T) {
	inbound := NewInboundTransformer()
	llmRequest, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(`{
		"model":"gpt-5.5",
		"input":"use tools",
		"tools":[{
			"type":"namespace",
			"name":"workspace",
			"tools":[
				{"type":"function","name":"read","parameters":{"type":"object"}},
				{
					"type":"future_client_tool",
					"name":"later",
					"execution":"client",
					"parameters":{"type":"object"},
					"future_option":{"mode":"lossless"}
				}
			]
		}]
	}`)})
	require.NoError(t, err)
	require.Len(t, llmRequest.Tools, 2)

	outbound, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)
	httpRequest, err := outbound.TransformRequest(context.Background(), llmRequest)
	require.NoError(t, err)

	var payload struct {
		Tools []struct {
			Type  string `json:"type"`
			Name  string `json:"name"`
			Tools []struct {
				Type         string         `json:"type"`
				Name         string         `json:"name"`
				FutureOption map[string]any `json:"future_option"`
			} `json:"tools"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(httpRequest.Body, &payload))
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "namespace", payload.Tools[0].Type)
	require.Equal(t, "workspace", payload.Tools[0].Name)
	require.Len(t, payload.Tools[0].Tools, 2)
	require.Equal(t, "function", payload.Tools[0].Tools[0].Type)
	require.Equal(t, "read", payload.Tools[0].Tools[0].Name)
	require.Equal(t, "future_client_tool", payload.Tools[0].Tools[1].Type)
	require.Equal(t, "later", payload.Tools[0].Tools[1].Name)
	require.Equal(t, "lossless", payload.Tools[0].Tools[1].FutureOption["mode"])
}
