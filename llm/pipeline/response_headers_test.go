package pipeline

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

type streamWithResponseHeaders struct {
	streams.Stream[*httpclient.StreamEvent]
	headers http.Header
}

func (s *streamWithResponseHeaders) UpstreamResponseHeaders() http.Header {
	return s.headers
}

type errorStreamWithResponseHeaders struct {
	err     error
	headers http.Header
}

func (s *errorStreamWithResponseHeaders) Next() bool                       { return false }
func (s *errorStreamWithResponseHeaders) Current() *httpclient.StreamEvent { return nil }
func (s *errorStreamWithResponseHeaders) Err() error                       { return s.err }
func (s *errorStreamWithResponseHeaders) Close() error                     { return nil }
func (s *errorStreamWithResponseHeaders) UpstreamResponseHeaders() http.Header {
	return s.headers
}

func TestPipelineProcess_ForwardsAllowedNonStreamingResponseHeaders(t *testing.T) {
	p := &pipeline{
		Executor: &mockExecutor{
			do: func(context.Context, *httpclient.Request) (*httpclient.Response, error) {
				return &httpclient.Response{
					Headers: http.Header{
						httpclient.ReasoningIncludedHeader: []string{"true"},
						"Set-Cookie":                       []string{"secret=1"},
					},
				}, nil
			},
		},
		Inbound:  &mockInbound{},
		Outbound: &mockOutbound{},
	}

	result, err := p.Process(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	require.Equal(t, "true", result.ResponseHeaders.Get(httpclient.ReasoningIncludedHeader))
	require.Empty(t, result.ResponseHeaders.Get("Set-Cookie"))
}

func TestPipelineProcess_ForwardsAllowedStreamingResponseHeaders(t *testing.T) {
	content := "ok"
	p := &pipeline{
		Executor: &mockExecutor{
			doStream: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
				return &streamWithResponseHeaders{
					Stream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("chunk")}}),
					headers: http.Header{
						httpclient.ReasoningIncludedHeader: []string{"true"},
						"Set-Cookie":                       []string{"secret=1"},
					},
				}, nil
			},
		},
		Inbound: &mockInbound{
			transformRequest: func(context.Context, *httpclient.Request) (*llm.Request, error) {
				stream := true
				return &llm.Request{Stream: &stream}, nil
			},
		},
		Outbound: &mockOutbound{
			transformStream: func(_ context.Context, _ *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
				return streams.Map(stream, func(*httpclient.StreamEvent) *llm.Response {
					return &llm.Response{Choices: []llm.Choice{{
						Delta: &llm.Message{Content: llm.MessageContent{Content: &content}},
					}}}
				}), nil
			},
		},
	}

	result, err := p.Process(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	require.Equal(t, "true", result.ResponseHeaders.Get(httpclient.ReasoningIncludedHeader))
	require.Empty(t, result.ResponseHeaders.Get("Set-Cookie"))
	require.NoError(t, result.EventStream.Close())
}

func TestPipelineProcess_ForwardsAllowedHeadersWhenAggregatingUpgradedStream(t *testing.T) {
	content := "ok"
	p := &pipeline{
		Executor: &mockExecutor{
			doStream: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
				return &streamWithResponseHeaders{
					Stream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("chunk")}}),
					headers: http.Header{
						httpclient.ReasoningIncludedHeader: []string{"true"},
					},
				}, nil
			},
		},
		Inbound: &mockInbound{},
		Outbound: &mockOutbound{
			transformRequest: func(_ context.Context, request *llm.Request) (*httpclient.Request, error) {
				stream := true
				request.Stream = &stream

				return &httpclient.Request{}, nil
			},
			transformStream: func(_ context.Context, _ *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
				return streams.Map(stream, func(*httpclient.StreamEvent) *llm.Response {
					return &llm.Response{Choices: []llm.Choice{{
						Delta: &llm.Message{Content: llm.MessageContent{Content: &content}},
					}}}
				}), nil
			},
		},
	}

	result, err := p.Process(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	require.False(t, result.Stream)
	require.NotNil(t, result.Response)
	require.Equal(t, "true", result.ResponseHeaders.Get(httpclient.ReasoningIncludedHeader))
}

func TestPipelineProcess_UsesOnlyFinalSuccessfulAttemptResponseHeaders(t *testing.T) {
	content := "ok"
	attempts := 0
	p := &pipeline{
		Executor: &mockExecutor{
			doStream: func(context.Context, *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
				attempts++
				if attempts == 1 {
					return &errorStreamWithResponseHeaders{
						err: errors.New("first channel failed before content"),
						headers: http.Header{
							httpclient.ReasoningIncludedHeader: []string{"true"},
						},
					}, nil
				}

				return &streamWithResponseHeaders{
					Stream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte("chunk")}}),
					headers: http.Header{
						httpclient.ReasoningIncludedHeader: []string{"false"},
					},
				}, nil
			},
		},
		Inbound: &mockInbound{
			transformRequest: func(context.Context, *httpclient.Request) (*llm.Request, error) {
				stream := true
				return &llm.Request{Stream: &stream}, nil
			},
		},
		Outbound: &mockOutbound{
			transformStream: func(_ context.Context, _ *httpclient.Request, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
				return streams.Map(stream, func(*httpclient.StreamEvent) *llm.Response {
					return &llm.Response{Choices: []llm.Choice{{
						Delta: &llm.Message{Content: llm.MessageContent{Content: &content}},
					}}}
				}), nil
			},
			hasMoreChannels: func() bool { return true },
			nextChannel:     func(context.Context) error { return nil },
		},
		maxChannelRetries: 1,
	}

	result, err := p.Process(t.Context(), &httpclient.Request{})
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Empty(t, result.ResponseHeaders.Get(httpclient.ReasoningIncludedHeader))
	require.NoError(t, result.EventStream.Close())
}
