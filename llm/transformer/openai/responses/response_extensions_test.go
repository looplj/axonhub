package responses

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestResponsesResponseRoundTrip_PreservesRawMetadataExtraAndUnknownOutput(t *testing.T) {
	llmResp := transformProviderResponsesBody(t, responseBodyWithRawExtensions())

	ext := requireResponsesResponseExtensions(t, llmResp)
	require.JSONEq(t, `{
		"str": "ok",
		"object_value": {"nested": true},
		"array_value": [1, 2],
		"number_value": 3,
		"bool_value": true,
		"null_value": null
	}`, string(ext.MetadataRaw))
	require.Len(t, ext.OutputItems, 9)

	httpResp, err := NewInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual map[string]any
	err = json.Unmarshal(httpResp.Body, &actual)
	require.NoError(t, err)

	require.Equal(t, "resp_raw", actual["id"])
	require.Equal(t, "provider-model", actual["model"])
	require.Equal(t, map[string]any{"trace": "abc"}, actual["client_metadata"])

	metadata := actual["metadata"].(map[string]any)
	require.Equal(t, "ok", metadata["str"])
	require.Equal(t, map[string]any{"nested": true}, metadata["object_value"])
	require.Equal(t, []any{float64(1), float64(2)}, metadata["array_value"])
	require.Equal(t, float64(3), metadata["number_value"])
	require.Equal(t, true, metadata["bool_value"])
	require.Nil(t, metadata["null_value"])

	output := actual["output"].([]any)
	require.Len(t, output, 8)

	message := output[0].(map[string]any)
	require.Equal(t, "message", message["type"])
	require.Equal(t, "final_answer", message["phase"])
	content := message["content"].([]any)[0].(map[string]any)
	require.Equal(t, "output_text", content["type"])
	require.Equal(t, "hello", content["text"])
	require.Equal(t, []any{}, content["logprobs"])
	require.Equal(t, map[string]any{"source": "provider"}, content["content_extra"])

	require.Equal(t, "mcp_call", output[1].(map[string]any)["type"])
	require.Equal(t, "shell_call", output[2].(map[string]any)["type"])
	require.Equal(t, "shell_call_output", output[3].(map[string]any)["type"])
	require.Equal(t, "tool_search_call", output[4].(map[string]any)["type"])
	require.Equal(t, "tool_search_output", output[5].(map[string]any)["type"])
	require.Equal(t, "mcp_tool_call_output", output[6].(map[string]any)["type"])
	require.Equal(t, "future_output", output[7].(map[string]any)["type"])
}

func TestResponsesResponseComposer_StructuredFieldsWinOverRawEnvelope(t *testing.T) {
	llmResp := transformProviderResponsesBody(t, responseBodyWithRawExtensions())
	llmResp.ID = "resp_client"
	llmResp.Model = "client-model"
	llmResp.Created = 99
	llmResp.PreviousResponseID = lo.ToPtr("resp_prev_new")
	llmResp.Usage = &llm.Usage{
		PromptTokens:     2,
		CompletionTokens: 3,
		TotalTokens:      5,
	}
	llmResp.Choices[0].FinishReason = lo.ToPtr("length")

	httpResp, err := NewInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual map[string]any
	err = json.Unmarshal(httpResp.Body, &actual)
	require.NoError(t, err)

	require.Equal(t, "resp_client", actual["id"])
	require.Equal(t, "client-model", actual["model"])
	require.Equal(t, float64(99), actual["created_at"])
	require.Equal(t, "resp_prev_new", actual["previous_response_id"])
	require.Equal(t, "incomplete", actual["status"])

	usage := actual["usage"].(map[string]any)
	require.Equal(t, float64(2), usage["input_tokens"])
	require.Equal(t, float64(3), usage["output_tokens"])
	require.Equal(t, float64(5), usage["total_tokens"])

	output := actual["output"].([]any)
	require.Len(t, output, 8)
	require.Equal(t, "future_output", output[7].(map[string]any)["type"])
}

func TestResponsesResponseComposer_DirtyOutputDoesNotRestoreRawOnlyOutput(t *testing.T) {
	llmResp := transformProviderResponsesBody(t, responseBodyWithRawExtensions())
	llmResp.ProviderExtensions.OpenAIResponses.Dirty.Mark(llm.OpenAIResponsesDirtyResponseOutput)

	httpResp, err := NewInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual map[string]any
	err = json.Unmarshal(httpResp.Body, &actual)
	require.NoError(t, err)

	output := actual["output"].([]any)
	require.Len(t, output, 1)
	message := output[0].(map[string]any)
	require.Equal(t, "message", message["type"])
	require.Equal(t, "final_answer", message["phase"])
	content := message["content"].([]any)[0].(map[string]any)
	require.Equal(t, []any{}, content["logprobs"])
	require.NotContains(t, string(httpResp.Body), "future_output")
	require.NotContains(t, string(httpResp.Body), "mcp_tool_call_output")
}

func TestResponsesResponseComposer_DirtyEnvelopeDoesNotRestoreRawEnvelopeExtras(t *testing.T) {
	llmResp := transformProviderResponsesBody(t, responseBodyWithRawExtensions())
	llmResp.ProviderExtensions.OpenAIResponses.Dirty.Mark(llm.OpenAIResponsesDirtyResponseEnvelope)

	httpResp, err := NewInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual map[string]any
	err = json.Unmarshal(httpResp.Body, &actual)
	require.NoError(t, err)

	require.NotContains(t, actual, "client_metadata")
	require.NotContains(t, actual, "metadata")
}

func TestResponsesResponseRoundTrip_RawOnlyOutputIsContentAndDoesNotCreateEmptyMessage(t *testing.T) {
	llmResp := transformProviderResponsesBody(t, []byte(`{
		"id": "resp_raw_only",
		"object": "response",
		"created_at": 1,
		"status": "completed",
		"model": "gpt-4o",
		"output": [
			{
				"id": "mcp_1",
				"type": "mcp_call",
				"call_id": "call_mcp",
				"server_label": "repo",
				"output": {"ok": true}
			}
		]
	}`))

	require.True(t, llm.HasRawOnlyResponseContent(llmResp))

	httpResp, err := NewInboundTransformer().TransformResponse(t.Context(), llmResp)
	require.NoError(t, err)

	var actual Response
	err = json.Unmarshal(httpResp.Body, &actual)
	require.NoError(t, err)
	require.Len(t, actual.Output, 1)
	require.Equal(t, "mcp_call", actual.Output[0].Type)
	require.Equal(t, "mcp_1", actual.Output[0].ID)
	require.Contains(t, actual.Output[0].Extra, "server_label")
}

func transformProviderResponsesBody(t *testing.T, body []byte) *llm.Response {
	t.Helper()

	transformer, err := NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	result, err := transformer.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body:       body,
	})
	require.NoError(t, err)

	return result
}

func requireResponsesResponseExtensions(t *testing.T, resp *llm.Response) *llm.OpenAIResponsesResponseExtensions {
	t.Helper()

	require.NotNil(t, resp.ProviderExtensions)
	require.NotNil(t, resp.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, resp.ProviderExtensions.OpenAIResponses.Response)

	return resp.ProviderExtensions.OpenAIResponses.Response
}

func responseBodyWithRawExtensions() []byte {
	return []byte(`{
		"id": "resp_raw",
		"object": "response",
		"created_at": 42,
		"status": "completed",
		"model": "provider-model",
		"client_metadata": {"trace": "abc"},
		"metadata": {
			"str": "ok",
			"object_value": {"nested": true},
			"array_value": [1, 2],
			"number_value": 3,
			"bool_value": true,
			"null_value": null
		},
		"output": [
			{
				"id": "msg_1",
				"type": "message",
				"status": "completed",
				"content": [
					{
						"type": "output_text",
						"text": "hello",
						"annotations": [],
						"logprobs": [],
						"content_extra": {"source": "provider"}
					}
				],
				"phase": "final_answer",
				"role": "assistant"
			},
			{
				"id": "mcp_1",
				"type": "mcp_call",
				"call_id": "call_mcp",
				"server_label": "repo",
				"output": {"ok": true}
			},
			{
				"id": "shell_call_1",
				"type": "shell_call",
				"call_id": "call_shell",
				"command": "pwd"
			},
			{
				"id": "shell_1",
				"type": "shell_call_output",
				"call_id": "call_shell",
				"output": "done"
			},
			{
				"id": "search_call_1",
				"type": "tool_search_call",
				"call_id": "call_search",
				"query": "docs"
			},
			{
				"id": "search_1",
				"type": "tool_search_output",
				"call_id": "call_search",
				"output": [{"title": "doc"}]
			},
			{
				"id": "legacy_1",
				"type": "mcp_tool_call_output",
				"call_id": "call_legacy",
				"output": "legacy"
			},
			{
				"id": "unknown_1",
				"type": "future_output",
				"payload": {"x": 1}
			}
		],
		"usage": {
			"input_tokens": 10,
			"output_tokens": 5,
			"total_tokens": 15
		}
	}`)
}
