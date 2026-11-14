package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

func TestPipeline_ModerationEmptyResponseDetection(t *testing.T) {
	inbound := openai.NewModerationInboundTransformer()
	outbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	executorCalls := 0
	executor := &mockExecutor{
		doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
			executorCalls++
			require.Equal(t, http.MethodPost, request.Method)
			require.Equal(t, "https://api.openai.com/v1/moderations", request.URL)
			require.Equal(t, string(llm.RequestTypeModeration), request.RequestType)
			require.Equal(t, string(llm.APIFormatOpenAIModeration), request.APIFormat)
			require.Nil(t, request.Auth)
			require.Equal(t, "Bearer test-api-key", request.Headers.Get("Authorization"))

			return &httpclient.Response{
				StatusCode: http.StatusOK,
				Headers: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Request: request,
				Body: []byte(`{
					"id":"modr-integration-test",
					"model":"omni-moderation-latest",
					"results":[{
						"flagged":false,
						"categories":{"hate":false,"sexual/minors":false},
						"category_scores":{"hate":0.01,"sexual/minors":0.001},
						"category_applied_input_types":{"hate":["text"],"sexual/minors":["text"]}
					}]
				}`),
			}, nil
		},
	}

	p := pipeline.NewFactory(executor).Pipeline(
		inbound,
		outbound,
		pipeline.WithEmptyResponseDetection(),
	)

	result, err := p.Process(context.Background(), &httpclient.Request{
		Method: http.MethodPost,
		URL:    "/v1/moderations",
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: []byte(`{"input":"这里是需要进行内容安全检测的文本","model":"omni-moderation-latest"}`),
	})
	require.NoError(t, err)
	require.Equal(t, 1, executorCalls)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)
	require.Equal(t, http.StatusOK, result.Response.StatusCode)
	require.Equal(t, "application/json", result.Response.Headers.Get("Content-Type"))

	var response struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Results []struct {
			Flagged                   *bool               `json:"flagged"`
			Categories                map[string]bool     `json:"categories"`
			CategoryScores            map[string]float64  `json:"category_scores"`
			CategoryAppliedInputTypes map[string][]string `json:"category_applied_input_types"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(result.Response.Body, &response))
	require.Equal(t, "modr-integration-test", response.ID)
	require.Equal(t, "omni-moderation-latest", response.Model)
	require.Len(t, response.Results, 1)
	require.NotNil(t, response.Results[0].Flagged)
	require.False(t, *response.Results[0].Flagged)
	require.Contains(t, response.Results[0].Categories, "sexual/minors")
	require.False(t, response.Results[0].Categories["sexual/minors"])
	require.Equal(t, 0.001, response.Results[0].CategoryScores["sexual/minors"])
	require.Equal(t, []string{"text"}, response.Results[0].CategoryAppliedInputTypes["sexual/minors"])
}

type countingRetryOutbound struct {
	transformer.Outbound

	channelSwitches atomic.Int32
}

func (o *countingRetryOutbound) HasMoreChannels() bool {
	return true
}

func (o *countingRetryOutbound) NextChannel(context.Context) error {
	o.channelSwitches.Add(1)
	return nil
}

func (o *countingRetryOutbound) CanRetry(error) bool {
	return false
}

func chatCompletionRequest(t *testing.T, stream bool) *httpclient.Request {
	t.Helper()

	body := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]any{{"role": "user", "content": "hello"}},
	}
	if stream {
		body["stream"] = true
	}

	data, err := json.Marshal(body)
	require.NoError(t, err)

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "/v1/chat/completions",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    data,
	}
}

func emptyFinishResponse(reason string) *httpclient.Response {
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"id":"chatcmpl-mod","object":"chat.completion","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant"},"finish_reason":"` + reason + `"}]}`),
	}
}

func moderationFinishStream(reason string) []*httpclient.StreamEvent {
	return []*httpclient.StreamEvent{
		{Data: []byte(`{"id":"chatcmpl-mod","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`)},
		{Data: []byte(`{"id":"chatcmpl-mod","object":"chat.completion.chunk","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"` + reason + `"}]}`)},
		{Data: []byte(`[DONE]`)},
	}
}

func finishReasonFromBody(t *testing.T, body []byte) string {
	t.Helper()

	var response struct {
		Choices []struct {
			FinishReason *string `json:"finish_reason"`
		} `json:"choices"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	require.Len(t, response.Choices, 1)

	if response.Choices[0].FinishReason == nil {
		return ""
	}

	return *response.Choices[0].FinishReason
}

func TestPipeline_ModerationTerminal_NonStreaming(t *testing.T) {
	tests := []struct {
		name        string
		reason      string
		wantSuccess bool
		wantFinish  string
	}{
		{name: "content_filter terminal", reason: "content_filter", wantSuccess: true, wantFinish: "content_filter"},
		{name: "sensitive normalized", reason: "sensitive", wantSuccess: true, wantFinish: "content_filter"},
		{name: "unknown reason stays empty", reason: "safety", wantSuccess: false},
		{name: "ordinary stop stays empty", reason: "stop", wantSuccess: false},
		{name: "length stays empty", reason: "length", wantSuccess: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := openai.NewInboundTransformer()

			baseOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)

			outbound := &countingRetryOutbound{Outbound: baseOutbound}

			var calls atomic.Int32

			executor := &mockExecutor{
				doFunc: func(ctx context.Context, request *httpclient.Request) (*httpclient.Response, error) {
					calls.Add(1)
					return emptyFinishResponse(tt.reason), nil
				},
				doStreamFunc: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
					t.Fatal("moderation terminal should not switch to streaming")
					return nil, nil
				},
			}

			p := pipeline.NewFactory(executor).Pipeline(
				inbound,
				outbound,
				pipeline.WithEmptyResponseDetection(),
				pipeline.WithRetry(1, 0, 0),
			)

			result, err := p.Process(context.Background(), chatCompletionRequest(t, false))
			if !tt.wantSuccess {
				require.ErrorIs(t, err, pipeline.ErrEmptyResponse)
				require.Equal(t, int32(2), calls.Load(), "ordinary empty responses must still be retried")
				require.Equal(t, int32(1), outbound.channelSwitches.Load())

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, int32(1), calls.Load(), "a moderation terminal must not trigger another upstream call")
			require.Equal(t, int32(0), outbound.channelSwitches.Load(), "a moderation terminal must not switch channels")
			require.Equal(t, tt.wantFinish, finishReasonFromBody(t, result.Response.Body))
		})
	}
}

func TestPipeline_ModerationTerminal_Streaming(t *testing.T) {
	for _, tt := range []struct {
		name   string
		reason string
	}{
		{name: "content_filter", reason: "content_filter"},
		{name: "sensitive normalized", reason: "sensitive"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			inbound := openai.NewInboundTransformer()

			baseOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)

			outbound := &countingRetryOutbound{Outbound: baseOutbound}

			var calls atomic.Int32

			executor := &mockExecutor{
				doStreamFunc: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
					calls.Add(1)
					return streams.SliceStream(moderationFinishStream(tt.reason)), nil
				},
				doFunc: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
					t.Fatal("streaming moderation terminal should not fall back to non-streaming")
					return nil, nil
				},
			}

			p := pipeline.NewFactory(executor).Pipeline(
				inbound,
				outbound,
				pipeline.WithEmptyResponseDetection(),
				pipeline.WithRetry(2, 0, 0),
			)

			result, err := p.Process(context.Background(), chatCompletionRequest(t, true))
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.Stream)
			require.Equal(t, int32(1), calls.Load())
			require.Equal(t, int32(0), outbound.channelSwitches.Load())

			events, err := streams.All(result.EventStream)
			require.NoError(t, err)
			require.NotEmpty(t, events)

			foundFinish := false
			for _, event := range events {
				if event != nil && strings.Contains(string(event.Data), `"finish_reason":"content_filter"`) {
					foundFinish = true
				}
			}
			require.True(t, foundFinish, "stream must preserve the content_filter terminal")
		})
	}
}

func TestPipeline_ModerationTerminal_AutoAggregated(t *testing.T) {
	inbound := openai.NewInboundTransformer()

	baseOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	outbound := &countingRetryOutbound{Outbound: &streamUpgradeOutboundWrapper{Outbound: baseOutbound}}

	var calls atomic.Int32

	executor := &mockExecutor{
		doStreamFunc: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
			calls.Add(1)
			return streams.SliceStream(moderationFinishStream("content_filter")), nil
		},
	}

	p := pipeline.NewFactory(executor).Pipeline(
		inbound,
		outbound,
		pipeline.WithEmptyResponseDetection(),
		pipeline.WithRetry(2, 0, 0),
	)

	result, err := p.Process(context.Background(), chatCompletionRequest(t, false))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.Stream)
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, int32(0), outbound.channelSwitches.Load())
	require.Equal(t, "content_filter", finishReasonFromBody(t, result.Response.Body))
}

func TestPipeline_NonModerationContentStillCompletes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "text",
			body: `{"id":"chatcmpl-text","object":"chat.completion","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}]}`,
		},
		{
			name: "tool call",
			body: `{"id":"chatcmpl-tool","object":"chat.completion","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"SF\"}"}}]},"finish_reason":"tool_calls"}]}`,
		},
		{
			name: "refusal",
			body: `{"id":"chatcmpl-refusal","object":"chat.completion","created":1720000000,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","refusal":"I cannot help with that."},"finish_reason":"stop"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inbound := openai.NewInboundTransformer()

			baseOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)

			outbound := &countingRetryOutbound{Outbound: baseOutbound}

			var calls atomic.Int32

			executor := &mockExecutor{
				doFunc: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
					calls.Add(1)
					return &httpclient.Response{
						StatusCode: http.StatusOK,
						Headers:    http.Header{"Content-Type": []string{"application/json"}},
						Body:       []byte(tt.body),
					}, nil
				},
			}

			p := pipeline.NewFactory(executor).Pipeline(
				inbound,
				outbound,
				pipeline.WithEmptyResponseDetection(),
				pipeline.WithRetry(2, 0, 0),
			)

			result, err := p.Process(context.Background(), chatCompletionRequest(t, false))
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, int32(1), calls.Load())
			require.Equal(t, int32(0), outbound.channelSwitches.Load(), "normal content must not retry")
		})
	}
}

func TestPipeline_RealErrorStillRetries(t *testing.T) {
	inbound := openai.NewInboundTransformer()

	baseOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	outbound := &countingRetryOutbound{Outbound: baseOutbound}

	var calls atomic.Int32

	executor := &mockExecutor{
		doFunc: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
			calls.Add(1)
			return nil, errors.New("upstream unavailable")
		},
	}

	p := pipeline.NewFactory(executor).Pipeline(
		inbound,
		outbound,
		pipeline.WithEmptyResponseDetection(),
		pipeline.WithRetry(1, 0, 0),
	)

	_, err = p.Process(context.Background(), chatCompletionRequest(t, false))
	require.ErrorContains(t, err, "upstream unavailable")
	require.NotErrorIs(t, err, pipeline.ErrEmptyResponse)
	require.Equal(t, int32(2), calls.Load(), "real errors keep the existing retry path")
	require.Equal(t, int32(1), outbound.channelSwitches.Load())
}

func anthropicStreamRequest(t *testing.T) *httpclient.Request {
	t.Helper()

	return &httpclient.Request{
		Method:  http.MethodPost,
		URL:     "/v1/messages",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"model":"synthetic","messages":[{"role":"user","content":"hi"}],"max_tokens":64,"stream":true}`),
	}
}

func TestPipeline_ModerationTerminal_AnthropicStreamBlocksBalanced(t *testing.T) {
	for _, reason := range []string{"content_filter", "sensitive"} {
		t.Run(reason, func(t *testing.T) {
			inbound := anthropic.NewInboundTransformer()

			baseOutbound, err := openai.NewOutboundTransformer("https://api.openai.com", "test-api-key")
			require.NoError(t, err)

			outbound := &countingRetryOutbound{Outbound: baseOutbound}

			executor := &mockExecutor{
				doStreamFunc: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
					return streams.SliceStream(moderationFinishStream(reason)), nil
				},
			}

			p := pipeline.NewFactory(executor).Pipeline(
				inbound,
				outbound,
				pipeline.WithEmptyResponseDetection(),
				pipeline.WithRetry(2, 0, 0),
			)

			result, err := p.Process(context.Background(), anthropicStreamRequest(t))
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.Stream)

			events, err := streams.All(result.EventStream)
			require.NoError(t, err)
			require.NotEmpty(t, events)

			opened := map[int]bool{}
			var joined strings.Builder

			for _, event := range events {
				if event == nil {
					continue
				}

				joined.WriteString(string(event.Data))
				joined.WriteString("\n")

				var payload struct {
					Type  string `json:"type"`
					Index int    `json:"index"`
				}
				require.NoError(t, json.Unmarshal(event.Data, &payload))

				switch payload.Type {
				case "content_block_start":
					opened[payload.Index] = true
				case "content_block_stop":
					require.True(t, opened[payload.Index],
						"content_block_stop index=%d must follow a content_block_start", payload.Index)
					delete(opened, payload.Index)
				}
			}

			require.Empty(t, opened)
			require.Contains(t, joined.String(), `"stop_reason":"refusal"`)
		})
	}
}
