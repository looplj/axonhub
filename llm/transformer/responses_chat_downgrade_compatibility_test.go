package transformer_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/auth"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/cline"
	"github.com/looplj/axonhub/llm/transformer/nanogpt"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/copilot"
	"github.com/looplj/axonhub/llm/transformer/shared"
)

func TestPlainChatOutboundsDowngradeSourceToolHistoryIdempotently(t *testing.T) {
	tests := []struct {
		name     string
		outbound transformer.Outbound
		model    string
	}{
		{name: "cline", outbound: newClineOutboundForCompatibilityTest(t), model: "test-model"},
		{name: "nanogpt", outbound: newNanoGPTOutboundForCompatibilityTest(t), model: "test-model"},
		{name: "copilot", outbound: newCopilotOutboundForCompatibilityTest(t), model: "gpt-4o"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := sourceToolLifecycleRequestForCompatibilityTest(tc.model)

			direct, err := tc.outbound.TransformRequest(t.Context(), request)
			require.NoError(t, err)

			preDowngraded, err := shared.DowngradeResponsesChatToolLifecycle(request)
			require.NoError(t, err)
			repeated, err := tc.outbound.TransformRequest(t.Context(), preDowngraded)
			require.NoError(t, err)
			require.JSONEq(t, string(direct.Body), string(repeated.Body))

			assertSourceToolLifecycleDowngradedForCompatibilityTest(t, direct)
			assertSourceToolLifecycleDowngradedForCompatibilityTest(t, repeated)
			require.Equal(t, llm.APIFormatOpenAIResponse, request.APIFormat)
			require.Len(t, request.Messages[1].ToolCalls, 2)
			require.Len(t, request.Tools, 2)
		})
	}
}

func newClineOutboundForCompatibilityTest(t *testing.T) transformer.Outbound {
	t.Helper()
	outbound, err := cline.NewOutboundTransformerWithConfig(&cline.Config{
		BaseURL:        "https://api.cline.bot/api/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	return outbound
}

func newNanoGPTOutboundForCompatibilityTest(t *testing.T) transformer.Outbound {
	t.Helper()
	outbound, err := nanogpt.NewOutboundTransformerWithConfig(&nanogpt.Config{
		BaseURL:        "https://nano-gpt.com/api/v1",
		APIKeyProvider: auth.NewStaticKeyProvider("test-key"),
	})
	require.NoError(t, err)
	return outbound
}

func newCopilotOutboundForCompatibilityTest(t *testing.T) transformer.Outbound {
	t.Helper()
	outbound, err := copilot.NewOutboundTransformer(copilot.OutboundTransformerParams{
		TokenProvider: staticCopilotTokenProviderForCompatibilityTest{token: "ghu_testtoken123"},
	})
	require.NoError(t, err)
	return outbound
}

type staticCopilotTokenProviderForCompatibilityTest struct {
	token string
}

func (p staticCopilotTokenProviderForCompatibilityTest) GetToken(context.Context) (string, error) {
	return p.token, nil
}

func sourceToolLifecycleRequestForCompatibilityTest(model string) *llm.Request {
	content := "run"
	return &llm.Request{
		Model:     model,
		APIFormat: llm.APIFormatOpenAIResponse,
		Messages: []llm.Message{
			{Role: "user", Content: llm.MessageContent{Content: &content}},
			{Role: "assistant", ToolCalls: []llm.ToolCall{
				{
					ID: "source_1", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "future_lookup", Arguments: `{}`},
				},
				{
					ID: "plain_1", Type: llm.ToolTypeFunction,
					Function: llm.FunctionCall{Name: "lookup", Arguments: `{}`},
				},
			}},
			{Role: "tool", ToolCallID: lo.ToPtr("source_1"), Content: llm.MessageContent{Content: lo.ToPtr("future output")}},
			{Role: "tool", ToolCallID: lo.ToPtr("plain_1"), Content: llm.MessageContent{Content: lo.ToPtr("plain output")}},
		},
		Tools: []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.Function{Name: "lookup", Parameters: json.RawMessage(`{"type":"object"}`)}},
			{
				Type:                llm.ToolTypeFunction,
				Function:            llm.Function{Name: "future_lookup", Parameters: json.RawMessage(`{"type":"object"}`)},
				ResponsesSourceType: "future_client_tool",
			},
		},
	}
}

func assertSourceToolLifecycleDowngradedForCompatibilityTest(t *testing.T, request *httpclient.Request) {
	t.Helper()

	var payload struct {
		Messages []struct {
			Role       string  `json:"role"`
			ToolCallID *string `json:"tool_call_id"`
			ToolCalls  []struct {
				ID       string `json:"id"`
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
		Tools []struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		} `json:"tools"`
	}
	require.NoError(t, json.Unmarshal(request.Body, &payload))
	require.Len(t, payload.Messages, 3)
	require.Len(t, payload.Messages[1].ToolCalls, 1)
	require.Equal(t, "plain_1", payload.Messages[1].ToolCalls[0].ID)
	require.Equal(t, "lookup", payload.Messages[1].ToolCalls[0].Function.Name)
	require.Equal(t, "plain_1", *payload.Messages[2].ToolCallID)
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "lookup", payload.Tools[0].Function.Name)
	require.NotContains(t, request.TransformerMetadata, openai.ResponsesChatToolMappingsMetadataKey)
}
