package responses

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// countingSource serves a fixed item slice following the streams package
// contract (Next advances) and counts how many items the transformer pulled.
// The pull count is what distinguishes hold-one-beat emission from the old
// buffer-until-signature behaviour.
type countingSource struct {
	items []*llm.Response
	pulls int
}

func (s *countingSource) Next() bool {
	if s.pulls < len(s.items) {
		s.pulls++
		return true
	}
	return false
}

func (s *countingSource) Current() *llm.Response { return s.items[s.pulls-1] }
func (s *countingSource) Err() error             { return nil }
func (s *countingSource) Close() error           { return nil }

func drainEvents(t *testing.T, stream streams.Stream[*httpclient.StreamEvent]) []StreamEvent {
	t.Helper()
	var events []StreamEvent
	for stream.Next() {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &ev))
		events = append(events, ev)
	}
	require.NoError(t, stream.Err())
	return events
}

func reasoningDeltaChunk(id, text string) *llm.Response {
	return &llm.Response{
		Object: "chat.completion.chunk",
		Model:  "gpt-5",
		TransformerMetadata: map[string]any{
			responsesReasoningItemTransformerMetadataKey: map[string]any{"id": id},
		},
		Choices: []llm.Choice{{Delta: &llm.Message{ReasoningContent: lo.ToPtr(text)}}},
	}
}

func reasoningSignatureChunk(id, blob string) *llm.Response {
	return &llm.Response{
		Object: "chat.completion.chunk",
		Model:  "gpt-5",
		TransformerMetadata: map[string]any{
			responsesReasoningItemTransformerMetadataKey: map[string]any{"id": id, "done": true},
		},
		Choices: []llm.Choice{{Delta: &llm.Message{ID: id, ReasoningSignature: lo.ToPtr(blob)}}},
	}
}

func reasoningDeltas(events []StreamEvent) []string {
	var deltas []string
	for _, event := range events {
		if event.Type == StreamEventTypeReasoningSummaryTextDelta {
			deltas = append(deltas, event.Delta)
		}
	}
	return deltas
}

func reasoningDoneItems(events []StreamEvent) []Item {
	var items []Item
	for _, event := range events {
		if event.Type == StreamEventTypeOutputItemDone && event.Item != nil && event.Item.Type == "reasoning" {
			items = append(items, *event.Item)
		}
	}
	return items
}

// A single long reasoning item must stream its deltas without waiting for the
// signature: the first delta is emitted once the second delta confirms item
// ownership, i.e. after exactly two source pulls. The old buffer-until-
// signature behaviour needs four pulls (d1 d2 d3 signature) before anything
// is emitted, so this test pins the new emission timing.
func TestInboundTransformer_TransformStream_SingleItemReasoningStreamsImmediately(t *testing.T) {
	trans := NewInboundTransformer()
	src := &countingSource{items: []*llm.Response{
		reasoningDeltaChunk("rs_1", "d1"),
		reasoningDeltaChunk("rs_1", "d2"),
		reasoningDeltaChunk("rs_1", "d3"),
		reasoningSignatureChunk("rs_1", "gAAAA_BLOB"),
		{Object: "chat.completion.chunk", Choices: []llm.Choice{{Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}},
		{Object: "chat.completion.chunk", Usage: &llm.Usage{PromptTokens: 1, CompletionTokens: 1}},
	}}
	stream, err := trans.TransformStream(t.Context(), src)
	require.NoError(t, err)

	// Consume events until the first reasoning content event appears. It must
	// materialize after exactly two source pulls (d2 confirms d1); the old
	// buffer-until-signature behaviour would need pull 4 (the signature).
	var firstContent *StreamEvent
	for stream.Next() {
		var ev StreamEvent
		require.NoError(t, json.Unmarshal(stream.Current().Data, &ev))
		if ev.Type == StreamEventTypeReasoningSummaryPartAdded || ev.Type == StreamEventTypeReasoningSummaryTextDelta {
			firstContent = &ev
			break
		}
	}
	require.NotNil(t, firstContent, "reasoning content must be emitted before the signature")
	require.Equal(t, 2, src.pulls,
		"first reasoning content must not wait for the signature (2 pulls, not 4)")

	// Drain the rest and verify full sequence and pairing.
	events := drainEvents(t, stream)
	require.Equal(t, []string{"d1", "d2", "d3"}, reasoningDeltas(events))

	doneItems := reasoningDoneItems(events)
	require.Len(t, doneItems, 1)
	require.Equal(t, "d1d2d3", doneItems[0].Summary[0].Text)
	require.Equal(t, "gAAAA_BLOB", lo.FromPtr(doneItems[0].EncryptedContent))

	require.NotEmpty(t, events)
	require.Equal(t, StreamEventTypeResponseCompleted, events[len(events)-1].Type)
}

// Interleaved items must keep each item's content paired with its own
// signature even when one item's signature arrives after the next item's
// content started - the invariant PR #2054 protects.
func TestInboundTransformer_TransformStream_InterleavedItemsKeepSignaturePairing(t *testing.T) {
	trans := NewInboundTransformer()
	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		reasoningDeltaChunk("rs_A", "a1"),
		reasoningDeltaChunk("rs_A", "a2"),
		reasoningDeltaChunk("rs_B", "b1"),
		reasoningSignatureChunk("rs_A", "gAAAA_A"),
		reasoningDeltaChunk("rs_B", "b2"),
		reasoningSignatureChunk("rs_B", "gAAAA_B"),
		{Object: "chat.completion.chunk", Choices: []llm.Choice{{Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}},
		{Object: "chat.completion.chunk", Usage: &llm.Usage{}},
	}))
	require.NoError(t, err)

	events := drainEvents(t, stream)

	require.Equal(t, []string{"a1", "a2", "b1", "b2"}, reasoningDeltas(events))

	doneItems := reasoningDoneItems(events)
	require.Len(t, doneItems, 2)
	require.Equal(t, "rs_A", doneItems[0].ID)
	require.Equal(t, "a1a2", doneItems[0].Summary[0].Text)
	require.Equal(t, "gAAAA_A", lo.FromPtr(doneItems[0].EncryptedContent))
	require.Equal(t, "rs_B", doneItems[1].ID)
	require.Equal(t, "b1b2", doneItems[1].Summary[0].Text)
	require.Equal(t, "gAAAA_B", lo.FromPtr(doneItems[1].EncryptedContent))
}

// A semantic boundary (text content) must flush the held beat without
// dropping or reordering it: every reasoning delta appears before the text
// that triggered the flush, in original order.
func TestInboundTransformer_TransformStream_BoundaryFlushesHeldDeltaInOrder(t *testing.T) {
	trans := NewInboundTransformer()
	stream, err := trans.TransformStream(t.Context(), streams.SliceStream([]*llm.Response{
		reasoningDeltaChunk("rs_1", "d1"),
		reasoningDeltaChunk("rs_1", "d2"),
		{Object: "chat.completion.chunk", Choices: []llm.Choice{{
			Delta: &llm.Message{Content: llm.MessageContent{Content: lo.ToPtr("answer")}},
		}}},
		{Object: "chat.completion.chunk", Choices: []llm.Choice{{Delta: &llm.Message{}, FinishReason: lo.ToPtr("stop")}}},
		{Object: "chat.completion.chunk", Usage: &llm.Usage{}},
	}))
	require.NoError(t, err)

	events := drainEvents(t, stream)

	require.Equal(t, []string{"d1", "d2"}, reasoningDeltas(events),
		"held tail must flush at the boundary, not be dropped")

	lastReasoningIdx, textIdx := -1, -1
	for i, event := range events {
		switch event.Type {
		case StreamEventTypeReasoningSummaryTextDelta:
			lastReasoningIdx = i
		case StreamEventTypeOutputTextDelta:
			textIdx = i
		}
	}
	require.NotEqual(t, -1, textIdx)
	require.NotEqual(t, -1, lastReasoningIdx)
	require.Less(t, lastReasoningIdx, textIdx, "reasoning must precede the text that triggered the flush")

	doneItems := reasoningDoneItems(events)
	require.NotEmpty(t, doneItems)
	require.Equal(t, "d1d2", doneItems[0].Summary[0].Text)
}
