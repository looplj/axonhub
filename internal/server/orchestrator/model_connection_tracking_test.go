package orchestrator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

func createTestChannelForModelTracking(id int, name string) *biz.Channel {
	entChannel := &ent.Channel{
		ID:   id,
		Name: name,
	}
	return &biz.Channel{
		Channel: entChannel,
	}
}

func TestModelConnectionTracking_OnOutboundRawRequest(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Increment model connection count multiple times
	for range 3 {
		_, err := middleware.OnOutboundRawRequest(ctx, nil)
		assert.NoError(t, err)
	}

	assert.Equal(t, 3, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
}

func TestModelConnectionTracking_OnOutboundRawRequest_NoChannel(t *testing.T) {
	tracker := NewModelConnectionTracker()

	state := &PersistenceState{
		CurrentCandidate: nil,
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Should not panic with no channel
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
}

func TestModelConnectionTracking_OnOutboundRawRequest_NoModel(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{}, // No models
		},
		CurrentModelIndex: 0,
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Should not panic with no model
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, ""))
}

func TestModelConnectionTracking_OnOutboundLlmResponse(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "claude-3"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "claude-3",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Use proper lifecycle: OnOutboundRawRequest increments, then OnOutboundLlmResponse decrements
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "claude-3"))

	// Decrement on response
	result, err := middleware.OnOutboundLlmResponse(ctx, &llm.Response{ID: "test"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "claude-3"))
}

func TestModelConnectionTracking_OnOutboundLlmResponse_NoChannel(t *testing.T) {
	tracker := NewModelConnectionTracker()

	state := &PersistenceState{
		CurrentCandidate: nil,
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Should not panic with no channel
	result, err := middleware.OnOutboundLlmResponse(ctx, &llm.Response{ID: "test"})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestModelConnectionTracking_OnOutboundRawError(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Use proper lifecycle: OnOutboundRawRequest increments
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// Decrement on error
	middleware.OnOutboundRawError(ctx, assert.AnError)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
}

func TestModelConnectionTracking_OnOutboundRawError_NoChannel(t *testing.T) {
	tracker := NewModelConnectionTracker()

	state := &PersistenceState{
		CurrentCandidate: nil,
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Should not panic with no channel
	middleware.OnOutboundRawError(ctx, assert.AnError)
}

func TestModelConnectionTracking_OnOutboundLlmStream(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	// Use proper lifecycle: OnOutboundRawRequest increments
	ctx := context.Background()
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// Create mock stream
	events := []*llm.Response{
		{ID: "1"},
		{ID: "2"},
	}
	mockStream := streams.SliceStream(events)

	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, mockStream)
	assert.NoError(t, err)

	for wrappedStream.Next() {
		_ = wrappedStream.Current()
	}
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
}

func TestModelConnectionTracking_OnOutboundLlmStream_CloseIdempotent(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	// Use proper lifecycle: OnOutboundRawRequest increments
	ctx := context.Background()
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	mockStream := streams.SliceStream([]*llm.Response{{ID: "1"}})

	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, mockStream)
	assert.NoError(t, err)

	// Close once - should decrement
	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// Close again - should NOT decrement again (idempotent)
	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
}

func TestModelConnectionTracking_MultipleModels(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state1 := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	state2 := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "claude-3"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "claude-3",
	}

	outbound1 := &PersistentOutboundTransformer{state: state1}
	outbound2 := &PersistentOutboundTransformer{state: state2}

	middleware1 := &modelConnectionTracking{outbound: outbound1, tracker: tracker}
	middleware2 := &modelConnectionTracking{outbound: outbound2, tracker: tracker}

	ctx := context.Background()

	// Increment for gpt-4
	_, _ = middleware1.OnOutboundRawRequest(ctx, nil)
	_, _ = middleware1.OnOutboundRawRequest(ctx, nil)

	// Increment for claude-3
	_, _ = middleware2.OnOutboundRawRequest(ctx, nil)

	assert.Equal(t, 2, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "claude-3"))

	// Decrement gpt-4
	_, _ = middleware1.OnOutboundLlmResponse(ctx, &llm.Response{ID: "test"})
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "claude-3"))
}

func TestModelConnectionTracking_MultipleChannels(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel1 := createTestChannelForModelTracking(1, "channel-1")
	channel2 := createTestChannelForModelTracking(2, "channel-2")

	state1 := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel1,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	state2 := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel2,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}

	outbound1 := &PersistentOutboundTransformer{state: state1}
	outbound2 := &PersistentOutboundTransformer{state: state2}

	middleware1 := &modelConnectionTracking{outbound: outbound1, tracker: tracker}
	middleware2 := &modelConnectionTracking{outbound: outbound2, tracker: tracker}

	ctx := context.Background()

	// Increment for channel 1
	_, _ = middleware1.OnOutboundRawRequest(ctx, nil)

	// Increment for channel 2
	_, _ = middleware2.OnOutboundRawRequest(ctx, nil)
	_, _ = middleware2.OnOutboundRawRequest(ctx, nil)

	assert.Equal(t, 1, tracker.GetModelConnectionCount(1, "gpt-4"))
	assert.Equal(t, 2, tracker.GetModelConnectionCount(2, "gpt-4"))
}

func TestModelConnectionTracking_Lifecycle(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// 1. Request starts - increment
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// 2. Response completes - decrement
	_, err = middleware.OnOutboundLlmResponse(ctx, &llm.Response{ID: "test"})
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// 3. Another request starts - increment
	_, err = middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// 4. Error occurs - decrement
	middleware.OnOutboundRawError(ctx, assert.AnError)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
}

func TestModelConnectionTracking_StreamLifecycle(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "gpt-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "gpt-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// 1. Request starts - increment
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	// 2. Stream starts
	events := []*llm.Response{{ID: "1"}, {ID: "2"}}
	mockStream := streams.SliceStream(events)
	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, mockStream)
	assert.NoError(t, err)

	for wrappedStream.Next() {
		_ = wrappedStream.Current()
	}
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))

	err = wrappedStream.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
}

func TestModelConnectionTracking_CaseInsensitiveModel(t *testing.T) {
	tracker := NewModelConnectionTracker()

	channel := createTestChannelForModelTracking(1, "test-channel")

	state := &PersistenceState{
		CurrentCandidate: &ChannelModelsCandidate{
			Channel: channel,
			Models:  []biz.ChannelModelEntry{{ActualModel: "GPT-4"}},
		},
		CurrentModelIndex: 0,
		OriginalModel:     "GPT-4",
	}
	outbound := &PersistentOutboundTransformer{state: state}

	middleware := &modelConnectionTracking{
		outbound: outbound,
		tracker:  tracker,
	}

	ctx := context.Background()

	// Increment with uppercase model name
	_, err := middleware.OnOutboundRawRequest(ctx, nil)
	assert.NoError(t, err)

	// The tracker normalizes to lowercase, so check lowercase
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "gpt-4"))
	assert.Equal(t, 1, tracker.GetModelConnectionCount(channel.ID, "GPT-4"))
}

func TestNoopModelConnectionTracking(t *testing.T) {
	middleware := &noopModelConnectionTracking{}

	ctx := context.Background()

	// Should return response unchanged
	resp := &llm.Response{ID: "test"}
	result, err := middleware.OnOutboundLlmResponse(ctx, resp)
	assert.NoError(t, err)
	assert.Equal(t, resp, result)

	// Should return stream unchanged
	stream := streams.SliceStream([]*llm.Response{{ID: "1"}})
	wrappedStream, err := middleware.OnOutboundLlmStream(ctx, stream)
	assert.NoError(t, err)
	assert.Equal(t, stream, wrappedStream)
}

func TestWithModelConnectionTracking_NilTracker(t *testing.T) {
	outbound := &PersistentOutboundTransformer{}
	middleware := withModelConnectionTracking(outbound, nil)

	// Should return noop middleware
	assert.Equal(t, "track-model-connections-noop", middleware.Name())
}

func TestWithModelConnectionTracking_NonNilTracker(t *testing.T) {
	tracker := NewModelConnectionTracker()
	outbound := &PersistentOutboundTransformer{}
	middleware := withModelConnectionTracking(outbound, tracker)

	// Should return real middleware
	assert.Equal(t, "track-model-connections", middleware.Name())
}
