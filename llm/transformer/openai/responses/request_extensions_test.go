package responses

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
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

func TestInboundTransformer_TransformRequest_CapturesOpenAIResponsesCompatibilityFields(t *testing.T) {
	inbound := NewInboundTransformer()
	req, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5.1",
			"include": ["reasoning.encrypted_content"],
			"max_tool_calls": 7,
			"prompt_cache_key": "cache-key",
			"prompt_cache_retention": "24h",
			"truncation": "auto",
			"stream": true,
			"stream_options": {"include_obfuscation": true},
			"input": "hello",
			"tools": [
				{"type": "image_generation", "output_format": "webp"}
			]
		}`),
	})
	require.NoError(t, err)

	ext := requireResponsesRequestExtensions(t, req)
	require.Equal(t, []string{"reasoning.encrypted_content"}, ext.Include)
	require.Equal(t, int64(7), *ext.MaxToolCalls)
	require.Equal(t, "cache-key", *ext.PromptCacheKey)
	require.Equal(t, "24h", *ext.PromptCacheRetention)
	require.Equal(t, "auto", *ext.Truncation)
	require.True(t, *ext.IncludeObfuscation)
	require.Equal(t, "webp", ext.ImageOutputFormat)

	require.Equal(t, ext.Include, req.TransformerMetadata[responsesMetadataKeyInclude])
	require.Equal(t, ext.MaxToolCalls, req.TransformerMetadata[responsesMetadataKeyMaxToolCalls])
	require.Equal(t, ext.PromptCacheRetention, req.TransformerMetadata[responsesMetadataKeyPromptCacheRetention])
	require.Equal(t, ext.Truncation, req.TransformerMetadata[responsesMetadataKeyTruncation])
	require.Equal(t, ext.IncludeObfuscation, req.TransformerMetadata[responsesMetadataKeyIncludeObfuscation])
	require.Equal(t, ext.ImageOutputFormat, req.TransformerMetadata[responsesMetadataKeyImageOutputFormat])
}

func TestInboundTransformer_TransformRequest_CapturesTopLevelSemanticExtraProtection(t *testing.T) {
	inbound := NewInboundTransformer()
	req, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5.1",
			"prompt": {"id": "prompt_1", "text": "system secret"},
			"local_shell": {"type": "local_shell", "input": "cat ~/.config/token"},
			"input": "hello"
		}`),
	})
	require.NoError(t, err)

	ext := requireResponsesRequestExtensions(t, req)
	require.Equal(t, fragmentClassSemanticControl, ext.TopLevelSemanticExtraClasses["prompt"])
	require.Equal(t, fragmentClassExecutableTool, ext.TopLevelSemanticExtraClasses["local_shell"])
	require.Equal(t, []string{"prompt.text"}, ext.TopLevelSemanticExtraTextPaths["prompt"])
	require.Equal(t, []string{"local_shell.input"}, ext.TopLevelSemanticExtraTextPaths["local_shell"])

	fragmentsByPath := map[string]llm.OpenAIResponsesProtectableFragment{}
	for _, fragment := range ext.ProtectableFragments {
		fragmentsByPath[fragment.Path] = fragment
	}
	require.Equal(t, fragmentClassSemanticControl, fragmentsByPath["prompt.text"].Scope)
	require.Equal(t, "system secret", fragmentsByPath["prompt.text"].Text)
	require.Equal(t, fragmentClassExecutableTool, fragmentsByPath["local_shell.input"].Scope)
	require.Equal(t, "cat ~/.config/token", fragmentsByPath["local_shell.input"].Text)
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

func TestOutboundTransformer_TransformRequest_ProviderExtensionsCompatibilityPreferredOverLegacyMetadata(t *testing.T) {
	legacyMaxToolCalls := int64(1)
	extMaxToolCalls := int64(3)
	req := &llm.Request{
		Model: "gpt-5.1",
		Messages: []llm.Message{
			{
				Role: "user",
				Content: llm.MessageContent{
					Content: lo.ToPtr("hello"),
				},
			},
		},
		Stream:        lo.ToPtr(true),
		StreamOptions: &llm.StreamOptions{IncludeUsage: true},
		TransformerMetadata: map[string]any{
			responsesMetadataKeyInclude:              []string{"legacy.include"},
			responsesMetadataKeyMaxToolCalls:         &legacyMaxToolCalls,
			responsesMetadataKeyPromptCacheKey:       lo.ToPtr("legacy-cache"),
			responsesMetadataKeyPromptCacheRetention: lo.ToPtr("legacy-retention"),
			responsesMetadataKeyTruncation:           lo.ToPtr("legacy-truncation"),
			responsesMetadataKeyIncludeObfuscation:   lo.ToPtr(false),
		},
		ProviderExtensions: &llm.ProviderExtensions{
			OpenAIResponses: &llm.OpenAIResponsesProviderExtensions{
				Request: &llm.OpenAIResponsesRequestExtensions{
					Include:              []string{"reasoning.encrypted_content"},
					MaxToolCalls:         &extMaxToolCalls,
					PromptCacheKey:       lo.ToPtr("ext-cache"),
					PromptCacheRetention: lo.ToPtr("24h"),
					Truncation:           lo.ToPtr("auto"),
					IncludeObfuscation:   lo.ToPtr(true),
				},
			},
		},
	}

	rawReq := outboundResponsesRequest(t, req)

	var payload Request
	require.NoError(t, json.Unmarshal(rawReq.Body, &payload))
	require.Equal(t, []string{"reasoning.encrypted_content"}, payload.Include)
	require.Equal(t, int64(3), *payload.MaxToolCalls)
	require.Equal(t, "ext-cache", *payload.PromptCacheKey)
	require.Equal(t, "24h", *payload.PromptCacheRetention)
	require.Equal(t, "auto", *payload.Truncation)
	require.NotNil(t, payload.StreamOptions)
	require.True(t, *payload.StreamOptions.IncludeObfuscation)

	require.Equal(t, []string{"reasoning.encrypted_content"}, rawReq.TransformerMetadata[responsesMetadataKeyInclude])
	require.Equal(t, &extMaxToolCalls, rawReq.TransformerMetadata[responsesMetadataKeyMaxToolCalls])
	require.Equal(t, "legacy-cache", *rawReq.TransformerMetadata[responsesMetadataKeyPromptCacheKey].(*string))
	require.Equal(t, "24h", *rawReq.TransformerMetadata[responsesMetadataKeyPromptCacheRetention].(*string))
	require.Equal(t, "auto", *rawReq.TransformerMetadata[responsesMetadataKeyTruncation].(*string))
	require.True(t, *rawReq.TransformerMetadata[responsesMetadataKeyIncludeObfuscation].(*bool))
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

func TestOutboundTransformer_TransformRequest_DropsUnscannedTopLevelSemanticExtraWithText(t *testing.T) {
	inbound := NewInboundTransformer()
	req, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5.1",
			"prompt": {"text": "system secret"},
			"input": "hello"
		}`),
	})
	require.NoError(t, err)

	body := outboundResponsesBody(t, req)
	require.NotContains(t, string(body), "system secret")
}

func TestOutboundTransformer_TransformRequest_ReplaysScannedCleanTopLevelSemanticExtraWithText(t *testing.T) {
	inbound := NewInboundTransformer()
	req, err := inbound.TransformRequest(context.Background(), &httpclient.Request{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body: []byte(`{
			"model": "gpt-5.1",
			"prompt": {"text": "system prompt"},
			"input": "hello"
		}`),
	})
	require.NoError(t, err)

	ext := requireResponsesRequestExtensions(t, req)
	ext.TopLevelSemanticExtraProtection["prompt"] = llm.OpenAIResponsesRawProtection{
		Status:        llm.OpenAIResponsesProtectionEvaluatedNoRules,
		Scanned:       true,
		TextExtracted: true,
		ReplayAllowed: true,
		Scope:         fragmentClassSemanticControl,
		TextPaths:     []string{"prompt.text"},
	}

	body := outboundResponsesBody(t, req)
	require.JSONEq(t, `{"text":"system prompt"}`, rawJSONField(t, body, "prompt"))
}

func TestOutboundTransformer_TransformRequest_MetadataDirtyOverlaysStructuredValues(t *testing.T) {
	req := transformRawExtensionFixture(t)
	req.Metadata["owner"] = "team-b"
	req.Metadata["new"] = "value"
	llm.MarkOpenAIResponsesDirty(req, llm.OpenAIResponsesDirtyMetadata)

	body := outboundResponsesBody(t, req)
	require.JSONEq(t, `{
		"owner": "team-b",
		"flags": {"safe": true},
		"count": 2,
		"enabled": true,
		"none": null,
		"new": "value"
	}`, rawJSONField(t, body, "metadata"))
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

	rawReq := outboundResponsesRequest(t, req)

	return rawReq.Body
}

func outboundResponsesRequest(t *testing.T, req *llm.Request) *httpclient.Request {
	t.Helper()

	outbound, err := NewOutboundTransformerWithConfig(&Config{
		BaseURL:        "https://api.openai.com",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)

	rawReq, err := outbound.TransformRequest(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, rawReq.Body)

	return rawReq
}

func rawJSONField(t *testing.T, body []byte, key string) string {
	t.Helper()

	var obj map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &obj))
	require.Contains(t, obj, key)

	return string(obj[key])
}
