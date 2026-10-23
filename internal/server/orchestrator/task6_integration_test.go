package orchestrator

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer/anthropic"
	"github.com/looplj/axonhub/llm/transformer/openai"
	"github.com/looplj/axonhub/llm/transformer/openai/responses"
)

func TestStreamDeliveryIntegration_UnacknowledgedTerminalDoesNotCompleteRequest(t *testing.T) {
	// Given: the converted stream exposes a provider terminal, but the client
	// writer has not acknowledged its semantic terminal boundary.
	state := &PersistenceState{}
	stream := NewInboundPersistentStream(
		t.Context(),
		streams.SliceStream([]*httpclient.StreamEvent{{
			Type: "response.completed",
			Data: []byte(`{"type":"response.completed","response":{"status":"completed"}}`),
		}}),
		nil, nil, nil, openai.NewInboundTransformer(), nil, state,
	)

	// When
	require.True(t, stream.Next())
	stream.Current()
	require.NoError(t, stream.Close())

	// Then: upstream evidence alone leaves the Request unsuccessful.
	require.Equal(t, streamTerminalIncomplete, stream.terminalState)
	require.False(t, state.StreamCompleted)
	require.Equal(t, request.StatusFailed, stream.terminalState.requestStatus())
}

func TestStreamDeliveryIntegration_AcknowledgedTerminalSurvivesLateCancel(t *testing.T) {
	// Given: a semantic terminal has been delivered and acknowledged before the
	// downstream context is canceled.
	downstream, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner, execution := newStreamLifecycle(downstream)
	state := &PersistenceState{}
	stream := NewInboundPersistentStream(
		execution,
		streams.SliceStream([]*httpclient.StreamEvent{{
			Type: "response.completed",
			Data: []byte(`{"type":"response.completed","response":{"status":"completed"}}`),
		}}),
		nil, nil, nil, openai.NewInboundTransformer(), nil, state,
	)

	// When
	require.True(t, stream.Next())
	stream.Current()
	stream.AcknowledgeDelivery()
	cancel()
	require.NoError(t, stream.Close())

	// Then: late cancellation does not demote acknowledged Request delivery.
	require.True(t, owner.DeliveryAcknowledged())
	require.Equal(t, streamTerminalCompleted, stream.terminalState)
	require.True(t, state.StreamCompleted)
}

func TestStreamDeliveryIntegration_ProviderIncompleteOverridesSyntheticSuccess(t *testing.T) {
	// Given: conversion exposes a generic success after the raw provider marked
	// the execution incomplete.
	state := &PersistenceState{OutboundStreamTerminal: streamTerminalIncomplete}
	stream := NewInboundPersistentStream(
		t.Context(),
		streams.SliceStream([]*httpclient.StreamEvent{{
			Type: "response.completed",
			Data: []byte(`{"type":"response.completed","response":{"status":"completed"}}`),
		}}),
		nil, nil, nil, openai.NewInboundTransformer(), nil, state,
	)

	// When
	require.True(t, stream.Next())
	stream.Current()
	require.NoError(t, stream.Close())

	// Then: provider outcome remains unsuccessful despite synthetic conversion.
	require.Equal(t, streamTerminalIncomplete, stream.terminalState)
	require.False(t, state.StreamCompleted)
}

func TestStreamDeliveryIntegration_PassThroughPreservesDeliveryGate(t *testing.T) {
	// Given: a raw pass-through branch wraps the retained source under the same
	// execution lifecycle as the converted branch.
	downstream, cancel := context.WithCancel(t.Context())
	defer cancel()
	owner, execution := newStreamLifecycle(downstream)
	raw := &passThroughChannelStream{
		ctx: execution,
		ch:  closedEventChannel(&httpclient.StreamEvent{Data: []byte("raw terminal")}),
	}
	wrapped := withPassThroughDeliveryGate(raw, owner)
	delivery, ok := wrapped.(streams.DeliveryAcknowledger)
	require.True(t, ok)

	// When
	delivery.AcknowledgeDelivery()

	// Then: raw pass-through acknowledges the shared lifecycle, not a second gate.
	require.True(t, delivery.DeliveryAcknowledged())
	require.True(t, owner.DeliveryAcknowledged())
}

func closedEventChannel(event *httpclient.StreamEvent) <-chan *httpclient.StreamEvent {
	ch := make(chan *httpclient.StreamEvent, 1)
	ch <- event
	close(ch)
	return ch
}

func TestStreamDeliveryIntegration_ConvertedResponsesAndAnthropicOrdering(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		transform func(context.Context, streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error)
		terminal  string
	}{
		{
			name: "responses completed",
			transform: func(ctx context.Context, source streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
				return responses.NewInboundTransformer().TransformStream(ctx, source)
			},
			terminal: "response.completed",
		},
		{
			name: "anthropic message stop",
			transform: func(ctx context.Context, source streams.Stream[*llm.Response]) (streams.Stream[*httpclient.StreamEvent], error) {
				return anthropic.NewInboundTransformer().TransformStream(ctx, source)
			},
			terminal: "message_stop",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given: a converter queues a semantic terminal before delivery ack.
			downstream, cancel := context.WithCancel(t.Context())
			defer cancel()
			_, execution := newStreamLifecycle(downstream)
			finishReason := "stop"
			converted, err := scenario.transform(execution, streams.SliceStream([]*llm.Response{{
				ID:    "converted-response",
				Model: "test-model",
				Choices: []llm.Choice{{
					Index:        0,
					Delta:        &llm.Message{},
					FinishReason: &finishReason,
				}},
				Usage: &llm.Usage{PromptTokens: 12, CompletionTokens: 34, TotalTokens: 46},
			}}))
			require.NoError(t, err)
			state := &PersistenceState{}
			stream := NewInboundPersistentStream(execution, converted, nil, nil, nil, openai.NewInboundTransformer(), nil, state)

			// When
			var events []*httpclient.StreamEvent
			for stream.Next() {
				event := stream.Current()
				events = append(events, event)
				if event != nil && event.Type == scenario.terminal {
					stream.AcknowledgeDelivery()
				}
			}
			require.NoError(t, stream.Err())
			require.NoError(t, stream.Close())

			// Then: the protocol terminal is present without requiring [DONE].
			terminalSeen := false
			for _, event := range events {
				if event != nil && event.Type == scenario.terminal {
					terminalSeen = true
				}
				if event != nil {
					require.NotEqual(t, []byte("[DONE]"), event.Data)
				}
			}
			require.True(t, terminalSeen)
			require.True(t, state.StreamCompleted)
		})
	}
}

func TestStreamDeliveryIntegration_UsagePlaceholderAndResetStayIncomplete(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		stream streams.Stream[*httpclient.StreamEvent]
	}{
		{
			name: "finish placeholder without choices-less usage",
			stream: streams.SliceStream([]*httpclient.StreamEvent{{Data: []byte(
				`{"choices":[{"index":0,"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}}`,
			)}}),
		},
		{
			name:   "reset before semantic terminal",
			stream: &sliceEventStream{err: io.ErrUnexpectedEOF},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given: provider data is incomplete or reset before a semantic terminal.
			state := &PersistenceState{}
			stream := NewInboundPersistentStream(t.Context(), scenario.stream, nil, nil, nil, openai.NewInboundTransformer(), nil, state)

			// When
			for stream.Next() {
				stream.Current()
			}
			require.NoError(t, stream.Close())

			// Then: neither usage placeholders nor reset transport errors promote delivery.
			require.False(t, state.StreamCompleted)
			require.NotEqual(t, streamTerminalCompleted, stream.terminalState)
		})
	}
}
