package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	chatoutbound "github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

// G8-S1: context_management is not a typed Request field. Same-protocol replay
// must keep it only via generic RawTopLevelFields fallback.
func TestResponsesContextManagement_SameProtocolRawTopLevelFallback(t *testing.T) {
	rawContextManagement := `[{"type":"compaction","compact_threshold":120000}]`
	payload, llmReq := roundTripResponsesRawPayload(t, `{
		"model": "gpt-4o",
		"input": "continue with compaction policy",
		"context_management": [{"type":"compaction","compact_threshold":120000}]
	}`, nil)

	require.NotNil(t, llmReq.ProviderExtensions)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses)
	require.NotNil(t, llmReq.ProviderExtensions.OpenAIResponses.Request)
	rawTop := llmReq.ProviderExtensions.OpenAIResponses.Request.RawTopLevelFields
	require.Contains(t, rawTop, "context_management")
	require.JSONEq(t, rawContextManagement, string(rawTop["context_management"]))
	require.JSONEq(t, rawContextManagement, string(payload["context_management"]))

	// Field-level raw fidelity only. Do not claim cross-protocol semantics.
	require.Nil(t, llmReq.ProviderExtensions.Diagnostics)
}

// G8-S2: Request.Conversation remains commented; string/object forms must still
// have field-specific same-protocol coverage through raw fallback.
func TestResponsesConversation_SameProtocolRawTopLevelFallback(t *testing.T) {
	t.Run("string_id", func(t *testing.T) {
		payload, llmReq := roundTripResponsesRawPayload(t, `{
			"model": "gpt-4o",
			"input": "continue conversation",
			"conversation": "conv_string_123"
		}`, nil)

		rawTop := llmReq.ProviderExtensions.OpenAIResponses.Request.RawTopLevelFields
		require.Contains(t, rawTop, "conversation")
		require.JSONEq(t, `"conv_string_123"`, string(rawTop["conversation"]))
		require.JSONEq(t, `"conv_string_123"`, string(payload["conversation"]))
	})

	t.Run("object_id", func(t *testing.T) {
		rawConversation := `{"id":"conv_obj_456","x_extra":"keep"}`
		payload, llmReq := roundTripResponsesRawPayload(t, `{
			"model": "gpt-4o",
			"input": "continue conversation object",
			"conversation": {"id":"conv_obj_456","x_extra":"keep"}
		}`, nil)

		rawTop := llmReq.ProviderExtensions.OpenAIResponses.Request.RawTopLevelFields
		require.Contains(t, rawTop, "conversation")
		require.JSONEq(t, rawConversation, string(rawTop["conversation"]))
		require.JSONEq(t, rawConversation, string(payload["conversation"]))
	})
}

// G8-S5: Hosted/native tool types recognized by classification, plus same-protocol
// raw fidelity for non-structurally-represented hosted tools. Chat remains lossy
// without synthesizing function tools.
func TestResponsesHostedTools_SameProtocolRawPreserveAndChatLossy(t *testing.T) {
	// Code-recognized hosted/native tool types (IsKnownOpenAIResponsesNativeToolType).
	// Structural owners: function/image_generation/web_search/custom.
	// Remaining known types are raw/native preserve for same-protocol only.
	recognized := []string{
		"function",
		"image_generation",
		"web_search",
		"custom",
		"namespace",
		"tool_search",
		"mcp",
		"file_search",
		"code_interpreter",
		"computer_use_preview",
		"local_shell",
		"shell",
		"apply_patch",
	}
	for _, toolType := range recognized {
		require.Truef(t, llm.IsKnownOpenAIResponsesNativeToolType(toolType), "expected known native tool type %q", toolType)
	}

	hostedRawOnlyCases := []struct {
		name string
		tool string
	}{
		{
			name: "file_search",
			tool: `{"type":"file_search","name":"file_search","vector_store_ids":["vs_g8_1"],"max_num_results":3}`,
		},
		{
			name: "code_interpreter",
			tool: `{"type":"code_interpreter","container":{"type":"auto"}}`,
		},
		{
			name: "computer_use_preview",
			tool: `{"type":"computer_use_preview","display_width":1024,"display_height":768,"environment":"browser"}`,
		},
		{
			name: "mcp",
			tool: `{"type":"mcp","server_label":"docs","server_url":"https://example.com/mcp","require_approval":"never"}`,
		},
		{
			name: "local_shell",
			tool: `{"type":"local_shell"}`,
		},
		{
			name: "shell",
			tool: `{"type":"shell"}`,
		},
		{
			name: "apply_patch",
			tool: `{"type":"apply_patch"}`,
		},
	}

	for _, tc := range hostedRawOnlyCases {
		t.Run("same_protocol_"+tc.name, func(t *testing.T) {
			body := `{
				"model": "gpt-4o",
				"input": "use hosted tool ` + tc.name + `",
				"tools": [` + tc.tool + `]
			}`
			payload, llmReq := roundTripResponsesRawPayload(t, body, nil)

			requestExt := llmReq.ProviderExtensions.OpenAIResponses.Request
			require.NotNil(t, requestExt)
			require.NotEmpty(t, requestExt.RawTools, "hosted tool %s must be raw-only fragment", tc.name)
			require.Equal(t, tc.name, requestExt.RawTools[0].Type)
			require.JSONEq(t, tc.tool, string(requestExt.RawTools[0].Raw))

			var tools []json.RawMessage
			require.NoError(t, json.Unmarshal(payload["tools"], &tools))
			require.Len(t, tools, 1)
			require.JSONEq(t, tc.tool, string(tools[0]))

			// Must not collapse hosted tools into common function tools.
			for _, tool := range llmReq.Tools {
				require.NotEqual(t, "function", tool.Type, "hosted tool %s must not become function", tc.name)
			}
		})
	}

	t.Run("chat_outbound_lossy_no_function_synth", func(t *testing.T) {
		body := `{
			"model": "gpt-4o",
			"input": "use file search",
			"tools": [
				{"type":"file_search","name":"file_search","vector_store_ids":["vs_g8_chat"]}
			]
		}`
		inbound := NewInboundTransformer()
		llmReq, err := inbound.TransformRequest(context.Background(), &httpclient.Request{Body: []byte(body)})
		require.NoError(t, err)
		llmReq.Model = "gpt-4o"

		chatOut, err := chatoutbound.NewOutboundTransformer("https://api.openai.com", "test-key")
		require.NoError(t, err)
		httpReq, err := chatOut.TransformRequest(context.Background(), llmReq)
		require.NoError(t, err)

		diagnosticsPtr := llm.ResponsesLossySummaryOf(llmReq)
	ok := diagnosticsPtr != nil
	var diagnostics shared.ResponsesLossyDowngradeDiagnostics
	if ok {
		diagnostics = *diagnosticsPtr
	}
		require.True(t, ok)
		require.True(t, diagnostics.LossyDowngrade)
		require.Equal(t, 1, diagnostics.RawOnlyToolCount)
		require.Equal(t, 0, diagnostics.UnknownToolCount)

		var chatPayload map[string]any
		require.NoError(t, json.Unmarshal(httpReq.Body, &chatPayload))
		if tools, exists := chatPayload["tools"]; exists {
			raw, err := json.Marshal(tools)
			require.NoError(t, err)
			require.NotContains(t, string(raw), `"type":"file_search"`)
			require.NotContains(t, string(raw), "vs_g8_chat")
		}
	})
}
