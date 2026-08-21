package biz

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/bailian"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestBailianResponsesChannel_RegistrationAndRequest(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())
	entity := client.Channel.Create().
		SetName("Bailian Responses").
		SetType(channel.TypeBailianResponses).
		SetBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-dashscope-key"}).
		SetSupportedModels([]string{"qwen3.7-max"}).
		SetDefaultTestModel("qwen3.7-max").
		SaveX(ctx)

	built, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(entity)
	require.NoError(t, err)
	require.IsType(t, &responses.OutboundTransformer{}, built.Outbound)
	require.Equal(t, []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIResponse.String()}}, built.ResolveEndpoints())

	outbound, err := BuildOutboundByAPIFormat(built, llm.APIFormatOpenAIResponse.String())
	require.NoError(t, err)
	require.Same(t, built.Outbound, outbound)

	request, err := outbound.TransformRequest(ctx, &llm.Request{
		Model:              "qwen3.7-max",
		Messages:           []llm.Message{{Role: "user", Content: llm.MessageContent{Content: lo.ToPtr("hello")}}},
		Stream:             lo.ToPtr(false),
		PreviousResponseID: lo.ToPtr("resp_previous"),
		ReasoningEffort:    "high",
		Tools: []llm.Tool{{
			Type: "function",
			Function: llm.Function{
				Name:       "get_weather",
				Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		}},
	})
	require.NoError(t, err)
	require.Equal(t, http.MethodPost, request.Method)
	require.Equal(t, "https://dashscope.aliyuncs.com/compatible-mode/v1/responses", request.URL)
	require.Equal(t, "application/json", request.Headers.Get("Content-Type"))
	require.Equal(t, "application/json", request.Headers.Get("Accept"))
	require.Equal(t, "bearer", request.Auth.Type)
	require.Equal(t, "test-dashscope-key", request.Auth.APIKey)
	require.Equal(t, llm.APIFormatOpenAIResponse.String(), request.APIFormat)

	var payload responses.Request
	require.NoError(t, json.Unmarshal(request.Body, &payload))
	require.Equal(t, "qwen3.7-max", payload.Model)
	require.NotNil(t, payload.Stream)
	require.False(t, *payload.Stream)
	require.Equal(t, "resp_previous", lo.FromPtr(payload.PreviousResponseID))
	require.NotNil(t, payload.Reasoning)
	require.Equal(t, "high", payload.Reasoning.Effort)
	require.Len(t, payload.Tools, 1)
	require.Equal(t, "function", payload.Tools[0].Type)
	require.Equal(t, "get_weather", payload.Tools[0].Name)
}

func TestBailianResponsesChannel_NonStreamingResponse(t *testing.T) {
	outbound, err := responses.NewOutboundTransformer(
		"https://dashscope.aliyuncs.com/compatible-mode/v1",
		"test-dashscope-key",
	)
	require.NoError(t, err)

	result, err := outbound.TransformResponse(t.Context(), &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: []byte(`{
			"id":"resp_bailian",
			"object":"response",
			"created_at":1700000000,
			"model":"qwen3.7-max",
			"status":"completed",
			"output":[{"id":"msg_bailian","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[]}]}],
			"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}
		}`),
	})
	require.NoError(t, err)
	require.Equal(t, "resp_bailian", result.ID)
	require.Equal(t, "qwen3.7-max", result.Model)
	require.EqualValues(t, 3, result.Usage.TotalTokens)
	require.Len(t, result.Choices, 1)
	require.Equal(t, "hello", lo.FromPtr(result.Choices[0].Message.Content.Content))
}

func TestBailianResponsesChannel_StreamingResponse(t *testing.T) {
	outbound, err := responses.NewOutboundTransformer(
		"https://dashscope.aliyuncs.com/compatible-mode/v1",
		"test-dashscope-key",
	)
	require.NoError(t, err)

	events := []*httpclient.StreamEvent{
		{
			Type: "response.created",
			Data: []byte(`{"type":"response.created","response":{"id":"resp_bailian","object":"response","created_at":1700000000,"model":"qwen3.7-max","status":"in_progress","output":[]}}`),
		},
		{
			Type: "response.output_text.delta",
			Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_bailian","output_index":0,"content_index":0,"delta":"hello"}`),
		},
		{
			Type: "response.completed",
			Data: []byte(`{"type":"response.completed","response":{"id":"resp_bailian","object":"response","created_at":1700000000,"model":"qwen3.7-max","status":"completed","output":[{"id":"msg_bailian","type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"hello","annotations":[]}]}],"usage":{"input_tokens":2,"output_tokens":1,"total_tokens":3,"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":0}}}}`),
		},
	}

	stream, err := outbound.TransformStream(t.Context(), nil, streams.SliceStream(events))
	require.NoError(t, err)
	chunks, err := streams.All(stream)
	require.NoError(t, err)
	require.NotEmpty(t, chunks)
	require.Equal(t, llm.DoneResponse, chunks[len(chunks)-1])
	require.True(t, lo.ContainsBy(chunks, func(chunk *llm.Response) bool {
		return chunk != nil && len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil &&
			lo.FromPtr(chunk.Choices[0].Delta.Content.Content) == "hello"
	}))
	require.True(t, lo.ContainsBy(chunks, func(chunk *llm.Response) bool {
		return chunk != nil && chunk.Usage != nil && chunk.Usage.TotalTokens == 3
	}))
}

func TestBailianChannel_ExistingProtocolsUnchanged(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = client.Close() })
	ctx := authz.WithTestBypass(context.Background())

	chatEntity := client.Channel.Create().
		SetName("Bailian Chat").
		SetType(channel.TypeBailian).
		SetBaseURL("https://dashscope.aliyuncs.com/compatible-mode/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"qwen3.7-max"}).
		SetDefaultTestModel("qwen3.7-max").
		SaveX(ctx)
	anthropicEntity := client.Channel.Create().
		SetName("Bailian Anthropic").
		SetType(channel.TypeBailianAnthropic).
		SetBaseURL("https://dashscope.aliyuncs.com/apps/anthropic").
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"qwen3.7-max"}).
		SetDefaultTestModel("qwen3.7-max").
		SaveX(ctx)

	chat, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(chatEntity)
	require.NoError(t, err)
	require.IsType(t, &bailian.OutboundTransformer{}, chat.Outbound)
	require.Equal(t, llm.APIFormatOpenAIChatCompletion, chat.Outbound.APIFormat())
	require.Equal(t, []objects.ChannelEndpoint{{APIFormat: llm.APIFormatOpenAIChatCompletion.String()}}, chat.ResolveEndpoints())

	anthropicChannel, err := NewChannelServiceForTest(client).buildChannelWithOutbounds(anthropicEntity)
	require.NoError(t, err)
	require.IsType(t, &anthropic.OutboundTransformer{}, anthropicChannel.Outbound)
	require.Equal(t, llm.APIFormatAnthropicMessage, anthropicChannel.Outbound.APIFormat())
	require.Equal(t, []objects.ChannelEndpoint{{APIFormat: llm.APIFormatAnthropicMessage.String()}}, anthropicChannel.ResolveEndpoints())
}
