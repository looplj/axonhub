package gemini

import (
	"context"
	"encoding/json"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
	"github.com/looplj/axonhub/llm/transformer"
)

// streamState tracks state across streaming events.
type streamState struct {
	toolCallIndex int
	hasToolCall   bool
}

// TransformStream transforms the HTTP stream response to the unified response format.
// Gemini's stream is a stream of GenerateContentResponse.
func (t *OutboundTransformer) TransformStream(
	ctx context.Context,
	stream streams.Stream[*httpclient.StreamEvent],
) (streams.Stream[*llm.Response], error) {
	stream = streams.AppendStream(stream, lo.ToPtr(llm.DoneStreamEvent))

	// Track tool call index across stream events
	state := &streamState{}

	return streams.MapErr(stream, func(event *httpclient.StreamEvent) (*llm.Response, error) {
		return t.transformStreamChunkWithState(event, state)
	}), nil
}

// TransformStreamChunk transforms a single Gemini streaming chunk to unified Response.
// Note: This method does not track tool call index across events. Use TransformStream for proper streaming.
func (t *OutboundTransformer) TransformStreamChunk(
	ctx context.Context,
	event *httpclient.StreamEvent,
) (*llm.Response, error) {
	return t.transformStreamChunkWithState(event, &streamState{})
}

// transformStreamChunkWithState transforms a single Gemini streaming chunk with state tracking.
func (t *OutboundTransformer) transformStreamChunkWithState(
	event *httpclient.StreamEvent,
	state *streamState,
) (*llm.Response, error) {
	if event == nil || len(event.Data) == 0 {
		return nil, nil
	}

	// Handle [DONE] marker - Gemini doesn't use this, but handle it for consistency
	if string(event.Data) == "[DONE]" {
		return llm.DoneResponse, nil
	}

	// Parse the Gemini response chunk
	var resp GenerateContentResponse
	if err := json.Unmarshal(event.Data, &resp); err != nil {
		return nil, err
	}

	// Check if the response is valid.
	// Gemini response empty event for some time, we should return error instead of continue to process.
	if resp.ResponseID == "" {
		return nil, transformer.ErrInvalidResponse
	}

	var finishReason string
	if len(resp.Candidates) > 0 {
		finishReason = resp.Candidates[0].FinishReason
	}

	// WORKAROUND: Check if response has empty/nil content when STOP is received
	// This can happen when Gemini hits context limits or gets confused in long conversations
	hasEmptyContent := false
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		hasOnlyEmptyText := true
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.FunctionCall != nil || part.FunctionResponse != nil || part.InlineData != nil {
				hasOnlyEmptyText = false
				break
			}
			if part.Text != "" {
				hasOnlyEmptyText = false
				break
			}
		}
		if hasOnlyEmptyText {
			hasEmptyContent = true
		}
	}

	// WORKAROUND: If STOP with empty content but we saw tool calls earlier, force hasToolCall
	// This helps when Gemini returns STOP prematurely in long tool-using conversations
	if finishReason == "STOP" && hasEmptyContent && state.hasToolCall {
		// Force accumulatedHasToolCall to be used
		resp.Candidates[0].FinishReason = "STOP" // Ensure it's set for conversion
	}

	// WORKAROUND: Check if request has unresponded tool calls and we're getting STOP
	// This handles the case where Gemini returns STOP prematurely across separate requests
	if finishReason == "STOP" && !state.hasToolCall {
		if t.hasUnrespondedToolCalls() {
			state.hasToolCall = true
		}
	}

	// Convert to unified response format (streaming) with tool call index tracking
	// Pass accumulated hasToolCall state and update it if this chunk contains tool calls
	llmResp, nextIndex, chunkHasToolCall := convertGeminiToLLMResponseWithState(&resp, true, state.toolCallIndex, state.hasToolCall)
	state.toolCallIndex = nextIndex
	if chunkHasToolCall {
		state.hasToolCall = true
	}

	return llmResp, nil
}

// AggregateStreamChunks aggregates Gemini streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
