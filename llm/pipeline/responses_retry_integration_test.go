package pipeline_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	llmpipeline "github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
	openairesponses "github.com/looplj/axonhub/llm/transformer/openai/responses"
)

type retryTestInbound struct{ transformer.Inbound }

func (*retryTestInbound) TransformRequest(context.Context, *httpclient.Request) (*llm.Request, error) {
	stream := true
	return &llm.Request{Stream: &stream}, nil
}
func (*retryTestInbound) TransformStream(_ context.Context, stream streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.Map(stream, func(resp *llm.Response) *httpclient.StreamEvent {
		if resp == llm.DoneResponse || (resp != nil && resp.Object == "[DONE]") {
			return &httpclient.StreamEvent{Data: []byte("[DONE]")}
		}
		for _, choice := range resp.Choices {
			if choice.Delta == nil {
				continue
			}
			if choice.Delta.Content.Content != nil {
				return &httpclient.StreamEvent{Data: []byte(*choice.Delta.Content.Content)}
			}
			if choice.Delta.ReasoningContent != nil {
				return &httpclient.StreamEvent{Data: []byte(*choice.Delta.ReasoningContent)}
			}
			if len(choice.Delta.ToolCalls) > 0 {
				return &httpclient.StreamEvent{Data: []byte("tool")}
			}
		}
		if resp.ID != "" {
			return &httpclient.StreamEvent{Data: []byte(resp.ID)}
		}
		return &httpclient.StreamEvent{Data: []byte("metadata")}
	}), nil
}

type retryTestOutbound struct {
	transformer.Outbound
	real         transformer.Outbound
	prepareCalls *int
}

func (o *retryTestOutbound) TransformRequest(ctx context.Context, req *llm.Request) (*httpclient.Request, error) {
	return o.real.TransformRequest(ctx, req)
}
func (o *retryTestOutbound) TransformStream(ctx context.Context, req *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	return o.real.TransformStream(ctx, req, stream)
}
func (*retryTestOutbound) CanRetry(err error) bool { return errors.Is(err, llm.ErrStreamIncomplete) }
func (o *retryTestOutbound) PrepareForRetry(context.Context) error {
	*o.prepareCalls++
	return nil
}

type retryTestExecutor struct {
	attempts *int
	streams  [][]*httpclient.StreamEvent
}

func (*retryTestExecutor) Do(context.Context, *httpclient.Request) (*httpclient.Response, error) {
	return nil, errors.New("unexpected non-stream request")
}
func (e *retryTestExecutor) DoStream(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
	index := *e.attempts
	*e.attempts++
	return streams.SliceStream(e.streams[index]), nil
}

func TestResponsesIncompleteStreamRetriesBeforeCommit(t *testing.T) {
	realResponses, err := openairesponses.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	attempts, prepareCalls := 0, 0
	executor := &retryTestExecutor{
		attempts: &attempts,
		streams: [][]*httpclient.StreamEvent{
			{{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_first","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)}},
			{
				{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_second","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
				{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_second","output_index":0,"content_index":0,"delta":"ok"}`)},
				{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_second","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
			},
		},
	}
	outbound := &retryTestOutbound{real: realResponses, prepareCalls: &prepareCalls}
	p := llmpipeline.NewFactory(executor).Pipeline(&retryTestInbound{}, outbound, llmpipeline.WithRetry(0, 1, 0))

	res, err := p.Process(context.Background(), &httpclient.Request{})
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, 1, prepareCalls)
	events, err := streams.All(res.EventStream)
	require.NoError(t, err)
	var payloads []string
	for _, event := range events {
		payloads = append(payloads, string(event.Data))
	}
	require.NotContains(t, payloads, "resp_first", "failed-attempt metadata must not leak")
	require.Equal(t, 1, countPayload(payloads, "ok"))
	require.Equal(t, 1, countPayload(payloads, "[DONE]"))
}

func TestResponsesIncompleteStreamRetriesAfterManyMetadataEvents(t *testing.T) {
	realResponses, err := openairesponses.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	firstAttempt := make([]*httpclient.StreamEvent, 0, 17)
	for i := 0; i < 17; i++ {
		firstAttempt = append(firstAttempt, &httpclient.StreamEvent{
			Type: "response.created",
			Data: []byte(`{"type":"response.created","response":{"id":"resp_first","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`),
		})
	}

	attempts, prepareCalls := 0, 0
	executor := &retryTestExecutor{
		attempts: &attempts,
		streams: [][]*httpclient.StreamEvent{
			firstAttempt,
			{
				{Type: "response.created", Data: []byte(`{"type":"response.created","response":{"id":"resp_second","object":"response","created_at":1700000000,"model":"gpt-5","status":"in_progress","output":[]}}`)},
				{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_second","output_index":0,"content_index":0,"delta":"ok"}`)},
				{Type: "response.completed", Data: []byte(`{"type":"response.completed","response":{"id":"resp_second","object":"response","created_at":1700000000,"model":"gpt-5","status":"completed","output":[]}}`)},
			},
		},
	}
	outbound := &retryTestOutbound{real: realResponses, prepareCalls: &prepareCalls}
	p := llmpipeline.NewFactory(executor).Pipeline(&retryTestInbound{}, outbound, llmpipeline.WithRetry(0, 1, 0))

	res, err := p.Process(context.Background(), &httpclient.Request{})
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, 1, prepareCalls)
	events, err := streams.All(res.EventStream)
	require.NoError(t, err)
	var payloads []string
	for _, event := range events {
		payloads = append(payloads, string(event.Data))
	}
	require.NotContains(t, payloads, "resp_first", "failed-attempt metadata must not leak")
	require.Equal(t, 1, countPayload(payloads, "ok"))
	require.Equal(t, 1, countPayload(payloads, "[DONE]"))
}

func countPayload(payloads []string, wanted string) int {
	count := 0
	for _, payload := range payloads {
		if payload == wanted {
			count++
		}
	}
	return count
}

func TestResponsesIncompleteStreamDoesNotRetryAfterMeaningfulOutput(t *testing.T) {
	realResponses, err := openairesponses.NewOutboundTransformer("https://api.openai.com", "test-api-key")
	require.NoError(t, err)

	tests := []struct {
		name   string
		events []*httpclient.StreamEvent
	}{
		{name: "text", events: []*httpclient.StreamEvent{
			{Type: "response.output_text.delta", Data: []byte(`{"type":"response.output_text.delta","item_id":"msg_partial","output_index":0,"content_index":0,"delta":"partial"}`)},
		}},
		{name: "reasoning", events: []*httpclient.StreamEvent{
			{Type: "response.reasoning_summary_text.delta", Data: []byte(`{"type":"response.reasoning_summary_text.delta","item_id":"rs_partial","output_index":0,"summary_index":0,"delta":"partial reasoning"}`)},
		}},
		{name: "function call", events: []*httpclient.StreamEvent{
			{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_partial","type":"function_call","call_id":"call_partial","name":"write_file","arguments":""}}`)},
			{Type: "response.function_call_arguments.delta", Data: []byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_partial","output_index":0,"delta":"{\"path\":\"/tmp/x\"}"}`)},
		}},
		{name: "custom tool call", events: []*httpclient.StreamEvent{
			{Type: "response.output_item.added", Data: []byte(`{"type":"response.output_item.added","output_index":0,"item":{"id":"ct_partial","type":"custom_tool_call","call_id":"custom_partial","name":"shell","input":""}}`)},
			{Type: "response.custom_tool_call_input.delta", Data: []byte(`{"type":"response.custom_tool_call_input.delta","item_id":"ct_partial","output_index":0,"delta":"pwd"}`)},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts, prepareCalls := 0, 0
			executor := &retryTestExecutor{attempts: &attempts, streams: [][]*httpclient.StreamEvent{tt.events}}
			outbound := &retryTestOutbound{real: realResponses, prepareCalls: &prepareCalls}
			p := llmpipeline.NewFactory(executor).Pipeline(&retryTestInbound{}, outbound, llmpipeline.WithRetry(0, 1, 0))

			res, err := p.Process(context.Background(), &httpclient.Request{})
			require.NoError(t, err)
			require.Equal(t, 1, attempts)
			require.Equal(t, 0, prepareCalls)
			events, streamErr := streams.All(res.EventStream)
			require.ErrorIs(t, streamErr, llm.ErrStreamIncomplete)
			require.NotEmpty(t, events)
		})
	}
}
