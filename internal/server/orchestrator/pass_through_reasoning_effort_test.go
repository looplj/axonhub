package orchestrator

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

// TestApplyPassThroughReasoningEffortMapping verifies the pass-through body helper
// rewrites or removes the effort field for each API format that carries one.
func TestApplyPassThroughReasoningEffortMapping(t *testing.T) {
	dropNone := []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: ""}}
	rewrite := []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: llm.ReasoningEffortLow}}

	t.Run("chat deletes empty mapped value and preserves other fields", func(t *testing.T) {
		body, err := applyPassThroughReasoningEffortMapping(
			[]byte(`{"model":"m","reasoning_effort":"none","top_k":5,"messages":[]}`),
			llm.APIFormatOpenAIChatCompletion,
			dropNone,
		)
		require.NoError(t, err)
		require.False(t, gjson.GetBytes(body, "reasoning_effort").Exists())
		require.Equal(t, int64(5), gjson.GetBytes(body, "top_k").Int())
		require.Equal(t, "m", gjson.GetBytes(body, "model").String())
	})

	t.Run("chat rewrites mapped value", func(t *testing.T) {
		body, err := applyPassThroughReasoningEffortMapping(
			[]byte(`{"reasoning_effort":"none","top_k":5}`),
			llm.APIFormatOpenAIChatCompletion,
			rewrite,
		)
		require.NoError(t, err)
		require.Equal(t, "low", gjson.GetBytes(body, "reasoning_effort").String())
		require.Equal(t, int64(5), gjson.GetBytes(body, "top_k").Int())
	})

	t.Run("responses uses reasoning effort path", func(t *testing.T) {
		body, err := applyPassThroughReasoningEffortMapping(
			[]byte(`{"reasoning":{"effort":"none","summary":"auto"},"input":"hi"}`),
			llm.APIFormatOpenAIResponse,
			dropNone,
		)
		require.NoError(t, err)
		require.False(t, gjson.GetBytes(body, "reasoning.effort").Exists())
		require.Equal(t, "auto", gjson.GetBytes(body, "reasoning.summary").String())
		require.Equal(t, "hi", gjson.GetBytes(body, "input").String())
	})

	t.Run("responses rewrites mapped value", func(t *testing.T) {
		body, err := applyPassThroughReasoningEffortMapping(
			[]byte(`{"reasoning":{"effort":"none"}}`),
			llm.APIFormatOpenAIResponseWebSocket,
			rewrite,
		)
		require.NoError(t, err)
		require.Equal(t, "low", gjson.GetBytes(body, "reasoning.effort").String())
	})

	t.Run("anthropic deletes empty mapped value and preserves output config", func(t *testing.T) {
		body, err := applyPassThroughReasoningEffortMapping(
			[]byte(`{"model":"claude","output_config":{"effort":"none","format":{"type":"json"}},"thinking":{"type":"between_tools"}}`),
			llm.APIFormatAnthropicMessage,
			dropNone,
		)
		require.NoError(t, err)
		require.False(t, gjson.GetBytes(body, "output_config.effort").Exists())
		require.Equal(t, "json", gjson.GetBytes(body, "output_config.format.type").String())
		require.Equal(t, "between_tools", gjson.GetBytes(body, "thinking.type").String())
	})

	t.Run("anthropic rewrites output config effort", func(t *testing.T) {
		body, err := applyPassThroughReasoningEffortMapping(
			[]byte(`{"output_config":{"effort":"none"}}`),
			llm.APIFormatAnthropicMessage,
			rewrite,
		)
		require.NoError(t, err)
		require.Equal(t, "low", gjson.GetBytes(body, "output_config.effort").String())
	})

	tests := []struct {
		name     string
		body     []byte
		format   llm.APIFormat
		mappings []llm.ReasoningEffortMapping
	}{
		{name: "unmatched value", body: []byte(`{"reasoning_effort":"high"}`), format: llm.APIFormatOpenAIChatCompletion, mappings: dropNone},
		{name: "missing field", body: []byte(`{"messages":[]}`), format: llm.APIFormatOpenAIChatCompletion, mappings: dropNone},
		{name: "nil mappings", body: []byte(`{"reasoning_effort":"none"}`), format: llm.APIFormatOpenAIChatCompletion},
		{name: "nil body", format: llm.APIFormatOpenAIChatCompletion, mappings: dropNone},
		{name: "non-string value", body: []byte(`{"reasoning_effort":1}`), format: llm.APIFormatOpenAIChatCompletion, mappings: dropNone},
		{name: "format without effort field", body: []byte(`{"reasoning_effort":"none"}`), format: llm.APIFormatOpenAIEmbedding, mappings: dropNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := applyPassThroughReasoningEffortMapping(tt.body, tt.format, tt.mappings)
			require.NoError(t, err)
			require.Equal(t, string(tt.body), string(body))
		})
	}
}

// TestResponsesTransportAppliesReasoningEffortMapping verifies the Responses
// cross-transport caller applies the channel reasoning effort mapping.
func TestResponsesTransportAppliesReasoningEffortMapping(t *testing.T) {
	tests := []struct {
		name   string
		source llm.APIFormat
		target llm.APIFormat
	}{
		{name: "http to http", source: llm.APIFormatOpenAIResponse, target: llm.APIFormatOpenAIResponse},
		{name: "websocket to websocket", source: llm.APIFormatOpenAIResponseWebSocket, target: llm.APIFormatOpenAIResponseWebSocket},
		{name: "websocket to http", source: llm.APIFormatOpenAIResponseWebSocket, target: llm.APIFormatOpenAIResponse},
		{name: "http to websocket", source: llm.APIFormatOpenAIResponse, target: llm.APIFormatOpenAIResponseWebSocket},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &biz.Channel{Channel: &ent.Channel{
				ID:   1,
				Name: "responses-reasoning-effort",
				Settings: &objects.ChannelSettings{
					PassThroughBody: lo.ToPtr(true),
					TransformOptions: objects.TransformOptions{
						ReasoningEffortMapping: []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: ""}},
					},
				},
			}}
			state := &PersistenceState{
				CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
				LlmRequest: &llm.Request{
					APIFormat: tt.source,
					Model:     "mapped-model",
					RawRequest: &httpclient.Request{
						APIFormat: tt.source.String(),
						Body:      []byte(`{"model":"client-model","input":"hi","reasoning":{"effort":"none","summary":"auto"},"provider_canary":"keep"}`),
					},
				},
				RawProviderRequest: &httpclient.Request{APIFormat: tt.target.String()},
			}
			outbound := &PersistentOutboundTransformer{state: state}
			request := &httpclient.Request{APIFormat: tt.target.String()}

			body, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(context.Background(), request)
			require.NoError(t, err)
			adapted, err := applyResponsesTransportRequestBody(outbound, nil).OnOutboundRawRequest(context.Background(), body)
			require.NoError(t, err)

			require.False(t, gjson.GetBytes(adapted.Body, "reasoning.effort").Exists())
			require.Equal(t, "auto", gjson.GetBytes(adapted.Body, "reasoning.summary").String())
			require.Equal(t, "keep", gjson.GetBytes(adapted.Body, "provider_canary").String())
			require.Equal(t, "hi", gjson.GetBytes(adapted.Body, "input").String())
			require.Equal(t, "mapped-model", gjson.GetBytes(adapted.Body, "model").String())
		})
	}
}

// TestPassThroughBodyAppliesReasoningEffortMapping verifies the chat completions
// pass-through middleware rewrites the mapped effort and keeps unmodeled fields.
func TestPassThroughBodyAppliesReasoningEffortMapping(t *testing.T) {
	channel := &biz.Channel{Channel: &ent.Channel{
		ID:   1,
		Name: "reasoning-effort-mapping",
		Settings: &objects.ChannelSettings{
			PassThroughBody: lo.ToPtr(true),
			TransformOptions: objects.TransformOptions{
				ReasoningEffortMapping: []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: ""}},
			},
		},
	}}
	outbound := &PersistentOutboundTransformer{state: &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest: &llm.Request{
			Model:     "provider-model",
			APIFormat: llm.APIFormatOpenAIChatCompletion,
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatOpenAIChatCompletion),
				Body:      []byte(`{"model":"client-alias","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"none","top_k":5}`),
			},
		},
	}}
	transformed := &httpclient.Request{
		APIFormat: string(llm.APIFormatOpenAIChatCompletion),
		Body:      []byte(`{"model":"provider-model","messages":[{"role":"user","content":"hi"}]}`),
	}

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(context.Background(), transformed)
	require.NoError(t, err)
	require.True(t, outbound.state.PassThroughApplied)
	require.Equal(t, "provider-model", gjson.GetBytes(processed.Body, "model").String())
	require.False(t, gjson.GetBytes(processed.Body, "reasoning_effort").Exists())
	require.Equal(t, int64(5), gjson.GetBytes(processed.Body, "top_k").Int())
}

// TestAnthropicPassThroughBodyAppliesReasoningEffortMapping verifies the Anthropic
// Messages pass-through middleware rewrites output_config.effort.
func TestAnthropicPassThroughBodyAppliesReasoningEffortMapping(t *testing.T) {
	channel := &biz.Channel{Channel: &ent.Channel{
		ID:   1,
		Name: "anthropic-reasoning-effort-mapping",
		Settings: &objects.ChannelSettings{
			PassThroughBody: lo.ToPtr(true),
			TransformOptions: objects.TransformOptions{
				ReasoningEffortMapping: []llm.ReasoningEffortMapping{{From: llm.ReasoningEffortNone, To: llm.ReasoningEffortLow}},
			},
		},
	}}
	outbound := &PersistentOutboundTransformer{state: &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
		LlmRequest: &llm.Request{
			Model:     "provider-model",
			APIFormat: llm.APIFormatAnthropicMessage,
			RawRequest: &httpclient.Request{
				APIFormat: string(llm.APIFormatAnthropicMessage),
				Body:      []byte(`{"model":"client-alias","max_tokens":16,"output_config":{"effort":"none"},"thinking":{"type":"between_tools"},"test_canary":true}`),
			},
		},
	}}
	transformed := &httpclient.Request{
		APIFormat: string(llm.APIFormatAnthropicMessage),
		Body:      []byte(`{"model":"provider-model","max_tokens":16}`),
	}

	processed, err := applyPassThroughRequestBody(outbound, nil).OnOutboundRawRequest(context.Background(), transformed)
	require.NoError(t, err)
	require.True(t, outbound.state.PassThroughApplied)
	require.Equal(t, "provider-model", gjson.GetBytes(processed.Body, "model").String())
	require.Equal(t, llm.ReasoningEffortLow, gjson.GetBytes(processed.Body, "output_config.effort").String())
	require.True(t, gjson.GetBytes(processed.Body, "test_canary").Bool())
}
