package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

func TestHasResponseContent(t *testing.T) {
	t.Run("empty response", func(t *testing.T) {
		require.False(t, hasResponseContent(&llm.Response{}))
	})

	t.Run("message text content", func(t *testing.T) {
		require.True(t, hasResponseContent(&llm.Response{
			Choices: []llm.Choice{{
				Message: &llm.Message{
					Content: llm.MessageContent{Content: lo.ToPtr("hello")},
				},
			}},
		}))
	})

	t.Run("tool calls", func(t *testing.T) {
		require.True(t, hasResponseContent(&llm.Response{
			Choices: []llm.Choice{{
				Message: &llm.Message{
					ToolCalls: []llm.ToolCall{{ID: "call_1"}},
				},
			}},
		}))
	})

	t.Run("reasoning content", func(t *testing.T) {
		require.True(t, hasResponseContent(&llm.Response{
			Choices: []llm.Choice{{
				Message: &llm.Message{
					ReasoningContent: lo.ToPtr("thinking"),
				},
			}},
		}))
	})

	t.Run("embedding response data", func(t *testing.T) {
		require.True(t, hasResponseContent(&llm.Response{
			Embedding: &llm.EmbeddingResponse{
				Object: "list",
				Data: []llm.EmbeddingData{{
					Object: "embedding",
					Embedding: llm.Embedding{
						Embedding: []float64{0.1, 0.2, 0.3},
					},
					Index: 0,
				}},
			},
		}))
	})
}

func TestPipeline_Process_StreamEmptyResponseDetection(t *testing.T) {
	ctx := context.Background()

	t.Run("retries on empty stream response", func(t *testing.T) {
		streamCalls := 0
		executor := &mockExecutor{
			doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
				streamCalls++
				// Return a minimal stream that the outbound TransformStream will convert
				return streams.SliceStream([]*httpclient.StreamEvent{{}}), nil
			},
		}

		prepareCalls := 0
		outbound := &mockOutbound{
			transformStream: func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
				if streamCalls == 1 {
					// First call: empty response (finish reason, no content)
					return streams.SliceStream([]*llm.Response{
						{Choices: []llm.Choice{{FinishReason: lo.ToPtr("stop"), Message: &llm.Message{}}}},
					}), nil
				}
				// Second call: response with content
				return streams.SliceStream([]*llm.Response{
					{Choices: []llm.Choice{{
						Message: &llm.Message{
							Content: llm.MessageContent{Content: lo.ToPtr("ok")},
						},
					}}},
					llm.DoneResponse,
				}), nil
			},
			canRetry: func(err error) bool { return errors.Is(err, ErrEmptyResponse) },
			prepareForRetry: func(ctx context.Context) error {
				prepareCalls++
				return nil
			},
		}

		streamFlag := true
		streamInbound := &mockInbound{
			transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
				return &llm.Request{Stream: &streamFlag}, nil
			},
		}

		p := &pipeline{
			Executor:               executor,
			Inbound:                streamInbound,
			Outbound:               outbound,
			maxSameChannelRetries:  1,
			emptyResponseDetection: true,
		}

		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.True(t, res.Stream)
		require.Equal(t, 2, streamCalls)
		require.Equal(t, 1, prepareCalls)
	})

	t.Run("accepts stream response with content", func(t *testing.T) {
		streamCalls := 0
		executor := &mockExecutor{
			doStream: func(ctx context.Context, req *httpclient.Request) (streams.Stream[*httpclient.StreamEvent], error) {
				streamCalls++
				return streams.SliceStream([]*httpclient.StreamEvent{{}}), nil
			},
		}

		outbound := &mockOutbound{
			transformStream: func(ctx context.Context, stream streams.Stream[*httpclient.StreamEvent]) (streams.Stream[*llm.Response], error) {
				return streams.SliceStream([]*llm.Response{
					{Choices: []llm.Choice{{
						Delta: &llm.Message{
							Content: llm.MessageContent{Content: lo.ToPtr("hello")},
						},
					}}},
					llm.DoneResponse,
				}), nil
			},
		}

		streamFlag := true
		streamInbound := &mockInbound{
			transformRequest: func(ctx context.Context, req *httpclient.Request) (*llm.Request, error) {
				return &llm.Request{Stream: &streamFlag}, nil
			},
		}

		p := &pipeline{
			Executor:               executor,
			Inbound:                streamInbound,
			Outbound:               outbound,
			emptyResponseDetection: true,
		}

		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.True(t, res.Stream)
		require.Equal(t, 1, streamCalls)
	})
}

func TestPipeline_Process_NonStreamEmptyResponseDetection(t *testing.T) {
	ctx := context.Background()

	t.Run("retries on empty non-stream response", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				return &httpclient.Response{}, nil
			},
		}

		prepareCalls := 0
		outbound := &mockOutbound{
			transformResponse: func(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
				if execCalls == 1 {
					return &llm.Response{
						Choices: []llm.Choice{{Message: &llm.Message{}}},
					}, nil
				}

				return &llm.Response{
					Choices: []llm.Choice{{
						Message: &llm.Message{
							Content: llm.MessageContent{Content: lo.ToPtr("ok")},
						},
					}},
				}, nil
			},
			canRetry: func(err error) bool { return errors.Is(err, ErrEmptyResponse) },
			prepareForRetry: func(ctx context.Context) error {
				prepareCalls++
				return nil
			},
		}

		p := &pipeline{
			Executor:               executor,
			Inbound:                &mockInbound{},
			Outbound:               outbound,
			maxSameChannelRetries:  1,
			emptyResponseDetection: true,
		}

		res, err := p.Process(ctx, &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, 2, execCalls)
		require.Equal(t, 1, prepareCalls)
	})

	t.Run("accepts non-stream tool-call response", func(t *testing.T) {
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				return &httpclient.Response{}, nil
			},
		}

		outbound := &mockOutbound{
			transformResponse: func(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
				return &llm.Response{
					Choices: []llm.Choice{{
						Message: &llm.Message{
							ToolCalls: []llm.ToolCall{{ID: "call_1"}},
						},
					}},
				}, nil
			},
		}

		p := &pipeline{
			Executor:               executor,
			Inbound:                &mockInbound{},
			Outbound:               outbound,
			emptyResponseDetection: true,
		}

		res, err := p.Process(context.Background(), &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("accepts non-stream embedding response", func(t *testing.T) {
		execCalls := 0
		executor := &mockExecutor{
			do: func(ctx context.Context, req *httpclient.Request) (*httpclient.Response, error) {
				execCalls++
				return &httpclient.Response{}, nil
			},
		}

		outbound := &mockOutbound{
			transformResponse: func(ctx context.Context, resp *httpclient.Response) (*llm.Response, error) {
				return &llm.Response{
					RequestType: llm.RequestTypeEmbedding,
					Embedding: &llm.EmbeddingResponse{
						Object: "list",
						Data: []llm.EmbeddingData{{
							Object: "embedding",
							Embedding: llm.Embedding{
								Embedding: []float64{0.1, 0.2, 0.3},
							},
							Index: 0,
						}},
					},
				}, nil
			},
		}

		p := &pipeline{
			Executor:               executor,
			Inbound:                &mockInbound{},
			Outbound:               outbound,
			emptyResponseDetection: true,
		}

		res, err := p.Process(context.Background(), &httpclient.Request{})
		require.NoError(t, err)
		require.NotNil(t, res)
		require.Equal(t, 1, execCalls)
	})
}
