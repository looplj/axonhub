package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func createTestChannelForConnTracking(id int, name string) *biz.Channel {
	entChannel := &ent.Channel{
		ID:   id,
		Name: name,
	}
	return &biz.Channel{
		Channel: entChannel,
	}
}

func TestConnectionTracking_OnOutboundRawRequest(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
		},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	for range 3 {
		_, err := middleware.OnOutboundRawRequest(ctx, nil)
		assert.NoError(t, err)
	}

	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))
}

func TestConnectionTracking_OnOutboundRawRequest_NoChannel(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	state := &PersistenceState{CurrentCandidate: nil}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
}

func TestConnectionTracking_OnOutboundLlmResponse(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	result, err := middleware.OnOutboundLlmResponse(ctx, &llm.Response{ID: "test"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))
}

func TestConnectionTracking_OnOutboundRawError(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	middleware.OnOutboundRawError(ctx, assert.AnError)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))
}

func TestConnectionTracking_OnOutboundLlmStream(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	events := []*llm.Response{{ID: "1"}, {ID: "2"}}
	mockStream := streams.SliceStream(events)

	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, mockStream)
	assert.NoError(t, err)

	for wrappedStream.Next() {
		_ = wrappedStream.Current()
	}
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))

	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))
}

func TestConnectionTracking_StreamCloseIdempotent(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	mockStream := streams.SliceStream([]*llm.Response{{ID: "1"}})
	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, mockStream)
	assert.NoError(t, err)

	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))

	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))
}

func TestConnectionTracking_RetryDecrementsOldChannel(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel1 := createTestChannelForConnTracking(1, "channel-1")
	channel2 := createTestChannelForConnTracking(2, "channel-2")

	outbound := &PersistentOutboundTransformer{}

	middleware := &connectionTracking{outbound: outbound, tracker: tracker}

	ctx := context.Background()

	// First request with channel 1
	outbound.state = &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel1},
	}
	_, _ = middleware.OnOutboundRawRequest(ctx, nil)
	assert.Equal(t, 1, tracker.GetActiveConnections(1))
	assert.Equal(t, 0, tracker.GetActiveConnections(2))

	// Retry with channel 2 (same middleware instance, different channel)
	outbound.state = &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel2},
	}
	_, _ = middleware.OnOutboundRawRequest(ctx, nil)
	assert.Equal(t, 0, tracker.GetActiveConnections(1))
	assert.Equal(t, 1, tracker.GetActiveConnections(2))
}

func TestConnectionTracking_MultipleChannels(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel1 := createTestChannelForConnTracking(1, "channel-1")
	channel2 := createTestChannelForConnTracking(2, "channel-2")

	state1 := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel1},
	}
	state2 := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel2},
	}
	outbound1 := &PersistentOutboundTransformer{state: state1}
	outbound2 := &PersistentOutboundTransformer{state: state2}

	middleware1 := &connectionTracking{outbound: outbound1, tracker: tracker}
	middleware2 := &connectionTracking{outbound: outbound2, tracker: tracker}

	ctx := context.Background()

	_, _ = middleware1.OnOutboundRawRequest(ctx, nil)
	_, _ = middleware2.OnOutboundRawRequest(ctx, nil)

	assert.Equal(t, 1, tracker.GetActiveConnections(1))
	assert.Equal(t, 1, tracker.GetActiveConnections(2))
}

func TestConnectionTracking_Lifecycle(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	_, err = middleware.OnOutboundLlmResponse(ctx, &llm.Response{ID: "test"})
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))

	_, err = middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	middleware.OnOutboundRawError(ctx, assert.AnError)
	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))
}

func TestWithConnectionTracking_NilTracker(t *testing.T) {
	outbound := &PersistentOutboundTransformer{}
	middleware := withConnectionTracking(outbound, nil)
	assert.Equal(t, "track-connections-noop", middleware.Name())
}

func TestWithConnectionTracking_NonNilTracker(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)
	outbound := &PersistentOutboundTransformer{}
	middleware := withConnectionTracking(outbound, tracker)
	assert.Equal(t, "track-connections", middleware.Name())
}

func TestConnectionTracking_OnOutboundLlmStream_ContextCancellation(t *testing.T) {
	tracker := NewDefaultConnectionTracker(100)

	channel := createTestChannelForConnTracking(1, "test-channel")
	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{Channel: channel},
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &connectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx, cancel := context.WithCancel(context.Background())

	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, tracker.GetActiveConnections(channel.ID))

	blockStream := &blockingStream{
		ready: make(chan struct{}),
	}

	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, blockStream)
	require.NoError(t, err)

	cancel()

	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, tracker.GetActiveConnections(channel.ID))

	_ = wrappedStream.Close()
}

// blockingStream is a mock stream that blocks until explicitly closed
type blockingStream struct {
	ready chan struct{}
}

func (b *blockingStream) Next() bool {
	<-b.ready
	return false
}

func (b *blockingStream) Current() *llm.Response {
	return nil
}

func (b *blockingStream) Err() error {
	return nil
}

func (b *blockingStream) Close() error {
	close(b.ready)
	return nil
}
