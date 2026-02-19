package anthropic

import (
	"encoding/json"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/streams"
)

// buildChunk is a helper that builds an llm.Response stream chunk.
func buildChunk(id, model string, opts ...func(*llm.Response)) *llm.Response {
	resp := &llm.Response{
		ID:    id,
		Model: model,
		Choices: []llm.Choice{
			{Index: 0},
		},
	}
	for _, opt := range opts {
		opt(resp)
	}

	return resp
}

func withUsage(prompt, completion int64) func(*llm.Response) {
	return func(r *llm.Response) {
		r.Usage = &llm.Usage{
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      prompt + completion,
		}
	}
}

func withReasoningContent(content string) func(*llm.Response) {
	return func(r *llm.Response) {
		r.Choices[0].Delta = &llm.Message{
			Role:             "assistant",
			ReasoningContent: lo.ToPtr(content),
		}
	}
}

func withReasoningSignature(sig string) func(*llm.Response) {
	return func(r *llm.Response) {
		if r.Choices[0].Delta == nil {
			r.Choices[0].Delta = &llm.Message{Role: "assistant"}
		}
		r.Choices[0].Delta.ReasoningSignature = lo.ToPtr(sig)
	}
}

func withTextContent(text string) func(*llm.Response) {
	return func(r *llm.Response) {
		r.Choices[0].Delta = &llm.Message{
			Role:    "assistant",
			Content: llm.MessageContent{Content: lo.ToPtr(text)},
		}
	}
}

func withFinishReason(reason string) func(*llm.Response) {
	return func(r *llm.Response) {
		r.Choices[0].FinishReason = lo.ToPtr(reason)
	}
}

// collectStreamEvents transforms an llm.Response slice through the inbound stream
// and returns the parsed Anthropic StreamEvent list.
func collectStreamEvents(t *testing.T, responses []*llm.Response) []StreamEvent {
	t.Helper()

	transformer := NewInboundTransformer()
	mockStream := streams.SliceStream(responses)

	transformedStream, err := transformer.TransformStream(t.Context(), mockStream)
	require.NoError(t, err)

	var events []StreamEvent

	for transformedStream.Next() {
		raw := transformedStream.Current()

		var ev StreamEvent

		err := json.Unmarshal(raw.Data, &ev)
		require.NoError(t, err)

		events = append(events, ev)
	}

	require.NoError(t, transformedStream.Err())

	return events
}

// TestPendingSignature_SignatureBeforeThinking verifies that when a signature
// arrives before any thinking content (like the Responses API encrypted_content),
// it is deferred and emitted as a signature_delta after the thinking content,
// just before the content_block_stop.
func TestPendingSignature_SignatureBeforeThinking(t *testing.T) {
	const (
		id    = "msg_test_001"
		model = "test-model"
		sig   = "encrypted_sig_data"
	)

	responses := []*llm.Response{
		// 1. Initial chunk with usage
		buildChunk(id, model, withUsage(10, 1)),
		// 2. Signature arrives BEFORE any thinking content
		buildChunk(id, model, withReasoningSignature(sig)),
		// 3. Thinking content arrives
		buildChunk(id, model, withReasoningContent("Hello")),
		buildChunk(id, model, withReasoningContent(" world")),
		// 4. Text content arrives (triggers thinking block close)
		buildChunk(id, model, withTextContent("Result")),
		// 5. Finish
		buildChunk(id, model, withFinishReason("stop")),
		// 6. Usage
		buildChunk(id, model, withUsage(10, 20)),
	}

	events := collectStreamEvents(t, responses)

	// Expected event order:
	// 0: message_start
	// 1: content_block_start (thinking)
	// 2: content_block_delta (thinking_delta "Hello")
	// 3: content_block_delta (thinking_delta " world")
	// 4: content_block_delta (signature_delta) <-- deferred signature emitted here
	// 5: content_block_stop (index 0)
	// 6: content_block_start (text)
	// 7: content_block_delta (text_delta "Result")
	// 8: content_block_stop (index 1)
	// 9: message_delta
	// 10: message_stop

	require.Len(t, events, 11)

	// Verify message_start
	require.Equal(t, "message_start", events[0].Type)

	// Verify thinking block start
	require.Equal(t, "content_block_start", events[1].Type)
	require.Equal(t, "thinking", events[1].ContentBlock.Type)

	// Verify thinking deltas
	require.Equal(t, "content_block_delta", events[2].Type)
	require.Equal(t, "thinking_delta", *events[2].Delta.Type)
	require.Equal(t, "Hello", *events[2].Delta.Thinking)

	require.Equal(t, "content_block_delta", events[3].Type)
	require.Equal(t, "thinking_delta", *events[3].Delta.Type)
	require.Equal(t, " world", *events[3].Delta.Thinking)

	// Verify deferred signature_delta comes AFTER thinking content
	require.Equal(t, "content_block_delta", events[4].Type)
	require.Equal(t, "signature_delta", *events[4].Delta.Type)
	require.Equal(t, sig, *events[4].Delta.Signature)

	// Verify thinking block stop
	require.Equal(t, "content_block_stop", events[5].Type)
	require.Equal(t, int64(0), *events[5].Index)

	// Verify text content
	require.Equal(t, "content_block_start", events[6].Type)
	require.Equal(t, "text", events[6].ContentBlock.Type)

	require.Equal(t, "content_block_delta", events[7].Type)
	require.Equal(t, "text_delta", *events[7].Delta.Type)
	require.Equal(t, "Result", *events[7].Delta.Text)

	require.Equal(t, "content_block_stop", events[8].Type)
	require.Equal(t, "message_delta", events[9].Type)
	require.Equal(t, "message_stop", events[10].Type)
}

// TestPendingSignature_SignatureAfterThinking verifies the normal case:
// when signature arrives after thinking has started, it is emitted immediately
// (no buffering needed).
func TestPendingSignature_SignatureAfterThinking(t *testing.T) {
	const (
		id    = "msg_test_002"
		model = "test-model"
		sig   = "normal_sig"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Thinking content first
		buildChunk(id, model, withReasoningContent("Think")),
		// Signature arrives AFTER thinking started
		buildChunk(id, model, withReasoningSignature(sig)),
		// Text content
		buildChunk(id, model, withTextContent("Answer")),
		buildChunk(id, model, withFinishReason("stop")),
		buildChunk(id, model, withUsage(10, 15)),
	}

	events := collectStreamEvents(t, responses)

	// Expected event order:
	// 0: message_start
	// 1: content_block_start (thinking)
	// 2: content_block_delta (thinking_delta "Think")
	// 3: content_block_delta (signature_delta) <-- emitted immediately
	// 4: content_block_stop (index 0)
	// 5: content_block_start (text)
	// 6: content_block_delta (text_delta "Answer")
	// 7: content_block_stop (index 1)
	// 8: message_delta
	// 9: message_stop

	require.Len(t, events, 10)

	// Signature is emitted right after thinking, before the block stop
	require.Equal(t, "content_block_delta", events[3].Type)
	require.Equal(t, "signature_delta", *events[3].Delta.Type)
	require.Equal(t, sig, *events[3].Delta.Signature)

	// Then thinking block stop
	require.Equal(t, "content_block_stop", events[4].Type)
}

// TestPendingSignature_SignatureBeforeThinking_FinishWithoutText verifies
// that the pending signature is flushed at finish_reason when there's no
// text content transition (thinking-only response).
func TestPendingSignature_SignatureBeforeThinking_FinishWithoutText(t *testing.T) {
	const (
		id    = "msg_test_003"
		model = "test-model"
		sig   = "encrypted_sig_finish"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Signature before thinking
		buildChunk(id, model, withReasoningSignature(sig)),
		// Thinking content
		buildChunk(id, model, withReasoningContent("Reasoning")),
		// Finish directly (no text content)
		buildChunk(id, model, withFinishReason("stop")),
		buildChunk(id, model, withUsage(10, 10)),
	}

	events := collectStreamEvents(t, responses)

	// Expected event order:
	// 0: message_start
	// 1: content_block_start (thinking)
	// 2: content_block_delta (thinking_delta "Reasoning")
	// 3: content_block_delta (signature_delta) <-- flushed at finish
	// 4: content_block_stop (index 0)
	// 5: message_delta
	// 6: message_stop

	require.Len(t, events, 7)

	// Verify signature is flushed before the stop
	require.Equal(t, "content_block_delta", events[3].Type)
	require.Equal(t, "signature_delta", *events[3].Delta.Type)
	require.Equal(t, sig, *events[3].Delta.Signature)

	require.Equal(t, "content_block_stop", events[4].Type)
}

// TestPendingSignature_NoSignature verifies that the normal flow without
// any signature works correctly (no regression).
func TestPendingSignature_NoSignature(t *testing.T) {
	const (
		id    = "msg_test_004"
		model = "test-model"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		buildChunk(id, model, withReasoningContent("Think")),
		buildChunk(id, model, withTextContent("Answer")),
		buildChunk(id, model, withFinishReason("stop")),
		buildChunk(id, model, withUsage(10, 5)),
	}

	events := collectStreamEvents(t, responses)

	// No signature_delta should appear
	for _, ev := range events {
		if ev.Delta != nil && ev.Delta.Type != nil {
			require.NotEqual(t, "signature_delta", *ev.Delta.Type, "signature_delta should not appear without signature")
		}
	}
}
