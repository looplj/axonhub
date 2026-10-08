package responses

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

// additionalToolsLiteRequest is the shape Codex CLI sends for a model it marks as
// Responses Lite: the tool definitions ride inside a `developer` input item
// instead of the top-level `tools` array.
const additionalToolsLiteRequest = `{
	"model": "gpt-6-luna",
	"tools": [{"type":"function","name":"top_level","parameters":{"type":"object"}}],
	"input": [
		{
			"type": "additional_tools",
			"id": "at_1",
			"role": "developer",
			"tools": [
				{
					"type": "namespace",
					"name": "functions",
					"tools": [
						{"type": "custom", "name": "exec", "description": "run a script", "x_nested": {"enabled": true}},
						{"type": "function", "name": "shell", "parameters": {"type": "object", "properties": {}}}
					]
				}
			]
		},
		{"type": "message", "role": "user", "content": "Hello"}
	]
}`

func additionalToolsOutbound(t *testing.T) *OutboundTransformer {
	t.Helper()

	out, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://example.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test"),
	})
	require.NoError(t, err)

	return out
}

// additionalToolsInput runs the lite request through the inbound and outbound
// transformers and returns the replayed `input` array of the outgoing body.
func additionalToolsInput(t *testing.T, out *OutboundTransformer) []json.RawMessage {
	t.Helper()

	req, err := NewInboundTransformer().TransformRequest(
		t.Context(), &httpclient.Request{Body: []byte(additionalToolsLiteRequest)})
	require.NoError(t, err)

	wire, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)

	var body struct {
		Input []json.RawMessage `json:"input"`
	}
	require.NoError(t, json.Unmarshal(wire.Body, &body))

	return body.Input
}

func TestAdditionalTools_ReplayedForEveryResponsesUpstream(t *testing.T) {
	input := additionalToolsInput(t, additionalToolsOutbound(t))

	require.Len(t, input, 2)
	require.Contains(t, string(input[0]), `"additional_tools"`)
	require.Contains(t, string(input[0]), `"exec"`)
	require.Contains(t, string(input[0]), `"shell"`)
	require.Contains(t, string(input[0]), `"x_nested":{"enabled":true}`)
	require.Contains(t, string(input[1]), "Hello")
}

func TestAdditionalTools_RepeatedTransformDoesNotDuplicateItems(t *testing.T) {
	req, err := NewInboundTransformer().TransformRequest(
		t.Context(), &httpclient.Request{Body: []byte(additionalToolsLiteRequest)})
	require.NoError(t, err)
	out := additionalToolsOutbound(t)

	first, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)
	second, err := out.TransformRequest(t.Context(), req)
	require.NoError(t, err)

	var firstBody, secondBody struct {
		Input []json.RawMessage `json:"input"`
	}
	require.NoError(t, json.Unmarshal(first.Body, &firstBody))
	require.NoError(t, json.Unmarshal(second.Body, &secondBody))
	require.Len(t, firstBody.Input, 2)
	require.Len(t, secondBody.Input, 2)
	require.Equal(t, firstBody.Input, secondBody.Input)
}

func TestAdditionalTools_ReplayedForResponsesUpstream(t *testing.T) {
	input := additionalToolsInput(t, additionalToolsOutbound(t))

	require.Len(t, input, 2)
	require.Contains(t, string(input[0]), `"additional_tools"`)
	require.Contains(t, string(input[0]), `"exec"`)
	require.Contains(t, string(input[0]), `"shell"`)
	require.Contains(t, string(input[1]), "Hello")
}

const additionalToolsMixedRequest = `{
	"model": "gpt-6-luna",
	"input": [
		{"type": "additional_tools", "id": "at_1", "role": "developer", "tools": []},
		{"type": "message", "role": "user", "content": "Hello"},
		{"type": "web_search_call", "id": "ws_1", "status": "completed"}
	]
}`

func additionalToolsMixedInput(t *testing.T) []json.RawMessage {
	t.Helper()

	req, err := NewInboundTransformer().TransformRequest(
		t.Context(), &httpclient.Request{Body: []byte(additionalToolsMixedRequest)})
	require.NoError(t, err)

	wire, err := additionalToolsOutbound(t).TransformRequest(t.Context(), req)
	require.NoError(t, err)

	var body struct {
		Input []json.RawMessage `json:"input"`
	}
	require.NoError(t, json.Unmarshal(wire.Body, &body))

	return body.Input
}

func TestAdditionalTools_MixedRawItemsKeepTheirPlace(t *testing.T) {
	input := additionalToolsMixedInput(t)

	require.Len(t, input, 3)
	require.Contains(t, string(input[0]), "additional_tools")
	require.Contains(t, string(input[1]), "Hello")
	require.Contains(t, string(input[2]), "web_search_call")
}

func TestAdditionalToolsAfterSkippedReasoningItemIsPreserved(t *testing.T) {
	const request = `{
		"model": "gpt-6-luna",
		"input": [
			{"type": "reasoning", "summary": []},
			{"type": "message", "role": "user", "content": "Hello"},
			{"type": "additional_tools", "id": "at_1", "role": "developer", "tools": [{"type": "custom", "name": "exec"}]}
		]
	}`

	req, err := NewInboundTransformer().TransformRequest(
		t.Context(), &httpclient.Request{Body: []byte(request)})
	require.NoError(t, err)

	wire, err := additionalToolsOutbound(t).TransformRequest(t.Context(), req)
	require.NoError(t, err)

	var body struct {
		Input []json.RawMessage `json:"input"`
	}
	require.NoError(t, json.Unmarshal(wire.Body, &body))
	require.Len(t, body.Input, 2)
	require.Contains(t, string(body.Input[0]), "Hello")
	require.Contains(t, string(body.Input[1]), "additional_tools")
}
