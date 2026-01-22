package gemini

import (
	"context"
	"encoding/json"

	"github.com/samber/lo"

	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/streams"
)

// defaultPendingResponseID is used when a chunk has content but no responseID.
// This provides a consistent ID across all chunks until a real responseID arrives.
const defaultPendingResponseID = "pending-response"

// streamState tracks state across streaming events.
type streamState struct {
	toolCallIndex int
	responseID    string // Track responseID from first valid chunk
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

	// Update state.responseID if chunk has a real responseID (prefer real over temporary)
	if resp.ResponseID != "" {
		state.responseID = resp.ResponseID
	}

	// Skip chunks with no meaningful content.
	// A chunk is meaningful if it has candidates OR usage metadata.
	// Intermediate chunks during thinking mode may have neither and can be safely skipped.
	if len(resp.Candidates) == 0 && resp.UsageMetadata == nil {
		return nil, nil
	}

	// Assign responseID if missing: use tracked ID or generate temporary
	if resp.ResponseID == "" {
		if state.responseID == "" {
			state.responseID = defaultPendingResponseID
		}
		resp.ResponseID = state.responseID
	}

	// Convert to unified response format (streaming) with tool call index tracking
	llmResp, nextIndex := convertGeminiToLLMResponseWithState(&resp, true, state.toolCallIndex)
	state.toolCallIndex = nextIndex

	return llmResp, nil
}

// AggregateStreamChunks aggregates Gemini streaming response chunks into a complete response.
func (t *OutboundTransformer) AggregateStreamChunks(
	ctx context.Context,
	chunks []*httpclient.StreamEvent,
) ([]byte, llm.ResponseMeta, error) {
	return AggregateStreamChunks(ctx, chunks)
}
