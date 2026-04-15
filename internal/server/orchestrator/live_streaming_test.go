package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/pkg/chunkbuffer"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
)

func TestLivePreviewMiddleware_OnInboundLlmRequest_DisablesPreviewForNonStreamingRequests(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	streaming := true
	nonStreaming := false

	t.Run("non-streaming request disables preview", func(t *testing.T) {
		t.Parallel()

		middleware := &livePreviewMiddleware{
			enabled:     true,
			initialized: true,
		}

		_, err := middleware.OnInboundLlmRequest(ctx, &llm.Request{Stream: &nonStreaming})
		require.NoError(t, err)
		require.False(t, middleware.enabled)
	})

	t.Run("streaming request keeps preview enabled", func(t *testing.T) {
		t.Parallel()

		middleware := &livePreviewMiddleware{
			liveStreamRegistry: biz.NewLiveStreamRegistry(),
			enabled:            true,
			initialized:        true,
		}

		_, err := middleware.OnInboundLlmRequest(ctx, &llm.Request{Stream: &streaming})
		require.NoError(t, err)
		require.True(t, middleware.enabled)
	})
}

func TestLivePreviewMiddleware_OnOutboundRawRequest_RegistersBuffersWhenEnabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := biz.NewLiveStreamRegistry()
	middleware := &livePreviewMiddleware{
		state: &PersistenceState{
			Request:     &ent.Request{ID: 33, Stream: true},
			RequestExec: &ent.RequestExecution{ID: 44, Stream: true},
		},
		liveStreamRegistry: registry,
		enabled:            true,
	}

	_, err := middleware.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, registry.GetRequestBuffer(33))
	require.NotNil(t, registry.GetExecutionBuffer(44))
}

func TestLivePreviewMiddleware_OnOutboundRawRequest_DoesNotRegisterBuffersWhenDisabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := biz.NewLiveStreamRegistry()
	middleware := &livePreviewMiddleware{
		state: &PersistenceState{
			Request:     &ent.Request{ID: 11, Stream: true},
			RequestExec: &ent.RequestExecution{ID: 22, Stream: true},
		},
		liveStreamRegistry: registry,
		enabled:            false,
	}

	_, err := middleware.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.Nil(t, registry.GetRequestBuffer(11))
	require.Nil(t, registry.GetExecutionBuffer(22))
}

func TestLivePreviewMiddleware_OnOutboundRawError_CleansRegisteredBuffers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := biz.NewLiveStreamRegistry()
	middleware := &livePreviewMiddleware{
		state: &PersistenceState{
			Request:     &ent.Request{ID: 33, Stream: true},
			RequestExec: &ent.RequestExecution{ID: 44, Stream: true},
		},
		liveStreamRegistry: registry,
		enabled:            true,
	}

	_, err := middleware.OnOutboundRawRequest(ctx, &httpclient.Request{})
	require.NoError(t, err)
	require.NotNil(t, registry.GetRequestBuffer(33))
	require.NotNil(t, registry.GetExecutionBuffer(44))

	middleware.OnOutboundRawError(ctx, assertAnError{})

	require.Nil(t, registry.GetRequestBuffer(33))
	require.Nil(t, registry.GetExecutionBuffer(44))
}

func TestLiveRequestStream_AppendsOncePerNext(t *testing.T) {
	t.Parallel()

	buffer := chunkbuffer.New()
	registry := biz.NewLiveStreamRegistry()
	stream := &liveRequestStream{
		stream: &mockStream{
			events: []*httpclient.StreamEvent{
				{Type: "message", Data: []byte(`{"index":1}`)},
			},
		},
		buffer:             buffer,
		liveStreamRegistry: registry,
		requestID:          1,
	}

	require.True(t, stream.Next())
	require.NotNil(t, stream.Current())
	require.NotNil(t, stream.Current())

	require.Equal(t, 1, buffer.Len())
}

type assertAnError struct{}

func (assertAnError) Error() string {
	return "boom"
}
