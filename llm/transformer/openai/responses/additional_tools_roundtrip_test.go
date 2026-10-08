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
						{"type": "custom", "name": "exec", "description": "run a script"},
						{"type": "function", "name": "shell", "parameters": {"type": "object", "properties": {}}}
					]
				}
			]
		},
		{"type": "message", "role": "user", "content": "Hello"}
	]
}`

func additionalToolsOutbound(t *testing.T, preserve bool) *OutboundTransformer {
	t.Helper()

	out, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:                 "https://example.com",
		APIKeyProvider:          auth.NewStaticKeyProvider("test"),
		PreserveAdditionalTools: preserve,
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

// TestAdditionalTools_DroppedForCompatibleUpstreams pins the behaviour of the
// OpenAI-compatible path: an upstream that is not the official Codex backend
// rejects `additional_tools` as an unsupported input item type, so the item must
// not be replayed there.
func TestAdditionalTools_DroppedForCompatibleUpstreams(t *testing.T) {
	input := additionalToolsInput(t, additionalToolsOutbound(t, false))

	require.Len(t, input, 1)
	require.NotContains(t, string(input[0]), "additional_tools")
	require.Contains(t, string(input[0]), "Hello")
}

// TestAdditionalTools_ReplayedForOfficialCodex is the counterpart: for the
// upstream that does speak the private protocol the item is where a Lite request
// keeps its tool definitions, so it has to come through verbatim, in place.
func TestAdditionalTools_ReplayedForOfficialCodex(t *testing.T) {
	input := additionalToolsInput(t, additionalToolsOutbound(t, true))

	require.Len(t, input, 2)
	require.Contains(t, string(input[0]), `"additional_tools"`)
	require.Contains(t, string(input[0]), `"exec"`)
	require.Contains(t, string(input[0]), `"shell"`)
	require.Contains(t, string(input[1]), "Hello")
}

// additionalToolsMixedRequest is the lite request with a second raw-only input
// item after the message: the tool definitions are not the only item that has to
// be replayed verbatim.
const additionalToolsMixedRequest = `{
	"model": "gpt-6-luna",
	"input": [
		{"type": "additional_tools", "id": "at_1", "role": "developer", "tools": []},
		{"type": "message", "role": "user", "content": "Hello"},
		{"type": "web_search_call", "id": "ws_1", "status": "completed"}
	]
}`

func additionalToolsMixedInput(t *testing.T, preserve bool) []json.RawMessage {
	t.Helper()

	req, err := NewInboundTransformer().TransformRequest(
		t.Context(), &httpclient.Request{Body: []byte(additionalToolsMixedRequest)})
	require.NoError(t, err)

	wire, err := additionalToolsOutbound(t, preserve).TransformRequest(t.Context(), req)
	require.NoError(t, err)

	var body struct {
		Input []json.RawMessage `json:"input"`
	}
	require.NoError(t, json.Unmarshal(wire.Body, &body))

	return body.Input
}

// TestAdditionalTools_MixedRawItemsKeepTheirPlace pins that dropping the lite
// item must not disturb the other raw-only items: they are replayed verbatim and
// in the position they had in the incoming request.
func TestAdditionalTools_MixedRawItemsKeepTheirPlace(t *testing.T) {
	input := additionalToolsMixedInput(t, false)

	require.Len(t, input, 2)
	require.Contains(t, string(input[0]), "Hello")
	require.Contains(t, string(input[1]), "web_search_call")
}
