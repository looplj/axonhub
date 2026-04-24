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

func withToolCall(index int, id, name, args string) func(*llm.Response) {
	return func(r *llm.Response) {
		if r.Choices[0].Delta == nil {
			r.Choices[0].Delta = &llm.Message{Role: "assistant"}
		}
		r.Choices[0].Delta.ToolCalls = append(r.Choices[0].Delta.ToolCalls, llm.ToolCall{
			Index: index,
			ID:    id,
			Type:  "function",
			Function: llm.FunctionCall{
				Name:      name,
				Arguments: args,
			},
		})
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

// TestPendingSignature_NoSignature verifies that when no signature is provided,
// a placeholder signature_delta is still emitted to ensure the thinking block
// always has a signature (required by Anthropic schema).
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

	// Exactly one signature_delta should appear (placeholder generated by closeThinkingBlock).
	var sigCount int

	for _, ev := range events {
		if ev.Delta != nil && ev.Delta.Type != nil && *ev.Delta.Type == "signature_delta" {
			sigCount++

			require.NotNil(t, ev.Delta.Signature)
			require.NotEmpty(t, *ev.Delta.Signature, "placeholder signature should not be empty")
		}
	}

	require.Equal(t, 1, sigCount, "exactly one placeholder signature_delta should be emitted")
}

// TestPendingSignature_SignatureWithoutThinking_AfterText verifies that when
// a text block is already open and a signature arrives without any thinking
// content, the text block is properly closed before the synthetic thinking
// block is created.
func TestPendingSignature_SignatureWithoutThinking_AfterText(t *testing.T) {
	const (
		id    = "msg_test_010"
		model = "test-model"
		sig   = "orphan_sig_after_text"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Text content first
		buildChunk(id, model, withTextContent("Hello")),
		// Signature without any thinking content
		buildChunk(id, model, withReasoningSignature(sig)),
		// Finish
		buildChunk(id, model, withFinishReason("stop")),
		buildChunk(id, model, withUsage(10, 10)),
	}

	events := collectStreamEvents(t, responses)

	// Expected event order:
	// 0: message_start
	// 1: content_block_start (text, index 0)
	// 2: content_block_delta (text_delta "Hello")
	// 3: content_block_stop (index 0) <-- close the text block
	// 4: content_block_start (thinking, index 1) <-- synthetic thinking block
	// 5: content_block_delta (signature_delta)
	// 6: content_block_stop (index 1)
	// 7: message_delta
	// 8: message_stop

	require.Len(t, events, 9)

	// Text block
	require.Equal(t, "content_block_start", events[1].Type)
	require.Equal(t, "text", events[1].ContentBlock.Type)
	require.Equal(t, int64(0), *events[1].Index)

	require.Equal(t, "content_block_delta", events[2].Type)
	require.Equal(t, "text_delta", *events[2].Delta.Type)
	require.Equal(t, "Hello", *events[2].Delta.Text)

	// Text block stop
	require.Equal(t, "content_block_stop", events[3].Type)
	require.Equal(t, int64(0), *events[3].Index)

	// Synthetic thinking block
	require.Equal(t, "content_block_start", events[4].Type)
	require.Equal(t, "thinking", events[4].ContentBlock.Type)
	require.Equal(t, int64(1), *events[4].Index)

	// Signature on the thinking block
	require.Equal(t, "content_block_delta", events[5].Type)
	require.Equal(t, "signature_delta", *events[5].Delta.Type)
	require.Equal(t, sig, *events[5].Delta.Signature)
	require.Equal(t, int64(1), *events[5].Index)

	require.Equal(t, "content_block_stop", events[6].Type)
	require.Equal(t, int64(1), *events[6].Index)

	require.Equal(t, "message_delta", events[7].Type)
	require.Equal(t, "message_stop", events[8].Type)
}

// TestPendingSignature_SignatureWithoutThinking_AfterToolUse verifies that when
// a tool block is already open and a signature arrives without any thinking
// content, the tool block is properly closed before the synthetic thinking
// block is created.
func TestPendingSignature_SignatureWithoutThinking_AfterToolUse(t *testing.T) {
	const (
		id    = "msg_test_011"
		model = "test-model"
		sig   = "orphan_sig_after_tool"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Tool call first
		buildChunk(id, model, withToolCall(0, "toolu_01", "Bash", `{"command":"ls"}`)),
		// Signature without any thinking content
		buildChunk(id, model, withReasoningSignature(sig)),
		// Finish
		buildChunk(id, model, withFinishReason("tool_calls")),
		buildChunk(id, model, withUsage(10, 20)),
	}

	events := collectStreamEvents(t, responses)

	// Expected event order:
	// 0: message_start
	// 1: content_block_start (tool_use, index 0)
	// 2: content_block_delta (input_json_delta)
	// 3: content_block_stop (index 0) <-- close the tool block
	// 4: content_block_start (thinking, index 1) <-- synthetic thinking block
	// 5: content_block_delta (signature_delta)
	// 6: content_block_stop (index 1)
	// 7: message_delta
	// 8: message_stop

	require.Len(t, events, 9)

	// Tool block
	require.Equal(t, "content_block_start", events[1].Type)
	require.Equal(t, "tool_use", events[1].ContentBlock.Type)
	require.Equal(t, int64(0), *events[1].Index)

	require.Equal(t, "content_block_delta", events[2].Type)
	require.Equal(t, "input_json_delta", *events[2].Delta.Type)

	// Tool block stop
	require.Equal(t, "content_block_stop", events[3].Type)
	require.Equal(t, int64(0), *events[3].Index)

	// Synthetic thinking block
	require.Equal(t, "content_block_start", events[4].Type)
	require.Equal(t, "thinking", events[4].ContentBlock.Type)
	require.Equal(t, int64(1), *events[4].Index)

	// Signature on the thinking block
	require.Equal(t, "content_block_delta", events[5].Type)
	require.Equal(t, "signature_delta", *events[5].Delta.Type)
	require.Equal(t, sig, *events[5].Delta.Signature)
	require.Equal(t, int64(1), *events[5].Index)

	require.Equal(t, "content_block_stop", events[6].Type)
	require.Equal(t, int64(1), *events[6].Index)

	require.Equal(t, "message_delta", events[7].Type)
	require.Equal(t, "message_stop", events[8].Type)
}

// TestPendingSignature_SignatureWithoutThinking_ToolUse tests the defensive path:
// signature arrives with no ReasoningContent at all, then tool_use directly.
// The flushPendingSignatureBlock() creates a synthetic empty thinking block.
func TestPendingSignature_SignatureWithoutThinking_ToolUse(t *testing.T) {
	const (
		id    = "msg_test_006"
		model = "test-model"
		sig   = "orphan_sig_tool"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Signature without any thinking content
		buildChunk(id, model, withReasoningSignature(sig)),
		// Tool use directly
		buildChunk(id, model, withToolCall(0, "toolu_01", "Bash", `{"command":"ls"}`)),
		buildChunk(id, model, withFinishReason("tool_calls")),
		buildChunk(id, model, withUsage(10, 20)),
	}

	events := collectStreamEvents(t, responses)

	// Expected: synthetic thinking block created for the signature
	// 0: message_start
	// 1: content_block_start (thinking)
	// 2: content_block_delta (signature_delta)
	// 3: content_block_stop (index 0)
	// 4: content_block_start (tool_use, index 1)
	// 5: content_block_delta (input_json_delta)
	// 6: content_block_stop (index 1)
	// 7: message_delta
	// 8: message_stop

	require.Len(t, events, 9)

	// Synthetic thinking block
	require.Equal(t, "content_block_start", events[1].Type)
	require.Equal(t, "thinking", events[1].ContentBlock.Type)
	require.Equal(t, int64(0), *events[1].Index)

	// Signature on the thinking block
	require.Equal(t, "content_block_delta", events[2].Type)
	require.Equal(t, "signature_delta", *events[2].Delta.Type)
	require.Equal(t, sig, *events[2].Delta.Signature)
	require.Equal(t, int64(0), *events[2].Index)

	require.Equal(t, "content_block_stop", events[3].Type)
	require.Equal(t, int64(0), *events[3].Index)

	// Tool use at index 1
	require.Equal(t, "content_block_start", events[4].Type)
	require.Equal(t, "tool_use", events[4].ContentBlock.Type)
	require.Equal(t, int64(1), *events[4].Index)
}

// TestPendingSignature_SignatureWithoutThinking_Text tests the defensive path:
// signature arrives with no ReasoningContent, then text directly.
func TestPendingSignature_SignatureWithoutThinking_Text(t *testing.T) {
	const (
		id    = "msg_test_007"
		model = "test-model"
		sig   = "orphan_sig_text"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Signature without any thinking content
		buildChunk(id, model, withReasoningSignature(sig)),
		// Text directly
		buildChunk(id, model, withTextContent("Hello")),
		buildChunk(id, model, withFinishReason("stop")),
		buildChunk(id, model, withUsage(10, 10)),
	}

	events := collectStreamEvents(t, responses)

	// Expected: synthetic thinking block, then text
	// 0: message_start
	// 1: content_block_start (thinking)
	// 2: content_block_delta (signature_delta)
	// 3: content_block_stop (index 0)
	// 4: content_block_start (text, index 1)
	// 5: content_block_delta (text_delta "Hello")
	// 6: content_block_stop (index 1)
	// 7: message_delta
	// 8: message_stop

	require.Len(t, events, 9)

	// Synthetic thinking block
	require.Equal(t, "content_block_start", events[1].Type)
	require.Equal(t, "thinking", events[1].ContentBlock.Type)
	require.Equal(t, int64(0), *events[1].Index)

	// Signature on the thinking block
	require.Equal(t, "content_block_delta", events[2].Type)
	require.Equal(t, "signature_delta", *events[2].Delta.Type)
	require.Equal(t, sig, *events[2].Delta.Signature)
	require.Equal(t, int64(0), *events[2].Index)

	require.Equal(t, "content_block_stop", events[3].Type)

	// Text at index 1
	require.Equal(t, "content_block_start", events[4].Type)
	require.Equal(t, "text", events[4].ContentBlock.Type)
	require.Equal(t, int64(1), *events[4].Index)
}

// TestPendingSignature_SignatureWithoutThinking_FinishOnly tests the defensive path:
// signature arrives with no ReasoningContent, then finish directly (no text/tool).
// The flushPendingSignatureBlock() at finish_reason creates a synthetic thinking block.
func TestPendingSignature_SignatureWithoutThinking_FinishOnly(t *testing.T) {
	const (
		id    = "msg_test_008"
		model = "test-model"
		sig   = "orphan_sig_finish"
	)

	responses := []*llm.Response{
		buildChunk(id, model, withUsage(10, 1)),
		// Signature without any thinking content
		buildChunk(id, model, withReasoningSignature(sig)),
		// Finish directly
		buildChunk(id, model, withFinishReason("stop")),
		buildChunk(id, model, withUsage(10, 5)),
	}

	events := collectStreamEvents(t, responses)

	// Expected: synthetic thinking block created at finish
	// 0: message_start
	// 1: content_block_start (thinking)
	// 2: content_block_delta (signature_delta)
	// 3: content_block_stop (index 0)
	// 4: content_block_stop (index 0) — from finish_reason (deduplicated)
	// But due to dedup logic, the second content_block_stop is skipped
	// So: 4: message_delta, 5: message_stop

	require.Len(t, events, 6)

	// Synthetic thinking block
	require.Equal(t, "content_block_start", events[1].Type)
	require.Equal(t, "thinking", events[1].ContentBlock.Type)
	require.Equal(t, int64(0), *events[1].Index)

	// Signature on the thinking block
	require.Equal(t, "content_block_delta", events[2].Type)
	require.Equal(t, "signature_delta", *events[2].Delta.Type)
	require.Equal(t, sig, *events[2].Delta.Signature)
	require.Equal(t, int64(0), *events[2].Index)

	require.Equal(t, "content_block_stop", events[3].Type)

	require.Equal(t, "message_delta", events[4].Type)
	require.Equal(t, "message_stop", events[5].Type)
}
