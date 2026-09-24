package pipeline

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

type metadataInbound struct {
	transformer.Inbound

	request    *llm.Request
	received   []*llm.Response
	aggregated bool
}

func (t *metadataInbound) TransformRequest(context.Context, *httpclient.Request) (*llm.Request, error) {
	return t.request, nil
}

func (t *metadataInbound) TransformResponse(_ context.Context, response *llm.Response) (*httpclient.Response, error) {
	t.received = append(t.received, response)
	return &httpclient.Response{Body: []byte(`{}`)}, nil
}

func (t *metadataInbound) TransformStream(_ context.Context, source streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
	return streams.MapErr(source, func(response *llm.Response) (*httpclient.StreamEvent, error) {
		if response != nil && response != llm.DoneResponse {
			t.received = append(t.received, response)
		}
		return &httpclient.StreamEvent{Data: []byte(`{}`)}, nil
	}), nil
}

func (t *metadataInbound) AggregateStreamChunks(context.Context, []*httpclient.StreamEvent) ([]byte, llm.ResponseMeta, error) {
	t.aggregated = true
	return []byte(`{}`), llm.ResponseMeta{}, nil
}

type metadataOutbound struct {
	transformer.Outbound

	request     *httpclient.Request
	response    *llm.Response
	forceStream bool
}

func (t *metadataOutbound) TransformRequest(_ context.Context, request *llm.Request) (*httpclient.Request, error) {
	if t.forceStream {
		request.Stream = lo.ToPtr(true)
	}
	return t.request, nil
}

func (t *metadataOutbound) TransformResponse(context.Context, *httpclient.Response) (*llm.Response, error) {
	return t.response, nil
}

func (t *metadataOutbound) TransformStream(_ context.Context, _ *httpclient.Request, source streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
	_ = source.Close()
	return streams.SliceStream([]*llm.Response{t.response, t.response, llm.DoneResponse}), nil
}

func TestPipelineTransformerMetadata(t *testing.T) {
	for _, mode := range []string{"response", "stream", "aggregate"} {
		t.Run(mode, func(t *testing.T) {
			requestMetadata := map[string]any{"request": true, "shared": "request"}
			httpMetadata := map[string]any{"http": true, "shared": "http"}
			responseMetadata := map[string]any{"response": true, "shared": "response"}
			inbound := &metadataInbound{request: &llm.Request{Stream: lo.ToPtr(mode == "stream"), TransformerMetadata: requestMetadata}}
			outbound := &metadataOutbound{
				request:     &httpclient.Request{URL: "https://example.com", TransformerMetadata: httpMetadata},
				response:    &llm.Response{TransformerMetadata: responseMetadata},
				forceStream: mode == "aggregate",
			}
			result, err := NewFactory(&testExecutor{}).Pipeline(inbound, outbound).Process(t.Context(), &httpclient.Request{})
			require.NoError(t, err)
			if mode == "stream" {
				defer result.EventStream.Close()
				for result.EventStream.Next() {
					_ = result.EventStream.Current()
				}
				require.NoError(t, result.EventStream.Err())
			}
			require.Equal(t, map[string]any{"request": true, "http": true, "shared": "http"}, outbound.request.TransformerMetadata)
			count := 1
			if mode != "response" {
				count = 2
			}
			require.Len(t, inbound.received, count)
			for _, response := range inbound.received {
				require.NotSame(t, outbound.response, response)
				require.Equal(t, map[string]any{"request": true, "http": true, "response": true, "shared": "response"}, response.TransformerMetadata)
			}
			inbound.received[0].TransformerMetadata["shared"] = "modified"
			if count == 2 {
				require.Equal(t, "response", inbound.received[1].TransformerMetadata["shared"])
			}
			require.Equal(t, "request", requestMetadata["shared"])
			require.Equal(t, "http", httpMetadata["shared"])
			require.Equal(t, "response", responseMetadata["shared"])
			require.NotContains(t, httpMetadata, "request")
			require.NotContains(t, responseMetadata, "request")
			require.Equal(t, mode == "aggregate", inbound.aggregated)
		})
	}
}

func TestTransformerMetadataPreservesSentinels(t *testing.T) {
	metadata := map[string]any{"request": true}
	require.Nil(t, withTransformerMetadata(metadata, nil))
	require.Same(t, llm.DoneResponse, withTransformerMetadata(metadata, llm.DoneResponse))
	response := &llm.Response{}
	require.Same(t, response, withTransformerMetadata(nil, response))
}
