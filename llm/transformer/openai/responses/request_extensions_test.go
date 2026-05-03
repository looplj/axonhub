package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestInboundTransformer_TransformRequest_CapturesOpenAIResponsesRawExtensions(t *testing.T) {
	req := transformRawExtensionFixture(t)

	require.Equal(t, map[string]string{"owner": "team-a"}, req.Metadata)
	require.Len(t, req.Tools, 1)
	require.Equal(t, "function", req.Tools[0].Type)

	ext := requireResponsesRequestExtensions(t, req)
	require.JSONEq(t, `{"owner":"team-a","flags":{"safe":true},"count":2,"enabled":true,"none":null}`, string(ext.MetadataRaw))
	require.Contains(t, ext.TopLevelExtra, "client_metadata")
	require.Contains(t, ext.TopLevelSemanticExtra, "conversation")
	require.Equal(t, inputKindArray, ext.InputKind)
	require.JSONEq(t, `[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},{"type":"shell_call_output","call_id":"call_shell","output":"raw secret"}]`, string(ext.InputRaw))

	require.Len(t, ext.InputItems, 2)
	require.Equal(t, "message", ext.InputItems[0].Type)
	require.NotEmpty(t, ext.InputItems[0].SemanticKey)
	require.Equal(t, "shell_call_output", ext.InputItems[1].Type)
	require.Empty(t, ext.InputItems[1].SemanticKey)
	require.True(t, ext.InputItems[1].Protection.TextExtracted)
	require.False(t, ext.InputItems[1].Protection.ReplayAllowed)
	require.NotEmpty(t, ext.ProtectableFragments)

	require.Len(t, ext.Tools, 2)
	require.Equal(t, "function", ext.Tools[0].Type)
	require.NotEmpty(t, ext.Tools[0].SemanticKey)
	require.Equal(t, "namespace", ext.Tools[1].Type)
	require.Empty(t, ext.Tools[1].SemanticKey)
	require.JSONEq(t, `{"mode":"required","tools":[{"type":"namespace","name":"shell"}]}`, string(ext.ToolChoiceRaw))
}

func TestOutboundTransformer_TransformRequest_PreservesRawExtensionsWhenClean(t *testing.T) {
	req := transformRawExtensionFixture(t)
	req.Model = "mapped-model"

	ext := requireResponsesRequestExtensions(t, req)
	for i := range ext.InputItems {
		if ext.InputItems[i].SemanticKey != "" {
			continue
		}
		ext.InputItems[i].Protection.Scanned = true
		ext.InputItems[i].Protection.Status = llm.OpenAIResponsesProtectionEvaluatedNoRules
		ext.InputItems[i].Protection.ReplayAllowed = true
	}

	body := outboundResponsesBody(t, req)
	require.JSONEq(t, `"mapped-model"`, rawJSONField(t, body, "model"))
	require.JSONEq(t, `{"owner":"team-a","flags":{"safe":true},"count":2,"enabled":true,"none":null}`, rawJSONField(t, body, "metadata"))
	require.JSONEq(t, `{"id":"conv_123"}`, rawJSONField(t, body, "conversation"))
	require.JSONEq(t, `{"trace":"abc"}`, rawJSONField(t, body, "client_metadata"))
	require.JSONEq(t, `[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},{"type":"shell_call_output","call_id":"call_shell","output":"raw secret"}]`, rawJSONField(t, body, "input"))
	require.JSONEq(t, `[{"type":"function","name":"lookup","parameters":{"type":"object"}},{"type":"namespace","name":"shell","description":"local shell"}]`, rawJSONField(t, body, "tools"))
	require.JSONEq(t, `{"mode":"required","tools":[{"type":"namespace","name":"shell"}]}`, rawJSONField(t, body, "tool_choice"))
}

func TestOutboundTransformer_TransformRequest_DirtyStructuredWinsOverRawReplay(t *testing.T) {
	req := transformRawExtensionFixture(t)
	req.Model = "mapped-model"
	llm.MarkOpenAIResponsesDirty(
		req,
		llm.OpenAIResponsesDirtyMessages,
		llm.OpenAIResponsesDirtyInputItems,
		llm.OpenAIResponsesDirtyTools,
		llm.OpenAIResponsesDirtyToolChoice,
		llm.OpenAIResponsesDirtyTopLevelSemanticExtra,
	)

	body := outboundResponsesBody(t, req)
	bodyText := string(body)
	require.Contains(t, bodyText, "mapped-model")
	require.NotContains(t, bodyText, "raw secret")
	require.NotContains(t, bodyText, "shell_call_output")
	require.NotContains(t, bodyText, "namespace")
	require.NotContains(t, bodyText, "conv_123")
	require.Contains(t, bodyText, "hello")
	require.JSONEq(t, `{"owner":"team-a","flags":{"safe":true},"count":2,"enabled":true,"none":null}`, rawJSONField(t, body, "metadata"))
}

func transformRawExtensionFixture(t *testing.T) *llm.Request {
	t.Helper()

	inbound := NewInboundTransformer()
	req, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5.1",
			"client_metadata": {"trace": "abc"},
			"conversation": {"id": "conv_123"},
			"metadata": {
				"owner": "team-a",
				"flags": {"safe": true},
				"count": 2,
				"enabled": true,
				"none": null
			},
			"input": [
				{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "hello"}]},
				{"type": "shell_call_output", "call_id": "call_shell", "output": "raw secret"}
			],
			"tools": [
				{"type": "function", "name": "lookup", "parameters": {"type": "object"}},
				{"type": "namespace", "name": "shell", "description": "local shell"}
			],
			"tool_choice": {"mode": "required", "tools": [{"type": "namespace", "name": "shell"}]}
		}`),
	})
	require.NoError(t, err)

	return req
}

func requireResponsesRequestExtensions(t *testing.T, req *llm.Request) *llm.OpenAIResponsesRequestExtensions {
	t.Helper()

	require.NotNil(t, req.ProviderExtensions)
	require.NotNil(t, req.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, req.ProviderExtensions.OpenAIResponses.Request)

	return req.ProviderExtensions.OpenAIResponses.Request
}

func outboundResponsesBody(t *testing.T, req *llm.Request) []byte {
	t.Helper()

	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.openai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	rawReq, err := outbound.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, rawReq.Body)

	return rawReq.Body
}

func rawJSONField(t *testing.T, body []byte, key string) string {
	t.Helper()

	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &obj))
	require.Contains(t, obj, key)

	return string(obj[key])
}
